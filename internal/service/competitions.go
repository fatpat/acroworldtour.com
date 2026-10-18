package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"awt/internal/domain"
	"awt/internal/scoring"
	"awt/internal/store/gen"
)

// CompetitionInput is the editable part of a competition (spec 02 §2.5.2).
// An empty Code is derived from Name at creation and kept on update.
type CompetitionInput struct {
	Name      string
	Code      string
	Type      domain.CompetitionType
	StartDate time.Time
	EndDate   time.Time
	Location  string
	Published bool
	Website   string
	ImageFile string
	LogoFile  string
	Seasons   []string
}

// Competition is a competition with its ordered lists. Pilots are kept in
// starting order of run 1: CIVL rank descending (spec 02 §2.5.2).
type Competition struct {
	ID               string
	Code             string
	Name             string
	Type             domain.CompetitionType
	State            domain.CompetitionState
	StartDate        time.Time
	EndDate          time.Time
	Location         string
	Published        bool
	Website          string
	ImageFile        string
	LogoFile         string
	Config           json.RawMessage // spec 02 §2.6 shape
	Frozen           bool
	ResultsFinal     bool
	Seasons          []string
	Pilots           []int
	Teams            []string
	Judges           []string
	RepeatableTricks []string // base trick names
	UpdatedAt        string
}

// Year is the competition year: it selects the trick catalog (decision
// 0008), the ruleset and the pilots' AWT status.
func (c Competition) Year() int { return c.StartDate.Year() }

func (c Competition) domain() domain.Competition {
	return domain.Competition{
		ID: c.ID, Name: c.Name, Code: c.Code, StartDate: c.StartDate, EndDate: c.EndDate, Location: c.Location,
		Type: c.Type, State: c.State, Seasons: c.Seasons, Pilots: c.Pilots, Teams: c.Teams, Judges: c.Judges,
	}
}

func (c Competition) topics() []string {
	out := []string{competitionTopic(c.Code)}
	for _, s := range c.Seasons {
		out = append(out, seasonTopic(s))
	}
	return out
}

// parseDate reads a stored date (YYYY-MM-DD, or a longer RFC 3339 value).
func parseDate(s string) (time.Time, error) {
	if len(s) > len(time.DateOnly) {
		s = s[:len(time.DateOnly)]
	}
	return time.Parse(time.DateOnly, s)
}

func loadCompetition(ctx context.Context, q *gen.Queries, id string, withDeleted bool) (Competition, gen.Competition, error) {
	r, err := q.GetCompetition(ctx, id)
	if err != nil {
		return Competition{}, r, notFound(err, "competition "+id)
	}
	if r.DeletedAt.Valid && !withDeleted {
		return Competition{}, r, fmt.Errorf("competition %s: %w", id, ErrNotFound)
	}
	c, err := competitionFromRow(ctx, q, r)
	return c, r, err
}

