package main

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"awt/internal/auth"
	"awt/internal/blob"
	"awt/internal/cache"
	"awt/internal/config"
	"awt/internal/service"
	"awt/internal/store"
	"awt/internal/web"
)

// TestServeModulesWiring checks the composition serve actually uses: the
// public JSON API is registered (and public), and every /admin route except
// the login endpoints is guarded (CLAUDE.md web rules).
func TestServeModulesWiring(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "awt.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	blobs, err := blob.NewFS(filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Env: "dev", Addr: ":0"}
	bus := service.NewBus()
	deps := web.Deps{Cfg: cfg, Auth: auth.New(db, cfg), DB: db, Purger: cache.NoopPurger{}, Blobs: blobs,
		Service: service.New(db, blobs, bus, nil), Bus: bus}

	table := web.RouteTable(deps, modules()...)
	want := map[string]bool{
		"GET /api/v1/competitions":        false,
		"GET /api/v1/competitions/{code}": false,
		"GET /competitions":               false,
		"GET /admin/competitions/{code}":  false,
	}
	admin := 0
	for _, rt := range table {
		if _, ok := want[rt.Pattern]; ok {
			want[rt.Pattern] = true
		}
		if strings.HasPrefix(rt.Pattern, "GET /api/v1/") && rt.Guard != web.GuardPublic {
			t.Errorf("%s guard = %s, want public", rt.Pattern, rt.Guard)
		}
		p := rt.Path()
		if p != "/admin" && !strings.HasPrefix(p, "/admin/") {
			continue
		}
		admin++
		if p == "/admin/login" {
			continue
		}
		if rt.Guard == web.GuardPublic {
			t.Errorf("%s is under /admin but public", rt.Pattern)
		}
	}
	for p, found := range want {
		if !found {
			t.Errorf("route %s not registered by serve's modules", p)
		}
	}
	if admin < 20 {
		t.Errorf("only %d admin routes: admin module missing?", admin)
	}
}

type recPurger struct {
	mu   sync.Mutex
	urls []string
}

func (p *recPurger) Purge(_ context.Context, urls []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.urls = append(p.urls, urls...)
	return nil
}

// TestPurgerOutlivesShutdown: a request still running when the stop signal
// arrives completes during the graceful shutdown, and the event it
// publishes is still purged (the purger stops after Shutdown returns).
func TestPurgerOutlivesShutdown(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "awt.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	bus := service.NewBus()
	fp := &recPurger{}
	started, release := make(chan struct{}), make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		bus.Publish(service.Event{Topic: "season:s1", Kind: service.KindCompetition})
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: mux}
	ctx, signal := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		errc <- runServer(ctx, srv, ln, func(pctx context.Context) <-chan struct{} {
			return web.StartPurger(pctx, bus, fp, db, "https://x.test")
		})
	}()
	go func() { _, _ = http.Get("http://" + ln.Addr().String() + "/slow") }()
	<-started
	signal()
	time.Sleep(100 * time.Millisecond) // the shutdown has begun
	close(release)
	if err := <-errc; err != nil {
		t.Fatalf("runServer: %v", err)
	}
	fp.mu.Lock()
	defer fp.mu.Unlock()
	if !slices.Contains(fp.urls, "https://x.test/seasons/s1") {
		t.Errorf("event of the in-flight request not purged: %v", fp.urls)
	}
}
