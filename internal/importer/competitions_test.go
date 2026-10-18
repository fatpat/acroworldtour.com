package importer_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"awt/internal/importer"
	"awt/internal/store"
)

const (
	soloID    = "eeeeeeeeeeeeeeeeeeeeeee1"
	synchroID = "eeeeeeeeeeeeeeeeeeeeeee2"
)

func importMini(t *testing.T) (*store.DB, importer.Report) {
	t.Helper()
	db, blobs := setup(t)
	rep, err := importer.Run(context.Background(), db, blobs, miniDir, importer.WithCatalogDir(catalogDir))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return db, rep
}

type entryRow struct {
	pos, rank int
	civlid    sql.NullInt64
	teamID    sql.NullString
	score     float64
}

func entries(t *testing.T, db *store.DB, scope, key, list string) []entryRow {
	t.Helper()
	rows, err := db.Read.Query(`SELECT position, rank, civlid, team_id, score FROM ranking_entries
		WHERE scope = ? AND scope_key = ? AND list_key = ? ORDER BY position`, scope, key, list)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []entryRow
	for rows.Next() {
		var e entryRow
		if err := rows.Scan(&e.pos, &e.rank, &e.civlid, &e.teamID, &e.score); err != nil {
			t.Fatal(err)
		}
		out = append(out, e)
	}
	return out
}

