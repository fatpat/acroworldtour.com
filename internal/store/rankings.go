package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"

	"awt/internal/ranking"
	"awt/internal/store/gen"
)

// Scopes of ranking_entries.
const (
	ScopeRun         = "run"
	ScopeCompetition = "competition"
	ScopeSeason      = "season"
)

// RunScopeKey is the scope key of a run's rankings: "<competition id>/<run index>".
func RunScopeKey(competitionID string, runIdx int) string {
	return fmt.Sprintf("%s/%d", competitionID, runIdx)
}

// SaveRankings persists ranking lists in ranking_entries, replacing whatever
// was stored for (scope, key). scope is ScopeRun, ScopeCompetition or
// ScopeSeason; key is RunScopeKey, the competition id or the season code.
// Each list is stored under its name ("overall", a season code, "women") with
// its 1-based position and shared rank; competition entries keep their
// per-run cells in details. An empty list stores no row (readers treat a
// missing list as empty). It is the single writer of ranking_entries: the
// importer and the live scoring service both use it.
func SaveRankings(ctx context.Context, q *gen.Queries, scope, key string, lists ranking.Lists) error {
	if err := q.DeleteRankingScope(ctx, gen.DeleteRankingScopeParams{Scope: scope, ScopeKey: key}); err != nil {
		return fmt.Errorf("clear rankings %s/%s: %w", scope, key, err)
	}
	names := make([]string, 0, len(lists))
	for name := range lists {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		for _, e := range lists[name] {
			details, err := entryDetails(e)
			if err != nil {
				return err
			}
			p := gen.InsertRankingEntryParams{
				Scope: scope, ScopeKey: key, ListKey: name, Position: int64(e.Position), Rank: int64(e.Rank),
				Score: e.Score, Details: details,
			}
			if e.Participant.TeamID != "" {
				p.TeamID = sql.NullString{String: e.Participant.TeamID, Valid: true}
			} else {
				p.Civlid = sql.NullInt64{Int64: int64(e.Participant.CivlID), Valid: true}
			}
			if err := q.InsertRankingEntry(ctx, p); err != nil {
				return fmt.Errorf("save ranking %s/%s/%s position %d: %w", scope, key, name, e.Position, err)
			}
		}
	}
	return nil
}

type cellJSON struct {
	Run   int     `json:"run"`
	Rank  int     `json:"rank"`
	Score float64 `json:"score"`
}

func entryDetails(e ranking.Entry) (string, error) {
	if len(e.PerRun) == 0 {
		return "{}", nil
	}
	cells := make([]cellJSON, len(e.PerRun))
	for i, c := range e.PerRun {
		cells[i] = cellJSON{Run: c.RunIndex, Rank: c.Rank, Score: c.Score}
	}
	b, err := json.Marshal(map[string]any{"per_run": cells})
	if err != nil {
		return "", fmt.Errorf("encode ranking details: %w", err)
	}
	return string(b), nil
}
