package web

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// statusRecorder remembers the status code and whether a response started.
// It forwards Flush (SSE) and exposes the wrapped writer through Unwrap so
// http.ResponseController keeps working.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status, s.wroteHeader = code, true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.status, s.wroteHeader = http.StatusOK, true
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusRecorder) Flush() {
	if !s.wroteHeader {
		s.status, s.wroteHeader = http.StatusOK, true
	}
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijack not supported")
	}
	return h.Hijack()
}

func (d Deps) clientIP(r *http.Request) string {
	if d.Auth != nil {
		return d.Auth.ClientIP(r)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// logRequests logs one slog line per request. Static files and the health
// check log at debug level to keep the stream readable.
func (d Deps) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		level := slog.LevelInfo
		if r.URL.Path == "/healthz" || strings.HasPrefix(r.URL.Path, "/static/") {
			level = slog.LevelDebug
		}
		slog.Log(r.Context(), level, "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start),
			"ip", d.clientIP(r),
		)
	})
}

// recoverPanic turns a handler panic into a logged 500 page. It runs inside
// logRequests so the 500 is logged.
func (d Deps) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec, ok := w.(*statusRecorder)
		if !ok {
			rec = &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			w = rec
		}
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler { //nolint:errorlint // sentinel compared by identity per net/http
				panic(v)
			}
			slog.Error("panic", "method", r.Method, "path", r.URL.Path, "panic", v, "stack", string(debug.Stack()))
			if !rec.wroteHeader {
				d.serverError(w, r)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// csp is strict: same-origin scripts and styles only (no inline), YouTube
// frames for the home video, images from https (object storage, flags).
const csp = "default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; " +
	"img-src 'self' data: https:; connect-src 'self'; media-src 'self' https:; " +
	"frame-src https://www.youtube-nocookie.com https://www.youtube.com; " +
	"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

func (d Deps) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if d.Cfg.Env == "prod" {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}
