// Package trainingsync holds the steps of the training refresh, which
// replaces the training environment with a copy of prod (docs/ops.md
// "Refresh training from prod", decision 0018): the bucket checks, the wipe
// and file copy, and the preparation of the database snapshot.
// deploy/sync-training.sh runs them through `awt sync-training`.
package trainingsync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"awt/internal/blob"
	"awt/internal/config"
	"awt/internal/store"
	"awt/internal/store/gen"
)

// ErrOverlap means wiping training could touch prod's objects.
var ErrOverlap = errors.New("prod and training overlap in object storage")

// ErrDrift means a running container does not use its env file's object
// storage settings (the file was edited, the container not recreated).
var ErrDrift = errors.New("running container differs from its env file")

// Live is what a running container uses (docker inspect): the non-secret
// object storage settings, as raw environment values.
type Live struct{ Endpoint, Bucket, Prefix string }

// CheckLive refuses when the running container of t's environment does
// not use t's endpoint, bucket and prefix. The overlap check judges the env
// files; it is only meaningful if they are what the containers run with.
// Values are named, never quoted: only the setting that differs.
func CheckLive(t Target, live Live) error {
	var differ []string
	if strings.TrimSuffix(live.Endpoint, "/") != strings.TrimSuffix(t.S3.Endpoint, "/") {
		differ = append(differ, "AWT_S3_ENDPOINT")
	}
	if live.Bucket != t.S3.Bucket {
		differ = append(differ, "AWT_S3_BUCKET")
	}
	if p, err := config.ParseBucketPrefix(live.Prefix); err != nil || p != t.Prefix {
		differ = append(differ, "AWT_BUCKET_PREFIX")
	}
	if len(differ) == 0 {
		return nil
	}
	return fmt.Errorf("%w: the running awt-%s uses another %s than .env.%s; recreate it with the file's settings (docker compose up -d awt-%s) or restore the file, then run again",
		ErrDrift, t.Env, strings.Join(differ, ", "), t.Env, t.Env)
}

// Target is where one environment keeps its objects.
type Target struct {
	Env    string // AWT_ENV
	S3     blob.S3Config
	Prefix string // normalised AWT_BUCKET_PREFIX
}

// TargetFromConfig takes an environment's object storage settings; region
// is LITESTREAM_S3_REGION (optional). Uploads must be in S3: with the fs
// backend they would live on the volume and not be copied.
func TargetFromConfig(cfg config.Config, region string) (Target, error) {
	if cfg.BlobBackend != "s3" {
		return Target{}, fmt.Errorf("%s: AWT_BLOB_BACKEND is %q, must be s3 (uploads must be in the bucket)", cfg.Env, cfg.BlobBackend)
	}
	// config.Load already requires these with the s3 backend; checked
	// again so a Target never has an empty bucket.
	for _, r := range []struct{ name, value string }{
		{"AWT_S3_ENDPOINT", cfg.S3Endpoint}, {"AWT_S3_BUCKET", cfg.S3Bucket},
		{"AWT_S3_ACCESS_KEY", cfg.S3AccessKey}, {"AWT_S3_SECRET_KEY", cfg.S3SecretKey},
	} {
		if r.value == "" {
			return Target{}, fmt.Errorf("%s: %s is required", cfg.Env, r.name)
		}
	}
	return Target{
		Env:    cfg.Env,
		Prefix: cfg.BucketPrefix,
		S3: blob.S3Config{Endpoint: cfg.S3Endpoint, Bucket: cfg.S3Bucket, AccessKey: cfg.S3AccessKey,
			SecretKey: cfg.S3SecretKey, UseSSL: true, Region: region, Prefix: cfg.BucketPrefix},
	}, nil
}

// FilesPrefix is where the environment's uploads are.
func (t Target) FilesPrefix() string { return blob.FilesPrefix(t.Prefix) }

// ReplicaPrefix is the Litestream replica's directory, with its slash so
// that it never matches a sibling key.
func (t Target) ReplicaPrefix() string {
	return config.Config{BucketPrefix: t.Prefix}.LitestreamReplicaPath() + "/"
}

// String describes the target without credentials.
func (t Target) String() string {
	p := t.Prefix
	if p == "" {
		p = "(none)"
	}
	return fmt.Sprintf("bucket %s, prefix %s, endpoint %s", t.S3.Bucket, p, t.S3.Endpoint)
}

// CheckSeparate refuses a prod and a training in one bucket whose prefixes
// are equal or contain one another. Buckets are compared by name only, and
// without case: two endpoints may reach the same bucket, so a same-named
// bucket counts as one (refusing too much is the safe side).
func CheckSeparate(prod, training Target) error {
	if !strings.EqualFold(prod.S3.Bucket, training.S3.Bucket) {
		return nil
	}
	if strings.HasPrefix(prod.Prefix, training.Prefix) || strings.HasPrefix(training.Prefix, prod.Prefix) {
		return fmt.Errorf("%w: both use bucket %s with prefixes %q and %q; give each its own AWT_BUCKET_PREFIX (for example prod/ and training/) or its own bucket",
			ErrOverlap, prod.S3.Bucket, prod.Prefix, training.Prefix)
	}
	return nil
}