func competitionFromRow(ctx context.Context, q *gen.Queries, r gen.Competition) (Competition, error) {
	c := Competition{
		ID: r.ID, Code: r.Code, Name: r.Name, Type: domain.CompetitionType(r.Type), State: domain.CompetitionState(r.State),
		Location: r.Location, Published: r.Published, Website: r.Website.String, ImageFile: r.ImageFile.String,
		LogoFile: r.LogoFile.String, Config: json.RawMessage(r.Config), Frozen: r.Frozen, ResultsFinal: r.ResultsFinal,
		UpdatedAt: r.UpdatedAt.String, Seasons: []string{}, Pilots: []int{}, Teams: []string{}, Judges: []string{},
		RepeatableTricks: []string{},
	}
	var err error
	if c.StartDate, err = parseDate(r.StartDate); err != nil {
		return c, fmt.Errorf("competition %s start date: %w", r.ID, err)
	}
	if c.EndDate, err = parseDate(r.EndDate); err != nil {
		return c, fmt.Errorf("competition %s end date: %w", r.ID, err)
	}
	seasons, err := q.ListCompetitionSeasons(ctx, r.ID)
	if err != nil {
		return c, fmt.Errorf("list seasons of %s: %w", r.ID, err)
	}
	pilots, err := q.ListCompetitionPilots(ctx, r.ID)
	if err != nil {
		return c, fmt.Errorf("list pilots of %s: %w", r.ID, err)
	}
	teams, err := q.ListCompetitionTeams(ctx, r.ID)
	if err != nil {
		return c, fmt.Errorf("list teams of %s: %w", r.ID, err)
	}
	judges, err := q.ListCompetitionJudges(ctx, r.ID)
	if err != nil {
		return c, fmt.Errorf("list judges of %s: %w", r.ID, err)
	}
	repeatable, err := q.ListCompetitionRepeatableTricks(ctx, r.ID)
	if err != nil {
		return c, fmt.Errorf("list repeatable tricks of %s: %w", r.ID, err)
	}
	c.Seasons = append(c.Seasons, seasons...)
	for _, p := range pilots {
		c.Pilots = append(c.Pilots, int(p))
	}
	c.Teams = append(c.Teams, teams...)
	c.Judges = append(c.Judges, judges...)
	c.RepeatableTricks = append(c.RepeatableTricks, repeatable...)
	return c, nil
}

// editable loads an active competition for a mutation: frozen (imported)
// competitions refuse every mutation (decision 0003).
func editable(ctx context.Context, q *gen.Queries, id string) (Competition, gen.Competition, error) {
	c, r, err := loadCompetition(ctx, q, id, false)
	if err != nil {
		return c, r, err
	}
	if r.Frozen {
		return c, r, fmt.Errorf("competition %s: %w", r.Code, ErrFrozen)
	}
	return c, r, nil
}

func upsertCompetition(ctx context.Context, q *gen.Queries, r gen.Competition) error {
	err := q.UpsertCompetition(ctx, gen.UpsertCompetitionParams(r))
	if err != nil {
		return conflict(err, fmt.Sprintf("competition %q (%s)", r.Name, r.Code))
	}
	return nil
}

// GetCompetition returns an active competition by id.
func (s *Service) GetCompetition(ctx context.Context, id string) (Competition, error) {
	c, _, err := loadCompetition(ctx, s.read(), id, false)
	return c, err
}

// GetCompetitionByCode returns an active competition by code.
func (s *Service) GetCompetitionByCode(ctx context.Context, code string) (Competition, error) {
	q := s.read()
	r, err := q.GetCompetitionByCode(ctx, code)
	if err != nil {
		return Competition{}, notFound(err, "competition "+code)
	}
	return competitionFromRow(ctx, q, r)
}

// ListCompetitions returns every active competition, published or not
// (back-office list, spec 05 §8), latest first.
func (s *Service) ListCompetitions(ctx context.Context) ([]Competition, error) {
	q := s.read()
	rows, err := q.ListCompetitions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list competitions: %w", err)
	}
	out := make([]Competition, 0, len(rows))
	for _, r := range rows {
		c, err := competitionFromRow(ctx, q, r)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// checkInput validates the editable fields and the references they hold.
func checkInput(ctx context.Context, q *gen.Queries, in CompetitionInput) []domain.FieldError {
	var errs []domain.FieldError
	add := func(f, m string) { errs = append(errs, domain.FieldError{Field: f, Msg: m}) }
	if strings.TrimSpace(in.Name) == "" {
		add("name", "required")
	}
	if in.Code == "" {
		add("code", "required (the name gives no usable code)")
	}
	if in.StartDate.IsZero() || in.EndDate.IsZero() {
		add("start_date", "start and end dates are required")
	}
	if in.Website != "" {
		if u, err := url.Parse(in.Website); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			add("website", "must be an http(s) URL")
		}
	}
	for _, f := range []struct{ field, id string }{{"image", in.ImageFile}, {"logo", in.LogoFile}} {
		if f.id == "" {
			continue
		}
		if err := checkFile(ctx, q, f.id); err != nil {
			add(f.field, err.Error())
		}
	}
	seen := map[string]bool{}
	for _, code := range in.Seasons {
		if seen[code] {
			add("seasons", fmt.Sprintf("duplicate season %q", code))
		}
		seen[code] = true
		if _, err := q.GetSeasonByCode(ctx, code); err != nil {
			add("seasons", fmt.Sprintf("season %q is unknown", code))
		}
	}
	return errs
}

