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

// FlightStatus is the judging navigator's view of a saved flight.
type FlightStatus struct {
	Published   bool
	DidNotStart bool
	Score       float64
}

// StoredFlight is a saved flight as the judging screen edits it: the input
// it was scored from, its published flag and its final mark. Per-trick
// marks are stored after the engine's fill-in (spec 03 §5.0 step 4), so a
// guessed mark comes back as a value.
type StoredFlight struct {
	Input     FlightInput
	Published bool
	Final     scoring.FinalMark
}

// RunFlightStatuses returns the saved flights of run idx keyed by
// participant (CIVL id for solo, team id for synchro).
func (s *Service) RunFlightStatuses(ctx context.Context, compID string, idx int) (map[string]FlightStatus, error) {
	rows, err := s.read().ListFlightsOfRun(ctx, gen.ListFlightsOfRunParams{CompetitionID: compID, RunIdx: int64(idx)})
	if err != nil {
		return nil, fmt.Errorf("list flights of run %d: %w", idx+1, err)
	}
	out := make(map[string]FlightStatus, len(rows))
	for _, f := range rows {
		out[participantID(f.Civlid, f.TeamID)] = FlightStatus{Published: f.Published, DidNotStart: f.DidNotStart, Score: f.Score.Float64}
	}
	return out, nil
}

// GetFlight returns the participant's saved flight of run idx; ok is false
// when there is none. It reads one consistent snapshot.
func (s *Service) GetFlight(ctx context.Context, compID string, idx int, participant string) (StoredFlight, bool, error) {
	tx, err := s.db.Read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return StoredFlight{}, false, fmt.Errorf("begin read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := gen.New(tx)
	c, _, err := loadCompetition(ctx, q, compID, false)
	if err != nil {
		return StoredFlight{}, false, err
	}
	p := ranking.Participant{TeamID: participant}
	if c.Type == domain.TypeSolo {
		n, err := strconv.Atoi(participant)
		if err != nil {
			return StoredFlight{}, false, nil
		}
		p = ranking.Participant{CivlID: n}
	}
	row, ok, err := storedFlight(ctx, q, c.ID, idx, p)
	if err != nil || !ok {
		return StoredFlight{}, false, err
	}
	f, err := flightFromStore(ctx, q, row)
	if err != nil {
		return StoredFlight{}, false, err
	}
	out := StoredFlight{Published: row.Published, Input: FlightInput{
		DidNotStart: f.DidNotStart, TipTouches: f.TipTouches, Warnings1: f.Warnings1, Warnings2: f.Warnings2,
	}}
	for _, t := range f.Tricks {
		out.Input.Tricks = append(out.Input.Tricks, t.Acronym)
	}
	for _, m := range f.Marks {
		out.Input.Marks = append(out.Input.Marks, MarkInput{
			Judge: m.Judge, Technical: m.Technical, Choreography: m.Choreography, Landing: m.Landing,
			Synchro: m.Synchro, TechnicalPerTrick: m.TechnicalPerTrick,
		})
	}
	if row.FinalMark.Valid {
		if err := json.Unmarshal([]byte(row.FinalMark.String), &out.Final); err != nil {
			return StoredFlight{}, false, fmt.Errorf("decode final mark of flight %d: %w", row.ID, err)
		}
	}
	return out, true, nil
}