// WipeResult counts the objects Wipe deleted.
type WipeResult struct{ Replica, Files int }

// Wipe deletes training's Litestream replica and uploaded files.
func Wipe(ctx context.Context, b blob.Bucket, prod, training Target) (WipeResult, error) {
	var w WipeResult
	if err := CheckSeparate(prod, training); err != nil {
		return w, err
	}
	var err error
	if w.Replica, err = blob.DeletePrefix(ctx, b, training.ReplicaPrefix()); err != nil {
		return w, fmt.Errorf("wipe training replica: %w", err)
	}
	if w.Files, err = blob.DeletePrefix(ctx, b, training.FilesPrefix()); err != nil {
		return w, fmt.Errorf("wipe training files: %w", err)
	}
	return w, nil
}

// CopyFiles copies prod's uploads to training's files prefix.
func CopyFiles(ctx context.Context, src blob.Bucket, prod Target, dst blob.Bucket, training Target) (blob.CopyResult, error) {
	if err := CheckSeparate(prod, training); err != nil {
		return blob.CopyResult{}, err
	}
	res, err := blob.CopyPrefix(ctx, src, prod.FilesPrefix(), dst, training.FilesPrefix())
	if err != nil {
		return res, fmt.Errorf("copy prod files to training: %w", err)
	}
	return res, nil
}

// Prepared reports PrepareDB.
type Prepared struct {
	Auth    store.TrainingAuthReset
	Summary gen.CloneSummaryRow
}

// PrepareDB turns a snapshot of the prod database into training's: it
// refuses a snapshot that fails PRAGMA integrity_check, migrates it to this
// binary's schema, removes sessions and second factors
// (store.ResetTrainingAuth) and leaves one self-contained file (rollback
// journal mode, no -wal beside it) to copy onto the training volume.
func PrepareDB(ctx context.Context, path string) (Prepared, error) {
	var p Prepared
	if fi, err := os.Stat(path); err != nil || !fi.Mode().IsRegular() || fi.Size() == 0 {
		return p, fmt.Errorf("snapshot %s: not a non-empty file (%v)", path, err)
	}
	if err := integrityCheck(ctx, path); err != nil {
		return p, err
	}
	db, err := store.Open(path)
	if err != nil {
		return p, err
	}
	err = func() error {
		// Migrate skips versions it does not know: a newer prod may hold
		// auth tables ResetTrainingAuth would not clear.
		unknown, err := store.UnknownMigrations(ctx, db)
		if err != nil {
			return err
		}
		if len(unknown) > 0 {
			// deploy/sync-training.sh runs this in prod's image: this
			// means prod runs an older revision than its database (a
			// rollback past a migration).
			return fmt.Errorf("the database holds migrations this image does not know (%s): it comes from a newer revision; refusing, as their tables may hold auth state the cleanup does not clear (deploy that revision first)", strings.Join(unknown, ", "))
		}
		if err := store.Migrate(ctx, db); err != nil {
			return err
		}
		if p.Auth, err = store.ResetTrainingAuth(ctx, db); err != nil {
			return err
		}
		p.Summary, err = store.CloneSummary(ctx, db)
		return err
	}()
	if err = errors.Join(err, db.Close()); err != nil {
		return p, fmt.Errorf("prepare %s: %w", path, err)
	}
	if err := rollbackJournal(ctx, path); err != nil {
		return p, err
	}
	return p, nil
}

func integrityCheck(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return fmt.Errorf("integrity check %s: %w", path, err)
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return fmt.Errorf("integrity check %s: %w", path, err)
	}
	defer rows.Close()
	var msgs []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return fmt.Errorf("integrity check %s: %w", path, err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("integrity check %s: %w", path, err)
	}
	if len(msgs) != 1 || msgs[0] != "ok" {
		return fmt.Errorf("integrity check %s failed: %s", path, strings.Join(msgs, "; "))
	}
	return nil
}

// rollbackJournal checkpoints the WAL into the file and leaves the file in
// rollback journal mode, so copying the one file copies the whole database.
// The app switches it back to WAL when it opens it.
func rollbackJournal(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return fmt.Errorf("journal mode %s: %w", path, err)
	}
	defer db.Close()
	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode = DELETE").Scan(&mode); err != nil {
		return fmt.Errorf("journal mode %s: %w", path, err)
	}
	if mode != "delete" {
		return fmt.Errorf("journal mode %s: still %q", path, mode)
	}
	return nil
}
