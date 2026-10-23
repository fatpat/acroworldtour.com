package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"awt/internal/blob"
	"awt/internal/cache"
	"awt/internal/config"
	"awt/internal/trainingsync"
)

const syncTrainingUsage = `usage: awt sync-training check|wipe-bucket|copy-files < .env.prod
       awt sync-training prepare-db <snapshot.db>
       awt sync-training purge-cdn

Internal steps of deploy/sync-training.sh, which replaces training with a
copy of prod (docs/ops.md "Refresh training from prod", decision 0018). The
bucket steps and purge-cdn run in a one-off container of the training
service: training's settings in the environment (AWT_ENV=training, else
refused), prod's env file on stdin. Nothing prints a secret.

  check        refuse unless both use S3 with every setting set and their
               prefixes are apart (one bucket: neither prefix contains the
               other); list both sides; change nothing
  wipe-bucket  delete training's <prefix>litestream/ and <prefix>files/
  copy-files   copy prod's <prefix>files/ to training's (server-side with
               the same endpoint and credentials, otherwise streamed)
  prepare-db   integrity check, migrate, then remove every session, pending
               login and second factor of a prod snapshot (needs no settings)
  purge-cdn    purge training's Cloudflare zone (skipped when unset)`

type cdnPurger interface {
	PurgeEverything(ctx context.Context) error
}

type syncDeps struct {
	getenv func(string) string
	stdin  io.Reader
	out    io.Writer
	bucket func(blob.S3Config) (blob.Bucket, error)
	purger func(zone, token string) cdnPurger
}

func defaultSyncDeps(getenv func(string) string, stdin io.Reader, out io.Writer) syncDeps {
	return syncDeps{
		getenv: getenv, stdin: stdin, out: out,
		bucket: func(c blob.S3Config) (blob.Bucket, error) { return blob.NewS3Bucket(c) },
		purger: func(zone, token string) cdnPurger { return cache.NewCloudflarePurger(zone, token) },
	}
}

// syncTrainingCmd runs one step of the training refresh.
func syncTrainingCmd(ctx context.Context, args []string, d syncDeps) error {
	if len(args) == 0 {
		return errors.New(syncTrainingUsage)
	}
	step := args[0]
	switch step {
	case "prepare-db":
		if len(args) != 2 {
			return errors.New(syncTrainingUsage)
		}
		return prepareDBStep(ctx, args[1], d.out)
	case "check", "wipe-bucket", "copy-files", "purge-cdn":
		if len(args) != 1 {
			return errors.New(syncTrainingUsage)
		}
	default:
		return errors.New(syncTrainingUsage)
	}

	training, err := config.Load(d.getenv)
	if err != nil {
		return fmt.Errorf("training environment: %w", err)
	}
	if training.Env != "training" {
		return fmt.Errorf("refused: AWT_ENV is %q; this runs in the training service only (docker compose run awt-training)", training.Env)
	}
	if step == "purge-cdn" {
		return purgeCDNStep(ctx, training, d)
	}

	prodT, trainingT, err := loadTargets(training, d)
	if err != nil {
		return err
	}
	prodB, err := d.bucket(prodT.S3)
	if err != nil {
		return fmt.Errorf("prod bucket: %w", err)
	}
	trainingB, err := d.bucket(trainingT.S3)
	if err != nil {
		return fmt.Errorf("training bucket: %w", err)
	}

	switch step {
	case "check":
		return checkStep(ctx, prodT, prodB, trainingT, trainingB, d.out)
	case "wipe-bucket":
		w, err := trainingsync.Wipe(ctx, trainingB, prodT, trainingT)
		if err != nil {
			return err
		}
		fmt.Fprintf(d.out, "training bucket: deleted %d replica objects and %d files\n", w.Replica, w.Files)
	case "copy-files":
		c, err := trainingsync.CopyFiles(ctx, prodB, prodT, trainingB, trainingT)
		if err != nil {
			return err
		}
		fmt.Fprintf(d.out, "training bucket: copied %d files (%d bytes, %s)\n", c.Objects, c.Bytes, copyMode(c.ServerSide))
	}
	return nil
}

