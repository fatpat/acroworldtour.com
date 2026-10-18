package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"awt/internal/domain"
	"awt/internal/service"
	"awt/internal/web"
)

type seasonForm struct {
	ID      string
	Name    string
	Code    string
	Year    string
	Country string
	Image   string // file id
	Index   string
}

type seasonsView struct {
	Seasons []service.Season
	Flash   string
}

type seasonFormView struct {
	Form   seasonForm
	Images []service.File
	Errors formErrors
}

func (h *handlers) registerSeasons(r *web.Router) {
	get(r, "/admin/seasons", h.seasonsList)
	get(r, "/admin/seasons/new", h.seasonNew)
	post(r, "/admin/seasons", h.seasonCreate)
	get(r, "/admin/seasons/{id}/edit", h.seasonEdit)
	post(r, "/admin/seasons/{id}", h.seasonUpdate)
	get(r, "/admin/seasons/{id}/delete", h.seasonDeleteConfirm)
	post(r, "/admin/seasons/{id}/delete", h.seasonDelete)
}

func (h *handlers) seasonsList(w http.ResponseWriter, r *http.Request) {
	seasons, err := h.svc.ListSeasons(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	render(w, r, http.StatusOK, seasonsPage(seasonsView{Seasons: seasons, Flash: flash(r)}))
}

// imageFiles lists the uploaded images offered as a season image.
func (h *handlers) imageFiles(w http.ResponseWriter, r *http.Request) ([]service.File, bool) {
	files, err := h.svc.ListFiles(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return nil, false
	}
	var out []service.File
	for _, f := range files {
		if strings.HasPrefix(f.ContentType, "image/") {
			out = append(out, f)
		}
	}
	return out, true
}

func (h *handlers) seasonFormPage(w http.ResponseWriter, r *http.Request, status int, f seasonForm, fe formErrors) {
	images, ok := h.imageFiles(w, r)
	if !ok {
		return
	}
	render(w, r, status, seasonFormPage(seasonFormView{Form: f, Images: images, Errors: fe}))
}

func (h *handlers) seasonNew(w http.ResponseWriter, r *http.Request) {
	// A fresh form every time: no state is shared between requests.
	h.seasonFormPage(w, r, http.StatusOK, seasonForm{Year: strconv.Itoa(time.Now().UTC().Year()), Index: "999"}, formErrors{})
}

func (h *handlers) seasonSave(w http.ResponseWriter, r *http.Request, id string) {
	f := seasonForm{ID: id, Name: r.PostFormValue("name"), Code: r.PostFormValue("code"), Year: r.PostFormValue("year"),
		Country: r.PostFormValue("country"), Image: r.PostFormValue("image"), Index: r.PostFormValue("index")}
	redo := func(status int, fe formErrors) { h.seasonFormPage(w, r, status, f, fe) }
	in := service.SeasonInput{Name: strings.TrimSpace(f.Name), Code: strings.TrimSpace(f.Code),
		Country: strings.ToLower(strings.TrimSpace(f.Country)), ImageFile: f.Image}
	var errs []domain.FieldError
	var bad *domain.FieldError
	if in.Year, bad = atoi("year", f.Year); bad != nil {
		errs = append(errs, *bad)
	}
	if in.SortIndex, bad = atoi("index", f.Index); bad != nil {
		errs = append(errs, *bad)
	}
	if len(errs) > 0 {
		redo(http.StatusUnprocessableEntity, validationErrors(errs))
		return
	}
	var err error
	if id == "" {
		_, err = h.svc.CreateSeason(r.Context(), in)
	} else {
		_, err = h.svc.UpdateSeason(r.Context(), id, in)
	}
	if err != nil {
		h.fail(w, r, err, redo)
		return
	}
	key := "created"
	if id != "" {
		key = "saved"
	}
	redirect(w, r, "/admin/seasons", key)
}

func (h *handlers) seasonCreate(w http.ResponseWriter, r *http.Request) { h.seasonSave(w, r, "") }
func (h *handlers) seasonUpdate(w http.ResponseWriter, r *http.Request) {
	h.seasonSave(w, r, pathID(r))
}

// seasonByID finds an active season by id (the service reads by code).
func (h *handlers) seasonByID(w http.ResponseWriter, r *http.Request) (service.Season, bool) {
	seasons, err := h.svc.ListSeasons(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return service.Season{}, false
	}
	for _, s := range seasons {
		if s.ID == pathID(r) {
			return s, true
		}
	}
	h.notFound(w, r)
	return service.Season{}, false
}

func (h *handlers) seasonEdit(w http.ResponseWriter, r *http.Request) {
	s, ok := h.seasonByID(w, r)
	if !ok {
		return
	}
	h.seasonFormPage(w, r, http.StatusOK, seasonForm{ID: s.ID, Name: s.Name, Code: s.Code, Year: strconv.Itoa(s.Year),
		Country: s.Country, Image: s.ImageFile, Index: strconv.Itoa(s.SortIndex)}, formErrors{})
}

func (h *handlers) seasonDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	s, ok := h.seasonByID(w, r)
	if !ok {
		return
	}
	render(w, r, http.StatusOK, confirmDelete("seasons", "season", s.Name+" ("+s.Code+")", "/admin/seasons/"+s.ID+"/delete", "/admin/seasons"))
}

func (h *handlers) seasonDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteSeason(r.Context(), pathID(r)); err != nil {
		h.fail(w, r, err, nil)
		return
	}
	redirect(w, r, "/admin/seasons", "deleted")
}
