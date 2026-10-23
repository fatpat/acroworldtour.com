#!/bin/sh
# Training refresh test: deploy/sync-training.sh against a temporary
# /opt/awt (the real compose.yaml, awt-prod and awt-training on the awt:dev
# image) and a local MinIO standing in for OVHcloud Object Storage.
# systemctl is a stub; the agent lock is a temporary file.
#
# Cases: refusals that change nothing (no terminal without --yes, prefixes
# overlapping, a missing setting); a run failing after the wipe, then a
# re-run (agent lock held at first); a shared bucket (prefixes prod/ and
# training/, server-side copy); separate buckets on different endpoints
# with an empty training prefix (streamed copy). After each refresh:
# training holds exactly prod's data and files, no trainee leftovers, no
# session or second factor, a new replica instead of the old one; prod's
# data, files and replica are unchanged.
# Run via `make sync-training-test` (it builds awt:dev first; needs Docker,
# not root; takes a few minutes).
set -eu
cd "$(dirname "$0")/.."
repo=$(pwd)

export COMPOSE_PROJECT_NAME=awt-sync-test
# aws-cli, pinned by digest (the image docs/ops.md section 6 uses).
AWSCLI=amazon/aws-cli@sha256:e3e329e1d2894b7b4bbb0aacacd0a155262159b2e7a3b4275eb1f24046d3e06c
HEALTH_TIMEOUT=${HEALTH_TIMEOUT:-90}
PASS='correct-horse-battery-staple-42'

root=$(mktemp -d)
work="$root/work"
mkdir -p "$root/deploy" "$root/bin" "$work"
failures=0

dc() { (cd "$root" && docker compose "$@"); }
cleanup() {
  dc --profile test down -v --remove-orphans >/dev/null 2>&1 || true
  docker volume rm -f "${COMPOSE_PROJECT_NAME}_sync_training_stage" >/dev/null 2>&1 || true
  rm -rf "$root"
}
trap cleanup EXIT

ok() { echo "  ok: $*"; }
bad() { echo "  FAIL: $*" >&2; failures=$((failures + 1)); }
check() { # check <description> <command...>
  d=$1; shift
  if "$@"; then ok "$d"; else bad "$d"; fi
}

# --- the temporary /opt/awt --------------------------------------------------
cp compose.yaml "$root/compose.yaml"
cp deploy/sync-training.sh "$root/deploy/sync-training.sh"
# A second name for MinIO: an endpoint that differs from prod's, so the copy
# cannot be server-side (separate buckets and credentials on the VPS).
cat > "$root/compose.override.yaml" <<'EOF'
services:
  minio:
    networks:
      edge:
        aliases: [minio-alt]
EOF
cat > "$root/.env" <<'EOF'
AWT_IMAGE=awt
AWT_PROD_TAG=dev
AWT_TRAINING_TAG=dev
AWT_PROD_HOST=prod.invalid
AWT_TRAINING_HOST=training.invalid
EOF
mfa_key() { head -c 32 /dev/urandom | base64; }
cat > "$root/.env.prod" <<EOF
AWT_ENV=prod
AWT_BASE_URL=http://prod.invalid
AWT_BLOB_BACKEND=s3
AWT_S3_ENDPOINT=http://minio:9000
AWT_S3_BUCKET=awt-test
AWT_BUCKET_PREFIX=prod/
AWT_S3_ACCESS_KEY=awt-test
AWT_S3_SECRET_KEY=awt-test-secret
AWT_MFA_KEY=$(mfa_key)
EOF
training_env() { # training_env <endpoint> <bucket> <prefix>
  cat > "$root/.env.training" <<EOF
AWT_ENV=training
AWT_BASE_URL=http://training.invalid
AWT_BLOB_BACKEND=s3
AWT_S3_ENDPOINT=$1
AWT_S3_BUCKET=$2
AWT_BUCKET_PREFIX=$3
AWT_S3_ACCESS_KEY=awt-test
AWT_S3_SECRET_KEY=awt-test-secret
AWT_MFA_KEY=$training_mfa
EOF
}
training_mfa=$(mfa_key)
training_env http://minio:9000 awt-test training/

