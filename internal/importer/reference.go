package importer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"awt/internal/blob"
	"awt/internal/catalog"
	"awt/internal/store/gen"
)

// Years whose catalog comes from JSON files; the export's catalog is 2026.
var fileCatalogYears = []int{2022, 2023, 2024, 2025}

const exportCatalogYear = 2026

func importReference(ctx context.Context, q *gen.Queries, blobs blob.Store, s *source, o options, rep *Report) error {
	pilots, err := importPilots(ctx, q, s, rep)
	if err != nil {
		return err
	}
	comps, err := decodeLines[competitionRow](s, "competitions")
	if err != nil {
		return err
	}
	if err := createPlaceholderPilots(ctx, q, s, comps, pilots, rep); err != nil {
		return err
	}
	teams, err := importTeams(ctx, q, s, pilots, rep)
	if err != nil {
		return err
	}
	judges, err := importJudges(ctx, q, s, pilots, rep)
	if err != nil {
		return err
	}
	fileIDs, err := importFiles(ctx, q, blobs, s, rep)
	if err != nil {
		return err
	}
	if err := importSeasons(ctx, q, s, fileIDs, rep); err != nil {
		return err
	}
	if err := importTricks(ctx, q, s, o.catalogDir, rep); err != nil {
		return err
	}
	return importCompetitions(ctx, q, s, comps, refData{pilots: pilots, teams: teams, judges: judges, files: fileIDs}, rep)
}

type pilotRow struct {
	ID            ID              `json:"_id"`
	Civlid        Int             `json:"civlid"`
	Name          string          `json:"name"`
	Country       string          `json:"country"`
	About         string          `json:"about"`
	Sponsors      json.RawMessage `json:"sponsors"`
	Photo         string          `json:"photo"`
	BackgroundPic string          `json:"background_picture"`
	LastUpdate    Date            `json:"last_update"`
	Rank          Int             `json:"rank"`
	Gender        string          `json:"gender"`
	SocialLinks   json.RawMessage `json:"social_links"`
	CivlLink      string          `json:"civl_link"`
	PhotoHighres  string          `json:"photo_highres"`
	AwtYears      []Int           `json:"awt_years"`
}

func importPilots(ctx context.Context, q *gen.Queries, s *source, rep *Report) (map[int64]bool, error) {
	rows, err := decodeLines[pilotRow](s, "pilots")
	if err != nil {
		return nil, err
	}
	imported := map[int64]bool{}
	for _, p := range rows {
		civlid := p.Civlid.V
		if !p.Civlid.Valid {
			id, err := strconv.ParseInt(string(p.ID), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("pilot %q has no civlid", p.Name)
			}
			civlid = id
		}
		gender := p.Gender
		switch gender {
		case "man", "woman", "none":
		case "":
			gender = "man" // spec 02 §2.1 default
		default:
			rep.warnf("pilot %d: unknown gender %q, using none", civlid, gender)
			gender = "none"
		}
		rank := int64(9999) // 9999 = unranked
		if p.Rank.Valid {
			rank = p.Rank.V
		}
		err := q.UpsertPilot(ctx, gen.UpsertPilotParams{
			Civlid: civlid, Name: p.Name, Country: p.Country, About: p.About, CivlLink: p.CivlLink,
			PhotoUrl: p.Photo, PhotoHighresUrl: p.PhotoHighres, BackgroundUrl: p.BackgroundPic,
			Rank: rank, Gender: gender, SocialLinks: rawJSONArray(p.SocialLinks), Sponsors: rawJSONArray(p.Sponsors),
			UpdatedAt: p.LastUpdate.null(),
		})
		if err != nil {
			return nil, fmt.Errorf("import pilot %d: %w", civlid, err)
		}
		imported[civlid] = true
		rep.Counts["pilots"]++
		// is_awt is a legacy flag: awt_years is the source of truth.
		for _, y := range p.AwtYears {
			if err := q.AddPilotAwtYear(ctx, gen.AddPilotAwtYearParams{Civlid: civlid, Year: y.V}); err != nil {
				return nil, fmt.Errorf("import awt year %d of pilot %d: %w", y.V, civlid, err)
			}
			rep.Counts["pilot_awt_years"]++
		}
	}
	return imported, nil
}

