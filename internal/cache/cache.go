// Package cache sets CDN cache headers on public responses and purges the
// CDN after a publish.
package cache

import (
	"fmt"
	"net/http"
	"time"
)

// Public marks a response cacheable by the CDN and browsers:
// `Cache-Control: public, max-age=N, stale-while-revalidate=M`. Wrap only
// anonymous, cookie-free GET pages. It never overrides a header the handler
// already set, and skips non-GET/HEAD requests.
func Public(maxAge, swr time.Duration) func(http.Handler) http.Handler {
	value := fmt.Sprintf("public, max-age=%d, stale-while-revalidate=%d",
		int(maxAge.Seconds()), int(swr.Seconds()))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				w.Header().Set("Cache-Control", value)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// NoStore forbids any caching, for admin and authenticated pages.
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
