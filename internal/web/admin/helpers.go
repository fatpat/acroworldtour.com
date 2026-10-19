package admin

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/a-h/templ"
	"golang.org/x/text/unicode/norm"

	"awt/internal/domain"
	"awt/internal/service"
	"awt/internal/web"
)

// Body limits: the router applies them before the CSRF check reads the body.
const (
	formMax   = 64 << 10 // forms
	uploadMax = 12 << 20 // file upload and catalog import (service cap is 10 MB)
)

// get registers an admin-only GET route.
func get(r *web.Router, pattern string, fn http.HandlerFunc) { r.Admin("GET "+pattern, fn) }

// post registers an admin-only form POST route.
func post(r *web.Router, pattern string, fn http.HandlerFunc) {
	r.AdminMax("POST "+pattern, formMax, fn)
}

// postUpload registers an admin-only multipart POST route.
func postUpload(r *web.Router, pattern string, fn http.HandlerFunc) {
	r.AdminMax("POST "+pattern, uploadMax, fn)
}

// render writes a component with a status, buffered so a template error
// cannot leave a half-written page.
func render(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	var buf bytes.Buffer
	if err := c.Render(r.Context(), &buf); err != nil {
		slog.Error("render", "path", r.URL.Path, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func isHTMX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

// redirect answers a successful POST: 303 to a page that shows the flash
// message key (one of flashMessages).
func redirect(w http.ResponseWriter, r *http.Request, to, flash string) {
	if flash != "" {
		sep := "?"
		if strings.Contains(to, "?") {
			sep = "&"
		}
		to += sep + "ok=" + flash
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

var flashMessages = map[string]string{
	"created":   "Created.",
	"saved":     "Saved.",
	"deleted":   "Deleted.",
	"imported":  "Imported.",
	"ranks":     "CIVL rankings refreshed.",
	"upload":    "File uploaded.",
	"password":  "Password changed; the user's sessions were closed.",
	"disabled":  "Account disabled.",
	"enabled":   "Account enabled.",
	"mfa_reset": "Second factor removed; the user's sessions were closed.",
	"opened":    "Opened.",
	"closed":    "Closed.",
	"reopened":  "Reopened.",
	// account page
	"totp_added":   "Authenticator app added; your other sessions were closed.",
	"totp_removed": "Authenticator app removed; your other sessions were closed.",
	"key_added":    "Security key added; your other sessions were closed.",
	"key_removed":  "Security key removed; your other sessions were closed.",
	"codes_saved":  "Keep your recovery codes somewhere safe.",
	"own_password": "Password changed; your other sessions were closed.",
}

func flash(r *http.Request) string { return flashMessages[r.URL.Query().Get("ok")] }

// formErrors carries what a re-rendered form shows: a summary line, the
// messages per field and a flat list.
type formErrors struct {
	Summary  string
	Fields   map[string]string
	Messages []string
}

func (e formErrors) Any() bool { return e.Summary != "" }

// Msg returns the message of a field ("" when valid).
func (e formErrors) Msg(field string) string { return e.Fields[field] }

func validationErrors(fields []domain.FieldError) formErrors {
	fe := formErrors{Summary: "Please fix the following:", Fields: map[string]string{}}
	for _, f := range fields {
		if old := fe.Fields[f.Field]; old != "" {
			fe.Fields[f.Field] = old + "; " + f.Msg
		} else {
			fe.Fields[f.Field] = f.Msg
		}
		fe.Messages = append(fe.Messages, f.Field+": "+f.Msg)
	}
	return fe
}

func messageError(msg string) formErrors { return formErrors{Summary: msg} }

// fail maps a service error to the response, in one place. redo re-renders
// the form with the status and the errors; it is nil for actions that have
// no form (a plain error page is shown).
func (h *handlers) fail(w http.ResponseWriter, r *http.Request, err error, redo func(status int, fe formErrors)) {
	var ve *service.ValidationError
	switch {
	case errors.As(err, &ve):
		h.redoOrPage(w, r, http.StatusUnprocessableEntity, validationErrors(ve.Fields), redo)
	case errors.Is(err, service.ErrNotFound):
		h.notFound(w, r)
	case errors.Is(err, service.ErrConflict), errors.Is(err, service.ErrFrozen), errors.Is(err, service.ErrRescore):
		h.redoOrPage(w, r, http.StatusConflict, messageError(capitalize(err.Error())), redo)
	case errors.Is(err, domain.ErrInvalidTransition):
		h.redoOrPage(w, r, http.StatusConflict, messageError(refusalMessage(err)), redo)
	default:
		slog.Error("admin", "method", r.Method, "path", r.URL.Path, "err", err)
		render(w, r, http.StatusInternalServerError, errorPage("Something went wrong", "An unexpected error occurred. Please try again."))
	}
}

// refusalMessage is the explanation of a refused state change without the
// "invalid state transition" wrapper text.
func refusalMessage(err error) string {
	msg := strings.ReplaceAll(err.Error(), domain.ErrInvalidTransition.Error()+": ", "")
	msg = strings.ReplaceAll(msg, ": "+domain.ErrInvalidTransition.Error(), "")
	return capitalize(msg)
}

func (h *handlers) redoOrPage(w http.ResponseWriter, r *http.Request, status int, fe formErrors, redo func(int, formErrors)) {
	if redo != nil {
		redo(status, fe)
		return
	}
	render(w, r, status, errorPage("Not done", strings.TrimSpace(fe.Summary+" "+strings.Join(fe.Messages, "; "))))
}

func (h *handlers) notFound(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusNotFound, errorPage("Not found", "This item does not exist or was deleted."))
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// atoi parses a required integer form value into a field error.
func atoi(field, raw string) (int, *domain.FieldError) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, &domain.FieldError{Field: field, Msg: "must be a whole number"}
	}
	return n, nil
}

// atof parses a decimal form value ("," accepted).
func atof(field, raw string) (float64, *domain.FieldError) {
	n, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(raw), ",", ".", 1), 64)
	if err != nil {
		return 0, &domain.FieldError{Field: field, Msg: "must be a number"}
	}
	return n, nil
}

// fold lower-cases and strips accents, for accent-insensitive search.
func fold(s string) string {
	var b strings.Builder
	for _, c := range norm.NFD.String(s) {
		if !unicode.Is(unicode.Mn, c) {
			b.WriteRune(unicode.ToLower(c))
		}
	}
	return b.String()
}

func itoa(n int) string { return strconv.Itoa(n) }

func pathID(r *http.Request) string { return r.PathValue("id") }

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func joinStrings(s []string) string { return strings.Join(s, ", ") }

func dupValue(posted string, year int) string {
	if posted != "" {
		return posted
	}
	return strconv.Itoa(year)
}
