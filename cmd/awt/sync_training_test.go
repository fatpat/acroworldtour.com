package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"awt/internal/blob"
	"awt/internal/store"
	"awt/internal/trainingsync"
)

type memObj struct {
	data []byte
	ct   string
}

// memBuckets is every bucket of a fake S3 server, by name.
type memBuckets map[string]map[string]memObj

type memBucket struct{ objs map[string]memObj }

func (b memBucket) List(_ context.Context, prefix string) ([]blob.Object, error) {
	var out []blob.Object
	for _, k := range slices.Sorted(maps.Keys(b.objs)) {
		if strings.HasPrefix(k, prefix) {
			out = append(out, blob.Object{Key: k, Size: int64(len(b.objs[k].data))})
		}
	}
	return out, nil
}
func (b memBucket) Open(_ context.Context, k string) (io.ReadCloser, string, error) {
	o, ok := b.objs[k]
	if !ok {
		return nil, "", blob.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(o.data)), o.ct, nil
}
func (b memBucket) Write(_ context.Context, k, ct string, r io.Reader, _ int64) error {
	d, err := io.ReadAll(r)
	b.objs[k] = memObj{d, ct}
	return err
}
func (b memBucket) Remove(_ context.Context, keys []string) error {
	for _, k := range keys {
		delete(b.objs, k)
	}
	return nil
}

const prodEnv = `# .env.prod
AWT_ENV=prod
AWT_BASE_URL=https://www.example.org
AWT_S3_ENDPOINT=https://s3.gra.io.cloud.ovh.net
AWT_S3_BUCKET=awt
AWT_BUCKET_PREFIX=prod/
AWT_S3_ACCESS_KEY=prod-access
AWT_S3_SECRET_KEY=prod-secret-value
AWT_BLOB_BACKEND=s3
`

func trainingEnv(over map[string]string) func(string) string {
	m := map[string]string{
		"AWT_ENV": "training", "AWT_BASE_URL": "https://training.example.org",
		"AWT_S3_ENDPOINT": "https://s3.gra.io.cloud.ovh.net", "AWT_S3_BUCKET": "awt",
		"AWT_BUCKET_PREFIX": "training/", "AWT_S3_ACCESS_KEY": "training-access",
		"AWT_S3_SECRET_KEY": "training-secret-value", "AWT_BLOB_BACKEND": "s3",
	}
	maps.Copy(m, over)
	return func(k string) string { return m[k] }
}

type fakePurger struct {
	calls int
	hosts []string
}

func (p *fakePurger) PurgeHosts(_ context.Context, hosts []string) error {
	p.calls++
	p.hosts = hosts
	return nil
}

// liveProd is what the script reads from the running awt-prod (docker
// inspect), matching prodEnv.
var liveProd = []string{"--live-prod-endpoint=https://s3.gra.io.cloud.ovh.net", "--live-prod-bucket=awt", "--live-prod-prefix=prod/"}

func step(name string, extra ...string) []string {
	return append(append([]string{name}, liveProd...), extra...)
}

func run(t *testing.T, args []string, getenv func(string) string, stdin string, server memBuckets, purger *fakePurger) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := syncTrainingCmd(context.Background(), args, syncDeps{
		getenv: getenv,
		stdin:  strings.NewReader(stdin),
		out:    &out,
		bucket: func(c blob.S3Config) (blob.Bucket, error) {
			if server == nil {
				t.Fatalf("%v: bucket opened", args)
			}
			if server[c.Bucket] == nil {
				server[c.Bucket] = map[string]memObj{}
			}
			return memBucket{server[c.Bucket]}, nil
		},
		purger: func(zone, token string) cdnPurger {
			if zone != "zone" || token != "token" {
				t.Errorf("purger(%q, %q)", zone, token)
			}
			return purger
		},
	})
	if strings.Contains(out.String(), "secret-value") || (err != nil && strings.Contains(err.Error(), "secret-value")) {
		t.Errorf("%v: a secret was printed", args)
	}
	return out.String(), err
}

func TestSyncTrainingUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"frobnicate"}, {"prepare-db"}, {"prepare-db", "a", "b"}, {"check", "x"}} {
		if _, err := run(t, args, trainingEnv(nil), prodEnv, nil, nil); err == nil || !strings.Contains(err.Error(), "usage") {
			t.Errorf("%v: err = %v", args, err)
		}
	}
}

