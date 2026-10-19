package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rsc.io/qr"

	"awt/internal/auth"
	"awt/internal/domain"
	"awt/internal/web"
)

// The account page (spec 2026-10-19-manager-mfa): any logged-in user
// manages their own second factors and password. No route names a user:
// every action applies to the session's account.

// Sections of the account page; a failed form re-renders with its errors in
// its own section.
const (
	secTOTP     = "totp"
	secCodes    = "codes"
	secPassword = "password"
)

type accountView struct {
	User   auth.User
	Status auth.Status
	Flash  string
	// Section names the form Errors belong to: one of the sec* constants or
	// "key:<id>" for a security key's removal.
	Section string
	Errors  formErrors
	// Enroll is set while an authenticator app is being set up.
	Enroll *totpEnroll
	// Codes are fresh recovery codes, shown once.
	Codes []string
}

type totpEnroll struct{ Secret, Pending string }

func (v accountView) errorsFor(section string) formErrors {
	if v.Section == section {
		return v.Errors
	}
	return formErrors{}
}

func (h *handlers) registerAccount(r *web.Router) {
	user := func(pattern string, fn http.HandlerFunc) { r.User(pattern, fn) }
	userPost := func(pattern string, fn http.HandlerFunc) { r.UserMax("POST "+pattern, formMax, fn) }
	user("GET /manager/account", h.account)
	user("GET /manager/account/totp/qr.png", h.accountQR)
	userPost("/manager/account/totp/start", h.accountTOTPStart)
	userPost("/manager/account/totp/confirm", h.accountTOTPConfirm)
	userPost("/manager/account/totp/remove", h.accountTOTPRemove)
	userPost("/manager/account/keys/begin", h.accountKeyBegin)
	userPost("/manager/account/keys/finish", h.accountKeyFinish)
	userPost("/manager/account/keys/{id}/remove", h.accountKeyRemove)
	userPost("/manager/account/recovery/regenerate", h.accountRegenerate)
	userPost("/manager/account/password", h.accountPassword)
}

func session(r *http.Request) auth.Session {
	s, _ := auth.SessionFromContext(r.Context())
	return s
}

// event logs an account security event (never secrets or codes).
func (h *handlers) event(r *http.Request, msg, event string, attrs ...any) {
	id := session(r).User.ID
	slog.Info(msg, append([]any{"event", event, "user_id", id, "actor", id, "ip", h.auth.ClientIP(r)}, attrs...)...)
}

func (h *handlers) accountPage(w http.ResponseWriter, r *http.Request, status int, v accountView) {
	s := session(r)
	st, err := h.auth.MFAStatus(r.Context(), s.User.ID)
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	v.User, v.Status = s.User, st
	if v.Flash == "" {
		v.Flash = flash(r)
	}
	render(w, r, status, accountPage(v))
}

func (h *handlers) account(w http.ResponseWriter, r *http.Request) {
	h.accountPage(w, r, http.StatusOK, accountView{})
}

// accountFail maps an account action error onto its section of the page.
func (h *handlers) accountFail(w http.ResponseWriter, r *http.Request, err error, v accountView) {
	var (
		in      *auth.InputError
		limited *auth.ErrReauthLimited
	)
	status := http.StatusUnprocessableEntity
	switch {
	case errors.As(err, &limited):
		status, v.Errors = http.StatusTooManyRequests, messageError(limitedMessage(w, limited))
	case errors.Is(err, auth.ErrPasswordChanged):
		status, v.Errors = http.StatusConflict, messageError(msgPasswordChanged)
	case errors.Is(err, auth.ErrWrongPassword):
		v.Errors = validationErrors([]domain.FieldError{{Field: "current_password", Msg: "Current password is wrong"}})
	case errors.Is(err, auth.ErrMFAInvalid):
		v.Errors = validationErrors([]domain.FieldError{{Field: "code", Msg: "The code does not match; check the time on your phone and try the next code"}})
	case errors.As(err, &in):
		v.Errors = validationErrors([]domain.FieldError{{Field: in.Field, Msg: in.Msg}})
	case errors.Is(err, auth.ErrEnrollmentExpired):
		v.Enroll, v.Errors = nil, messageError("The setup expired. Start again.")
	case errors.Is(err, auth.ErrTOTPEnrolled):
		status, v.Enroll, v.Errors = http.StatusConflict, nil, messageError("An authenticator app is already set up. Remove it first to replace it.")
	case errors.Is(err, auth.ErrNoTOTP):
		status, v.Errors = http.StatusConflict, messageError("No authenticator app is set up.")
	case errors.Is(err, auth.ErrNoFactor):
		status, v.Errors = http.StatusConflict, messageError("Add an authenticator app or a security key first.")
	case errors.Is(err, auth.ErrNoMFAKey):
		status, v.Enroll, v.Errors = http.StatusServiceUnavailable, nil, messageError("Authenticator apps are not available: the server has no MFA key configured")
	case errors.Is(err, auth.ErrKeyNotFound):
		h.notFound(w, r)
		return
	default:
		h.fail(w, r, err, nil)
		return
	}
	h.accountPage(w, r, status, v)
}

