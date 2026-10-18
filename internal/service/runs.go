package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"awt/internal/domain"
	"awt/internal/ranking"
	"awt/internal/scoring"
	"awt/internal/store/gen"
)

// Run is a run of a competition, addressed by its 0-based index. Pilots
// (solo) or Teams (synchro) are in starting order.
type Run struct {
	CompetitionID string
	Index         int
	State         domain.RunState
	Config        json.RawMessage // copied from the competition at creation
	ResetPolicy   scoring.ResetPolicy
	ResultsFinal  bool
	Pilots        []int
	Teams         []string
	Judges        []string
	UpdatedAt     string
}

func loadRun(ctx context.Context, q *gen.Queries, compID string, idx int) (Run, gen.Run, error) {
	r, err := q.GetRun(ctx, gen.GetRunParams{CompetitionID: compID, Idx: int64(idx)})
	if err != nil {
		return Run{}, r, notFound(err, fmt.Sprintf("run %d of competition %s", idx+1, compID))
	}
	out := Run{
		CompetitionID: r.CompetitionID, Index: int(r.Idx), State: domain.RunState(r.State), Config: json.RawMessage(r.Config),
		ResetPolicy: scoring.ResetPolicy(r.ResetPolicy), ResultsFinal: r.ResultsFinal, UpdatedAt: r.UpdatedAt.String,
		Pilots: []int{}, Teams: []string{},
	}
	parts, err := q.ListRunParticipants(ctx, gen.ListRunParticipantsParams{CompetitionID: compID, RunIdx: r.Idx})
	if err != nil {
		return out, r, fmt.Errorf("list participants of run %d: %w", idx+1, err)
	}
	for _, p := range parts {
		if p.Civlid.Valid {
			out.Pilots = append(out.Pilots, int(p.Civlid.Int64))
		} else {
			out.Teams = append(out.Teams, p.TeamID.String)
		}
	}
	judges, err := q.ListRunJudges(ctx, gen.ListRunJudgesParams{CompetitionID: compID, RunIdx: r.Idx})
	if err != nil {
		return out, r, fmt.Errorf("list judges of run %d: %w", idx+1, err)
	}
	out.Judges = append([]string{}, judges...)
	return out, r, nil
}

// GetRun returns run idx (0-based) of an active competition.
func (s *Service) GetRun(ctx context.Context, compID string, idx int) (Run, error) {
	q := s.read()
	if _, _, err := loadCompetition(ctx, q, compID, false); err != nil {
		return Run{}, err
	}
	r, _, err := loadRun(ctx, q, compID, idx)
	return r, err
}

