package auth

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
)

// CSRF rejects non-GET/HEAD/OPTIONS requests whose `_csrf` form field or
// X-CSRF-Token header does not match the session token. Run it after
// RequireUser.
func (a *Auth) CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		s, ok := SessionFromContext(r.Context())
		if !ok {
			var err error
			if s, err = a.UserFromRequest(r); err != nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		got := r.Header.Get("X-CSRF-Token")
		if got == "" {
			// Parse here to tell an oversized body (MaxBytesReader) from a
			// missing token.
			var err error
			if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				err = r.ParseMultipartForm(32 << 20)
			} else {
				err = r.ParseForm()
			}
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			got = r.PostFormValue("_csrf")
		}
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.CSRF)) != 1 {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