func TestImportCompetitions(t *testing.T) {
	db, rep := importMini(t)

	want := map[string]int{
		"competitions": 2, "runs": 3, "flights": 6, "flight_tricks": 3, "flight_marks": 5, "flight_warnings": 2,
		"placeholder_pilots": 1,
	}
	for k, v := range want {
		if rep.Counts[k] != v {
			t.Errorf("Counts[%s] = %d, want %d", k, rep.Counts[k], v)
		}
	}
	if !hasWarning(rep, "77777") {
		t.Errorf("no placeholder warning: %v", rep.Warnings)
	}

	// competition: frozen, links in stored order, config completed from the ruleset (warning2).
	var frozen, final bool
	var logo, website, cfg, start string
	if err := db.Read.QueryRow(`SELECT frozen, results_final, logo_file, website, config, start_date FROM competitions WHERE id = ?`, soloID).
		Scan(&frozen, &final, &logo, &website, &cfg, &start); err != nil {
		t.Fatal(err)
	}
	if !frozen || !final || logo != helloID || website != "https://example.org/mini" || start != "2024-08-15" {
		t.Errorf("competition = frozen %v final %v logo %q website %q start %q", frozen, final, logo, website, start)
	}
	var c map[string]any
	if err := json.Unmarshal([]byte(cfg), &c); err != nil || c["warning2"] != float64(1) || c["warning"] != 0.5 {
		t.Errorf("config = %s (%v)", cfg, err)
	}
	var pilots []int
	rows, err := db.Read.Query(`SELECT civlid FROM competition_pilots WHERE competition_id = ? ORDER BY position`, soloID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var p int
		_ = rows.Scan(&p)
		pilots = append(pilots, p)
	}
	rows.Close()
	if len(pilots) != 3 || pilots[0] != 1002 || pilots[1] != 1001 || pilots[2] != 77777 {
		t.Errorf("competition pilots = %v", pilots)
	}
	var repeatable string
	if err := db.Read.QueryRow(`SELECT trick_name FROM competition_repeatable_tricks WHERE competition_id = ?`, soloID).Scan(&repeatable); err != nil || repeatable != "Misty Flip" {
		t.Errorf("repeatable trick = %q err=%v", repeatable, err)
	}

	// placeholder pilot
	var pname, country, gender string
	var rank int
	if err := db.Read.QueryRow(`SELECT name, country, rank, gender FROM pilots WHERE civlid = 77777`).Scan(&pname, &country, &rank, &gender); err != nil ||
		pname != "Pilot #77777" || country != "" || rank != 9999 || gender != "none" {
		t.Errorf("placeholder = %q %q %d %q err=%v", pname, country, rank, gender, err)
	}

	// runs: reset policy, participants in stored order, judges
	var policy int
	if err := db.Read.QueryRow(`SELECT reset_policy FROM runs WHERE competition_id = ? AND idx = 1`, soloID).Scan(&policy); err != nil || policy != 2 {
		t.Errorf("run 1 reset policy = %d err=%v", policy, err)
	}
	if n := count(t, db, `SELECT reset_policy FROM runs WHERE competition_id = ? AND idx = 0`, soloID); n != 0 {
		t.Errorf("absent reset policy = %d", n)
	}
	var first, third int
	if err := db.Read.QueryRow(`SELECT
		(SELECT civlid FROM run_participants WHERE competition_id = ? AND run_idx = 0 AND position = 0),
		(SELECT civlid FROM run_participants WHERE competition_id = ? AND run_idx = 0 AND position = 2)`, soloID, soloID).Scan(&first, &third); err != nil || first != 1002 || third != 77777 {
		t.Errorf("run participants = %d, %d err=%v", first, third, err)
	}
	if n := count(t, db, `SELECT count(*) FROM run_judges WHERE competition_id = ? AND run_idx = 0`, soloID); n != 2 {
		t.Errorf("run judges = %d", n)
	}

	// flight 1001 in run 0: snapshot, marks (null per-trick entry kept), warnings, final mark verbatim
	var fid int64
	var seq int
	var score float64
	var published, dns bool
	var markType sql.NullString
	var finalMark string
	if err := db.Read.QueryRow(`SELECT id, save_seq, score, published, did_not_start, mark_type, final_mark FROM flights
		WHERE competition_id = ? AND run_idx = 0 AND civlid = 1001`, soloID).Scan(&fid, &seq, &score, &published, &dns, &markType, &finalMark); err != nil {
		t.Fatal(err)
	}
	if seq != 1 || score != 8.5 || !published || dns || markType.String != "awt" {
		t.Errorf("flight = seq %d score %v pub %v dns %v mt %v", seq, score, published, dns, markType)
	}
	var fm map[string]any
	if err := json.Unmarshal([]byte(finalMark), &fm); err != nil {
		t.Fatal(err)
	}
	w1, _ := fm["warnings1"].([]any)
	w2, ok := fm["warnings2"].([]any)
	if fm["score"] != 8.5 || fm["technicity"] != 1.65 || fm["mark_type"] != "awt" || len(w1) != 1 || !ok || len(w2) != 0 || fm["warnings"] != nil {
		t.Errorf("final_mark = %s", finalMark)
	}
	if seq2 := count(t, db, `SELECT save_seq FROM flights WHERE competition_id = ? AND run_idx = 0 AND civlid = 1002`, soloID); seq2 != 2 {
		t.Errorf("second flight save_seq = %d", seq2)
	}
	var tricks int
	var uniq, bonuses, marksJSON string
	var soloFlag sql.NullBool
	if err := db.Read.QueryRow(`SELECT count(*) FROM flight_tricks WHERE flight_id = ?`, fid).Scan(&tricks); err != nil || tricks != 2 {
		t.Errorf("tricks = %d err=%v", tricks, err)
	}
	if err := db.Read.QueryRow(`SELECT uniqueness, bonuses, solo, technical_marks FROM flight_tricks WHERE flight_id = ? AND position = 1`, fid).
		Scan(&uniq, &bonuses, &soloFlag, new(sql.NullString)); err != nil || uniq != `["right", "twisted"]` || soloFlag.Valid {
		t.Errorf("older snapshot trick: uniqueness=%s bonuses=%s solo=%v err=%v", uniq, bonuses, soloFlag, err)
	}
	var tm sql.NullString
	if err := db.Read.QueryRow(`SELECT technical_marks FROM flight_tricks WHERE flight_id = ? AND position = 0`, fid).Scan(&tm); err != nil || !tm.Valid {
		t.Errorf("technical_marks = %v err=%v", tm, err)
	}
	var tech sql.NullFloat64
	if err := db.Read.QueryRow(`SELECT technical, technical_per_trick FROM flight_marks WHERE flight_id = ? AND judge_id = 'bbbbbbbbbbbbbbbbbbbbbbb1'`, fid).Scan(&tech, &marksJSON); err != nil ||
		marksJSON != "[6.5, null]" || !tech.Valid || tech.Float64 != 0 {
		t.Errorf("mark = %v %q err=%v", tech, marksJSON, err)
	}
	var perTrickNull sql.NullString
	if err := db.Read.QueryRow(`SELECT m.technical_per_trick FROM flight_marks m JOIN flights f ON f.id = m.flight_id
		WHERE f.competition_id = ? AND f.run_idx = 0 AND f.civlid = 1002`, soloID).Scan(&perTrickNull); err != nil || perTrickNull.Valid {
		t.Errorf("absent technical_per_trick = %v err=%v", perTrickNull, err)
	}
	var wtext string
	if err := db.Read.QueryRow(`SELECT text FROM flight_warnings WHERE flight_id = ? AND category = 1 AND position = 0`, fid).Scan(&wtext); err != nil || wtext != "w cat1" {
		t.Errorf("warning = %q err=%v", wtext, err)
	}

	// synchro flight: team participant, tip touches, cat 2 warning, Round3 score, mark_type awq
	var tip int
	var sscore float64
	var team, smt, sfinal string
	if err := db.Read.QueryRow(`SELECT team_id, tip_touches, score, mark_type, final_mark FROM flights WHERE competition_id = ?`, synchroID).
		Scan(&team, &tip, &sscore, &smt, &sfinal); err != nil {
		t.Fatal(err)
	}
	if team != "aaaaaaaaaaaaaaaaaaaaaaa1" || tip != 1 || sscore != 12.346 || smt != "awq" {
		t.Errorf("synchro flight = %s %d %v %s", team, tip, sscore, smt)
	}
	var sfm map[string]any
	_ = json.Unmarshal([]byte(sfinal), &sfm)
	if sfm["score"] != 12.3456 { // stored verbatim, only the column is rounded
		t.Errorf("synchro final_mark score = %v", sfm["score"])
	}
	if n := count(t, db, `SELECT count(*) FROM flight_warnings w JOIN flights f ON f.id = w.flight_id WHERE f.competition_id = ? AND w.category = 2 AND w.text = 'w cat2'`, synchroID); n != 1 {
		t.Errorf("cat 2 warning missing")
	}

	// rankings: run 0, run 1, competition, season lists
	r0 := entries(t, db, "run", store.RunScopeKey(soloID, 0), "overall")
	if len(r0) != 3 || r0[0].civlid.Int64 != 1001 || r0[0].score != 8.5 || r0[2].civlid.Int64 != 77777 || r0[2].score != 0 {
		t.Errorf("run 0 overall = %+v", r0)
	}
	overall := entries(t, db, "competition", soloID, "overall")
	if len(overall) != 3 || overall[0].civlid.Int64 != 1002 || overall[0].score != 16.25 || overall[1].civlid.Int64 != 1001 || overall[1].score != 14.5 {
		t.Errorf("competition overall = %+v", overall)
	}
	awt := entries(t, db, "competition", soloID, "awt-2024")
	if len(awt) != 1 || awt[0].civlid.Int64 != 1001 {
		t.Errorf("competition awt-2024 = %+v", awt)
	}
	if n := len(entries(t, db, "competition", soloID, "women")); n != 0 {
		t.Errorf("women list with 1 woman has %d entries", n)
	}
	var details string
	if err := db.Read.QueryRow(`SELECT details FROM ranking_entries WHERE scope = 'competition' AND scope_key = ? AND list_key = 'overall' AND position = 1`, soloID).Scan(&details); err != nil ||
		details != `{"per_run":[{"run":0,"rank":2,"score":7.25},{"run":1,"rank":1,"score":9}]}` {
		t.Errorf("details = %s err=%v", details, err)
	}
	seasonSolo := entries(t, db, "season", "awt-2024", "overall")
	if len(seasonSolo) != 1 || seasonSolo[0].civlid.Int64 != 1001 || seasonSolo[0].score != 14.5 {
		t.Errorf("season awt-2024 = %+v", seasonSolo)
	}
	seasonSynchro := entries(t, db, "season", "aws-2024", "overall")
	if len(seasonSynchro) != 1 || seasonSynchro[0].teamID.String != "aaaaaaaaaaaaaaaaaaaaaaa1" || seasonSynchro[0].score != 12.346 {
		t.Errorf("season aws-2024 = %+v", seasonSynchro)
	}
	if n := count(t, db, `SELECT count(*) FROM runs WHERE results_final = 1`); n != 3 {
		t.Errorf("runs with results_final = %d", n)
	}
	if n := count(t, db, `SELECT count(*) FROM competitions WHERE frozen = 1 AND results_final = 1`); n != 2 {
		t.Errorf("frozen final competitions = %d", n)
	}
}

// Links to rows that were not imported are dropped with a warning, flights of
// unknown teams are skipped, and a replace import is repeatable.
func TestImportCompetitionsToleratesMissingReferences(t *testing.T) {
	ctx := context.Background()
	db, blobs := setup(t)
	dir := copyMini(t)
	if err := os.WriteFile(filepath.Join(dir, "teams.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "judges.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := importer.Run(ctx, db, blobs, dir, importer.WithCatalogDir(catalogDir))
	if err != nil {
		t.Fatal(err)
	}
	if !hasWarning(rep, "flight of team") || !hasWarning(rep, "judge bbbbbbbbbbbbbbbbbbbbbbb1 was not imported") {
		t.Errorf("warnings = %v", rep.Warnings)
	}
	if n := count(t, db, `SELECT count(*) FROM flights WHERE competition_id = ?`, synchroID); n != 0 {
		t.Errorf("synchro flights = %d", n)
	}
	if n := count(t, db, `SELECT count(*) FROM competition_judges`); n != 0 {
		t.Errorf("judge links = %d", n)
	}
	if _, err := importer.Run(ctx, db, blobs, dir, importer.WithCatalogDir(catalogDir), importer.WithReplace()); err != nil {
		t.Fatalf("replace: %v", err)
	}
}
