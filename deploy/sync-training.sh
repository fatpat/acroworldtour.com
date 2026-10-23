#!/bin/sh
# Training refresh: replaces the training environment with a copy of prod
# (docs/ops.md "Refresh training from prod", decision 0018). Installed at
# /opt/awt/deploy/sync-training.sh; run it as root on the VPS:
#
#   sudo /opt/awt/deploy/sync-training.sh          # asks to type "training"
#   sudo /opt/awt/deploy/sync-training.sh --yes    # no question
#
# Prod is only read and keeps running. Training is wiped first, so nothing
# of it remains (trainee accounts and data, its files, its Litestream
# replica), then gets prod's database and files. On training every session,
# pending login and second factor is removed: managers log in with their
# prod password and no second factor.
#
# Steps:
#   1. checks, before anything changes: both env files, both image tags, S3
#      settings complete, awt-prod running, the running containers using
#      their env files' bucket settings, prod and training apart in object
#      storage (`awt sync-training check`), confirmation
#   2. pauses the deploy agent: stops awt-agent.timer and takes its lock;
#      the timer's previous state comes back on any exit
#   3. snapshot of the prod database (VACUUM INTO from a read-only
#      connection, prod keeps serving), integrity check, schema check,
#      migration and removal of sessions and second factors
#      (`awt sync-training prepare-db`)
#   4. wipe: writes state/sync-training.incomplete (deploy.sh then refuses
#      training), removes the awt-training containers, deletes training's
#      <prefix>litestream/ and <prefix>files/ objects, removes its volume
#   5. clone: the snapshot becomes /data/awt.db on a new training volume;
#      prod's files are copied to training's <prefix>files/
#   6. starts training (Litestream starts a new replica from the copy),
#      waits until healthy, removes the incomplete marker, purges training's
#      host from its Cloudflare zone if configured
#
# Re-runnable: a run that fails or is killed at any step leaves training
# stopped or partial; running it again starts over from step 1 and ends
# with the same result.
#
# Exit codes: 0 training refreshed (a failed CDN purge only warns);
# 1 failed after the wipe began: training may be partial, run it again;
# 2 refused, bad usage, or failed before the wipe: nothing was changed on
# training.
#
# Environment (tests; all optional): AWT_ROOT (default: the directory above
# this script), AWT_LOCK (default /run/lock/awt-agent.lock), SYSTEMCTL
# (default systemctl), AWT_LOCK_TIMEOUT (default 600 s), HEALTH_TIMEOUT
# (default 120 s), AWT_SYNC_TEST_FAIL_AFTER (test hook: fail after the named
# step: wipe or clone).
set -eu

root=${AWT_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}
lock=${AWT_LOCK:-/run/lock/awt-agent.lock}
SYSTEMCTL=${SYSTEMCTL:-systemctl}
lock_timeout=${AWT_LOCK_TIMEOUT:-600}
health_timeout=${HEALTH_TIMEOUT:-120}
timer=awt-agent.timer
started=$(date +%s)

log() { echo "sync-training: $*"; }
refuse() {
  echo "sync-training: refused: $*" >&2
  echo "sync-training: nothing was changed" >&2
  exit 2
}
usage() { echo "usage: $0 [--yes]   (run as root from the VPS; docs/ops.md \"Refresh training from prod\")"; }

yes=0
for a in "$@"; do
  case "$a" in
    --yes|-y) yes=1 ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; refuse "unknown argument '$a'" ;;
  esac
done
if [ "$yes" = 0 ] && [ ! -t 0 ]; then
  refuse "no terminal to confirm on: pass --yes to run without the question"
fi

cd "$root"
for f in compose.yaml .env .env.prod .env.training; do
  [ -r "$f" ] || refuse "$root/$f is missing or not readable (run as root: sudo $0)"