# systemctl stub: records calls, keeps the timer state in a file.
cat > "$root/bin/systemctl" <<'EOF'
#!/bin/sh
echo "$*" >> "$STUB_DIR/calls"
case "$1" in
  is-active) [ "$(cat "$STUB_DIR/timer")" = active ] ;;
  stop) echo inactive > "$STUB_DIR/timer" ;;
  start) echo active > "$STUB_DIR/timer" ;;
esac
EOF
chmod +x "$root/bin/systemctl"
export STUB_DIR="$root"
echo active > "$root/timer"
: > "$root/calls"
: > "$root/agent.lock"

# run_sync [args]: the script as on the VPS; output in $out, exit in $rc.
run_sync() {
  set +e
  out=$(AWT_LOCK="$root/agent.lock" SYSTEMCTL="$root/bin/systemctl" HEALTH_TIMEOUT="$HEALTH_TIMEOUT" \
    AWT_LOCK_TIMEOUT=60 "$root/deploy/sync-training.sh" "$@" 2>&1)
  rc=$?
  set -e
  printf '%s\n' "$out" | sed 's/^/    | /'
  echo "    (exit $rc)"
}
has() { printf '%s\n' "$out" | grep -q -- "$1"; }

# --- helpers: S3 and SQLite ------------------------------------------------------
aws() {
  docker run --rm --network "${COMPOSE_PROJECT_NAME}_edge" -v "$work:/work" \
    -e AWS_ACCESS_KEY_ID=awt-test -e AWS_SECRET_ACCESS_KEY=awt-test-secret -e AWS_DEFAULT_REGION=us-east-1 \
    "$AWSCLI" --endpoint-url http://minio:9000 "$@"
}
# objects <bucket> <prefix>: "<key without prefix> <etag>" lines, sorted.
objects() {
  aws s3api list-objects-v2 --bucket "$1" --prefix "$2" --query 'Contents[].[Key,ETag]' --output text \
    | grep -v '^None$' | sed "s|^$2||" | sort || true
}
put() { # put <bucket> <key> <content-type> <content>
  printf '%s' "$4" > "$work/obj"
  aws s3 cp /work/obj "s3://$1/$2" --content-type "$3" >/dev/null
}
content_type() { aws s3api head-object --bucket "$1" --key "$2" --query ContentType --output text; }
sha() { printf '%s' "$1" | sha256sum | cut -c1-64; }
# generations <bucket> <replica prefix>: Litestream generation ids.
generations() { objects "$1" "$2generations/" | cut -d/ -f1 | sort -u; }

# Per table: row count and a hash of the sorted rows, for every table but
# Litestream's and the auth tables (users without the MFA failure columns).
cat > "$work/fingerprint.sh" <<'EOF'
db=/data/awt.db
for t in $(sqlite3 "$db" "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '_litestream_%' ORDER BY name"); do
  case "$t" in
    sessions|login_challenges|user_totp|user_webauthn_credentials|user_recovery_codes) continue ;;
    users) q="SELECT id, email, name, password_hash, role, disabled FROM users" ;;
    *) q="SELECT * FROM \"$t\"" ;;
  esac
  echo "$t $(sqlite3 "$db" "SELECT count(*) FROM \"$t\"") $(sqlite3 -cmd '.mode quote' "$db" "$q" | sort | sha256sum | cut -c1-16)"
done
EOF
fingerprint() { dc exec -T "$1" sh -s < "$work/fingerprint.sh"; }
sql() { dc exec -T "$1" sqlite3 -cmd '.timeout 5000' /data/awt.db "$2"; }
auth_rows() {
  sql "$1" "SELECT (SELECT count(*) FROM sessions) || ' ' || (SELECT count(*) FROM login_challenges) || ' ' ||
    (SELECT count(*) FROM user_totp) || ' ' || (SELECT count(*) FROM user_webauthn_credentials) || ' ' ||
    (SELECT count(*) FROM user_recovery_codes) || ' ' ||
    (SELECT count(*) FROM users WHERE mfa_failed_count <> 0 OR mfa_locked_until IS NOT NULL)"
}
ctr_id() { dc ps -q "$1" 2>/dev/null || true; }