func setSeasons(ctx context.Context, q *gen.Queries, id string, seasons []string) error {
	if err := q.DeleteCompetitionSeasons(ctx, id); err != nil {
		return fmt.Errorf("clear seasons of %s: %w", id, err)
	}
	for i, code := range seasons {
		if err := q.AddCompetitionSeason(ctx, gen.AddCompetitionSeasonParams{CompetitionID: id, Position: int64(i), SeasonCode: code}); err != nil {
			return fmt.Errorf("add season %s to %s: %w", code, id, err)
		}
	}
	return nil
}

// CreateCompetition creates a competition in state init. Its config is
// copied from the ruleset of start_date.year and its repeatable tricks are
// the repeatable tricks of that year's catalog (spec 02 §5.1, decision 0008).
func (s *Service) CreateCompetition(ctx context.Context, in CompetitionInput) (Competition, error) {
	id, err := newID()
	if err != nil {
		return Competition{}, err
	}
	if in.Code == "" {
		in.Code = domain.Slug(in.Name)
	}
	var out Competition
	err = s.db.Tx(ctx, func(q *gen.Queries) error {
		c := Competition{Name: in.Name, Code: in.Code, Type: in.Type, State: domain.CompetitionInit,
			StartDate: in.StartDate, EndDate: in.EndDate}
		errs := append(checkInput(ctx, q, in), domain.ValidateCompetition(c.domain())...)
		year := in.StartDate.Year()
		rs, err := q.GetRuleset(ctx, int64(year))
		if errors.Is(err, sql.ErrNoRows) {
			errs = append(errs, domain.FieldError{Field: "start_date", Msg: fmt.Sprintf("no ruleset for year %d", year)})
		} else if err != nil {
			return fmt.Errorf("get ruleset %d: %w", year, err)
		}
		if err := invalid(errs...); err != nil {
			return err
		}
		err = upsertCompetition(ctx, q, gen.Competition{
			ID: id, Code: in.Code, Name: in.Name, Type: string(in.Type), State: string(domain.CompetitionInit),
			StartDate: in.StartDate.Format(time.DateOnly), EndDate: in.EndDate.Format(time.DateOnly), Location: in.Location,
			Published: in.Published, Website: nullString(in.Website), ImageFile: nullString(in.ImageFile),
			LogoFile: nullString(in.LogoFile), Config: rs.Config, UpdatedAt: nullString(now()),
		})
		if err != nil {
			return err
		}
		if err := setSeasons(ctx, q, id, in.Seasons); err != nil {
			return err
		}
		tricks, err := q.ListTricksByYear(ctx, int64(year))
		if err != nil {
			return fmt.Errorf("list tricks %d: %w", year, err)
		}
		var repeatable []string
		for _, t := range tricks {
			if t.Repeatable {
				repeatable = append(repeatable, t.Name)
			}
		}
		if err := setRepeatable(ctx, q, id, repeatable); err != nil {
			return err
		}
		out, _, err = loadCompetition(ctx, q, id, false)
		return err
	})
	if err != nil {
		return Competition{}, err
	}
	s.publish(KindCompetition, competitionTopic(out.Code))
	return out, nil
}