// loadTargets reads prod's env file from stdin and checks both targets.
// Every refusal happens here, before any bucket is opened.
func loadTargets(training config.Config, d syncDeps) (prod, train trainingsync.Target, err error) {
	vars, err := config.ParseEnvFile(d.stdin)
	if err != nil {
		return prod, train, fmt.Errorf("prod env file (stdin): %w", err)
	}
	if e := vars["AWT_ENV"]; e != "" && e != "prod" {
		return prod, train, fmt.Errorf("refused: the prod env file on stdin has AWT_ENV=%s", e)
	}
	// compose.yaml sets AWT_ENV=prod for awt-prod whatever the file says.
	prodCfg, err := config.Load(func(k string) string {
		if k == "AWT_ENV" {
			return "prod"
		}
		return vars[k]
	})
	if err != nil {
		return prod, train, fmt.Errorf("prod env file (stdin): %w", err)
	}
	if prod, err = trainingsync.TargetFromConfig(prodCfg, vars["LITESTREAM_S3_REGION"]); err != nil {
		return prod, train, err
	}
	if train, err = trainingsync.TargetFromConfig(training, d.getenv("LITESTREAM_S3_REGION")); err != nil {
		return prod, train, err
	}
	if err := trainingsync.CheckSeparate(prod, train); err != nil {
		return prod, train, err
	}
	return prod, train, nil
}

// checkStep lists both sides (which also proves both credentials work)
// and says how files will be copied.
func checkStep(ctx context.Context, prodT trainingsync.Target, prodB blob.Bucket, trainingT trainingsync.Target, trainingB blob.Bucket, out io.Writer) error {
	count := func(b blob.Bucket, env, prefix string) (int, error) {
		objs, err := b.List(ctx, prefix)
		if err != nil {
			return 0, fmt.Errorf("%s bucket: %w", env, err)
		}
		return len(objs), nil
	}
	prodFiles, err := count(prodB, "prod", prodT.FilesPrefix())
	if err != nil {
		return err
	}
	trainingFiles, err := count(trainingB, "training", trainingT.FilesPrefix())
	if err != nil {
		return err
	}
	trainingReplica, err := count(trainingB, "training", trainingT.ReplicaPrefix())
	if err != nil {
		return err
	}
	c, ok := trainingB.(blob.Copier)
	fmt.Fprintf(out, "prod:     %s; %d files\n", prodT, prodFiles)
	fmt.Fprintf(out, "training: %s; %d files, %d replica objects\n", trainingT, trainingFiles, trainingReplica)
	fmt.Fprintf(out, "file copy: %s\n", copyMode(ok && c.CanCopyFrom(prodB)))
	return nil
}

func copyMode(serverSide bool) string {
	if serverSide {
		return "server-side"
	}
	return "streamed"
}

func prepareDBStep(ctx context.Context, path string, out io.Writer) error {
	p, err := trainingsync.PrepareDB(ctx, path)
	if err != nil {
		return err
	}
	a, s := p.Auth, p.Summary
	fmt.Fprintln(out, "integrity: ok")
	fmt.Fprintf(out, "removed: %d sessions, %d pending logins, %d authenticator apps, %d security keys, %d recovery codes, %d mfa lockouts\n",
		a.Sessions, a.LoginChallenges, a.TOTP, a.SecurityKeys, a.RecoveryCodes, a.MFALocks)
	fmt.Fprintf(out, "database: %d users, %d competitions, %d news, %d files\n", s.Users, s.Competitions, s.News, s.Files)
	return nil
}

func purgeCDNStep(ctx context.Context, cfg config.Config, d syncDeps) error {
	switch {
	case cfg.CloudflareZoneID == "" && cfg.CloudflareToken == "":
		fmt.Fprintln(d.out, "cdn purge: skipped (AWT_CLOUDFLARE_ZONE_ID and AWT_CLOUDFLARE_TOKEN not set for training)")
		return nil
	case cfg.CloudflareZoneID == "" || cfg.CloudflareToken == "":
		return errors.New("cdn purge: set both AWT_CLOUDFLARE_ZONE_ID and AWT_CLOUDFLARE_TOKEN for training, or neither")
	}
	if err := d.purger(cfg.CloudflareZoneID, cfg.CloudflareToken).PurgeEverything(ctx); err != nil {
		return err
	}
	fmt.Fprintln(d.out, "cdn purge: purged everything in training's Cloudflare zone")
	return nil
}
