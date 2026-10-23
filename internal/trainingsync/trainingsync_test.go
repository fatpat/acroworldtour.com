package trainingsync_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"awt/internal/blob"
	"awt/internal/config"
	"awt/internal/store"
	"awt/internal/trainingsync"
)

func target(t *testing.T, name, bucket, prefix string) trainingsync.Target {
	t.Helper()
	cfg, err := config.Load(func(k string) string {
		return map[string]string{
			"AWT_ENV": name, "AWT_BASE_URL": "https://" + name + ".example.org",
			"AWT_BLOB_BACKEND": "s3", "AWT_S3_ENDPOINT": "https://s3.gra.io.cloud.ovh.net",
			"AWT_S3_BUCKET": bucket, "AWT_BUCKET_PREFIX": prefix,
			"AWT_S3_ACCESS_KEY": name + "-key", "AWT_S3_SECRET_KEY": name + "-secret",
		}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	tg, err := trainingsync.TargetFromConfig(cfg, "gra")
	if err != nil {
		t.Fatal(err)
	}
	return tg
}

func TestTargetPrefixes(t *testing.T) {
	tg := target(t, "training", "b", "training")
	if tg.FilesPrefix() != "training/files/" || tg.ReplicaPrefix() != "training/litestream/" {
		t.Errorf("files %q replica %q", tg.FilesPrefix(), tg.ReplicaPrefix())
	}
	root := target(t, "training", "b", "")
	if root.FilesPrefix() != "files/" || root.ReplicaPrefix() != "litestream/" {
		t.Errorf("files %q replica %q", root.FilesPrefix(), root.ReplicaPrefix())
	}
	if s := tg.String(); strings.Contains(s, "secret") || strings.Contains(s, "-key") || !strings.Contains(s, "bucket b") {
		t.Errorf("String() = %q", s)
	}
}

// Wiping training must never reach prod's objects: one bucket needs
// prefixes where neither contains the other.
func TestCheckSeparate(t *testing.T) {
	ok := [][4]string{
		{"awt", "prod/", "awt", "training/"},
		{"awt-prod", "", "awt-training", ""},
		{"awt-prod", "x/", "awt-training", "x/"},
		{"awt", "prod/", "awt", "prod2/"},
		{"awt", "env/prod/", "awt", "env/training/"},
	}
	for _, c := range ok {
		if err := trainingsync.CheckSeparate(target(t, "prod", c[0], c[1]), target(t, "training", c[2], c[3])); err != nil {
			t.Errorf("%v: %v", c, err)
		}
	}
	overlap := [][4]string{
		{"awt", "prod/", "awt", "prod/"},
		{"awt", "", "awt", ""},
		{"awt", "", "awt", "training/"},
		{"awt", "prod/", "awt", ""},
		{"awt", "prod/", "awt", "prod/training/"},
		{"awt", "env/training/", "awt", "env/"},
	}
	for _, c := range overlap {
		err := trainingsync.CheckSeparate(target(t, "prod", c[0], c[1]), target(t, "training", c[2], c[3]))
		if !errors.Is(err, trainingsync.ErrOverlap) {
			t.Errorf("%v: err = %v, want ErrOverlap", c, err)
		}
	}
}

func TestTargetFromConfigRequiresS3(t *testing.T) {
	cfg, err := config.Load(func(k string) string {
		return map[string]string{"AWT_ENV": "training", "AWT_BASE_URL": "https://t.example.org", "AWT_S3_BUCKET": "b"}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := trainingsync.TargetFromConfig(cfg, ""); err == nil || !strings.Contains(err.Error(), "AWT_BLOB_BACKEND") {
		t.Errorf("fs backend: err = %v", err)
	}
}

type obj struct {
	data []byte
	ct   string
}

// fake is an in-memory blob.Bucket (streamed copies only).
type fake map[string]obj

func (f fake) List(_ context.Context, prefix string) ([]blob.Object, error) {
	var out []blob.Object
	for _, k := range slices.Sorted(maps.Keys(f)) {
		if strings.HasPrefix(k, prefix) {
			out = append(out, blob.Object{Key: k, Size: int64(len(f[k].data))})
		}
	}
	return out, nil
}
func (f fake) Open(_ context.Context, k string) (io.ReadCloser, string, error) {
	o, ok := f[k]
	if !ok {
		return nil, "", blob.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(o.data)), o.ct, nil
}
func (f fake) Write(_ context.Context, k, ct string, r io.Reader, _ int64) error {
	b, err := io.ReadAll(r)
	f[k] = obj{b, ct}
	return err
}
func (f fake) Remove(_ context.Context, keys []string) error {
	for _, k := range keys {
		delete(f, k)
	}
	return nil
}

// One shared bucket: training is emptied and gets prod's files; prod and
// unrelated keys stay.
func TestWipeAndCopyFiles(t *testing.T) {
	ctx := context.Background()
	b := fake{
		"prod/files/aa":                      {[]byte("A"), "image/png"},
		"prod/files/bb":                      {[]byte("BB"), "application/pdf"},
		"prod/litestream/generations/g1/x":   {[]byte("p"), ""},
		"training/files/aa":                  {[]byte("A"), "image/png"},
		"training/files/trainee":             {[]byte("T"), "image/jpeg"},
		"training/litestream/generations/g2": {[]byte("t"), ""},
		"training/litestream/generations/g3": {[]byte("t"), ""},
		"training/notes.txt":                 {[]byte("n"), "text/plain"},
	}
	prod, training := target(t, "prod", "awt", "prod/"), target(t, "training", "awt", "training/")

	w, err := trainingsync.Wipe(ctx, b, prod, training)
	if err != nil {
		t.Fatal(err)
	}
	if w.Replica != 2 || w.Files != 2 {
		t.Errorf("wipe = %+v", w)
	}
	c, err := trainingsync.CopyFiles(ctx, b, prod, b, training)
	if err != nil {
		t.Fatal(err)
	}
	if c.Objects != 2 || c.Bytes != 3 {
		t.Errorf("copy = %+v", c)
	}
	want := []string{
		"prod/files/aa", "prod/files/bb", "prod/litestream/generations/g1/x",
		"training/files/aa", "training/files/bb", "training/notes.txt",
	}
	if got := slices.Sorted(maps.Keys(b)); !slices.Equal(got, want) {
		t.Errorf("keys %v\nwant %v", got, want)
	}
	if b["training/files/bb"].ct != "application/pdf" {
		t.Errorf("content type %q", b["training/files/bb"].ct)
	}
}

// Wipe and CopyFiles check the prefixes themselves: no caller can delete
// or overwrite prod's objects.
func TestWipeRefusesOverlap(t *testing.T) {
	b := fake{"prod/files/aa": {[]byte("A"), ""}}
	prod := target(t, "prod", "awt", "prod/")
	if _, err := trainingsync.CopyFiles(context.Background(), b, prod, b, target(t, "training", "awt", "prod/")); !errors.Is(err, trainingsync.ErrOverlap) {
		t.Errorf("copy onto prod: %v", err)
	}
	if _, err := trainingsync.Wipe(context.Background(), b, prod, target(t, "training", "awt", "")); !errors.Is(err, trainingsync.ErrOverlap) {
		t.Errorf("wipe over prod: %v", err)
	}
	if len(b) != 1 {
		t.Error("objects deleted")
	}
}

func seedDB(t *testing.T, path string) {
	t.Helper()
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO users (id, email, name, password_hash, role) VALUES ('u1', 'a@x.org', 'A', 'h', 'admin')`,
		`INSERT INTO news (id, slug, title) VALUES ('n1', 'n', 'N'), ('n2', 'm', 'M')`,
		`INSERT INTO sessions (token_hash, user_id, csrf_token, expires_at, created_at) VALUES ('s', 'u1', 'c', '2099-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
		`INSERT INTO user_totp (user_id, secret_enc, created_at) VALUES ('u1', 'enc', '2026-01-01T00:00:00Z')`,
	} {
		if _, err := db.Write.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.db")
	seedDB(t, path)
	p, err := trainingsync.PrepareDB(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Auth.Sessions != 1 || p.Auth.TOTP != 1 || p.Summary.Users != 1 || p.Summary.News != 2 {
		t.Errorf("prepared %+v", p)
	}
	// One self-contained file: no WAL left beside it.
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(path + suffix); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s left: %v", suffix, err)
		}
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var mode string
	var sessions, totp int
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "delete" {
		t.Errorf("journal_mode %q, %v", mode, err)
	}
	_ = db.QueryRow(`SELECT (SELECT count(*) FROM sessions), (SELECT count(*) FROM user_totp)`).Scan(&sessions, &totp)
	if sessions != 0 || totp != 0 {
		t.Errorf("sessions %d totp %d", sessions, totp)
	}
}

func TestPrepareDBRefuses(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.db")
	if _, err := trainingsync.PrepareDB(context.Background(), missing); err == nil {
		t.Error("missing file accepted")
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Error("PrepareDB created the missing file")
	}
	garbage := filepath.Join(dir, "garbage.db")
	if err := os.WriteFile(garbage, bytes.Repeat([]byte("not a database "), 512), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := trainingsync.PrepareDB(context.Background(), garbage); err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Errorf("garbage: err = %v", err)
	}
}

// A page whose content no longer matches the index: SQLite opens it, only
// integrity_check notices.
func TestPrepareDBRefusesCorruptIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.db")
	seedDB(t, path)
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	// The index is built on title, then its definition is changed to
	// another column: its entries no longer match the rows.
	for _, q := range []string{
		`CREATE INDEX news_probe ON news (title)`,
		`PRAGMA writable_schema = ON`,
		`UPDATE sqlite_master SET sql = 'CREATE INDEX news_probe ON news (slug)' WHERE name = 'news_probe'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	db.Close()
	if _, err := trainingsync.PrepareDB(context.Background(), path); err == nil || !strings.Contains(err.Error(), "integrity check "+path+" failed") {
		t.Errorf("err = %v", err)
	}
}