wait_healthy() {
  for svc in "$@"; do
    i=0
    until cid=$(ctr_id "$svc") && [ -n "$cid" ] && [ "$(docker inspect -f '{{.State.Health.Status}}' "$cid")" = healthy ]; do
      i=$((i + 1)); [ "$i" -lt 60 ] || { dc logs "$svc" | tail -30 >&2; echo "$svc not healthy" >&2; exit 1; }
      sleep 2
    done
  done
}
# wait_new_generation <bucket> <replica prefix> <old generations>
wait_new_generation() {
  i=0
  while [ "$i" -lt 30 ]; do
    now=$(generations "$1" "$2")
    for g in $now; do
      printf '%s\n' "$3" | grep -qx "$g" || return 0
    done
    i=$((i + 1)); sleep 2
  done
  return 1
}

# --- setup -------------------------------------------------------------------------
echo "== start minio, prod and training"
dc --profile test up -d minio
i=0; until aws s3api head-bucket --bucket awt-test >/dev/null 2>&1; do i=$((i + 1)); [ "$i" -lt 30 ] || exit 1; sleep 1; done
dc up -d awt-prod awt-training
wait_healthy awt-prod awt-training

echo "== prod data: an admin, a competition, news, two files, sessions and second factors"
printf '%s\n' "$PASS" | dc exec -T awt-prod awt user add --email admin@example.org --name Admin --role admin >/dev/null
put awt-test "prod/files/$(sha png-content)" image/png png-content
put awt-test "prod/files/$(sha pdf-content)" application/pdf pdf-content
sql awt-prod "
  INSERT INTO files (id, filename, content_type, size, created_at) VALUES
    ('$(sha png-content)', 'logo.png', 'image/png', 11, '2026-10-01T00:00:00Z'),
    ('$(sha pdf-content)', 'rules.pdf', 'application/pdf', 11, '2026-10-01T00:00:00Z');
  INSERT INTO competitions (id, code, name, type, start_date, end_date, config, logo_file)
    VALUES ('c1', 'awt-test', 'AWT Test', 'solo', '2026-06-01', '2026-06-03', '{}', '$(sha png-content)');
  INSERT INTO news (id, slug, title) VALUES ('n1', 'prod-news', 'Prod news');
  UPDATE users SET mfa_failed_count = 2, mfa_locked_until = '2099-01-01T00:00:00Z';
  INSERT INTO sessions (token_hash, user_id, csrf_token, expires_at, created_at)
    SELECT 'prod-session', id, 'c', '2099-01-01T00:00:00Z', '2026-10-01T00:00:00Z' FROM users;
  INSERT INTO login_challenges (token_hash, user_id, kind, expires_at)
    SELECT 'prod-challenge', id, 'pending_login', '2099-01-01T00:00:00Z' FROM users;
  INSERT INTO user_totp (user_id, secret_enc, created_at) SELECT id, 'enc', '2026-10-01T00:00:00Z' FROM users;
  INSERT INTO user_webauthn_credentials (id, user_id, name, credential, created_at)
    SELECT 'key1', id, 'Key', '{}', '2026-10-01T00:00:00Z' FROM users;
  INSERT INTO user_recovery_codes (user_id, code_hash) SELECT id, 'code1' FROM users;"

echo "== training data: a trainee, trainee news and files"
printf '%s\n' "$PASS" | dc exec -T awt-training awt user add --email trainee@example.org --name Trainee --role scorer >/dev/null
put awt-test "training/files/$(sha trainee-content)" image/jpeg trainee-content
put awt-test "training/files/$(sha png-content)" image/png png-content
sql awt-training "
  INSERT INTO news (id, slug, title) VALUES ('t1', 'trainee-news', 'Trainee news');
  INSERT INTO sessions (token_hash, user_id, csrf_token, expires_at, created_at)
    SELECT 'trainee-session', id, 'c', '2099-01-01T00:00:00Z', '2026-10-01T00:00:00Z' FROM users;"
# Both replicas must hold a generation before the refresh.
wait_new_generation awt-test prod/litestream/ "" || { echo "no prod replica" >&2; exit 1; }
wait_new_generation awt-test training/litestream/ "" || { echo "no training replica" >&2; exit 1; }
sleep 3   # let Litestream ship the writes above

