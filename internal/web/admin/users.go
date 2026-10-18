package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strings"

	"awt/internal/auth"
	"awt/internal/domain"
	"awt/internal/service"
	"awt/internal/web"
)

type usersView struct {
	Users  []auth.User
	Flash  string
	Errors formErrors
	Form   userForm
}

type userForm struct{ Email, Name, Role string }

type userView struct {
	User     auth.User
	Comps    []service.Competition
	Assigned []string
	Flash    string
	Errors   formErrors
	Self     bool
}

var roleOptions = []option{{auth.RoleScorer, "scorer (judges assigned competitions)"}, {auth.RoleAdmin, "admin"}}

func (h *handlers) registerUsers(r *web.Router) {
	get(r, "/admin/users", h.usersList)
	post(r, "/admin/users", h.userCreate)
	get(r, "/admin/users/{id}", h.userShow)
	post(r, "/admin/users/{id}/disable", func(w http.ResponseWriter, r *http.Request) { h.userDisabled(w, r, true) })
	post(r, "/admin/users/{id}/enable", func(w http.ResponseWriter, r *http.Request) { h.userDisabled(w, r, false) })
	post(r, "/admin/users/{id}/competitions", h.userCompetitions)
	post(r, "/admin/users/{id}/password", h.userPassword)
}

func (h *handlers) usersPage(w http.ResponseWriter, r *http.Request, status int, f userForm, fe formErrors) {
	users, err := h.auth.ListUsers(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	if f.Role == "" {
		f.Role = auth.RoleScorer
	}
	render(w, r, status, usersPage(usersView{Users: users, Flash: flash(r), Errors: fe, Form: f}))
}

func (h *handlers) usersList(w http.ResponseWriter, r *http.Request) {
	h.usersPage(w, r, http.StatusOK, userForm{}, formErrors{})
}

// authFail maps auth input errors onto the form (422) or a conflict (409).
func (h *handlers) authFail(err error, status func(int, formErrors)) bool {
	var ie *auth.InputError
	var ee *auth.ExistsError
	switch {
	case errors.As(err, &ie):
		status(http.StatusUnprocessableEntity, validationErrors([]domain.FieldError{{Field: ie.Field, Msg: ie.Msg}}))
	case errors.As(err, &ee):
		status(http.StatusConflict, messageError(capitalize(ee.Error())))
	default:
		return false
	}
	return true
}

func (h *handlers) userCreate(w http.ResponseWriter, r *http.Request) {
	f := userForm{Email: r.PostFormValue("email"), Name: strings.TrimSpace(r.PostFormValue("name")), Role: r.PostFormValue("role")}
	redo := func(status int, fe formErrors) { h.usersPage(w, r, status, f, fe) }
	if _, err := h.auth.CreateUser(r.Context(), f.Email, f.Name, f.Role, r.PostFormValue("password")); err != nil {
		if !h.authFail(err, redo) {
			h.fail(w, r, err, redo)
		}
		return
	}
	redirect(w, r, "/admin/users", "created")
}

func (h *handlers) loadUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	u, err := h.auth.GetUser(r.Context(), pathID(r))
	if errors.Is(err, sql.ErrNoRows) {
		h.notFound(w, r)
		return u, false
	}
	if err != nil {
		h.fail(w, r, err, nil)
		return u, false
	}
	return u, true
}

func (h *handlers) userPage(w http.ResponseWriter, r *http.Request, status int, u auth.User, fe formErrors) {
	comps, err := h.svc.ListCompetitions(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	assigned, err := h.auth.UserCompetitions(r.Context(), u.ID)
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	sess, _ := auth.SessionFromContext(r.Context())
	render(w, r, status, userPage(userView{User: u, Comps: comps, Assigned: assigned, Flash: flash(r), Errors: fe, Self: sess.User.ID == u.ID}))
}

func (h *handlers) userShow(w http.ResponseWriter, r *http.Request) {
	if u, ok := h.loadUser(w, r); ok {
		h.userPage(w, r, http.StatusOK, u, formErrors{})
	}
}

func (h *handlers) userDisabled(w http.ResponseWriter, r *http.Request, disabled bool) {
	u, ok := h.loadUser(w, r)
	if !ok {
		return
	}
	if sess, _ := auth.SessionFromContext(r.Context()); disabled && sess.User.ID == u.ID {
		h.userPage(w, r, http.StatusConflict, u, messageError("You cannot disable your own account."))
		return
	}
	if err := h.auth.SetDisabled(r.Context(), u.ID, disabled); err != nil {
		h.fail(w, r, err, nil)
		return
	}
	key := "enabled"
	if disabled {
		key = "disabled"
	}
	redirect(w, r, "/admin/users/"+u.ID, key)
}

func (h *handlers) userCompetitions(w http.ResponseWriter, r *http.Request) {
	u, ok := h.loadUser(w, r)
	if !ok {
		return
	}
	comps, err := h.svc.ListCompetitions(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	var ids []string
	for _, c := range comps { // only ids of existing competitions are kept
		if slices.Contains(r.PostForm["competition"], c.ID) {
			ids = append(ids, c.ID)
		}
	}
	if err := h.auth.SetUserCompetitions(r.Context(), u.ID, ids); err != nil {
		h.fail(w, r, err, nil)
		return
	}
	redirect(w, r, "/admin/users/"+u.ID, "saved")
}

func (h *handlers) userPassword(w http.ResponseWriter, r *http.Request) {
	u, ok := h.loadUser(w, r)
	if !ok {
		return
	}
	redo := func(status int, fe formErrors) { h.userPage(w, r, status, u, fe) }
	if err := h.auth.SetPasswordByID(r.Context(), u.ID, r.PostFormValue("password")); err != nil {
		if !h.authFail(err, redo) {
			h.fail(w, r, err, redo)
		}
		return
	}
	redirect(w, r, "/admin/users/"+u.ID, "password")
}

func roleLabel(u auth.User) string {
	if u.Disabled {
		return u.Role + " (disabled)"
	}
	return u.Role
}

func assignedTo(assigned []string, id string) bool { return slices.Contains(assigned, id) }
