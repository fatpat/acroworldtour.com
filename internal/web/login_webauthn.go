package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"awt/internal/auth"
)

// Security key login step (spec 2026-10-19-manager-mfa): JSON endpoints
// called by web/static/js/webauthn.js. Same protection as the other second
// step routes: SameSite=Strict pending cookie, Origin check, body cap.

const (
	msgKeyRejected = "This security key was not accepted"
	msgNoKey       = "No security key is registered for your account"
)

// keyAnswer writes the JSON answer webauthn.js reads: {"redirect": ...} on
// success, {"error": ...} otherwise.
func keyAnswer(w http.ResponseWriter, status int, field, value string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{field: value})
}

// keyError maps a ceremony error to its JSON answer.
func (d Deps) keyError(w http.ResponseWriter, err error) {
	var locked *auth.ErrMFALocked
	switch {
	case errors.Is(err, auth.ErrPendingExpired):
		d.Auth.ClearPendingCookie(w)
		keyAnswer(w, http.StatusUnauthorized, "error", msgLoginExpired)
	case errors.As(err, &locked):
		left := time.Until(locked.Until)
		w.Header().Set("Retry-After", strconv.Itoa(int(left.Seconds()+1)))
		minutes := int((left + time.Minute - 1) / time.Minute)
		keyAnswer(w, http.StatusTooManyRequests, "error", fmt.Sprintf("Too many attempts, try again in %d minutes", max(minutes, 1)))
	case errors.Is(err, auth.ErrMFAInvalid):
		keyAnswer(w, http.StatusUnauthorized, "error", msgKeyRejected)
	case errors.Is(err, auth.ErrNoSecurityKey):
		keyAnswer(w, http.StatusConflict, "error", msgNoKey)
	default:
		slog.Error("security key step", "err", err)
		keyAnswer(w, http.StatusInternalServerError, "error", msgStepFailed)
	}
}

// mfaKeyBegin returns the options for navigator.credentials.get.
func (d Deps) mfaKeyBegin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	opts, err := d.Auth.BeginWebAuthnLogin(r.Context(), r)
	if err != nil {
		d.keyError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(opts)
}

// mfaKeyFinish checks the assertion and opens the session.
func (d Deps) mfaKeyFinish(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	p, err := d.Auth.PendingFromRequest(r)
	if err != nil {
		d.keyError(w, err)
		return
	}
	s, err := d.Auth.FinishWebAuthnLogin(r.Context(), r, body)
	if err != nil {
		d.keyError(w, err)
		return
	}
	d.Auth.ClearPendingCookie(w)
	d.Auth.SetCookie(w, s)
	keyAnswer(w, http.StatusOK, "redirect", p.Next)
}
