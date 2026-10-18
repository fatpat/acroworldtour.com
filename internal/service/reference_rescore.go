package service

import (
	"context"
	"fmt"
	"slices"

	"awt/internal/domain"
	"awt/internal/store/gen"
)

// refreshCompetitions rescores every stored flight of the given competitions
// and rebuilds their rankings (and their seasons' when closed) inside the
// caller's transaction. Reference-data edits that change an input of stored
// scores or ranking lists (a pilot's AWT status or gender, a judge's level,
// a competition's repeatable tricks) call it (compute on write, final
// review ruling). The ids must be active, non-frozen competitions: imported
// results are never rescored. It returns the topics to publish after
// commit; an engine refusal is an ErrRescore and rolls everything back.
func refreshCompetitions(ctx context.Context, q *gen.Queries, ids []string) ([]string, error) {
	var topics []string
	for _, id := range ids {
		c, _, err := loadCompetition(ctx, q, id, false)
		if err != nil {
			return nil, err
		}
		if err := rescoreCompetition(ctx, q, c); err != nil {
			return nil, fmt.Errorf("competition %s: %w", c.Code, err)
		}
		if err := rebuildRankings(ctx, q, c.ID, c.State == domain.CompetitionClosed); err != nil {
			return nil, err
		}
		for _, t := range c.topics() {
			if !slices.Contains(topics, t) {
				topics = append(topics, t)
			}
		}
	}
	return topics, nil
}

// pilotCompetitions lists the active, non-frozen competitions a pilot takes
// part in or flew in; year > 0 keeps those starting that year only (the
// AWT status is per competition year, spec 03 §7.1).
func pilotCompetitions(ctx context.Context, q *gen.Queries, civlid, year int) ([]string, error) {
	rows, err := q.ListRescorableCompetitionsOfPilot(ctx, int64(civlid))
	if err != nil {
		return nil, fmt.Errorf("list competitions of pilot %d: %w", civlid, err)
	}
	var ids []string
	for _, r := range rows {
		if year > 0 {
			d, err := parseDate(r.StartDate)
			if err != nil {
				return nil, fmt.Errorf("competition %s start date: %w", r.ID, err)
			}
			if d.Year() != year {
				continue
			}
		}
		ids = append(ids, r.ID)
	}
	return ids, nil
}
