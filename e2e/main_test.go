// Package e2e drives the real awt binary with a real browser (playwright-go).
//
// The suite only runs with AWT_E2E=1 (make e2e); a plain `go test ./...`
// skips it so that other targets never launch a browser. TestMain builds the
// binary, migrates a temporary database, imports the mini fixture, creates an
// admin user and starts `awt serve` on a free port.
package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

const (
	adminEmail    = "admin@e2e.test"
	adminPassword = "e2e-password-123"
	catalogYear   = 2025
)

// env is the shared fixture of the whole suite.
type env struct {
	baseURL string
	pw      *playwright.Playwright
	browser playwright.Browser
	logs    *syncBuffer
}

var e *env

// syncBuffer is a goroutine-safe log sink for the server's output.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestMain(m *testing.M) {
	if os.Getenv("AWT_E2E") != "1" {
		fmt.Println("e2e: AWT_E2E is not 1, skipping (run `make e2e`)")
		os.Exit(0)
	}
	code, err := run(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e setup:", err)
		code = 1
	}
	os.Exit(code)
}

func run(m *testing.M) (int, error) {
	root, err := filepath.Abs("..")
	if err != nil {
		return 0, err
	}
	dir, err := os.MkdirTemp("", "awt-e2e-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)

	bin := filepath.Join(dir, "awt")
	build := exec.Command("go", "build", "-o", bin, "./cmd/awt")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		return 0, fmt.Errorf("go build: %v\n%s", err, out)
	}

	db, blobs := filepath.Join(dir, "awt.db"), filepath.Join(dir, "blobs")
	// The server sees only the settings below: an AWT_* variable of the
	// developer's shell (S3 backend, base URL, env) must not leak into it.
	var base []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "AWT_") {
			base = append(base, kv)
		}
	}
	base = append(base, "AWT_ENV=dev", "AWT_DB_PATH="+db, "AWT_BLOB_DIR="+blobs)
	cli := func(stdin string, args ...string) error {
		cmd := exec.Command(bin, args...)
		cmd.Env = base
		cmd.Stdin = strings.NewReader(stdin)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("awt %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return nil
	}
	if err := cli("", "migrate"); err != nil {
		return 0, err
	}
	mini := filepath.Join(root, "internal", "importer", "testdata", "mini")
	if err := cli("", "import", "--from", mini, "--db", db, "--blob-dir", blobs); err != nil {
		return 0, err
	}
	if err := cli(adminPassword+"\n", "user", "add", "--email", adminEmail, "--name", "E2E Admin", "--role", "admin"); err != nil {
		return 0, err
	}

	port, err := freePort()
	if err != nil {
		return 0, err
	}
	logs := &syncBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := exec.CommandContext(ctx, bin, "serve")
	srv.Env = append(base, "AWT_ADDR=127.0.0.1:"+port)
	srv.Stdout, srv.Stderr = logs, logs
	if err := srv.Start(); err != nil {
		return 0, err
	}
	exited := make(chan struct{})
	go func() { _ = srv.Wait(); close(exited) }()
	defer func() {
		cancel()
		<-exited
	}()
	baseURL := "http://127.0.0.1:" + port
	if err := waitHealthy(baseURL, exited); err != nil {
		return 0, fmt.Errorf("%v\nserver logs:\n%s", err, logs)
	}

	pw, err := playwright.Run()
	if err != nil {
		return 0, fmt.Errorf("start playwright (run `go run github.com/playwright-community/playwright-go/cmd/playwright install chromium`): %w", err)
	}
	defer pw.Stop()
	browser, err := pw.Chromium.Launch()
	if err != nil {
		return 0, fmt.Errorf("launch chromium: %w", err)
	}
	defer browser.Close()

	e = &env{baseURL: baseURL, pw: pw, browser: browser, logs: logs}
	if err := seed(); err != nil {
		return 0, fmt.Errorf("seed: %w\nserver logs:\n%s", err, logs)
	}
	code := m.Run()
	if code != 0 {
		fmt.Fprintf(os.Stderr, "--- awt serve logs ---\n%s\n", logs)
	}
	return code, nil
}

func freePort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	return port, err
}

func waitHealthy(base string, exited <-chan struct{}) error {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-exited:
			return fmt.Errorf("awt serve exited before /healthz answered")
		default:
		}
		resp, err := http.Get(base + "/healthz")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for /healthz")
}

// seed creates what the suite needs on top of the mini fixture: two judges (it has
// a single live one) and a second team.
func seed() error {
	p, done, err := newAdminPage()
	if err != nil {
		return err
	}
	defer done()
	for _, j := range []struct{ name, level, country string }{
		{"E2E Senior", "senior", "fra"},
		{"E2E Certified", "certified", "che"},
	} {
		if _, err := p.Goto(e.baseURL + "/admin/judges/new"); err != nil {
			return err
		}
		if err := p.GetByLabel("Name").Fill(j.name); err != nil {
			return err
		}
		if _, err := p.GetByLabel("Level").SelectOption(playwright.SelectOptionValues{Values: &[]string{j.level}}); err != nil {
			return err
		}
		if err := p.GetByLabel("Country").Fill(j.country); err != nil {
			return err
		}
		if err := p.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Save"}).Click(); err != nil {
			return err
		}
		if err := p.GetByRole("cell", playwright.PageGetByRoleOptions{Name: j.name, Exact: playwright.Bool(true)}).WaitFor(); err != nil {
			return fmt.Errorf("judge %s: %w", j.name, err)
		}
	}
	// A synchro competition needs 2 teams; the fixture has one.
	if _, err := p.Goto(e.baseURL + "/admin/teams/new"); err != nil {
		return err
	}
	if err := p.Locator("#name").Fill("E2E Duo"); err != nil {
		return err
	}
	if err := p.Locator("#pilot1").Fill("Alice Aerial (1001)"); err != nil {
		return err
	}
	if err := p.Locator("#pilot2").Fill("Bob Barrel (1002)"); err != nil {
		return err
	}
	if err := p.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Save"}).Click(); err != nil {
		return err
	}
	if err := p.GetByRole("cell", playwright.PageGetByRoleOptions{Name: "E2E Duo"}).First().WaitFor(); err != nil {
		return fmt.Errorf("team E2E Duo: %w", err)
	}
	return nil
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// slug turns a test name into a competition code fragment.
func slug(s string) string {
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(s), "-"), "-")
}
