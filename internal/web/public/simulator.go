package public

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"awt/internal/catalog"
	"awt/internal/domain"
	"awt/internal/scoring"
	"awt/internal/store/gen"
	"awt/internal/web"

	"github.com/a-h/templ"
)

// maxSimRuns is the number of runs the simulator offers (spec 06 §6).
const maxSimRuns = 5

// simulatorJudge is the single senior judge all marks are attributed to
// (spec 03 §13).
const simulatorJudge = "simulator"

func registerSimulator(r *web.Router) {
	s := &simulator{q: gen.New(r.Deps().DB.Read), now: time.Now}
	r.Public("GET /simulator", http.HandlerFunc(s.page))
	r.Public("POST /simulator", http.HandlerFunc(s.submit))
	r.PublicCached("GET /simulator/tricks.json", http.HandlerFunc(s.tricksJSON), time.Hour, 24*time.Hour)
}

type simulator struct {
	q   *gen.Queries
	now func() time.Time
}

// simRun is one run of the form and, once computed, its result.
type simRun struct {
	N            int
	Tricks       string
	Technical    string
	Choreography string
	Landing      string
	Errors       []string
	Result       *simResult
}

// simResult is a run's displayed score; numbers are Round3d.
type simResult struct {
	Score, Technical, Choreography, Landing, Bonus, Malus float64
	Notes                                                 []string
}

// simView is the page model.
type simView struct {
	Runs    [maxSimRuns]simRun
	Reset   string
	Total   float64
	Scored  bool
	Message string // a page-level error
	Year    int
}

// catalogData is what a simulation needs from the database.
type catalogData struct {
	year      int
	cfg       scoring.Config
	rules     scoring.Rules
	tricks    map[string]catalog.UniqueTrick // by upper-cased acronym
	acronyms  []string
	positions map[string]scoring.PositionRule
	repeat    []string
}

func (s *simulator) page(w http.ResponseWriter, r *http.Request) {
	v := newView()
	if d, err := s.load(r.Context()); err == nil {
		v.Year = d.year
	}
	s.render(w, r, http.StatusOK, v)
}

func newView() *simView {
	v := &simView{}
	for i := range v.Runs {
		v.Runs[i].N = i + 1
	}
	return v
}

func (s *simulator) submit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	v := newView()
	v.Reset = strings.TrimSpace(r.PostForm.Get("reset"))
	for i := range v.Runs {
		n := strconv.Itoa(i + 1)
		v.Runs[i].Tricks = strings.TrimSpace(r.PostForm.Get("tricks" + n))
		v.Runs[i].Technical = strings.TrimSpace(r.PostForm.Get("technical" + n))
		v.Runs[i].Choreography = strings.TrimSpace(r.PostForm.Get("choreography" + n))
		v.Runs[i].Landing = strings.TrimSpace(r.PostForm.Get("landing" + n))
	}
	d, err := s.load(r.Context())
	if err != nil {
		slog.Error("simulator: load catalog", "err", err)
		v.Message = "The simulator is not available right now."
		s.render(w, r, http.StatusServiceUnavailable, v)
		return
	}
	v.Year = d.year
	v.compute(d)
	s.render(w, r, http.StatusOK, v)
}