func (h *handlers) accountTOTPStart(w http.ResponseWriter, r *http.Request) {
	secret, _, pending, err := h.auth.StartTOTPEnrollment(r.Context(), session(r).User.ID)
	if errors.Is(err, auth.ErrNoMFAKey) {
		h.accountFail(w, r, err, accountView{Section: secTOTP})
		return
	}
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	h.accountPage(w, r, http.StatusOK, accountView{Section: secTOTP, Enroll: &totpEnroll{Secret: secret, Pending: pending}})
}

// accountQR serves the setup's otpauth URI as a QR code. The pending value
// must be the caller's own and live.
func (h *handlers) accountQR(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	_, uri, err := h.auth.TOTPEnrollment(r.Context(), session(r).User.ID, r.URL.Query().Get("p"))
	if errors.Is(err, auth.ErrEnrollmentExpired) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	code, err := qr.Encode(uri, qr.M)
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	code.Scale = 6
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(code.PNG())
}

func (h *handlers) accountTOTPConfirm(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	pending := r.PostFormValue("pending")
	codes, err := h.auth.ConfirmTOTP(r.Context(), s.User.ID, pending, r.PostFormValue("code"), r.PostFormValue("current_password"), s.Token)
	if err != nil {
		v := accountView{Section: secTOTP}
		if secret, _, oerr := h.auth.TOTPEnrollment(r.Context(), s.User.ID, pending); oerr == nil {
			v.Enroll = &totpEnroll{Secret: secret, Pending: pending}
		}
		h.accountFail(w, r, err, v)
		return
	}
	h.event(r, "authenticator app added", "mfa.enroll.totp")
	if codes == nil {
		redirect(w, r, "/manager/account", "totp_added")
		return
	}
	h.accountPage(w, r, http.StatusOK, accountView{Flash: flashMessages["totp_added"], Codes: codes})
}

func (h *handlers) accountTOTPRemove(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	if err := h.auth.RemoveTOTP(r.Context(), s.User.ID, r.PostFormValue("current_password"), s.Token); err != nil {
		h.accountFail(w, r, err, accountView{Section: secTOTP})
		return
	}
	h.event(r, "authenticator app removed", "mfa.remove", "method", auth.MethodTOTP)
	redirect(w, r, "/manager/account", "totp_removed")
}

const msgPasswordChanged = "Your password changed, please try again"

// limitedMessage sets Retry-After and returns the message of a refused
// password re-check (5 wrong current passwords in 15 minutes).
func limitedMessage(w http.ResponseWriter, e *auth.ErrReauthLimited) string {
	secs := int((e.RetryAfter + time.Second - 1) / time.Second)
	w.Header().Set("Retry-After", strconv.Itoa(max(secs, 1)))
	return fmt.Sprintf("Too many wrong passwords, try again in %d minutes", max((secs+59)/60, 1))
}

// keyJSON writes the JSON answer webauthn.js reads.
func keyJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func keyErr(w http.ResponseWriter, status int, msg string) {
	keyJSON(w, status, map[string]string{"error": msg})
}

// accountKeyBegin returns the creation options for navigator.credentials
// .create, plus the challenge token the finish request sends back.
func (h *handlers) accountKeyBegin(w http.ResponseWriter, r *http.Request) {
	opts, token, err := h.auth.BeginWebAuthnRegistration(r.Context(), session(r).User.ID)
	if err != nil {
		slog.Error("security key registration", "err", err)
		keyErr(w, http.StatusInternalServerError, "Something went wrong, please try again.")
		return
	}
	var answer map[string]json.RawMessage
	if err := json.Unmarshal(opts, &answer); err != nil {
		slog.Error("security key registration options", "err", err)
		keyErr(w, http.StatusInternalServerError, "Something went wrong, please try again.")
		return
	}
	answer["token"], _ = json.Marshal(token)
	keyJSON(w, http.StatusOK, answer)
}