// Every refusal happens before any bucket is opened (run fails the test if
// one is).
func TestSyncTrainingRefuses(t *testing.T) {
	cases := []struct {
		name   string
		getenv func(string) string
		stdin  string
		want   string
	}{
		{"not the training service", trainingEnv(map[string]string{"AWT_ENV": "prod"}), prodEnv, "AWT_ENV"},
		{"same prefix", trainingEnv(map[string]string{"AWT_BUCKET_PREFIX": "prod"}), prodEnv, "overlap"},
		{"training inside prod", trainingEnv(map[string]string{"AWT_BUCKET_PREFIX": "prod/training/"}), prodEnv, "overlap"},
		{"empty training prefix in the shared bucket", trainingEnv(map[string]string{"AWT_BUCKET_PREFIX": ""}), prodEnv, "overlap"},
		{"prod file is training's", trainingEnv(nil), strings.Replace(prodEnv, "AWT_ENV=prod", "AWT_ENV=training", 1), "AWT_ENV"},
		{"prod bucket missing", trainingEnv(nil), strings.Replace(prodEnv, "AWT_S3_BUCKET=awt", "AWT_S3_BUCKET=", 1), "AWT_S3_BUCKET"},
		{"training key missing", trainingEnv(map[string]string{"AWT_S3_SECRET_KEY": ""}), prodEnv, "AWT_S3_SECRET_KEY"},
		{"training on fs", trainingEnv(map[string]string{"AWT_BLOB_BACKEND": "fs"}), prodEnv, "AWT_BLOB_BACKEND"},
		{"empty stdin", trainingEnv(nil), "", "prod"},
		{"bad prod prefix", trainingEnv(nil), strings.Replace(prodEnv, "prod/", "../prod", 1), "AWT_BUCKET_PREFIX"},
	}
	for _, c := range cases {
		for _, name := range []string{"check", "wipe-bucket", "copy-files"} {
			_, err := run(t, step(name), c.getenv, c.stdin, nil, nil)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("%s, %s: err = %v, want one mentioning %q", c.name, name, err, c.want)
			}
		}
	}
	_, err := run(t, step("check"), trainingEnv(map[string]string{"AWT_BUCKET_PREFIX": "prod"}), prodEnv, nil, nil)
	if !errors.Is(err, trainingsync.ErrOverlap) {
		t.Errorf("overlap: %v", err)
	}
}

// The overlap check judges the env files: it must refuse when a running
// container does not use its file's settings. The case that matters: .env.prod
// was moved to a new bucket but awt-prod still writes awt with no prefix,
// where an empty-prefix training in bucket awt would be wiped.
func TestSyncTrainingRefusesDrift(t *testing.T) {
	movedProd := strings.Replace(prodEnv, "AWT_S3_BUCKET=awt", "AWT_S3_BUCKET=awt-new", 1)
	rootTraining := trainingEnv(map[string]string{"AWT_BUCKET_PREFIX": ""})
	oldLive := []string{"--live-prod-endpoint=https://s3.gra.io.cloud.ovh.net", "--live-prod-bucket=awt", "--live-prod-prefix="}
	liveTraining := []string{"--live-training-endpoint=https://s3.gra.io.cloud.ovh.net", "--live-training-bucket=awt", "--live-training-prefix=training/"}
	for _, name := range []string{"check", "wipe-bucket", "copy-files"} {
		_, err := run(t, append([]string{name}, oldLive...), rootTraining, movedProd, nil, nil)
		if !errors.Is(err, trainingsync.ErrDrift) || !strings.Contains(err.Error(), "AWT_S3_BUCKET") {
			t.Errorf("%s, prod moved in its file only: err = %v, want ErrDrift naming AWT_S3_BUCKET", name, err)
		}
		// Training running with another prefix than its file.
		drifted := append(step(name), strings.Replace(liveTraining[2], "training/", "other/", 1), liveTraining[0], liveTraining[1])
		if _, err := run(t, drifted, trainingEnv(nil), prodEnv, nil, nil); !errors.Is(err, trainingsync.ErrDrift) || !strings.Contains(err.Error(), "awt-training") {
			t.Errorf("%s, training drifted: err = %v", name, err)
		}
		// Prod not running: no live values to compare.
		if _, err := run(t, []string{name}, trainingEnv(nil), prodEnv, nil, nil); err == nil || !strings.Contains(err.Error(), "awt-prod is not running") {
			t.Errorf("%s, no live prod: err = %v", name, err)
		}
		// Half the training values: a script bug, not a stopped container.
		if _, err := run(t, append(step(name), liveTraining[0]), trainingEnv(nil), prodEnv, nil, nil); err == nil || !strings.Contains(err.Error(), "usage") {
			t.Errorf("%s, partial training values: err = %v", name, err)
		}
	}
}

