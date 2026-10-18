package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"awt/internal/domain"
)

func TestSetRunJudgesAndConfig(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, []string{"awt-2025"})
	ctx := context.Background()
	id := e.comp.ID

	ok(t, e.svc.SetRunJudges(ctx, id, 0, e.judges[:1]))
	if r := must(e.svc.GetRun(ctx, id, 0))(t); len(r.Judges) != 1 || r.Judges[0] != e.judges[0] {
		t.Fatalf("run judges = %v", r.Judges)
	}
	var ve *ValidationError
	if err := e.svc.SetRunJudges(ctx, id, 0, []string{"nobody"}); !errors.As(err, &ve) {
		t.Fatalf("unknown judge: err = %v, want a validation error", err)
	}
	if err := e.svc.SetRunJudges(ctx, id, 0, []string{e.judges[0], e.judges[0]}); !errors.As(err, &ve) {
		t.Fatalf("duplicate judge: err = %v", err)
	}
	if err := e.svc.SetRunJudges(ctx, id, 5, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing run: err = %v", err)
	}

	cfg := string(must(e.svc.GetRun(ctx, id, 0))(t).Config)
	if err := e.svc.SetRunConfig(ctx, id, 0, json.RawMessage(`{"warning":1}`)); !errors.As(err, &ve) {
		t.Fatalf("incomplete config: err = %v", err)
	}
	changed := strings.Replace(cfg, `"warning2":1`, `"warning2":2`, 1)
	if changed == cfg {
		t.Fatalf("config has no warning2 of 1: %s", cfg)
	}
	ok(t, e.svc.SetRunConfig(ctx, id, 0, json.RawMessage(changed)))
	if got := string(must(e.svc.GetRun(ctx, id, 0))(t).Config); !strings.Contains(got, `"warning2":2`) {
		t.Fatalf("run config not saved: %s", got)
	}
	if c := must(e.svc.GetCompetition(ctx, id))(t); strings.Contains(string(c.Config), `"warning2":2`) {
		t.Fatal("the competition config changed with the run config")
	}

	// A closed run refuses both until reopened.
	ok(t, e.svc.SetRunJudges(ctx, id, 0, e.judges))
	e.save(t, 0, 201, e.input(7, "LM", "RMC", "LS"), true)
	e.save(t, 0, 202, e.input(8, "LM", "RMC", "LS"), true)
	ok(t, e.svc.CloseRun(ctx, id, 0))
	if err := e.svc.SetRunJudges(ctx, id, 0, e.judges); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("closed run judges: err = %v", err)
	}
	if err := e.svc.SetRunConfig(ctx, id, 0, json.RawMessage(cfg)); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("closed run config: err = %v", err)
	}
	ok(t, e.svc.ReopenRun(ctx, id, 0))
	ok(t, e.svc.SetRunJudges(ctx, id, 0, e.judges))

	// A frozen competition refuses them.
	_, err := e.db.Write.Exec(`UPDATE competitions SET frozen = 1 WHERE id = ?`, id)
	ok(t, err)
	if err := e.svc.SetRunJudges(ctx, id, 0, e.judges); !errors.Is(err, ErrFrozen) {
		t.Fatalf("frozen: err = %v", err)
	}
	if err := e.svc.SetRunConfig(ctx, id, 0, json.RawMessage(cfg)); !errors.Is(err, ErrFrozen) {
		t.Fatalf("frozen config: err = %v", err)
	}
}