type teamRow struct {
	ID      ID     `json:"_id"`
	Name    string `json:"name"`
	Pilots  []Int  `json:"pilots"`
	Deleted Date   `json:"deleted"`
}

func importTeams(ctx context.Context, q *gen.Queries, s *source, pilots map[int64]bool, rep *Report) (map[string]bool, error) {
	rows, err := decodeLines[teamRow](s, "teams")
	if err != nil {
		return nil, err
	}
	imported := map[string]bool{}
	for _, t := range rows {
		if len(t.Pilots) != 2 {
			rep.warnf("team %s (%s): %d pilots instead of 2, skipped", t.ID, t.Name, len(t.Pilots))
			continue
		}
		if !pilots[t.Pilots[0].V] || !pilots[t.Pilots[1].V] {
			rep.warnf("team %s (%s): unknown pilot, skipped", t.ID, t.Name)
			continue
		}
		if err := q.UpsertTeam(ctx, gen.UpsertTeamParams{ID: string(t.ID), Name: t.Name, DeletedAt: t.Deleted.null()}); err != nil {
			return nil, fmt.Errorf("import team %s: %w", t.ID, err)
		}
		for pos, p := range t.Pilots { // order = position 0, 1
			if err := q.SetTeamPilot(ctx, gen.SetTeamPilotParams{TeamID: string(t.ID), Position: int64(pos), Civlid: p.V}); err != nil {
				return nil, fmt.Errorf("import team %s pilot: %w", t.ID, err)
			}
		}
		imported[string(t.ID)] = true
		rep.Counts["teams"]++
	}
	return imported, nil
}

type judgeRow struct {
	ID      ID     `json:"_id"`
	Name    string `json:"name"`
	Country string `json:"country"`
	Level   string `json:"level"`
	Deleted Date   `json:"deleted"`
	Civlid  Int    `json:"civlid"`
}

func importJudges(ctx context.Context, q *gen.Queries, s *source, pilots map[int64]bool, rep *Report) (map[string]bool, error) {
	rows, err := decodeLines[judgeRow](s, "judges")
	if err != nil {
		return nil, err
	}
	imported := map[string]bool{}
	for _, j := range rows {
		if !slices.Contains([]string{"trainee", "certified", "senior"}, j.Level) {
			rep.warnf("judge %s (%s): unknown level %q, skipped", j.ID, j.Name, j.Level)
			continue
		}
		civlid := sql.NullInt64{Int64: j.Civlid.V, Valid: j.Civlid.Valid}
		if civlid.Valid && !pilots[civlid.Int64] {
			rep.warnf("judge %s (%s): civlid %d is not a known pilot, link dropped", j.ID, j.Name, civlid.Int64)
			civlid = sql.NullInt64{}
		}
		err := q.UpsertJudge(ctx, gen.UpsertJudgeParams{
			ID: string(j.ID), Name: j.Name, Country: j.Country, Level: j.Level, Civlid: civlid, DeletedAt: j.Deleted.null(),
		})
		if err != nil {
			return nil, fmt.Errorf("import judge %s: %w", j.ID, err)
		}
		imported[string(j.ID)] = true
		rep.Counts["judges"]++
	}
	return imported, nil
}