func TestSyncTrainingBucketSteps(t *testing.T) {
	server := memBuckets{"awt": {
		"prod/files/aa":                     {[]byte("A"), "image/png"},
		"prod/files/bb":                     {[]byte("BB"), "application/pdf"},
		"prod/litestream/generations/g1/x":  {[]byte("p"), ""},
		"training/files/trainee":            {[]byte("T"), "image/jpeg"},
		"training/litestream/generations/x": {[]byte("t"), ""},
	}}
	liveTraining := []string{"--live-training-endpoint=https://s3.gra.io.cloud.ovh.net", "--live-training-bucket=awt", "--live-training-prefix=training"}
	out, err := run(t, step("check", liveTraining...), trainingEnv(nil), prodEnv, server, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"prod:", "bucket awt, prefix prod/", "2 files", "training:", "1 files, 1 replica objects", "streamed"} {
		if !strings.Contains(out, want) {
			t.Errorf("check output lacks %q:\n%s", want, out)
		}
	}
	if len(server["awt"]) != 5 {
		t.Fatal("check changed the bucket")
	}

	out, err = run(t, step("wipe-bucket"), trainingEnv(nil), prodEnv, server, nil)
	if err != nil || !strings.Contains(out, "deleted 1 replica objects and 1 files") {
		t.Fatalf("wipe: %v\n%s", err, out)
	}
	out, err = run(t, step("copy-files"), trainingEnv(nil), prodEnv, server, nil)
	if err != nil || !strings.Contains(out, "copied 2 files (3 bytes, streamed)") {
		t.Fatalf("copy: %v\n%s", err, out)
	}
	want := []string{"prod/files/aa", "prod/files/bb", "prod/litestream/generations/g1/x", "training/files/aa", "training/files/bb"}
	if got := slices.Sorted(maps.Keys(server["awt"])); !slices.Equal(got, want) {
		t.Errorf("bucket %v\nwant %v", got, want)
	}
}

func TestSyncTrainingPrepareDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prod.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.Exec(`INSERT INTO users (id, email, name, password_hash, role) VALUES ('u1', 'a@x.org', 'A', 'h', 'admin');
		INSERT INTO sessions (token_hash, user_id, csrf_token, expires_at, created_at) VALUES ('s', 'u1', 'c', '2099-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	// Needs no environment: it only touches the file.
	out, err := run(t, []string{"prepare-db", path}, func(string) string { return "" }, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"integrity: ok", "1 sessions", "1 users, 0 competitions, 0 news, 0 files"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestSyncTrainingPurgeCDN(t *testing.T) {
	p := &fakePurger{}
	out, err := run(t, []string{"purge-cdn"}, trainingEnv(nil), "", nil, p)
	if err != nil || p.calls != 0 || !strings.Contains(out, "skipped") {
		t.Errorf("unset: %v %d %q", err, p.calls, out)
	}
	set := trainingEnv(map[string]string{"AWT_CLOUDFLARE_ZONE_ID": "zone", "AWT_CLOUDFLARE_TOKEN": "token"})
	out, err = run(t, []string{"purge-cdn"}, set, "", nil, p)
	if err != nil || p.calls != 1 || !slices.Equal(p.hosts, []string{"training.example.org"}) || !strings.Contains(out, "purged training.example.org") {
		t.Errorf("set: %v %d %v %q", err, p.calls, p.hosts, out)
	}
	half := trainingEnv(map[string]string{"AWT_CLOUDFLARE_ZONE_ID": "zone"})
	if _, err := run(t, []string{"purge-cdn"}, half, "", nil, p); err == nil || !strings.Contains(err.Error(), "AWT_CLOUDFLARE_TOKEN") {
		t.Errorf("half set: %v", err)
	}
	if _, err := run(t, []string{"purge-cdn"}, trainingEnv(map[string]string{"AWT_ENV": "prod"}), "", nil, p); err == nil {
		t.Error("purge-cdn ran outside training")
	}
}