prod_fp=$(fingerprint awt-prod)
prod_auth=$(auth_rows awt-prod)
prod_files=$(objects awt-test prod/files/)
prod_replica=$(objects awt-test prod/litestream/)
old_gens=$(generations awt-test training/litestream/)
training_ctr=$(ctr_id awt-training)
training_fp=$(fingerprint awt-training)
check "fixture: prod has 2 files, 1 news, every auth table filled" \
  eval '[ "$(printf "%s\n" "$prod_files" | grep -c .)" = 2 ] && printf "%s\n" "$prod_fp" | grep -q "^news 1 " && [ "$prod_auth" = "1 1 1 1 1 1" ]'

# training_unchanged: the refusals must leave training running as it was.
training_unchanged() {
  [ "$(ctr_id awt-training)" = "$training_ctr" ] && [ "$(fingerprint awt-training)" = "$training_fp" ] \
    && [ "$(objects awt-test training/files/ | grep -c .)" = 2 ] && [ "$(cat "$root/timer")" = active ] \
    && ! grep -q '^stop' "$root/calls"
}

# --- refusals --------------------------------------------------------------------
echo "== refused: no terminal and no --yes"
run_sync < /dev/null
check "exit 2, says to pass --yes" eval '[ "$rc" = 2 ] && has "pass --yes" && has "nothing was changed"'
check "nothing changed" training_unchanged

echo "== refused: a terminal, answer other than \"training\""
if command -v script >/dev/null 2>&1; then
  set +e
  out=$(printf 'prod\n' | AWT_LOCK="$root/agent.lock" SYSTEMCTL="$root/bin/systemctl" \
    script -qec "$root/deploy/sync-training.sh" /dev/null 2>&1)
  rc=$?
  set -e
  printf '%s\n' "$out" | sed 's/^/    | /'
  check "asked, exit 2, not confirmed" eval '[ "$rc" = 2 ] && has "Type \"training\" to continue" && has "not confirmed"'
  check "nothing changed" training_unchanged
else
  echo "  skipped: no script(1) for a pseudo-terminal"
fi

echo "== refused: training prefix inside prod's in the same bucket"
training_env http://minio:9000 awt-test prod/training/
run_sync --yes
check "exit 2, names the overlap" eval '[ "$rc" = 2 ] && has "overlap" && has "nothing was changed"'
check "nothing changed" training_unchanged

echo "== refused: same prefix as prod"
training_env http://minio:9000 awt-test prod
run_sync --yes
check "exit 2, names the overlap" eval '[ "$rc" = 2 ] && has "overlap"'
check "nothing changed" training_unchanged

echo "== refused: a missing setting"
training_env http://minio:9000 "" training/
run_sync --yes
check "exit 2, names AWT_S3_BUCKET" eval '[ "$rc" = 2 ] && has "AWT_S3_BUCKET"'
check "nothing changed" training_unchanged
training_env http://minio:9000 awt-test training/

# --- a run failing midway, then a re-run -------------------------------------------
echo "== run failing after the wipe"
set +e
out=$(AWT_SYNC_TEST_FAIL_AFTER=wipe AWT_LOCK="$root/agent.lock" SYSTEMCTL="$root/bin/systemctl" \
  "$root/deploy/sync-training.sh" --yes 2>&1)
rc=$?
set -e
printf '%s\n' "$out" | sed 's/^/    | /'
check "exit 1, says to run it again" eval '[ "$rc" = 1 ] && has "FAILED" && has "run this script again"'
check "timer restarted" eval '[ "$(cat "$root/timer")" = active ] && grep -q "^start" "$root/calls"'
check "staging volume removed" eval '! docker volume inspect ${COMPOSE_PROJECT_NAME}_sync_training_stage >/dev/null 2>&1'
check "training is down and wiped" eval '[ -z "$(ctr_id awt-training)" ] && [ -z "$(objects awt-test training/files/)" ] && [ -z "$(objects awt-test training/litestream/)" ]'
check "prod untouched" eval '[ "$(fingerprint awt-prod)" = "$prod_fp" ] && [ "$(objects awt-test prod/files/)" = "$prod_files" ]'