func (s *simulator) render(w http.ResponseWriter, r *http.Request, status int, v *simView) {
	var c templ.Component = SimulatorPage(v)
	if r.Header.Get("HX-Request") == "true" && r.Method == http.MethodPost {
		c = SimulatorResults(v)
	}
	var buf bytes.Buffer
	if err := c.Render(r.Context(), &buf); err != nil {
		slog.Error("simulator: render", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func (s *simulator) tricksJSON(w http.ResponseWriter, r *http.Request) {
	d, err := s.load(r.Context())
	if err != nil {
		slog.Error("simulator: load catalog", "err", err)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	type item struct {
		Acronym string `json:"acronym"`
		Name    string `json:"name"`
	}
	out := make([]item, 0, len(d.acronyms))
	for _, a := range d.acronyms {
		out = append(out, item{Acronym: a, Name: d.tricks[strings.ToUpper(a)].Name})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(out)
}

// load builds the simulator's ruleset and catalog: the current year's, or the
// latest earlier one that exists.
func (s *simulator) load(ctx context.Context) (*catalogData, error) {
	cur := s.now().Year()
	rulesets, err := s.q.ListRulesets(ctx)
	if err != nil {
		return nil, err
	}
	var rs *gen.Ruleset
	for i := range rulesets {
		if int(rulesets[i].Year) <= cur {
			rs = &rulesets[i] // ascending: the last match is the latest
		}
	}
	if rs == nil {
		return nil, errors.New("no ruleset")
	}
	d := &catalogData{
		tricks:    map[string]catalog.UniqueTrick{},
		positions: map[string]scoring.PositionRule{},
		rules: scoring.Rules{
			TipTouchPctPerTouch: rs.TipTouchPctPerTouch,
			TipTouchPctMax:      rs.TipTouchPctMax,
			DefaultMaxBonus:     rs.DefaultMaxBonus,
		},
	}
	if err := json.Unmarshal([]byte(rs.Config), &d.cfg); err != nil {
		return nil, fmt.Errorf("ruleset config: %w", err)
	}
	if err := json.Unmarshal([]byte(rs.MaxTricksPerType), &d.rules.MaxTricksPerType); err != nil {
		return nil, fmt.Errorf("ruleset max_tricks_per_type: %w", err)
	}

	var uts []gen.UniqueTrick
	for y := cur; y > cur-10; y-- {
		if uts, err = s.q.ListUniqueTricksByYear(ctx, int64(y)); err != nil {
			return nil, err
		}
		if len(uts) > 0 {
			d.year = y
			break
		}
	}
	if len(uts) == 0 {
		d.year = cur
		return d, nil
	}
	for _, u := range uts {
		if !u.Solo {
			continue
		}
		ut, err := toUniqueTrick(u)
		if err != nil {
			return nil, err
		}
		d.tricks[strings.ToUpper(ut.Acronym)] = ut
		d.acronyms = append(d.acronyms, ut.Acronym)
	}
	tricks, err := s.q.ListTricksByYear(ctx, int64(d.year))
	if err != nil {
		return nil, err
	}
	for _, t := range tricks {
		d.positions[t.Name] = scoring.PositionRule{
			First: int(t.FirstManeuver), NoFirst: int(t.NoFirstManeuver),
			Last: int(t.LastManeuver), NoLast: int(t.NoLastManeuver),
		}
		if t.Repeatable {
			d.repeat = append(d.repeat, t.Name)
		}
	}
	return d, nil
}

func toUniqueTrick(u gen.UniqueTrick) (catalog.UniqueTrick, error) {
	ut := catalog.UniqueTrick{
		Name: u.Name, Acronym: u.Acronym, TechnicalCoefficient: u.TechnicalCoefficient,
		Bonus: u.Bonus, BaseTrick: u.BaseTrick, Synchro: u.Synchro, Solo: u.Solo, SoloAWT: u.SoloAwt,
	}
	for _, f := range []struct {
		raw string
		dst any
	}{{u.BonusTypes, &ut.BonusTypes}, {u.Uniqueness, &ut.Uniqueness}, {u.Bonuses, &ut.Bonuses}, {u.Types, &ut.Types}} {
		if err := json.Unmarshal([]byte(f.raw), f.dst); err != nil {
			return ut, fmt.Errorf("unique trick %s: %w", u.Acronym, err)
		}
	}
	return ut, nil
}

// parseTricks splits the acronym text (space or comma separated) and resolves
// each acronym against the catalog; unknown ones are returned.
func (d *catalogData) parseTricks(text string) (found []catalog.UniqueTrick, unknown []string) {
	for _, tok := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r' }) {
		if ut, ok := d.tricks[strings.ToUpper(tok)]; ok {
			found = append(found, ut)
		} else {
			unknown = append(unknown, tok)
		}
	}
	return found, unknown
}

// compute validates the runs, then scores them in order, each run seeing the
// earlier submitted runs as previous flights (spec 03 §13).
func (v *simView) compute(d *catalogData) {
	k, err := strconv.Atoi(v.Reset)
	if v.Reset != "" && (err != nil || k < 0) {
		v.Message = "The reset frequency must be a positive whole number."
	}

	type parsed struct {
		src    *simRun
		tricks []catalog.UniqueTrick
		mark   scoring.JudgeMark
	}
	var runs []parsed
	valid := v.Message == ""
	for i := range v.Runs {
		run := &v.Runs[i]
		if run.Tricks == "" && run.Technical == "" && run.Choreography == "" && run.Landing == "" {
			continue
		}
		p := parsed{src: run, mark: scoring.JudgeMark{Judge: simulatorJudge}}
		var unknown []string
		p.tricks, unknown = d.parseTricks(run.Tricks)
		if len(unknown) > 0 {
			run.Errors = append(run.Errors, "Unknown trick(s): "+strings.Join(unknown, ", "))
		}
		for _, m := range []struct {
			label, raw string
			dst        **float64
		}{{"technical", run.Technical, &p.mark.Technical}, {"choreography", run.Choreography, &p.mark.Choreography}, {"landing", run.Landing, &p.mark.Landing}} {
			val, err := domain.ValidateMark(m.raw)
			if err != nil {
				run.Errors = append(run.Errors, fmt.Sprintf("Invalid %s mark: %v", m.label, err))
				continue
			}
			*m.dst = val
		}
		if len(run.Errors) > 0 {
			valid = false
		}
		runs = append(runs, p)
	}
	if len(runs) == 0 && v.Message == "" {
		v.Message = "Enter the tricks and marks of at least one run."
		return
	}
	if !valid {
		return
	}

	v.Scored = true
	var previous []scoring.PreviousFlight
	var policies []scoring.ResetPolicy
	var total float64
	for i, p := range runs {
		policy := scoring.ResetNone
		if i > 0 && k > 0 && i%k == 0 {
			policy = scoring.ResetAll
		}
		policies = append(policies, policy)
		flown := make([]scoring.FlownTrick, len(p.tricks))
		for j, t := range p.tricks {
			flown[j] = scoring.FlownTrick{UniqueTrick: t}
		}
		in := scoring.Input{
			Type: scoring.Solo, Config: d.cfg, Rules: d.rules,
			RunIndex: i, MarkType: scoring.MarkClassic,
			RepeatableBaseTricks: d.repeat,
			ResetPolicies:        append([]scoring.ResetPolicy(nil), policies...),
			Previous:             append([]scoring.PreviousFlight(nil), previous...),
			JudgeLevels:          map[string]scoring.JudgeLevel{simulatorJudge: scoring.Senior},
			PositionRules:        d.positions,
			Flight:               scoring.Flight{Tricks: flown, Marks: []scoring.JudgeMark{p.mark}},
		}
		res, err := scoring.Score(in)
		prev := scoring.PreviousFlight{RunIndex: i, Tricks: flown}
		if err != nil {
			var ve scoring.ValidationError
			if errors.As(err, &ve) {
				p.src.Errors = append(p.src.Errors, ve.Msg)
			} else {
				slog.Error("simulator: score", "err", err)
				p.src.Errors = append(p.src.Errors, "This run could not be scored.")
			}
			previous = append(previous, prev)
			continue
		}
		// Warnings in later runs count the stored operator warnings only
		// (none here), so the previous flight carries no warnings.
		previous = append(previous, prev)
		f := res.Final
		p.src.Result = &simResult{
			Score: scoring.Round3(f.Score), Technical: scoring.Round3(f.Technical),
			Choreography: scoring.Round3(f.Choreography), Landing: scoring.Round3(f.Landing),
			Bonus: scoring.Round3(f.Bonus), Malus: scoring.Round3(f.Malus), Notes: f.Notes,
		}
		total = scoring.Round3(total + p.src.Result.Score)
	}
	v.Total = total
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
