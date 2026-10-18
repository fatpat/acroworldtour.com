package admin

import (
	"net/http"
	"strconv"
	"strings"

	"awt/internal/domain"
	"awt/internal/service"
	"awt/internal/web"
)

type judgeForm struct {
	ID      string
	Name    string
	Level   string
	Country string
	CivlID  string
}

type judgesView struct {
	Judges []service.Judge
	Flash  string
}

type judgeFormView struct {
	Form   judgeForm
	Errors formErrors
}

var judgeLevels = []option{
	{string(domain.JudgeTrainee), "trainee"},
	{string(domain.JudgeCertified), "certified"},
	{string(domain.JudgeSenior), "senior"},
}

func (h *handlers) registerJudges(r *web.Router) {
	get(r, "/admin/judges", h.judgesList)
	get(r, "/admin/judges/new", h.judgeNew)
	post(r, "/admin/judges", h.judgeCreate)
	get(r, "/admin/judges/{id}/edit", h.judgeEdit)
	post(r, "/admin/judges/{id}", h.judgeUpdate)
	get(r, "/admin/judges/{id}/delete", h.judgeDeleteConfirm)
	post(r, "/admin/judges/{id}/delete", h.judgeDelete)
}

func (h *handlers) judgesList(w http.ResponseWriter, r *http.Request) {
	judges, err := h.svc.ListJudges(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	render(w, r, http.StatusOK, judgesPage(judgesView{Judges: judges, Flash: flash(r)}))
}

func (h *handlers) judgeNew(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, judgeFormPage(judgeFormView{Form: judgeForm{Level: string(domain.JudgeCertified)}}))
}

func (h *handlers) judgeSave(w http.ResponseWriter, r *http.Request, id string) {
	f := judgeForm{ID: id, Name: r.PostFormValue("name"), Level: r.PostFormValue("level"),
		Country: r.PostFormValue("country"), CivlID: strings.TrimSpace(r.PostFormValue("civlid"))}
	redo := func(status int, fe formErrors) {
		render(w, r, status, judgeFormPage(judgeFormView{Form: f, Errors: fe}))
	}
	in := service.JudgeInput{Name: strings.TrimSpace(f.Name), Country: strings.TrimSpace(f.Country), Level: domain.JudgeLevel(f.Level)}
	if f.CivlID != "" {
		n, bad := atoi("civlid", f.CivlID)
		if bad != nil || n < 1 {
			redo(http.StatusUnprocessableEntity, validationErrors([]domain.FieldError{{Field: "civlid", Msg: "must be a positive CIVL id, or empty"}}))
			return
		}
		in.CivlID = n
	}
	var err error
	if id == "" {
		_, err = h.svc.CreateJudge(r.Context(), in)
	} else {
		_, err = h.svc.UpdateJudge(r.Context(), id, in)
	}
	if err != nil {
		h.fail(w, r, err, redo)
		return
	}
	key := "created"
	if id != "" {
		key = "saved"
	}
	redirect(w, r, "/admin/judges", key)
}

func (h *handlers) judgeCreate(w http.ResponseWriter, r *http.Request) { h.judgeSave(w, r, "") }
func (h *handlers) judgeUpdate(w http.ResponseWriter, r *http.Request) { h.judgeSave(w, r, pathID(r)) }

func (h *handlers) judgeEdit(w http.ResponseWriter, r *http.Request) {
	j, err := h.svc.GetJudge(r.Context(), pathID(r))
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	f := judgeForm{ID: j.ID, Name: j.Name, Level: string(j.Level), Country: j.Country}
	if j.CivlID != 0 {
		f.CivlID = strconv.Itoa(j.CivlID)
	}
	render(w, r, http.StatusOK, judgeFormPage(judgeFormView{Form: f}))
}

func (h *handlers) judgeDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	j, err := h.svc.GetJudge(r.Context(), pathID(r))
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	render(w, r, http.StatusOK, confirmDelete("judges", "judge", j.Name, "/admin/judges/"+j.ID+"/delete", "/admin/judges"))
}

func (h *handlers) judgeDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteJudge(r.Context(), pathID(r)); err != nil {
		h.fail(w, r, err, nil)
		return
	}
	redirect(w, r, "/admin/judges", "deleted")
}
