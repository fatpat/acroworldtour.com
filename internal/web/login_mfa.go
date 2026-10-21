package web

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"awt/internal/auth"
	"awt/internal/web/layout"
)

// Second login step (spec 2026-10-19-manager-mfa). No session exists yet, so
// there is no CSRF token: the pending cookie is SameSite=Strict and every
// POST checks the Origin header.

const (
	msgInvalidCode  = "Invalid code"
	msgLoginExpired = "Your login expired, please sign in again"
	msgTOTPUnusable = "Authenticator codes can't be checked for your account right now. Use a security key or a recovery code."
	msgStepFailed   = "Something went wrong, please try again."
)

// checkOrigin refuses a request whose Origin header names another origin.
// Browsers send Origin on every cross-site POST; its absence (older clients,
// same-origin navigations in some browsers) is let through, the SameSite=Strict
// pending cookie covering that case.
func (d Deps) checkOrigin(next http.Handler) http.Handler {
	allowed := d.Cfg.Origins()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" {
			match := false
			for _, a := range allowed {
				if strings.EqualFold(o, a) {
					match = true
				}
			}
			if !match {
				http.Error(w, "forbidden: cross-origin request", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func mfaStep(p auth.PendingLogin, errMsg string) layout.MFAStep {
	s := layout.MFAStep{TOTPAvailable: p.TOTPAvailable, Error: errMsg}
	for _, m := range p.Methods {
		switch m {
		case auth.MethodTOTP:
			s.TOTP = true
		case auth.MethodWebAuthn:
			s.WebAuthn = true
		case auth.MethodRecovery:
			s.Recovery = true
		}
	}
	return s
}

func (d Deps) loginExpired(w http.ResponseWriter, r *http.Request) {
	d.Auth.ClearPendingCookie(w)
	render(w, r, http.StatusUnauthorized, layout.LoginPage("", msgLoginExpired))
}

func (d Deps) mfaPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if _, err := d.Auth.UserFromRequest(r); err == nil {
		http.Redirect(w, r, "/manager", http.StatusSeeOther)
		return
	}
	p, err := d.Auth.PendingFromRequest(r)
	if errors.Is(err, auth.ErrPendingExpired) {
		d.loginExpired(w, r)
		return
	}
	if err != nil {
		slog.Error("mfa page", "err", err)
		d.serverError(w, r)
		return
	}
	render(w, r, http.StatusOK, layout.MFAPage(mfaStep(p, "")))
}

// mfaSubmit handles one factor POST. complete runs the factor and returns the
// session and, for a recovery code, the codes left (negative otherwise).
func (d Deps) mfaSubmit(complete func(r *http.Request, code string) (auth.Session, int, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if err := r.ParseForm(); err != nil {
			if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		p, err := d.Auth.PendingFromRequest(r)
		if errors.Is(err, auth.ErrPendingExpired) {
			d.loginExpired(w, r)
			return
		}
		if err != nil {
			slog.Error("mfa step", "err", err)
			d.serverError(w, r)
			return
		}
		s, recoveryLeft, err := complete(r, r.PostFormValue("code"))
		var locked *auth.ErrMFALocked
		switch {
		case err == nil:
			d.Auth.ClearPendingCookie(w)
			d.Auth.SetCookie(w, s)
			if recoveryLeft >= 0 {
				render(w, r, http.StatusOK, layout.RecoveryUsedPage(recoveryLeft, p.Next))
				return
			}
			http.Redirect(w, r, p.Next, http.StatusSeeOther)
		case errors.Is(err, auth.ErrPendingExpired):
			d.loginExpired(w, r)
		case errors.As(err, &locked):
			left := time.Until(locked.Until)
			w.Header().Set("Retry-After", strconv.Itoa(int(left.Seconds()+1)))
			minutes := int((left + time.Minute - 1) / time.Minute)
			render(w, r, http.StatusTooManyRequests, layout.MFAPage(mfaStep(p, fmt.Sprintf("Too many attempts, try again in %d minutes", max(minutes, 1)))))
		case errors.Is(err, auth.ErrMFAInvalid):
			render(w, r, http.StatusUnauthorized, layout.MFAPage(mfaStep(p, msgInvalidCode)))
		case errors.Is(err, auth.ErrTOTPUnavailable):
			p.TOTPAvailable = false
			render(w, r, http.StatusConflict, layout.MFAPage(mfaStep(p, msgTOTPUnusable)))
		default:
			slog.Error("mfa step", "err", err)
			render(w, r, http.StatusInternalServerError, layout.MFAPage(mfaStep(p, msgStepFailed)))
		}
	})
}

func (d Deps) mfaTOTP() http.Handler {
	return d.mfaSubmit(func(r *http.Request, code string) (auth.Session, int, error) {
		s, err := d.Auth.CompleteWithTOTP(r.Context(), r, code)
		return s, -1, err
	})
}

func (d Deps) mfaRecovery() http.Handler {
	return d.mfaSubmit(func(r *http.Request, code string) (auth.Session, int, error) {
		return d.Auth.CompleteWithRecovery(r.Context(), r, code)
	})
}