// keyFinish is what webauthn.js posts after navigator.credentials.create.
type keyFinish struct {
	Name       string          `json:"name"`
	Password   string          `json:"password"`
	Token      string          `json:"token"`
	Credential json.RawMessage `json:"credential"`
}

func (h *handlers) accountKeyFinish(w http.ResponseWriter, r *http.Request) {
	var in keyFinish
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			keyErr(w, http.StatusRequestEntityTooLarge, "Request too large")
			return
		}
		keyErr(w, http.StatusBadRequest, "Bad request")
		return
	}
	s := session(r)
	codes, err := h.auth.FinishWebAuthnRegistration(r.Context(), s.User.ID, in.Token, in.Name, in.Password, s.Token, in.Credential)
	var (
		input   *auth.InputError
		limited *auth.ErrReauthLimited
	)
	switch {
	case err == nil:
	case errors.As(err, &limited):
		keyErr(w, http.StatusTooManyRequests, limitedMessage(w, limited))
		return
	case errors.Is(err, auth.ErrPasswordChanged):
		keyErr(w, http.StatusConflict, msgPasswordChanged)
		return
	case errors.Is(err, auth.ErrWrongPassword):
		keyErr(w, http.StatusUnprocessableEntity, "Current password is wrong")
		return
	case errors.As(err, &input):
		keyErr(w, http.StatusUnprocessableEntity, capitalize(input.Msg))
		return
	case errors.Is(err, auth.ErrWebAuthnChallenge):
		keyErr(w, http.StatusConflict, "The security key request expired, please try again")
		return
	case errors.Is(err, auth.ErrWebAuthnRegistered):
		keyErr(w, http.StatusUnprocessableEntity, "This security key is already registered")
		return
	case errors.Is(err, auth.ErrWebAuthnInvalid):
		keyErr(w, http.StatusUnprocessableEntity, "This security key was not accepted")
		return
	default:
		slog.Error("security key registration", "err", err)
		keyErr(w, http.StatusInternalServerError, "Something went wrong, please try again.")
		return
	}
	h.event(r, "security key added", "mfa.enroll.webauthn")
	if codes != nil {
		keyJSON(w, http.StatusOK, map[string][]string{"recoveryCodes": codes})
		return
	}
	keyJSON(w, http.StatusOK, map[string]string{"redirect": "/manager/account?ok=key_added"})
}

func (h *handlers) accountKeyRemove(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	id := pathID(r)
	if err := h.auth.RemoveKey(r.Context(), s.User.ID, id, r.PostFormValue("current_password"), s.Token); err != nil {
		h.accountFail(w, r, err, accountView{Section: "key:" + id})
		return
	}
	h.event(r, "security key removed", "mfa.remove", "method", auth.MethodWebAuthn)
	redirect(w, r, "/manager/account", "key_removed")
}

func (h *handlers) accountRegenerate(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	codes, err := h.auth.RegenerateRecoveryCodes(r.Context(), s.User.ID, r.PostFormValue("current_password"), s.Token)
	if err != nil {
		h.accountFail(w, r, err, accountView{Section: secCodes})
		return
	}
	h.event(r, "recovery codes regenerated", "mfa.recovery_regenerated")
	h.accountPage(w, r, http.StatusOK, accountView{Flash: "New recovery codes created; the old ones no longer work.", Codes: codes})
}

func (h *handlers) accountPassword(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	if err := h.auth.ChangePassword(r.Context(), s.User.ID, r.PostFormValue("current_password"), r.PostFormValue("new_password"), s.Token); err != nil {
		h.accountFail(w, r, err, accountView{Section: secPassword})
		return
	}
	redirect(w, r, "/manager/account", "own_password")
}

// groupSecret shows a base32 secret in groups of 4 for manual entry.
func groupSecret(s string) string {
	var b strings.Builder
	for i, c := range s {
		if i > 0 && i%4 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(c)
	}
	return b.String()
}