done
# env_value <key>: from .env, as compose reads it (one pair of matching
# surrounding quotes removed).
env_value() {
  sed -n "s/^$1=//p" .env | tail -1 | sed -e "s/^\"\(.*\)\"\$/\1/" -e "s/^'\(.*\)'\$/\1/"
}
image=$(env_value AWT_IMAGE)
prod_tag=$(env_value AWT_PROD_TAG)
training_tag=$(env_value AWT_TRAINING_TAG)
for kv in "AWT_IMAGE=$image" "AWT_PROD_TAG=$prod_tag" "AWT_TRAINING_TAG=$training_tag"; do
  case "${kv#*=}" in ""|none) refuse "${kv%%=*} is not set in $root/.env (deploy both environments first)" ;; esac
done
project=$(docker compose config 2>/dev/null | sed -n '1s/^name: //p')
[ -n "$project" ] || refuse "docker compose config failed in $root (check compose.yaml and .env)"
stage_vol="${project}_sync_training_stage"
training_vol="${project}_training_data"
# Present while training may be partial (a run touched it and did not
# finish): deploy.sh refuses to deploy training then (docs/ops.md 6b).
incomplete=state/sync-training.incomplete
timer_state=state/sync-training.timer

# awt_training <args>: the awt binary in a one-off container of the training
# service: training's env file, AWT_ENV=training, its volume, the compose
# network. The bucket steps read prod's env file on stdin.
awt_training() { docker compose run --rm --no-deps -T --entrypoint awt awt-training "$@"; }

# read_live: the object storage settings the running containers use
# (docker inspect), never the files: the bucket steps compare them with the
# env files. Only these three variables are kept; no other value is read
# into the script. live_training=0 when awt-training is not running.
live_value() { printf '%s\n' "$2" | sed -n "s/^$1=//p" | tail -1; }
read_live() {
  cid=$(docker compose ps -q --status running awt-prod 2>/dev/null || true)
  [ -n "$cid" ] || return 1
  envs=$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$cid" \
    | grep -E '^(AWT_S3_ENDPOINT|AWT_S3_BUCKET|AWT_BUCKET_PREFIX)=' || true)
  lp_endpoint=$(live_value AWT_S3_ENDPOINT "$envs")
  lp_bucket=$(live_value AWT_S3_BUCKET "$envs")
  lp_prefix=$(live_value AWT_BUCKET_PREFIX "$envs")
  live_training=0
  cid=$(docker compose ps -q --status running awt-training 2>/dev/null || true)
  if [ -n "$cid" ]; then
    envs=$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$cid" \
      | grep -E '^(AWT_S3_ENDPOINT|AWT_S3_BUCKET|AWT_BUCKET_PREFIX)=' || true)
    lt_endpoint=$(live_value AWT_S3_ENDPOINT "$envs")
    lt_bucket=$(live_value AWT_S3_BUCKET "$envs")
    lt_prefix=$(live_value AWT_BUCKET_PREFIX "$envs")
    live_training=1
  fi
}
# bucket_step <step>: a bucket step with the live settings, prod's env file
# on stdin.
bucket_step() {
  name=$1
  set -- sync-training "$name" "--live-prod-endpoint=$lp_endpoint" "--live-prod-bucket=$lp_bucket" "--live-prod-prefix=$lp_prefix"
  if [ "$live_training" = 1 ]; then
    set -- "$@" "--live-training-endpoint=$lt_endpoint" "--live-training-bucket=$lt_bucket" "--live-training-prefix=$lt_prefix"
  fi
  awt_training "$@" < .env.prod
}

log "checking settings and buckets"
read_live || refuse "awt-prod is not running: start it first (the check compares what it runs with .env.prod)"
if ! plan=$(bucket_step check); then
  refuse "the check above failed"
fi
if [ "$prod_tag" != "$training_tag" ]; then
  log "note: prod runs $prod_tag and training $training_tag; training migrates the copy to its own schema"
fi
if [ -f "$incomplete" ]; then
  log "note: an earlier refresh did not finish ($root/$incomplete): training is partial; this run replaces it"
fi

cat <<EOF

This replaces TRAINING with a copy of PROD.
$(printf '%s\n' "$plan" | sed 's/^/  /')

Destroyed on training, with no way back:
  - its database: every account (trainee accounts included), competition,
    run, flight, news and setting created or changed on training
  - every object under its Litestream replica and its files (above)
Then training gets prod's database and files. Every session and second
factor is removed on training: managers log in with their prod password.
Prod is only read and keeps running. Training is down until the end
(about a minute, plus the file copy).

