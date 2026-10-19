package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"awt/internal/catalog"
	"awt/internal/domain"
	"awt/internal/service"
	"awt/internal/web"
)

// trickForm holds the raw values of the trick form, so a re-render shows
// exactly what was typed (all catalog fields of spec 02 §2.4.2).
type trickForm struct {
	ID          string // empty when creating
	Year        int
	Name        string
	Acronym     string
	Types       string // comma separated
	Solo        bool
	SoloAWT     bool
	Synchro     bool
	Repeatable  bool
	Directions  map[string]bool
	Coefficient string
	Bonuses     []bonusRow
	Constraints string // one pair per line, "a, b"
	First       string
	NoFirst     string
	Last        string
	NoLast      string
	Video       string
}

type bonusRow struct {
	Name  string
	On    bool
	Value string
}

type tricksView struct {
	Year     int
	Years    []int
	Tricks   []service.Trick
	Flash    string
	Errors   formErrors // import / duplicate errors
	DupFrom  string
	DupTo    string
	ImportOK bool
}

type trickFormView struct {
	Form   trickForm
	Errors formErrors
}

type uniqueView struct {
	Year   int
	Tricks []catalog.UniqueTrick
	Query  string
	Total  int
}

func (h *handlers) registerTricks(r *web.Router) {
	get(r, "/manager/tricks", h.tricksList)
	get(r, "/manager/tricks/unique", h.uniqueList)
	get(r, "/manager/tricks/new", h.trickNew)
	post(r, "/manager/tricks", h.trickCreate)
	postUpload(r, "/manager/tricks/import", h.tricksImport)
	post(r, "/manager/tricks/duplicate", h.tricksDuplicate)
	get(r, "/manager/tricks/{id}/edit", h.trickEdit)
	post(r, "/manager/tricks/{id}", h.trickUpdate)
	get(r, "/manager/tricks/{id}/delete", h.trickDeleteConfirm)
	post(r, "/manager/tricks/{id}/delete", h.trickDelete)
}