// ListRuns returns the runs of an active competition in order.
func (s *Service) ListRuns(ctx context.Context, compID string) ([]Run, error) {
	q := s.read()
	if _, _, err := loadCompetition(ctx, q, compID, false); err != nil {
		return nil, err
	}
	rows, err := q.ListRuns(ctx, compID)
	if err != nil {
		return nil, fmt.Errorf("list runs of %s: %w", compID, err)
	}
	out := make([]Run, 0, len(rows))
	for _, r := range rows {
		run, _, err := loadRun(ctx, q, compID, int(r.Idx))
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, nil
}

func toParticipants(pilots []int, teams []string) []ranking.Participant {
	out := make([]ranking.Participant, 0, len(pilots)+len(teams))
	for _, p := range pilots {
		out = append(out, ranking.Participant{CivlID: p})
	}
	for _, t := range teams {
		out = append(out, ranking.Participant{TeamID: t})
	}
	return out
}

// storedCompetitionRanking reads the competition's current stored ranking
// lists (overall, season codes, women). NextStartOrder only uses list order
// and participant identity.
func storedCompetitionRanking(ctx context.Context, q *gen.Queries, compID string) (ranking.CompetitionResult, error) {
	rows, err := q.ListCompetitionRankingEntries(ctx, compID)
	if err != nil {
		return ranking.CompetitionResult{}, fmt.Errorf("read ranking of %s: %w", compID, err)
	}
	res := ranking.CompetitionResult{Lists: ranking.Lists{}}
	for _, r := range rows {
		res.Lists[r.ListKey] = append(res.Lists[r.ListKey], ranking.Entry{
			Participant: ranking.Participant{CivlID: int(r.Civlid.Int64), TeamID: r.TeamID.String},
			Score:       r.Score, Position: int(r.Position), Rank: int(r.Rank),
		})
	}
	return res, nil
}

func writeRunParticipants(ctx context.Context, q *gen.Queries, compID string, idx int64, order []ranking.Participant) error {
	if err := q.DeleteRunParticipants(ctx, gen.DeleteRunParticipantsParams{CompetitionID: compID, RunIdx: idx}); err != nil {
		return fmt.Errorf("clear participants of run %d: %w", idx+1, err)
	}
	for i, p := range order {
		err := q.AddRunParticipant(ctx, gen.AddRunParticipantParams{
			CompetitionID: compID, RunIdx: idx, Position: int64(i),
			Civlid: sql.NullInt64{Int64: int64(p.CivlID), Valid: p.TeamID == ""}, TeamID: nullString(p.TeamID),
		})
		if err != nil {
			return fmt.Errorf("add participant to run %d: %w", idx+1, err)
		}
	}
	return nil
}

// NewRun creates the next run of an open competition (spec 02 §5.2). Run 1
// starts in the competition order; later runs follow ranking.NextStartOrder
// on the previous run's list and the current stored competition ranking.
// The previous run is left untouched (decision 0006 #2). With qualify > 0
// only the best qualify participants are kept. The competition config and
// judges are copied into the run.
func (s *Service) NewRun(ctx context.Context, compID string, qualify int, policy scoring.ResetPolicy) (Run, error) {
	var errs []domain.FieldError
	if qualify < 0 {
		errs = append(errs, domain.FieldError{Field: "qualify", Msg: "must be >= 0"})
	}
	if policy < scoring.ResetNone || policy > scoring.ResetAll {
		errs = append(errs, domain.FieldError{Field: "reset_policy", Msg: "must be 0 (none), 1 (awt), 2 (awq) or 3 (all)"})
	}
	if err := invalid(errs...); err != nil {
		return Run{}, err
	}
	var out Run
	var c Competition
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		var err error
		c, _, err = editable(ctx, q, compID)
		if err != nil {
			return err
		}
		if err := domain.CanCreateRun(c.State); err != nil {
			return err
		}
		runs, err := q.ListRuns(ctx, compID)
		if err != nil {
			return fmt.Errorf("list runs of %s: %w", compID, err)
		}
		idx := len(runs)
		prev := toParticipants(c.Pilots, c.Teams)
		standing := ranking.CompetitionResult{}
		if idx > 0 {
			p, _, err := loadRun(ctx, q, compID, idx-1)
			if err != nil {
				return err
			}
			prev = toParticipants(p.Pilots, p.Teams)
			if standing, err = storedCompetitionRanking(ctx, q, compID); err != nil {
				return err
			}
		}
		order := ranking.NextStartOrder(standing, prev, c.Seasons, qualify)

		err = q.UpsertRun(ctx, gen.UpsertRunParams{
			CompetitionID: compID, Idx: int64(idx), State: string(domain.RunInit), Config: string(c.Config),
			ResetPolicy: int64(policy), UpdatedAt: nullString(now()),
		})
		if err != nil {
			return fmt.Errorf("create run %d: %w", idx+1, err)
		}
		if err := writeRunParticipants(ctx, q, compID, int64(idx), order); err != nil {
			return err
		}
		for i, j := range c.Judges {
			err := q.AddRunJudge(ctx, gen.AddRunJudgeParams{CompetitionID: compID, RunIdx: int64(idx), Position: int64(i), JudgeID: j})
			if err != nil {
				return fmt.Errorf("add judge to run %d: %w", idx+1, err)
			}
		}
		out, _, err = loadRun(ctx, q, compID, idx)
		return err
	})
	if err != nil {
		return Run{}, err
	}
	s.publish(KindRun, competitionTopic(c.Code))
	return out, nil
}

// SetRunOrder replaces a run's starting list. Participants must be
// competition participants, without duplicates. A closed run must be
// reopened first (spec 05 §8: the backend enforces the run state on roster
// edits).
func (s *Service) SetRunOrder(ctx context.Context, compID string, idx int, pilots []int, teams []string) error {
	return s.runOp(ctx, compID, idx, false, func(q *gen.Queries, c Competition, r Run) (domain.RunState, error) {
		if r.State == domain.RunClosed {
			return r.State, fmt.Errorf("run %d is closed, reopen it first: %w", idx+1, domain.ErrInvalidTransition)
		}
		var errs []domain.FieldError
		add := func(f, m string) { errs = append(errs, domain.FieldError{Field: f, Msg: m}) }
		inComp := map[string]bool{}
		for _, p := range toParticipants(c.Pilots, c.Teams) {
			inComp[participantKey(p)] = true
		}
		if c.Type == domain.TypeSolo && len(teams) > 0 {
			add("teams", "a solo run has pilots, not teams")
		}
		if c.Type == domain.TypeSynchro && len(pilots) > 0 {
			add("pilots", "a synchro run has teams, not pilots")
		}
		order := toParticipants(pilots, teams)
		seen := map[string]bool{}
		for _, p := range order {
			k := participantKey(p)
			if seen[k] {
				add("participants", fmt.Sprintf("%s is listed twice", k))
			}
			seen[k] = true
			if !inComp[k] {
				add("participants", fmt.Sprintf("%s is not a participant of the competition", k))
			}
		}
		if err := invalid(errs...); err != nil {
			return r.State, err
		}
		return r.State, writeRunParticipants(ctx, q, compID, int64(idx), order)
	})
}

