package importer_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"awt/internal/blob"
	"awt/internal/catalog"
	"awt/internal/importer"
	"awt/internal/store"
)

const (
	miniDir    = "testdata/mini"
	catalogDir = "../../testdata/catalog"
	helloID    = "acc52d05cd264a4c2a73b5078129a858bb0d408112ebb8539c2fd1eab0a7a18f"
)

func setup(t *testing.T) (*store.DB, *blob.FS) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "awt.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	blobs, err := blob.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("blobs: %v", err)
	}
	return db, blobs
}

func count(t *testing.T, db *store.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.Read.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// copyMini copies the mini export to a temp dir so a test can alter it.
func copyMini(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	entries, err := os.ReadDir(miniDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(miniDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

func TestImportReference(t *testing.T) {
	ctx := context.Background()
	db, blobs := setup(t)
	rep, err := importer.Run(ctx, db, blobs, miniDir, importer.WithCatalogDir(catalogDir))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// pilots: extended JSON ids/dates, gender, awt years, defaults for nulls.
	var name, gender, updated, hr string
	var rank int
	if err := db.Read.QueryRow(`SELECT name, gender, rank, updated_at, photo_highres_url FROM pilots WHERE civlid = 1001`).
		Scan(&name, &gender, &rank, &updated, &hr); err != nil {
		t.Fatal(err)
	}
	if name != "Alice Aerial" || gender != "woman" || rank != 2 || updated != "2026-10-11T15:05:43Z" || hr == "" {
		t.Errorf("pilot 1001 = %q %q %d %q %q", name, gender, rank, updated, hr)
	}
	if err := db.Read.QueryRow(`SELECT updated_at, photo_highres_url FROM pilots WHERE civlid = 1002`).Scan(&updated, &hr); err != nil {
		t.Fatal(err)
	}
	if updated != "2026-01-02T03:04:05Z" || hr != "" {
		t.Errorf("pilot 1002 updated=%q highres=%q", updated, hr)
	}
	if n := count(t, db, `SELECT count(*) FROM pilot_awt_years WHERE civlid = 1001 AND year IN (2024, 2025, 2026)`); n != 3 {
		t.Errorf("awt years of 1001 = %d, want 3", n)
	}
	if n := count(t, db, `SELECT count(*) FROM pilot_awt_years WHERE civlid = 1002`); n != 0 {
		t.Errorf("awt years of 1002 = %d, want 0", n)
	}

	// teams keep the pilot order of the export.
	var p0, p1 int
	if err := db.Read.QueryRow(`SELECT
		(SELECT civlid FROM team_pilots WHERE team_id = 'aaaaaaaaaaaaaaaaaaaaaaa1' AND position = 0),
		(SELECT civlid FROM team_pilots WHERE team_id = 'aaaaaaaaaaaaaaaaaaaaaaa1' AND position = 1)`).Scan(&p0, &p1); err != nil {
		t.Fatal(err)
	}
	if p0 != 1002 || p1 != 1001 {
		t.Errorf("team pilots = %d, %d, want 1002, 1001", p0, p1)
	}

	// judges: $oid id, civlid link, soft delete.
	var level, deleted string
	if err := db.Read.QueryRow(`SELECT level FROM judges WHERE id = 'bbbbbbbbbbbbbbbbbbbbbbb1' AND civlid = 1001 AND deleted_at IS NULL`).Scan(&level); err != nil || level != "senior" {
		t.Errorf("judge 1: level=%q err=%v", level, err)
	}
	if err := db.Read.QueryRow(`SELECT deleted_at FROM judges WHERE id = 'bbbbbbbbbbbbbbbbbbbbbbb2'`).Scan(&deleted); err != nil || deleted != "2025-03-04T10:00:00Z" {
		t.Errorf("judge 2 deleted_at=%q err=%v", deleted, err)
	}

	// seasons: the duplicate aws-2024 keeps the active row and reports the other.
	if n := count(t, db, `SELECT count(*) FROM seasons`); n != 1 {
		t.Errorf("seasons = %d, want 1", n)
	}
	var img string
	if err := db.Read.QueryRow(`SELECT image_file FROM seasons WHERE code = 'aws-2024' AND deleted_at IS NULL AND id = 'ccccccccccccccccccccccc1'`).Scan(&img); err != nil || img != helloID {
		t.Errorf("season image=%q err=%v", img, err)
	}
	if !hasWarning(rep, "ccccccccccccccccccccccc2") {
		t.Errorf("no warning for the skipped duplicate season: %v", rep.Warnings)
	}

	// files: decoded content in the blob store under the sha256 id.
	r, ct, err := blobs.Get(ctx, helloID)
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	got, _ := io.ReadAll(r)
	_ = r.Close()
	if string(got) != "hello awt" || ct != "text/plain" {
		t.Errorf("blob = %q (%s)", got, ct)
	}
	if n := count(t, db, `SELECT count(*) FROM files WHERE id = ? AND size = 9 AND filename = 'hello.txt'`, helloID); n != 1 {
		t.Errorf("files row missing")
	}

	// tricks: 2022-2025 from the catalog files, 2026 from the export.
	wantTricks, wantUnique := 0, 0
	for _, year := range []string{"2022", "2023", "2024", "2025"} {
		f, err := os.Open(filepath.Join(catalogDir, year+".json"))
		if err != nil {
			t.Fatal(err)
		}
		tricks, err := catalog.LoadJSON(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		y := 0
		for _, tr := range tricks {
			y += len(catalog.Generate(tr))
		}
		if n := count(t, db, `SELECT count(*) FROM tricks WHERE year = ?`, year); n != len(tricks) {
			t.Errorf("tricks %s = %d, want %d", year, n, len(tricks))
		}
		if n := count(t, db, `SELECT count(*) FROM unique_tricks WHERE year = ?`, year); n != y {
			t.Errorf("unique tricks %s = %d, want %d", year, n, y)
		}
		wantTricks += len(tricks)
		wantUnique += y
	}
	// Misty Flip: 2 directions + opposite, x (none, twisted, flip, twisted+flip) = 12 variants.
	if n := count(t, db, `SELECT count(*) FROM tricks WHERE year = 2026 AND id = 'ddddddddddddddddddddddd1'`); n != 1 {
		t.Errorf("2026 trick id not kept")
	}
	if n := count(t, db, `SELECT count(*) FROM unique_tricks WHERE year = 2026`); n != 12 {
		t.Errorf("2026 unique tricks = %d, want 12", n)
	}
	if n := count(t, db, `SELECT count(*) FROM tricks WHERE year = 2026`); n != 1 {
		t.Errorf("deleted 2026 trick imported")
	}
	if n := count(t, db, `SELECT count(*) FROM tricks WHERE year < 2026 AND length(id) = 24`); n != wantTricks {
		t.Errorf("catalog trick ids are not 24 chars: %d of %d", n, wantTricks)
	}

	want := map[string]int{
		"pilots": 2, "pilot_awt_years": 3, "teams": 1, "judges": 2, "seasons": 1, "files": 1,
		"tricks": wantTricks + 1, "unique_tricks": wantUnique + 12,
	}
	for k, v := range want {
		if rep.Counts[k] != v {
			t.Errorf("Counts[%s] = %d, want %d", k, rep.Counts[k], v)
		}
	}
}

func hasWarning(rep importer.Report, substr string) bool {
	for _, w := range rep.Warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

func TestImportRefusesNonEmptyUnlessReplace(t *testing.T) {
	ctx := context.Background()
	db, blobs := setup(t)
	opt := importer.WithCatalogDir(catalogDir)
	if _, err := importer.Run(ctx, db, blobs, miniDir, opt); err != nil {
		t.Fatal(err)
	}
	if _, err := importer.Run(ctx, db, blobs, miniDir, opt); !errors.Is(err, importer.ErrNotEmpty) {
		t.Fatalf("second import err = %v, want ErrNotEmpty", err)
	}
	rep, err := importer.Run(ctx, db, blobs, miniDir, opt, importer.WithReplace())
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if rep.Counts["pilots"] != 2 || count(t, db, `SELECT count(*) FROM pilots`) != 2 {
		t.Errorf("replace left %d pilots", count(t, db, `SELECT count(*) FROM pilots`))
	}
}

func TestImportSeasonsAndFilesEdgeCases(t *testing.T) {
	ctx := context.Background()
	db, blobs := setup(t)
	dir := copyMini(t)
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Two active aws-2024 rows (first kept), plus a deleted che-2023 without collision.
	write("seasons.json", `{"_id":"s1","name":"A","year":2024,"image":null,"deleted":null,"code":"aws-2024","country":null,"index":1}
{"_id":"s2","name":"B","year":2024,"image":null,"deleted":null,"code":"aws-2024","country":null,"index":2}
{"_id":"s3","name":"C","year":2023,"image":null,"deleted":"2023-08-01T09:59:43.884562","code":"che-2023","country":"che","index":501}
`)
	// A file whose content does not match its id, and a legacy one whose id is
	// the sha256 of the base64 text (what production data uses).
	legacy := sha256.Sum256([]byte("aGVsbG8="))
	legacyID := hex.EncodeToString(legacy[:])
	write("files.json", `{"_id":"`+helloID+`","filename":"bad.txt","content_type":"text/plain","content":"aGVsbG8=","deleted":null}
{"_id":"`+legacyID+`","filename":"legacy.txt","content_type":"text/plain","content":"aGVsbG8=","deleted":null}
`)
	rep, err := importer.Run(ctx, db, blobs, dir, importer.WithCatalogDir(catalogDir))
	if err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, `SELECT count(*) FROM seasons WHERE id = 's1' AND deleted_at IS NULL`); n != 1 {
		t.Errorf("first active season not kept")
	}
	if n := count(t, db, `SELECT count(*) FROM seasons WHERE id = 's3' AND deleted_at = '2023-08-01T09:59:43Z'`); n != 1 {
		t.Errorf("deleted non-colliding season not imported with deleted_at")
	}
	if rep.Counts["seasons"] != 2 || !hasWarning(rep, "s2") {
		t.Errorf("seasons=%d warnings=%v", rep.Counts["seasons"], rep.Warnings)
	}
	if rep.Counts["files"] != 1 || !hasWarning(rep, "sha256") {
		t.Errorf("files=%d warnings=%v", rep.Counts["files"], rep.Warnings)
	}
	if _, _, err := blobs.Get(ctx, helloID); !errors.Is(err, blob.ErrNotFound) {
		t.Errorf("mismatching file stored: %v", err)
	}
	if _, _, err := blobs.Get(ctx, legacyID); err != nil {
		t.Errorf("legacy-hash file not stored: %v", err)
	}
}

func TestImportTarGz(t *testing.T) {
	ctx := context.Background()
	db, blobs := setup(t)
	tgz := filepath.Join(t.TempDir(), "export.tar.gz")
	if err := writeTarGz(tgz, miniDir); err != nil {
		t.Fatal(err)
	}
	rep, err := importer.Run(ctx, db, blobs, tgz, importer.WithCatalogDir(catalogDir))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Counts["pilots"] != 2 || rep.Counts["files"] != 1 {
		t.Errorf("counts = %v", rep.Counts)
	}
}

func TestImportMissingCollection(t *testing.T) {
	ctx := context.Background()
	db, blobs := setup(t)
	dir := copyMini(t)
	if err := os.Remove(filepath.Join(dir, "pilots.json")); err != nil {
		t.Fatal(err)
	}
	_, err := importer.Run(ctx, db, blobs, dir, importer.WithCatalogDir(catalogDir))
	if err == nil || !strings.Contains(err.Error(), "pilots.json") {
		t.Fatalf("err = %v, want missing pilots.json", err)
	}
}

func TestImportDeletedDuplicateSeasonsAndSkips(t *testing.T) {
	ctx := context.Background()
	db, blobs := setup(t)
	dir := copyMini(t)
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("seasons.json", `{"_id":"d1","name":"A","year":2023,"deleted":"2023-01-01T00:00:00","code":"che-2023","index":1}
{"_id":"d2","name":"B","year":2023,"deleted":"2023-08-01T00:00:00","code":"che-2023","index":2}
`)
	write("teams.json", `{"_id":"t1","name":"Ghosts","pilots":[1001,9999],"deleted":null}
{"_id":"t2","name":"Solo","pilots":[1001],"deleted":null}
`)
	write("judges.json", `{"_id":"j1","name":"Odd","country":"fra","level":"expert","deleted":null}
`)
	rep, err := importer.Run(ctx, db, blobs, dir, importer.WithCatalogDir(catalogDir))
	if err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, `SELECT count(*) FROM seasons WHERE id = 'd2'`); n != 1 || count(t, db, `SELECT count(*) FROM seasons`) != 1 {
		t.Errorf("most recent deleted season not the only one kept")
	}
	if rep.Counts["teams"] != 0 || rep.Counts["judges"] != 0 {
		t.Errorf("counts = %v", rep.Counts)
	}
	for _, w := range []string{"d1", "t1", "t2", "j1"} {
		if !hasWarning(rep, w) {
			t.Errorf("no warning mentioning %s: %v", w, rep.Warnings)
		}
	}

	// A season without year is an error, and the partial report is returned.
	db2, blobs2 := setup(t)
	write("seasons.json", `{"_id":"n1","name":"N","year":null,"deleted":null,"code":"che-2030"}
`)
	rep, err = importer.Run(ctx, db2, blobs2, dir, importer.WithCatalogDir(catalogDir))
	if err == nil || !strings.Contains(err.Error(), "missing year") {
		t.Fatalf("err = %v, want missing year", err)
	}
	if rep.Counts["pilots"] != 2 {
		t.Errorf("partial report lost counts: %v", rep.Counts)
	}
}