// UpdateCompetition replaces the editable fields. Name, code and type only
// change in init; a closed competition is read-only (spec 02 §5.1).
func (s *Service) UpdateCompetition(ctx context.Context, id string, in CompetitionInput) (Competition, error) {
	var out Competition
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		cur, row, err := editable(ctx, q, id)
		if err != nil {
			return err
		}
		if in.Code == "" {
			in.Code = cur.Code
		}
		identity := in.Name != cur.Name || in.Code != cur.Code || in.Type != cur.Type
		if err := domain.CanUpdateCompetition(cur.State, identity); err != nil {
			return err
		}
		next := cur
		next.Name, next.Code, next.Type, next.StartDate, next.EndDate = in.Name, in.Code, in.Type, in.StartDate, in.EndDate
		next.Seasons = in.Seasons
		if err := invalid(append(checkInput(ctx, q, in), domain.ValidateCompetition(next.domain())...)...); err != nil {
			return err
		}
		row.Name, row.Code, row.Type = in.Name, in.Code, string(in.Type)
		row.StartDate, row.EndDate = in.StartDate.Format(time.DateOnly), in.EndDate.Format(time.DateOnly)
		row.Location, row.Published, row.Website = in.Location, in.Published, nullString(in.Website)
		row.ImageFile, row.LogoFile, row.UpdatedAt = nullString(in.ImageFile), nullString(in.LogoFile), nullString(now())
		if err := upsertCompetition(ctx, q, row); err != nil {
			return err
		}
		if err := setSeasons(ctx, q, id, in.Seasons); err != nil {
			return err
		}
		out, _, err = loadCompetition(ctx, q, id, false)
		return err
	})
	if err != nil {
		return Competition{}, err
	}
	s.publish(KindCompetition, competitionTopic(out.Code))
	return out, nil
}

// update runs fn on an editable, not closed competition, validates the
// result with domain.ValidateCompetition, then lets fn's write run and
// publishes a competition event.
func (s *Service) update(ctx context.Context, id string, fn func(q *gen.Queries, c *Competition) (write func() error, err error)) error {
	var code string
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		c, row, err := editable(ctx, q, id)
		if err != nil {
			return err
		}
		if err := domain.CanUpdateCompetition(c.State, false); err != nil {
			return err
		}
		write, err := fn(q, &c)
		if err != nil {
			return err
		}
		if err := invalid(domain.ValidateCompetition(c.domain())...); err != nil {
			return err
		}
		if err := write(); err != nil {
			return err
		}
		row.UpdatedAt = nullString(now())
		code = c.Code
		return upsertCompetition(ctx, q, row)
	})
	if err != nil {
		return err
	}
	s.publish(KindCompetition, competitionTopic(code))
	return nil
}

// SetParticipants replaces the pilots (solo) or teams (synchro). Pilots are
// stored by CIVL rank descending, unranked first (spec 02 §2.5.2); teams
// keep the given order.
func (s *Service) SetParticipants(ctx context.Context, id string, pilots []int, teams []string) error {
	return s.update(ctx, id, func(q *gen.Queries, c *Competition) (func() error, error) {
		var errs []domain.FieldError
		add := func(f, m string) { errs = append(errs, domain.FieldError{Field: f, Msg: m}) }
		if c.Type == domain.TypeSolo {
			if len(teams) > 0 {
				add("teams", "a solo competition has pilots, not teams")
			}
			ranks := map[int]int64{}
			for _, p := range pilots {
				r, err := q.GetPilot(ctx, int64(p))
				if err != nil {
					add("pilots", fmt.Sprintf("pilot %d is unknown", p))
					continue
				}
				ranks[p] = r.Rank
			}
			pilots = slices.Clone(pilots)
			slices.SortStableFunc(pilots, func(a, b int) int { return int(ranks[b] - ranks[a]) })
			c.Pilots = pilots
		} else {
			if len(pilots) > 0 {
				add("pilots", "a synchro competition has teams, not pilots")
			}
			for _, t := range teams {
				if _, _, err := loadTeam(ctx, q, t, false); err != nil {
					add("teams", fmt.Sprintf("team %s is unknown", t))
				}
			}
			c.Teams = teams
		}
		if err := invalid(errs...); err != nil {
			return nil, err
		}
		return func() error {
			if err := q.DeleteCompetitionPilots(ctx, id); err != nil {
				return err
			}
			if err := q.DeleteCompetitionTeams(ctx, id); err != nil {
				return err
			}
			for i, p := range c.Pilots {
				if err := q.AddCompetitionPilot(ctx, gen.AddCompetitionPilotParams{CompetitionID: id, Position: int64(i), Civlid: int64(p)}); err != nil {
					return fmt.Errorf("add pilot %d: %w", p, err)
				}
			}
			for i, t := range c.Teams {
				if err := q.AddCompetitionTeam(ctx, gen.AddCompetitionTeamParams{CompetitionID: id, Position: int64(i), TeamID: t}); err != nil {
					return fmt.Errorf("add team %s: %w", t, err)
				}
			}
			return nil
		}, nil
	})
}