func participantKey(p ranking.Participant) string {
	if p.TeamID != "" {
		return "team " + p.TeamID
	}
	return "pilot " + strconv.Itoa(p.CivlID)
}

// runOp loads an editable competition and one of its runs, applies fn and
// stores the run state fn returns, then publishes a run event. With
// rebuild, the stored rankings are rebuilt in the same transaction (a run
// state change changes the results_final flags) and the event also goes to
// the competition's seasons.
func (s *Service) runOp(ctx context.Context, compID string, idx int, rebuild bool, fn func(q *gen.Queries, c Competition, r Run) (domain.RunState, error)) error {
	var c Competition
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		var err error
		c, _, err = editable(ctx, q, compID)
		if err != nil {
			return err
		}
		r, row, err := loadRun(ctx, q, compID, idx)
		if err != nil {
			return err
		}
		state, err := fn(q, c, r)
		if err != nil {
			return err
		}
		err = q.UpsertRun(ctx, gen.UpsertRunParams{
			CompetitionID: compID, Idx: row.Idx, State: string(state), Config: row.Config,
			ResetPolicy: row.ResetPolicy, ResultsFinal: row.ResultsFinal, UpdatedAt: nullString(now()),
		})
		if err != nil || !rebuild {
			return err
		}
		return RebuildRankings(ctx, q, compID)
	})
	if err != nil {
		return err
	}
	if rebuild {
		s.publish(KindRun, c.topics()...)
	} else {
		s.publish(KindRun, competitionTopic(c.Code))
	}
	return nil
}

func (s *Service) runTransition(ctx context.Context, compID string, idx int, from, to domain.RunState, check func(q *gen.Queries, r Run) error) error {
	return s.runOp(ctx, compID, idx, true, func(q *gen.Queries, _ Competition, r Run) (domain.RunState, error) {
		if r.State != from {
			return r.State, fmt.Errorf("run %d %s -> %s needs state %s: %w", idx+1, r.State, to, from, domain.ErrInvalidTransition)
		}
		if err := domain.RunTransition(r.State, to); err != nil {
			return r.State, fmt.Errorf("run %d: %w", idx+1, err)
		}
		if check != nil {
			if err := check(q, r); err != nil {
				return r.State, err
			}
		}
		return to, nil
	})
}

// OpenRun opens an init run.
func (s *Service) OpenRun(ctx context.Context, compID string, idx int) error {
	return s.runTransition(ctx, compID, idx, domain.RunInit, domain.RunOpen, nil)
}

// CloseRun closes an open run when every participant has a published
// flight (spec 02 §5.2); the error lists the others by name.
func (s *Service) CloseRun(ctx context.Context, compID string, idx int) error {
	return s.runTransition(ctx, compID, idx, domain.RunOpen, domain.RunClosed, func(q *gen.Queries, r Run) error {
		var parts []domain.Participant
		for _, p := range r.Pilots {
			name := ""
			if row, err := q.GetPilot(ctx, int64(p)); err == nil {
				name = row.Name
			}
			parts = append(parts, domain.Participant{ID: strconv.Itoa(p), Name: name})
		}
		for _, t := range r.Teams {
			name := ""
			if row, err := q.GetTeam(ctx, t); err == nil {
				name = row.Name
			}
			parts = append(parts, domain.Participant{ID: t, Name: name})
		}
		flights, err := q.ListFlightsOfRun(ctx, gen.ListFlightsOfRunParams{CompetitionID: compID, RunIdx: int64(idx)})
		if err != nil {
			return fmt.Errorf("list flights of run %d: %w", idx+1, err)
		}
		refs := make([]domain.FlightRef, 0, len(flights))
		for _, f := range flights {
			refs = append(refs, domain.FlightRef{ParticipantID: participantID(f.Civlid, f.TeamID), Published: f.Published})
		}
		if err := domain.CloseRunCheck(parts, refs); err != nil {
			return refused(fmt.Errorf("run %d: %w", idx+1, err))
		}
		return nil
	})
}

// ReopenRun reopens a closed run.
func (s *Service) ReopenRun(ctx context.Context, compID string, idx int) error {
	return s.runTransition(ctx, compID, idx, domain.RunClosed, domain.RunOpen, nil)
}

// participantID is the participant key used in state checks: the CIVL id
// of a pilot or the id of a team.
func participantID(civlid sql.NullInt64, team sql.NullString) string {
	if civlid.Valid {
		return strconv.FormatInt(civlid.Int64, 10)
	}
	return team.String
}