type fileRow struct {
	ID          ID     `json:"_id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Content     string `json:"content"`
}

// importFiles decodes the base64 content, checks sha256(content) == _id (spec
// 02 §2.13) and stores the bytes in the blob store under the id. It returns
// the ids that were imported.
func importFiles(ctx context.Context, q *gen.Queries, blobs blob.Store, s *source, rep *Report) (map[string]bool, error) {
	rows, err := decodeLines[fileRow](s, "files")
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, f := range rows {
		content, err := base64.StdEncoding.DecodeString(f.Content)
		if err != nil {
			rep.warnf("file %s (%s): invalid base64, skipped", f.ID, f.Filename)
			continue
		}
		if !fileIDMatches(string(f.ID), content, f.Content) {
			rep.warnf("file %s (%s): sha256 of content does not match id, skipped", f.ID, f.Filename)
			continue
		}
		if err := blobs.Put(ctx, string(f.ID), f.ContentType, bytes.NewReader(content)); err != nil {
			return nil, fmt.Errorf("store file %s: %w", f.ID, err)
		}
		err = q.UpsertFile(ctx, gen.UpsertFileParams{
			ID: string(f.ID), Filename: f.Filename, ContentType: f.ContentType, Size: int64(len(content)), CreatedAt: now,
		})
		if err != nil {
			return nil, fmt.Errorf("import file %s: %w", f.ID, err)
		}
		ids[string(f.ID)] = true
		rep.Counts["files"]++
	}
	return ids, nil
}

// fileIDMatches checks the content-addressed id. The legacy code computed it
// as sha256 of the base64 text (api/models/files.py), so every production id
// is that hash; sha256 of the decoded bytes is accepted too (spec 02 §2.13).
func fileIDMatches(id string, content []byte, encoded string) bool {
	for _, sum := range [][32]byte{sha256.Sum256(content), sha256.Sum256([]byte(encoded))} {
		if hex.EncodeToString(sum[:]) == id {
			return true
		}
	}
	return false
}

type seasonRow struct {
	ID      ID      `json:"_id"`
	Name    string  `json:"name"`
	Year    Int     `json:"year"`
	Image   *string `json:"image"`
	Deleted Date    `json:"deleted"`
	Code    string  `json:"code"`
	Country *string `json:"country"`
	Index   Int     `json:"index"`
}

// importSeasons resolves the legacy duplicate season codes (the legacy unique
// index included `deleted`, so a code can appear several times): the active
// row wins; soft-deleted rows are kept only when no active row holds their
// code. Competitions reference seasons by code, so two active rows with the
// same code are referenced equally: the first one in the export is kept.
func importSeasons(ctx context.Context, q *gen.Queries, s *source, fileIDs map[string]bool, rep *Report) error {
	rows, err := decodeLines[seasonRow](s, "seasons")
	if err != nil {
		return err
	}
	// kept[code]: the active row (first one), else the most recently deleted.
	kept := map[string]seasonRow{}
	for _, r := range rows {
		k, ok := kept[r.Code]
		switch {
		case !ok, k.Deleted != "" && (r.Deleted == "" || r.Deleted > k.Deleted):
			kept[r.Code] = r
		}
	}
	for _, r := range rows {
		if k := kept[r.Code]; k.ID != r.ID {
			state := "active"
			if r.Deleted != "" {
				state = "soft-deleted"
			}
			rep.warnf("season %s (%s): %s duplicate of code %q kept as %s, skipped", r.ID, r.Name, state, r.Code, k.ID)
			continue
		}
		if !r.Year.Valid {
			return fmt.Errorf("season %s (%s): missing year", r.ID, r.Code)
		}
		image := sql.NullString{}
		if r.Image != nil && *r.Image != "" {
			if fileIDs[*r.Image] {
				image = sql.NullString{String: *r.Image, Valid: true}
			} else {
				rep.warnf("season %s (%s): image file %s was not imported, image dropped", r.ID, r.Code, *r.Image)
			}
		}
		country := sql.NullString{}
		if r.Country != nil && *r.Country != "" {
			country = sql.NullString{String: *r.Country, Valid: true}
		}
		sortIndex := int64(999) // spec 02 §2.11 default
		if r.Index.Valid {
			sortIndex = r.Index.V
		}
		err := q.UpsertSeason(ctx, gen.UpsertSeasonParams{
			ID: string(r.ID), Code: r.Code, Name: r.Name, Year: r.Year.V, Country: country,
			ImageFile: image, SortIndex: sortIndex, DeletedAt: r.Deleted.null(),
		})
		if err != nil {
			return fmt.Errorf("import season %s: %w", r.ID, err)
		}
		rep.Counts["seasons"]++
	}
	return nil
}

type trickRow struct {
	ID      ID   `json:"_id"`
	Deleted Date `json:"deleted"`
}

// importTricks imports the 2022-2025 catalogs from <catalogDir>/<year>.json
// (new ids) and the export's non-deleted tricks as 2026 (ids kept). Variants
// are always regenerated with catalog.Generate.
func importTricks(ctx context.Context, q *gen.Queries, s *source, catalogDir string, rep *Report) error {
	for _, year := range fileCatalogYears {
		f, err := os.Open(filepath.Join(catalogDir, strconv.Itoa(year)+".json"))
		if err != nil {
			return fmt.Errorf("open catalog %d: %w", year, err)
		}
		tricks, err := catalog.LoadJSON(f)
		_ = f.Close()
		if err != nil {
			return fmt.Errorf("catalog %d: %w", year, err)
		}
		for _, t := range tricks {
			id, err := newID()
			if err != nil {
				return err
			}
			if err := insertTrick(ctx, q, id, year, t, rep); err != nil {
				return err
			}
		}
	}

	raws, err := decodeLines[json.RawMessage](s, "tricks")
	if err != nil {
		return err
	}
	for _, raw := range raws {
		var meta trickRow
		var t catalog.Trick
		if err := json.Unmarshal(raw, &meta); err != nil {
			return fmt.Errorf("decode trick: %w", err)
		}
		if err := json.Unmarshal(raw, &t); err != nil {
			return fmt.Errorf("decode trick %s: %w", meta.ID, err)
		}
		if meta.Deleted != "" {
			rep.Counts["tricks_skipped_deleted"]++
			continue
		}
		if err := checkTrick(t); err != nil {
			return fmt.Errorf("trick %s: %w", meta.ID, err)
		}
		if err := insertTrick(ctx, q, string(meta.ID), exportCatalogYear, t, rep); err != nil {
			return err
		}
	}
	return nil
}

// checkTrick rejects what catalog.Generate would silently mishandle.
func checkTrick(t catalog.Trick) error {
	for _, b := range t.Bonuses {
		if _, ok := catalog.LookupBonus(b.Name); !ok {
			return fmt.Errorf("%q: unknown bonus %q", t.Name, b.Name)
		}
	}
	for _, d := range t.Directions {
		if !slices.Contains([]string{"left", "right", "opposite"}, d) {
			return fmt.Errorf("%q: unknown direction %q", t.Name, d)
		}
	}
	return nil
}

func insertTrick(ctx context.Context, q *gen.Queries, id string, year int, t catalog.Trick, rep *Report) error {
	err := q.UpsertTrick(ctx, gen.UpsertTrickParams{
		ID: id, Year: int64(year), Name: t.Name, Acronym: t.Acronym, Solo: t.Solo, SoloAwt: t.SoloAWT, Synchro: t.Synchro,
		Directions: jsonText(t.Directions, "[]"), TechnicalCoefficient: t.TechnicalCoefficient,
		Bonuses: jsonText(t.Bonuses, "[]"), BonusConstraints: jsonText(t.BonusConstraints, "[]"),
		FirstManeuver: int64(t.FirstManeuver), NoFirstManeuver: int64(t.NoFirstManeuver),
		LastManeuver: int64(t.LastManeuver), NoLastManeuver: int64(t.NoLastManeuver),
		Repeatable: t.Repeatable, Types: jsonText(t.Types, "[]"),
		SampleVideo: sql.NullString{String: t.SampleVideo, Valid: t.SampleVideo != ""},
	})
	if err != nil {
		return fmt.Errorf("import trick %q (%d): %w", t.Name, year, err)
	}
	rep.Counts["tricks"]++
	for _, u := range catalog.Generate(t) {
		err := q.InsertUniqueTrick(ctx, gen.InsertUniqueTrickParams{
			TrickID: id, Year: int64(year), Acronym: u.Acronym, Name: u.Name, BaseTrick: u.BaseTrick,
			TechnicalCoefficient: u.TechnicalCoefficient, Bonus: u.Bonus, BonusTypes: jsonText(u.BonusTypes, "[]"),
			Uniqueness: jsonText(u.Uniqueness, "[]"), Bonuses: jsonText(u.Bonuses, "[]"),
			Solo: u.Solo, SoloAwt: u.SoloAWT, Synchro: u.Synchro, Types: jsonText(u.Types, "[]"),
		})
		if err != nil {
			return fmt.Errorf("import unique trick %q of %q (%d): %w", u.Acronym, t.Name, year, err)
		}
		rep.Counts["unique_tricks"]++
	}
	return nil
}

// jsonText marshals v; nil slices become empty.
func jsonText(v any, empty string) string {
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return empty
	}
	return string(b)
}

// newID returns a new 24-char lowercase hex id.
func newID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