// SetJudges replaces the competition judges.
func (s *Service) SetJudges(ctx context.Context, id string, judges []string) error {
	return s.update(ctx, id, func(q *gen.Queries, c *Competition) (func() error, error) {
		var errs []domain.FieldError
		seen := map[string]bool{}
		for _, j := range judges {
			if seen[j] {
				errs = append(errs, domain.FieldError{Field: "judges", Msg: "duplicate judges"})
			}
			seen[j] = true
			if _, err := loadJudge(ctx, q, j, false); err != nil {
				errs = append(errs, domain.FieldError{Field: "judges", Msg: fmt.Sprintf("judge %s is unknown", j)})
			}
		}
		if err := invalid(errs...); err != nil {
			return nil, err
		}
		c.Judges = judges
		return func() error {
			if err := q.DeleteCompetitionJudges(ctx, id); err != nil {
				return err
			}
			for i, j := range judges {
				if err := q.AddCompetitionJudge(ctx, gen.AddCompetitionJudgeParams{CompetitionID: id, Position: int64(i), JudgeID: j}); err != nil {
					return fmt.Errorf("add judge %s: %w", j, err)
				}
			}
			return nil
		}, nil
	})
}

func setRepeatable(ctx context.Context, q *gen.Queries, id string, names []string) error {
	if err := q.DeleteCompetitionRepeatableTricks(ctx, id); err != nil {
		return fmt.Errorf("clear repeatable tricks of %s: %w", id, err)
	}
	for i, n := range names {
		if err := q.AddCompetitionRepeatableTrick(ctx, gen.AddCompetitionRepeatableTrickParams{CompetitionID: id, Position: int64(i), TrickName: n}); err != nil {
			return fmt.Errorf("add repeatable trick %q: %w", n, err)
		}
	}
	return nil
}

// SetRepeatableTricks replaces the base tricks exempt from the repetition
// malus; they must be active tricks of the competition year's catalog.
func (s *Service) SetRepeatableTricks(ctx context.Context, id string, names []string) error {
	return s.update(ctx, id, func(q *gen.Queries, c *Competition) (func() error, error) {
		var errs []domain.FieldError
		seen := map[string]bool{}
		for _, n := range names {
			if seen[n] {
				errs = append(errs, domain.FieldError{Field: "repeatable_tricks", Msg: fmt.Sprintf("duplicate trick %q", n)})
			}
			seen[n] = true
			_, err := q.GetActiveTrickByYearName(ctx, gen.GetActiveTrickByYearNameParams{Year: int64(c.Year()), Name: n})
			if err != nil {
				errs = append(errs, domain.FieldError{Field: "repeatable_tricks", Msg: fmt.Sprintf("trick %q is not in the %d catalog", n, c.Year())})
			}
		}
		if err := invalid(errs...); err != nil {
			return nil, err
		}
		return func() error { return setRepeatable(ctx, q, id, names) }, nil
	})
}

// SetConfig replaces the competition scoring config (spec 02 §2.6). It is
// copied into each new run.
func (s *Service) SetConfig(ctx context.Context, id string, config json.RawMessage) error {
	dec := json.NewDecoder(bytes.NewReader(config))
	dec.DisallowUnknownFields()
	var cfg scoring.Config
	if err := dec.Decode(&cfg); err != nil || !bytes.HasPrefix(bytes.TrimSpace(config), []byte("{")) {
		return invalid(domain.FieldError{Field: "config", Msg: fmt.Sprintf("invalid config: %v", err)})
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, config); err != nil {
		return invalid(domain.FieldError{Field: "config", Msg: err.Error()})
	}
	return s.db.Tx(ctx, func(q *gen.Queries) error {
		c, row, err := editable(ctx, q, id)
		if err != nil {
			return err
		}
		if err := domain.CanUpdateCompetition(c.State, false); err != nil {
			return err
		}
		row.Config, row.UpdatedAt = compact.String(), nullString(now())
		return upsertCompetition(ctx, q, row)
	})
}

