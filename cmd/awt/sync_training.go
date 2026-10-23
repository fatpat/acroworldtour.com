package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"

	"awt/internal/blob"
	"awt/internal/cache"
	"awt/internal/config"
	"awt/internal/trainingsync"
)

const syncTrainingUsage = `usage: awt sync-training check|wipe-bucket|copy-files LIVE < .env.prod
       awt sync-training prepare-db <snapshot.db>
       awt sync-training purge-cdn
LIVE: --live-prod-endpoint=V --live-prod-bucket=V --live-prod-prefix=V
      [--live-training-endpoint=V --live-training-bucket=V --live-training-prefix=V]

Internal steps of deploy/sync-training.sh, which replaces training with a
copy of prod (docs/ops.md "Refresh training from prod", decision 0018). The
bucket steps and purge-cdn run in a one-off container of the training
service: training's settings in the environment (AWT_ENV=training, else
refused), prod's env file on stdin. LIVE are AWT_S3_ENDPOINT, AWT_S3_BUCKET
and AWT_BUCKET_PREFIX of the running awt-prod (required: prod must run) and
awt-training (when it runs), from docker inspect: a running container that
does not use its env file's values is refused. Nothing prints a secret.

  check        refuse unless both use S3 with every setting set, the
               running containers use their files' settings and the
               prefixes are apart (one bucket: neither prefix contains the
               other); list both sides; change nothing
  wipe-bucket  delete training's <prefix>litestream/ and <prefix>files/
  copy-files   copy prod's <prefix>files/ to training's (server-side with
               the same endpoint and credentials, otherwise streamed)
  prepare-db   integrity check, migrate, then remove every session, pending
               login and second factor of a prod snapshot (needs no settings)
  purge-cdn    purge training's host (AWT_BASE_URL) in its Cloudflare zone
               (skipped when the zone and token are unset)`

type cdnPurger interface {
	PurgeHosts(ctx context.Context, hosts []string) error
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
	case "purge-cdn":
		if len(args) != 1 {
			return errors.New(syncTrainingUsage)
		}
	case "check", "wipe-bucket", "copy-files":
	default:
		return errors.New(syncTrainingUsage)
	}
	var live liveValues
	if step != "purge-cdn" {
		var err error
		if live, err = parseLive(args[1:]); err != nil {
			return err
		}
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

	prodT, trainingT, err := loadTargets(training, live, d)
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

// liveValues are the running containers' object storage settings.
type liveValues struct {
	prod     *trainingsync.Live
	training *trainingsync.Live // nil: awt-training is not running
}

// parseLive reads the --live-* flags: all three of an environment, or
// none (that container is not running).
func parseLive(args []string) (liveValues, error) {
	fs := flag.NewFlagSet("sync-training", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var p, t trainingsync.Live
	fs.StringVar(&p.Endpoint, "live-prod-endpoint", "", "")
	fs.StringVar(&p.Bucket, "live-prod-bucket", "", "")
	fs.StringVar(&p.Prefix, "live-prod-prefix", "", "")
	fs.StringVar(&t.Endpoint, "live-training-endpoint", "", "")
	fs.StringVar(&t.Bucket, "live-training-bucket", "", "")
	fs.StringVar(&t.Prefix, "live-training-prefix", "", "")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		return liveValues{}, errors.New(syncTrainingUsage)
	}
	set := map[string]int{}
	fs.Visit(func(f *flag.Flag) { set[strings.SplitN(f.Name, "-", 3)[1]]++ })
	var lv liveValues
	for env, n := range set {
		if n != 3 {
			return liveValues{}, fmt.Errorf("%s: give all three --live-%s-* values or none", syncTrainingUsage, env)
		}
	}
	if set["prod"] == 3 {
		lv.prod = &p
	}
	if set["training"] == 3 {
		lv.training = &t
	}
	return lv, nil
}

// loadTargets reads prod's env file from stdin and checks both targets
// against each other and against the running containers. Every refusal
// happens here, before any bucket is opened.
func loadTargets(training config.Config, live liveValues, d syncDeps) (prod, train trainingsync.Target, err error) {
	if live.prod == nil {
		return prod, train, errors.New("refused: awt-prod is not running (no live settings given): start prod first; the check compares what it runs with .env.prod")
	}
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
	// Prod's file must be what runs before it can judge the overlap; an
	// overlap of the files is reported before a training drift.
	if err := trainingsync.CheckLive(prod, *live.prod); err != nil {
		return prod, train, err
	}
	if err := trainingsync.CheckSeparate(prod, train); err != nil {
		return prod, train, err
	}
	if live.training != nil {
		if err := trainingsync.CheckLive(train, *live.training); err != nil {
			return prod, train, err
		}
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

// purgeCDNStep purges training's host only: prod may share the zone.
func purgeCDNStep(ctx context.Context, cfg config.Config, d syncDeps) error {
	switch {
	case cfg.CloudflareZoneID == "" && cfg.CloudflareToken == "":
		fmt.Fprintln(d.out, "cdn purge: skipped (AWT_CLOUDFLARE_ZONE_ID and AWT_CLOUDFLARE_TOKEN not set for training)")
		return nil
	case cfg.CloudflareZoneID == "" || cfg.CloudflareToken == "":
		return errors.New("cdn purge: set both AWT_CLOUDFLARE_ZONE_ID and AWT_CLOUDFLARE_TOKEN for training, or neither")
	}
	u, err := url.Parse(cfg.BaseURL) // validated by config.Load
	if err != nil || u.Hostname() == "" {
		return errors.New("cdn purge: AWT_BASE_URL has no host")
	}
	host := u.Hostname()
	if err := d.purger(cfg.CloudflareZoneID, cfg.CloudflareToken).PurgeHosts(ctx, []string{host}); err != nil {
		return err
	}
	fmt.Fprintf(d.out, "cdn purge: purged %s\n", host)
	return nil
}