trainee_rows() {
  sql awt-training "SELECT (SELECT count(*) FROM users WHERE email = 'trainee@example.org') + (SELECT count(*) FROM news WHERE slug = 'trainee-news')"
}
prod_replica_kept() { # every prod replica object from before is still there, unchanged
  now=$(objects awt-test prod/litestream/)
  [ -z "$(printf '%s\n' "$prod_replica" | grep . | grep -vxF "$now")" ]
}

# after_refresh <bucket> <files prefix> <replica prefix> <old generations>
after_refresh() {
  ar_bucket=$1 ar_files=$2 ar_replica=$3 ar_old=$4
  check "exit 0" [ "$rc" = 0 ]
  check "training has exactly prod's data" eval '[ "$(fingerprint awt-training)" = "$prod_fp" ]'
  check "no trainee leftovers" eval '[ "$(trainee_rows)" = 0 ]'
  check "no session, pending login, second factor or MFA lock on training" eval '[ "$(auth_rows awt-training)" = "0 0 0 0 0 0" ]'
  check "training's files are prod's files" eval '[ "$(objects "$ar_bucket" "$ar_files")" = "$prod_files" ]'
  check "content types kept" eval '[ "$(content_type "$ar_bucket" "$ar_files$(sha pdf-content)")" = application/pdf ]'
  check "old training replica gone, a new one started" eval 'wait_new_generation "$ar_bucket" "$ar_replica" "$ar_old" && ! generations "$ar_bucket" "$ar_replica" | grep -qxF "$ar_old"'
  check "prod data, auth rows and files unchanged" eval '[ "$(fingerprint awt-prod)" = "$prod_fp" ] && [ "$(auth_rows awt-prod)" = "$prod_auth" ] && [ "$(objects awt-test prod/files/)" = "$prod_files" ]'
  check "prod replica: nothing removed or changed" prod_replica_kept
  check "staging volume removed" eval '! docker volume inspect ${COMPOSE_PROJECT_NAME}_sync_training_stage >/dev/null 2>&1'
  check "summary printed" eval 'has "Training refreshed from prod" && has "1 users, 1 competitions, 1 news, 2 files"'
}

echo "== re-run, shared bucket (prod/ and training/), agent lock held for 5 s"
: > "$root/calls"
flock "$root/agent.lock" sleep 5 &
locker=$!
sleep 1
run_sync --yes
wait "$locker" || true
after_refresh awt-test training/files/ training/litestream/ "$old_gens"
check "timer back to active" eval '[ "$(cat "$root/timer")" = active ]'
check "waited for the agent lock" has "waiting for the running deploy"
check "timer stopped, then started" eval '[ "$(grep -E "^(stop|start)" "$root/calls" | tr "\n" " ")" = "stop awt-agent.timer start awt-agent.timer " ]'
check "server-side copy" has "server-side"
check "removed prod's sessions and factors in the copy" has "removed: 1 sessions, 1 pending logins, 1 authenticator apps, 1 security keys, 1 recovery codes, 1 mfa lockouts"

# --- separate buckets, other endpoint, empty prefix -----------------------------
echo "== separate buckets: training in awt-train (prefix empty) on another endpoint"
aws s3 mb s3://awt-train >/dev/null
put awt-train "files/$(sha trainee-content)" image/jpeg trainee-content
put awt-train "litestream/generations/0000000000000000/snapshots/00000000.snapshot.lz4" application/octet-stream old-replica
put awt-train "unrelated/keep.txt" text/plain keep
training_env http://minio-alt:9000 awt-train ""
echo inactive > "$root/timer"
: > "$root/calls"
run_sync --yes
after_refresh awt-train files/ litestream/ 0000000000000000
check "streamed copy" has "streamed"
check "keys outside files/ and litestream/ kept" eval 'objects awt-train unrelated/ | grep -q "^keep.txt[[:space:]]"'
check "the shared bucket's old training prefix is not touched" eval '[ "$(objects awt-test training/files/)" = "$prod_files" ]'
check "paused timer left paused" eval '[ "$(cat "$root/timer")" = inactive ] && ! grep -q "^start" "$root/calls" && has "left paused"'

echo
if [ "$failures" -gt 0 ]; then
  echo "FAIL: $failures check(s) failed"
  exit 1
fi
echo "PASS: sync-training"