EOF
if [ "$yes" = 0 ]; then
  printf 'Type "training" to continue: '
  read -r answer || answer=
  [ "$answer" = training ] || refuse "not confirmed"
fi

# state/ belongs to the deploy user (init-vps.sh); create it so when missing.
if [ ! -d state ]; then
  if id deploy >/dev/null 2>&1; then install -d -o deploy -g deploy -m 750 state; else mkdir -p state; fi
fi

# From here on, any exit restores the timer and removes the staging volume.
# A failure before the wipe changed nothing on training (exit 2); after it,
# training may be partial (exit 1, state/sync-training.incomplete stays).
touched=0
on_exit() {
  rc=$?
  trap - EXIT INT TERM HUP
  if [ -n "${timer_was:-}" ]; then
    if [ "$timer_was" = active ]; then
      if $SYSTEMCTL start "$timer"; then log "deploy agent resumed ($timer started)"
      else echo "sync-training: WARNING: could not start $timer: start it by hand" >&2; fi
    else
      log "deploy agent left paused ($timer was not active before this run)"
    fi
    rm -f "$timer_state"
  fi
  docker volume rm -f "$stage_vol" >/dev/null 2>&1 || true
  if [ "$rc" != 0 ]; then
    if [ "$touched" = 1 ]; then
      echo "sync-training: FAILED (exit $rc) after training was changed: it may be stopped or partial." >&2
      echo "sync-training: prod was not changed. Fix the cause above and run this script again: it starts over." >&2
      rc=1
    else
      echo "sync-training: FAILED before the wipe: nothing was changed on training (prod was only read)." >&2
      [ ! -f "$incomplete" ] || echo "sync-training: training is still partial from an earlier run ($root/$incomplete): run this script again once the cause above is fixed." >&2
      rc=2
    fi
  fi
  exit "$rc"
}
trap on_exit EXIT
trap 'exit 1' INT TERM HUP

# 2. Pause the deploy agent. A run killed before restoring the timer left
# its state file: the timer's state before that run is the one to restore.
if [ -f "$timer_state" ]; then
  timer_was=$(cat "$timer_state")
else
  if $SYSTEMCTL is-active --quiet "$timer"; then timer_was=active; else timer_was=inactive; fi
  echo "$timer_was" > "$timer_state"
fi
if ! $SYSTEMCTL stop "$timer"; then
  echo "sync-training: could not stop $timer (run as root)" >&2
  exit 2
fi
if [ ! -e "$lock" ]; then
  mkdir -p "$(dirname "$lock")"
  # The agent (user deploy) opens it for writing: keep it theirs.
  if id deploy >/dev/null 2>&1; then install -o deploy -g deploy -m 644 /dev/null "$lock"; else : > "$lock"; fi
fi
# Read-only: root may not open another user's file in the sticky /run/lock
# for writing (fs.protected_regular); flock works on any open file.
exec 9<"$lock"
if ! flock -n 9; then
  log "waiting for the running deploy to finish (up to ${lock_timeout}s)"
  flock -w "$lock_timeout" 9 || { echo "sync-training: the deploy agent still holds $lock after ${lock_timeout}s" >&2; exit 2; }
fi
log "deploy agent paused"
# The agent may have deployed before it was paused: use the tags and the
# running containers' settings as they are now.
prod_tag=$(env_value AWT_PROD_TAG)
training_tag=$(env_value AWT_TRAINING_TAG)
read_live || { echo "sync-training: awt-prod is not running" >&2; exit 2; }

# 3. Snapshot of prod: a read-only connection (mode=ro never creates or
# writes the file), VACUUM INTO a staging volume. The staging volume is
# first mounted on the image's /data, so it gets /data's owner (the awt user).
docker volume rm -f "$stage_vol" >/dev/null 2>&1 || true
docker run --rm --network none -v "$stage_vol:/data" --entrypoint true "$image:$training_tag"
log "snapshot of the prod database"
docker compose run --rm --no-deps -T -v "$stage_vol:/stage" --entrypoint sqlite3 awt-prod \
  -bail -cmd ".timeout 30000" "file:/data/awt.db?mode=ro" "VACUUM INTO '/stage/prod.db'"