// transition moves an editable competition from state from to state to
// after check passes,
// then publishes to the competition and its seasons (the season rankings
// count closed competitions only).
func (s *Service) transition(ctx context.Context, id string, from, to domain.CompetitionState, check func(q *gen.Queries, c Competition) error) error {
	var c Competition
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		var row gen.Competition
		var err error
		c, row, err = editable(ctx, q, id)
		if err != nil {
			return err
		}
		if c.State != from {
			return fmt.Errorf("competition %s -> %s needs state %s: %w", c.State, to, from, domain.ErrInvalidTransition)
		}
		if err := domain.CanTransition(c.State, to); err != nil {
			return err
		}
		if err := check(q, c); err != nil {
			return err
		}
		row.State, row.UpdatedAt = string(to), nullString(now())
		return upsertCompetition(ctx, q, row)
	})
	if err != nil {
		return err
	}
	s.publish(KindCompetition, c.topics()...)
	return nil
}

// OpenCompetition opens an init competition: it needs at least 2 pilots or
// teams and 2 judges.
func (s *Service) OpenCompetition(ctx context.Context, id string) error {
	return s.transition(ctx, id, domain.CompetitionInit, domain.CompetitionOpen, func(_ *gen.Queries, c Competition) error {
		c.State = domain.CompetitionOpen
		return invalid(domain.ValidateCompetition(c.domain())...)
	})
}

// CloseCompetition closes an open competition whose runs are all closed.
func (s *Service) CloseCompetition(ctx context.Context, id string) error {
	return s.transition(ctx, id, domain.CompetitionOpen, domain.CompetitionClosed, func(q *gen.Queries, c Competition) error {
		runs, err := q.ListRuns(ctx, id)
		if err != nil {
			return fmt.Errorf("list runs of %s: %w", id, err)
		}
		states := make([]domain.RunState, len(runs))
		for i, r := range runs {
			states[i] = domain.RunState(r.State)
		}
		if err := domain.CloseCompetitionCheck(states); err != nil {
			return refused(err)
		}
		return nil
	})
}

// ReopenCompetition reopens a closed competition.
func (s *Service) ReopenCompetition(ctx context.Context, id string) error {
	return s.transition(ctx, id, domain.CompetitionClosed, domain.CompetitionOpen, func(*gen.Queries, Competition) error { return nil })
}

// DeleteCompetition soft-deletes a competition.
func (s *Service) DeleteCompetition(ctx context.Context, id string) error {
	return s.setCompetitionDeleted(ctx, id, true)
}

// RestoreCompetition restores a soft-deleted competition in its previous
// state (spec 02 §5.1).
func (s *Service) RestoreCompetition(ctx context.Context, id string) error {
	return s.setCompetitionDeleted(ctx, id, false)
}

func (s *Service) setCompetitionDeleted(ctx context.Context, id string, deleted bool) error {
	var c Competition
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		var row gen.Competition
		var err error
		c, row, err = loadCompetition(ctx, q, id, true)
		if err != nil {
			return err
		}
		if row.Frozen {
			return fmt.Errorf("competition %s: %w", row.Code, ErrFrozen)
		}
		if row.DeletedAt.Valid == deleted {
			return fmt.Errorf("competition %s: %w", id, ErrNotFound)
		}
		row.DeletedAt, row.UpdatedAt = deletedAt(deleted), nullString(now())
		return upsertCompetition(ctx, q, row)
	})
	if err != nil {
		return err
	}
	s.publish(KindCompetition, c.topics()...)
	return nil
}
