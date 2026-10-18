package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"awt/internal/config"
	"awt/internal/web"
)

func TestHealthz(t *testing.T) {
	srv := web.NewServer(web.Deps{Cfg: config.Config{Addr: ":0"}})
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	res := rec.Result()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}
}
