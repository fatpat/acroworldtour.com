package cache_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"awt/internal/cache"
)

func TestPublicSetsCacheControl(t *testing.T) {
	h := cache.Public(60*time.Second, 300*time.Second)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got, want := rec.Header().Get("Cache-Control"), "public, max-age=60, stale-while-revalidate=300"; got != want {
		t.Fatalf("Cache-Control = %q, want %q", got, want)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("POST got Cache-Control %q", got)
	}
}

func TestCloudflarePurger(t *testing.T) {
	var gotPath, gotAuth string
	var batches [][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		var body struct {
			Files []string `json:"files"`
		}
		if err := json.Unmarshal(b, &body); err != nil {
			t.Errorf("bad json: %v", err)
		}
		batches = append(batches, body.Files)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	p := cache.NewCloudflarePurger("zone1", "tok")
	p.Endpoint = srv.URL
	var urls []string
	for i := range 31 {
		urls = append(urls, "https://x.test/"+strconv.Itoa(i))
	}
	if err := p.Purge(context.Background(), urls); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/zones/zone1/purge_cache" || gotAuth != "Bearer tok" {
		t.Fatalf("path %q auth %q", gotPath, gotAuth)
	}
	if len(batches) != 2 || len(batches[0]) != 30 || len(batches[1]) != 1 {
		t.Fatalf("batches = %d", len(batches))
	}
}

func TestCloudflarePurgerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"success":false}`))
	}))
	defer srv.Close()
	p := cache.NewCloudflarePurger("z", "t")
	p.Endpoint = srv.URL
	err := p.Purge(context.Background(), []string{"https://x.test/"})
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v", err)
	}
}

func TestNoopPurger(t *testing.T) {
	if err := (cache.NoopPurger{}).Purge(context.Background(), []string{"a"}); err != nil {
		t.Fatal(err)
	}
}
