package admin

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"awt/internal/civl"
	"awt/internal/domain"
	"awt/internal/service"
	"awt/internal/web"
)

type pilotsView struct {
	Pilots  []service.Pilot
	Query   string
	Total   int
	Year    int
	Flash   string
	Errors  formErrors
	ImportV string // civlid typed in the import form
}

func (h *handlers) registerPilots(r *web.Router) {
	get(r, "/manager/pilots", h.pilotsList)
	post(r, "/manager/pilots/import", h.pilotsImport)
	post(r, "/manager/pilots/rankings", h.pilotsRankings)
	post(r, "/manager/pilots/awt", h.pilotsAWTForm)
	post(r, "/manager/pilots/{id}/gender", h.pilotsGender)
	post(r, "/manager/pilots/{id}/awt", h.pilotsAWT)
}

// filterPilots keeps the pilots matching q: a positive integer is a CIVL id,
// anything else is an accent- and case-insensitive substring of the name
// (plain text, never a regex: spec 05 §7.1).
func filterPilots(all []service.Pilot, q string) []service.Pilot {
	q = strings.TrimSpace(q)
	if q == "" {
		return all
	}
	id, err := strconv.Atoi(q)
	var out []service.Pilot
	for _, p := range all {
		if (err == nil && id > 0 && p.CivlID == id) || strings.Contains(fold(p.Name), fold(q)) {
			out = append(out, p)
		}
	}
	return out
}

func (h *handlers) pilotsPage(w http.ResponseWriter, r *http.Request, status int, query, importV string, fe formErrors) {
	all, err := h.svc.ListPilots(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	render(w, r, status, pilotsPage(pilotsView{
		Pilots: filterPilots(all, query), Query: query, Total: len(all), Year: time.Now().UTC().Year(),
		Flash: flash(r), Errors: fe, ImportV: importV,
	}))
}

func (h *handlers) pilotsList(w http.ResponseWriter, r *http.Request) {
	h.pilotsPage(w, r, http.StatusOK, r.URL.Query().Get("q"), "", formErrors{})
}

func (h *handlers) pilotsImport(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.PostFormValue("civlid"))
	id, bad := atoi("civlid", raw)
	if bad != nil || id < 1 {
		h.pilotsPage(w, r, http.StatusUnprocessableEntity, "", raw,
			validationErrors([]domain.FieldError{{Field: "civlid", Msg: "enter a positive CIVL id"}}))
		return
	}
	if _, err := h.svc.ImportPilotFromCIVL(r.Context(), id); err != nil {
		var se *civl.StatusError
		switch {
		case errors.Is(err, service.ErrNotFound), errors.As(err, &se) && se.Status == http.StatusNotFound:
			h.pilotsPage(w, r, http.StatusUnprocessableEntity, "", raw, validationErrors([]domain.FieldError{
				{Field: "civlid", Msg: "CIVL id " + raw + " was not found on civlcomps.org"}}))
		case upstreamDown(err):
			h.pilotsPage(w, r, http.StatusBadGateway, "", raw, messageError("CIVL is not reachable right now, try again later."))
		default:
			h.fail(w, r, err, func(status int, fe formErrors) { h.pilotsPage(w, r, status, "", raw, fe) })
		}
		return
	}
	redirect(w, r, "/manager/pilots?q="+strconv.Itoa(id), "imported")
}

func (h *handlers) pilotsRankings(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RefreshRankings(r.Context()); err != nil {
		if upstreamDown(err) {
			h.pilotsPage(w, r, http.StatusBadGateway, "", "", messageError("CIVL rankings are not reachable right now, try again later."))
			return
		}
		h.fail(w, r, err, nil)
		return
	}
	redirect(w, r, "/manager/pilots", "ranks")
}

// pilotRowResponse answers a row action: the updated row for htmx, the list
// for a plain form post.
func (h *handlers) pilotRowResponse(w http.ResponseWriter, r *http.Request, civlid int) {
	if !isHTMX(r) {
		redirect(w, r, "/manager/pilots?q="+strconv.Itoa(civlid), "saved")
		return
	}
	p, err := h.svc.GetPilot(r.Context(), civlid)
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	render(w, r, http.StatusOK, pilotRow(p, time.Now().UTC().Year()))
}

// upstreamDown reports a CIVL failure (HTTP status or network), as opposed
// to a local error.
func upstreamDown(err error) bool {
	var se *civl.StatusError
	var ue *url.Error
	return errors.As(err, &se) || errors.As(err, &ue)
}

func pilotID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	return id, err == nil && id > 0
}

func (h *handlers) pilotsGender(w http.ResponseWriter, r *http.Request) {
	id, ok := pilotID(r)
	if !ok {
		h.notFound(w, r)
		return
	}
	if _, err := h.svc.ToggleGender(r.Context(), id); err != nil {
		h.fail(w, r, err, nil)
		return
	}
	h.pilotRowResponse(w, r, id)
}

func (h *handlers) pilotsAWT(w http.ResponseWriter, r *http.Request) {
	id, ok := pilotID(r)
	if !ok {
		h.notFound(w, r)
		return
	}
	year, bad := atoi("year", r.PostFormValue("year"))
	if bad != nil || year < 1900 || year > 3000 {
		h.pilotsPage(w, r, http.StatusUnprocessableEntity, "", "", validationErrors([]domain.FieldError{{Field: "year", Msg: "enter a year"}}))
		return
	}
	if err := h.svc.SetAWTYear(r.Context(), id, year, r.PostFormValue("awt") == "1"); err != nil {
		h.fail(w, r, err, nil)
		return
	}
	h.pilotRowResponse(w, r, id)
}

// pilotsAWTForm sets the AWT status of a pilot for any year (the row
// buttons only cover the current and next year).
func (h *handlers) pilotsAWTForm(w http.ResponseWriter, r *http.Request) {
	id, bad1 := atoi("civlid", r.PostFormValue("civlid"))
	year, bad2 := atoi("year", r.PostFormValue("year"))
	var errs []domain.FieldError
	for _, b := range []*domain.FieldError{bad1, bad2} {
		if b != nil {
			errs = append(errs, *b)
		}
	}
	if len(errs) == 0 && (year < 1900 || year > 3000) {
		errs = append(errs, domain.FieldError{Field: "year", Msg: "enter a year"})
	}
	if len(errs) > 0 {
		h.pilotsPage(w, r, http.StatusUnprocessableEntity, "", "", validationErrors(errs))
		return
	}
	if err := h.svc.SetAWTYear(r.Context(), id, year, r.PostFormValue("awt") == "1"); err != nil {
		h.fail(w, r, err, func(status int, fe formErrors) { h.pilotsPage(w, r, status, "", "", fe) })
		return
	}
	redirect(w, r, "/manager/pilots?q="+strconv.Itoa(id), "saved")
}

// pilotYears lists the years shown as toggles: the current and next year,
// then every other year the pilot has a status for.
func pilotYears(p service.Pilot, now int) []int {
	years := []int{now, now + 1}
	for _, y := range p.AWTYears {
		if !slices.Contains(years, y) {
			years = append(years, y)
		}
	}
	slices.Sort(years)
	return years
}

func hasYear(p service.Pilot, y int) bool { return slices.Contains(p.AWTYears, y) }

func rankLabel(rank int) string {
	if rank >= civl.UnrankedRank {
		return "unranked"
	}
	return strconv.Itoa(rank)
}

func genderLabel(g domain.Gender) string {
	switch g {
	case domain.GenderMan:
		return "man"
	case domain.GenderWoman:
		return "woman"
	}
	return "not set"
}