log "integrity check and cleanup of the snapshot"
prepared=$(docker run --rm --network none -v "$stage_vol:/stage" --entrypoint awt "$image:$training_tag" \
  sync-training prepare-db /stage/prod.db)
printf '%s\n' "$prepared" | sed 's/^/  /'
# Re-checked with the settings read after the pause, before deleting.
bucket_step check >/dev/null

# 4. Wipe training.
touched=1
: > "$incomplete"
log "wiping training: container, replica, files, volume"
docker compose rm -s -f awt-training
wiped=$(bucket_step wipe-bucket)
log "$wiped"
# Every container of the service (one-offs left by a killed run too), then
# the volume by label and by name: nothing of the old volume (a stale -wal,
# -shm or Litestream state) may sit next to the new database.
ctrs=$(docker ps -aq --filter "label=com.docker.compose.project=$project" \
  --filter "label=com.docker.compose.service=awt-training")
[ -z "$ctrs" ] || printf '%s\n' "$ctrs" | xargs docker rm -f -v >/dev/null
vols=$(docker volume ls -q --filter "label=com.docker.compose.project=$project" \
  --filter "label=com.docker.compose.volume=training_data")
for v in $vols "$training_vol"; do docker volume rm -f "$v" >/dev/null; done
if docker volume inspect "$training_vol" >/dev/null 2>&1; then
  echo "sync-training: volume $training_vol could not be removed" >&2
  exit 1
fi
[ "${AWT_SYNC_TEST_FAIL_AFTER:-}" != wipe ] || { echo "sync-training: test failure after wipe" >&2; exit 1; }

# 5. Clone: the database onto a new volume (compose run creates it), then
# the files.
log "copying the database to the new training volume"
docker compose run --rm --no-deps -T -v "$stage_vol:/stage:ro" --entrypoint sh awt-training \
  -c 'cp /stage/prod.db /data/awt.db.part && mv /data/awt.db.part /data/awt.db'
log "copying prod's files to training"
copied=$(bucket_step copy-files)
log "$copied"
[ "${AWT_SYNC_TEST_FAIL_AFTER:-}" != clone ] || { echo "sync-training: test failure after clone" >&2; exit 1; }

# 6. Start training, as deploy.sh checks a deploy (directly: deploy.sh
# refuses training while the refresh is incomplete).
log "starting training"
docker compose up -d awt-training
waited=0
until cid=$(docker compose ps -q awt-training 2>/dev/null) && [ -n "$cid" ] \
  && [ "$(docker inspect -f '{{.State.Health.Status}}' "$cid")" = healthy ] \
  && docker compose exec -T awt-training wget -q -O /dev/null http://127.0.0.1:8080/healthz; do
  waited=$((waited + 3))
  if [ "$waited" -ge "$health_timeout" ]; then
    docker compose logs --tail 50 awt-training >&2 || true
    echo "sync-training: training is not healthy after ${health_timeout}s" >&2
    exit 1
  fi
  sleep 3
done
if [ -n "$(docker compose ps -q caddy 2>/dev/null)" ]; then
  n=0
  until docker compose exec -T caddy wget -q -O /dev/null http://127.0.0.1:8081/training/healthz; do
    n=$((n + 1))
    [ "$n" -lt 10 ] || { echo "sync-training: training /healthz through caddy failed" >&2; exit 1; }
    sleep 2
  done
fi
log "training is healthy"
rm -f "$incomplete"
touched=0

warn=0
if ! purged=$(awt_training sync-training purge-cdn < /dev/null); then
  warn=1
  echo "sync-training: WARNING: the Cloudflare purge failed (above): purge training's host in the Cloudflare dashboard" >&2
else
  log "$purged"
fi

cat <<EOF

Training refreshed from prod in $(( $(date +%s) - started ))s (training image $training_tag, prod image $prod_tag).
$(printf '%s\n' "$prepared" | sed 's/^/  /')
  $wiped
  $copied
Litestream starts a new training replica from the copy.
EOF
[ "$warn" = 0 ] || log "done, with the warning above"
exit 0