// yearParam reads a year form value: absent means the current year, a
// tampered or out-of-range value is a validation error (422).
func yearParam(r *http.Request, key string) (int, error) {
	raw := strings.TrimSpace(r.FormValue(key))
	if raw == "" {
		return time.Now().UTC().Year(), nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 1900 || n >= 3000 {
		return 0, &service.ValidationError{Fields: []domain.FieldError{{Field: key, Msg: "must be a year between 1901 and 2999"}}}
	}
	return n, nil
}

// yearChoices lists the years offered as shortcuts, latest first: the last
// two years, the current one and the next, plus the selected year.
func yearChoices(year int) []int {
	now := time.Now().UTC().Year()
	out := []int{now + 1, now, now - 1, now - 2}
	if year > now+1 {
		out = append([]int{year}, out...)
	} else if year < now-2 {
		out = append(out, year)
	}
	return out
}

func (h *handlers) tricksPage(w http.ResponseWriter, r *http.Request, status, year int, v tricksView) {
	tricks, err := h.svc.ListTricks(r.Context(), year)
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	v.Year, v.Years, v.Tricks, v.Flash = year, yearChoices(year), tricks, flash(r)
	render(w, r, status, tricksPage(v))
}

func (h *handlers) tricksList(w http.ResponseWriter, r *http.Request) {
	year, err := yearParam(r, "year")
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	h.tricksPage(w, r, http.StatusOK, year, tricksView{})
}

func (h *handlers) uniqueList(w http.ResponseWriter, r *http.Request) {
	year, err := yearParam(r, "year")
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	all, err := h.svc.ListUniqueTricks(r.Context(), year)
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	shown := all
	if q != "" {
		shown = nil
		for _, u := range all {
			if strings.Contains(fold(u.Name), fold(q)) || strings.Contains(fold(u.Acronym), fold(q)) {
				shown = append(shown, u)
			}
		}
	}
	render(w, r, http.StatusOK, uniquePage(uniqueView{Year: year, Tricks: shown, Query: q, Total: len(all)}))
}

// tricksImport loads a JSON catalog file into a year (12 MB cap).
func (h *handlers) tricksImport(w http.ResponseWriter, r *http.Request) {
	year, yerr := yearParam(r, "year")
	if yerr != nil {
		h.fail(w, r, yerr, nil)
		return
	}
	redo := func(status int, fe formErrors) { h.tricksPage(w, r, status, year, tricksView{Errors: fe}) }
	f, _, err := r.FormFile("file")
	if err != nil {
		redo(http.StatusUnprocessableEntity, validationErrors([]domain.FieldError{{Field: "file", Msg: "choose a JSON catalog file"}}))
		return
	}
	defer f.Close()
	if _, err := h.svc.ImportCatalog(r.Context(), year, f); err != nil {
		h.fail(w, r, err, redo)
		return
	}
	redirect(w, r, "/manager/tricks?year="+strconv.Itoa(year), "imported")
}

func (h *handlers) tricksDuplicate(w http.ResponseWriter, r *http.Request) {
	fromRaw, toRaw := r.PostFormValue("from"), r.PostFormValue("to")
	from, bad1 := atoi("from", fromRaw)
	to, bad2 := atoi("to", toRaw)
	year, _ := yearParam(r, "from") // only for the re-render; from/to are validated below
	redo := func(status int, fe formErrors) {
		h.tricksPage(w, r, status, year, tricksView{Errors: fe, DupFrom: fromRaw, DupTo: toRaw})
	}
	var errs []domain.FieldError
	for _, b := range []*domain.FieldError{bad1, bad2} {
		if b != nil {
			errs = append(errs, *b)
		}
	}
	if len(errs) > 0 {
		redo(http.StatusUnprocessableEntity, validationErrors(errs))
		return
	}
	if _, err := h.svc.DuplicateCatalog(r.Context(), from, to); err != nil {
		h.fail(w, r, err, redo)
		return
	}
	redirect(w, r, "/manager/tricks?year="+strconv.Itoa(to), "created")
}

func (h *handlers) trickNew(w http.ResponseWriter, r *http.Request) {
	year, err := yearParam(r, "year")
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	f := trickForm{Year: year, Solo: true, SoloAWT: true, Coefficient: "1", Directions: map[string]bool{}}
	f.Bonuses = bonusRows(nil)
	render(w, r, http.StatusOK, trickFormPage(trickFormView{Form: f}))
}

// bonusRows lists every configured bonus, with the trick's values filled in

func bonusRows(have []catalog.Bonus) []bonusRow {
	var rows []bonusRow
	for _, name := range catalog.BonusNames() {
		row := bonusRow{Name: name}
		for _, b := range have {
			if b.Name == name {
				row.On, row.Value = true, strconv.FormatFloat(b.Bonus, 'f', -1, 64)
			}
		}
		rows = append(rows, row)
	}
	return rows
}

var allDirections = []string{"left", "right", "opposite"}

func formFromTrick(t service.Trick) trickForm {
	f := trickForm{
		ID: t.ID, Year: t.Year, Name: t.Name, Acronym: t.Acronym, Types: strings.Join(t.Types, ", "),
		Solo: t.Solo, SoloAWT: t.SoloAWT, Synchro: t.Synchro, Repeatable: t.Repeatable, Directions: map[string]bool{},
		Coefficient: strconv.FormatFloat(t.TechnicalCoefficient, 'f', -1, 64),
		Bonuses:     bonusRows(t.Bonuses),
		First:       strconv.Itoa(t.FirstManeuver), NoFirst: strconv.Itoa(t.NoFirstManeuver),
		Last: strconv.Itoa(t.LastManeuver), NoLast: strconv.Itoa(t.NoLastManeuver), Video: t.SampleVideo,
	}
	for _, d := range t.Directions {
		f.Directions[d] = true
	}
	var lines []string
	for _, c := range t.BonusConstraints {
		lines = append(lines, strings.Join(c, ", "))
	}
	f.Constraints = strings.Join(lines, "\n")
	return f
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// readTrickForm parses the posted form. Unparsable numbers become field
// errors (merged with the service's own validation by the caller).
func readTrickForm(r *http.Request) (trickForm, catalog.Trick, []domain.FieldError) {
	f := trickForm{
		Name: r.PostFormValue("name"), Acronym: r.PostFormValue("acronym"), Types: r.PostFormValue("types"),
		Solo: r.PostFormValue("solo") != "", SoloAWT: r.PostFormValue("solo_awt") != "",
		Synchro: r.PostFormValue("synchro") != "", Repeatable: r.PostFormValue("repeatable") != "",
		Directions: map[string]bool{}, Coefficient: r.PostFormValue("technical_coefficient"),
		Constraints: r.PostFormValue("bonus_constraints"),
		First:       r.PostFormValue("first_maneuver"), NoFirst: r.PostFormValue("no_first_maneuver"),
		Last: r.PostFormValue("last_maneuver"), NoLast: r.PostFormValue("no_last_maneuver"),
		Video: strings.TrimSpace(r.PostFormValue("sample_video")),
	}
	t := catalog.Trick{
		Name: strings.TrimSpace(f.Name), Acronym: strings.TrimSpace(f.Acronym), Types: splitList(f.Types),
		Solo: f.Solo, SoloAWT: f.SoloAWT, Synchro: f.Synchro, Repeatable: f.Repeatable, SampleVideo: f.Video,
		Directions: []string{},
	}
	var errs []domain.FieldError
	keep := func(e *domain.FieldError) {
		if e != nil {
			errs = append(errs, *e)
		}
	}
	for _, d := range allDirections {
		if r.PostFormValue("dir_"+d) != "" {
			f.Directions[d] = true
			t.Directions = append(t.Directions, d)
		}
	}
	t.TechnicalCoefficient, _ = parseFloat(&errs, "technical_coefficient", f.Coefficient)
	ints := []struct {
		field, raw string
		dst        *int
	}{
		{"first_maneuver", f.First, &t.FirstManeuver}, {"no_first_maneuver", f.NoFirst, &t.NoFirstManeuver},
		{"last_maneuver", f.Last, &t.LastManeuver}, {"no_last_maneuver", f.NoLast, &t.NoLastManeuver},
	}
	for _, in := range ints {
		if strings.TrimSpace(in.raw) == "" {
			continue
		}
		n, bad := atoi(in.field, in.raw)
		keep(bad)
		*in.dst = n
	}
	for i, name := range catalog.BonusNames() {
		row := bonusRow{Name: name, On: r.PostFormValue("bonus_on_"+strconv.Itoa(i)) != "", Value: strings.TrimSpace(r.PostFormValue("bonus_val_" + strconv.Itoa(i)))}
		f.Bonuses = append(f.Bonuses, row)
		if !row.On {
			continue
		}
		v, ok := parseFloat(&errs, "bonuses", row.Value)
		if !ok {
			errs[len(errs)-1].Msg = "bonus " + name + " must be a number"
		}
		t.Bonuses = append(t.Bonuses, catalog.Bonus{Name: name, Bonus: v})
	}
	for _, line := range strings.Split(f.Constraints, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			parts := splitList(line)
			t.BonusConstraints = append(t.BonusConstraints, parts)
		}
	}
	return f, t, errs
}

func parseFloat(errs *[]domain.FieldError, field, raw string) (float64, bool) {
	v, bad := atof(field, raw)
	if bad != nil {
		*errs = append(*errs, *bad)
		return 0, false
	}
	return v, true
}

func (h *handlers) trickSave(w http.ResponseWriter, r *http.Request, id string) {
	f, t, errs := readTrickForm(r)
	year, yerr := yearParam(r, "year")
	if yerr != nil && id == "" {
		h.fail(w, r, yerr, nil)
		return
	}
	f.ID, f.Year = id, year
	redo := func(status int, fe formErrors) {
		render(w, r, status, trickFormPage(trickFormView{Form: f, Errors: fe}))
	}
	var old service.Trick
	if id != "" {
		var err error
		if old, err = h.svc.GetTrick(r.Context(), id); err != nil {
			h.fail(w, r, err, nil)
			return
		}
		f.Year = old.Year // a trick stays in its catalog year
		carryBonusVideos(&t, old.Trick)
	}
	if len(errs) > 0 { // unparsable numbers; the service checks the rest
		redo(http.StatusUnprocessableEntity, validationErrors(errs))
		return
	}
	var err error
	if id == "" {
		_, err = h.svc.CreateTrick(r.Context(), f.Year, t)
	} else {
		_, err = h.svc.UpdateTrick(r.Context(), id, t)
	}
	if err != nil {
		h.fail(w, r, err, redo)
		return
	}
	key := "saved"
	if id == "" {
		key = "created"
	}
	redirect(w, r, "/manager/tricks?year="+strconv.Itoa(f.Year), key)
}

// carryBonusVideos keeps the sample videos of bonuses the form cannot edit.
func carryBonusVideos(t *catalog.Trick, old catalog.Trick) {
	for i, b := range t.Bonuses {
		for _, o := range old.Bonuses {
			if o.Name == b.Name {
				t.Bonuses[i].SampleVideo = o.SampleVideo
			}
		}
	}
}

func (h *handlers) trickCreate(w http.ResponseWriter, r *http.Request) { h.trickSave(w, r, "") }
func (h *handlers) trickUpdate(w http.ResponseWriter, r *http.Request) { h.trickSave(w, r, pathID(r)) }

func (h *handlers) trickEdit(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.GetTrick(r.Context(), pathID(r))
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	render(w, r, http.StatusOK, trickFormPage(trickFormView{Form: formFromTrick(t)}))
}

func (h *handlers) trickDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.GetTrick(r.Context(), pathID(r))
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	back := "/manager/tricks?year=" + strconv.Itoa(t.Year)
	render(w, r, http.StatusOK, confirmDelete("tricks", "trick", t.Name+" ("+strconv.Itoa(t.Year)+")", "/manager/tricks/"+t.ID+"/delete", back))
}

func (h *handlers) trickDelete(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.GetTrick(r.Context(), pathID(r))
	if err == nil {
		err = h.svc.DeleteTrick(r.Context(), t.ID)
	}
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	redirect(w, r, "/manager/tricks?year="+strconv.Itoa(t.Year), "deleted")
}
