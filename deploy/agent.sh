#!/bin/sh
# Deploy agent, installed at /opt/awt/deploy/agent.sh and run every 60 s by
# awt-agent.timer as the deploy user (docs/ops.md section 5, decisions 0016
# and 0019).
#
# Pulls the moving tag $AWT_IMAGE:$AWT_CHANNEL (public image, no login), reads
# the commit from its org.opencontainers.image.revision label, installs the
# compose.yaml and Caddyfile baked into the image once they validate, then
# deploys the revision to training and to prod: both in the same run,
# independently (decision 0019). Neither result gates the other; the two
# deploys run one after the other because they share the Caddy reload.
#
# Failures are kept per environment: a revision that fails on an environment
# is appended to state/failed-revisions.<env> and never retried there (delete
# the line to retry); the other environment still gets it. A revision whose
# baked config is invalid reaches neither and is recorded for both. A failed
# pull is not recorded: the next run retries. A failed environment is rolled
# back to its previous revision; when no environment took the revision, the
# previous compose.yaml and Caddyfile come back too. While a training refresh
# is incomplete (state/sync-training.incomplete, docs/ops.md 6b) training is
# skipped. Nothing new = no output.
#
# An old state/failed-revisions (one list for both, before decision 0019) is
# split once: its lines count as failed on both environments; the file is
# kept as state/failed-revisions.migrated.
#
# Environment (all optional): AWT_ROOT (default /opt/awt); AWT_IMAGE and
# AWT_CHANNEL (default: from $AWT_ROOT/.env; channel rewrite-go); AWT_LOCK
# (default /run/lock/awt-agent.lock); HEALTH_TIMEOUT (passed to deploy.sh);
# AWT_PRUNE_DAYS (default 30: age of local images removed after a deploy).
set -eu

root=${AWT_ROOT:-/opt/awt}
lock=${AWT_LOCK:-/run/lock/awt-agent.lock}
LABEL=org.opencontainers.image.revision
BAKED=/usr/share/awt/deploy
SOURCE=https://github.com/fatpat/acroworldtour.com

log() { echo "agent: $*"; }
err() { echo "agent: $*" >&2; }

# One run at a time; a run that finds the lock taken leaves silently.
exec 9>"$lock"
flock -n 9 || exit 0

cd "$root"
env_value() { sed -n "s/^$1=//p" .env | tail -1; }
image=${AWT_IMAGE:-$(env_value AWT_IMAGE)}
channel=${AWT_CHANNEL:-$(env_value AWT_CHANNEL)}
channel=${channel:-rewrite-go}
[ -n "$image" ] || { err "AWT_IMAGE is not set (in $root/.env)"; exit 1; }
ref="$image:$channel"

if ! out=$(docker pull -q "$ref" 2>&1); then
  err "pull $ref failed: $(printf '%s' "$out" | tail -1)"
  exit 1
fi

rev=$(docker image inspect -f "{{index .Config.Labels \"$LABEL\"}}" "$ref" 2>/dev/null || true)
if ! printf '%s' "$rev" | grep -Eqx '[0-9a-f]{40}'; then
  err "refused $ref: label $LABEL is '$rev', not a 40-character commit id"
  exit 1
fi

mkdir -p state
if [ -f state/failed-revisions ]; then
  for e in training prod; do cat state/failed-revisions >> "state/failed-revisions.$e"; done
  mv state/failed-revisions state/failed-revisions.migrated
  log "state/failed-revisions split into state/failed-revisions.training and .prod (its revisions count as failed on both)"
fi
tag_of() { env_value "AWT_$(echo "$1" | tr '[:lower:]' '[:upper:]')_TAG"; }
failed_on() { [ -f "state/failed-revisions.$1" ] && grep -qx "$rev" "state/failed-revisions.$1"; }
record_failed() {
  for e in "$@"; do
    echo "$rev" >> "state/failed-revisions.$e"
    err "$rev recorded as failed on $e in $root/state/failed-revisions.$e"
  done
}

# What each environment needs: already on the revision, skipped (training
# while a refresh is incomplete), failed there before, or to deploy.
pending="" failed_before="" on_rev=0 training_blocked=0
for e in training prod; do
  if [ "$(tag_of "$e")" = "$rev" ]; then on_rev=1; continue; fi
  if [ "$e" = training ] && [ -f state/sync-training.incomplete ]; then training_blocked=1; continue; fi
  if failed_on "$e"; then failed_before="$failed_before $e"; continue; fi
  pending="$pending $e"
done
pending=${pending# } failed_before=${failed_before# }
if [ -z "$pending" ]; then
  # Nothing to deploy. An environment on this revision means the installed
  # files are its (that deploy finished): restore the marker if a run died
  # just before writing it, so the next revision saves them as state/prev.
  if [ "$on_rev" = 1 ] && [ ! -f state/installed-ok ]; then : > state/installed-ok; fi
  [ -z "$failed_before" ] || log "skip $rev: failed before on $failed_before"
  exit 0
fi

log "new revision $rev on $ref: deploying to $pending"
[ -z "$failed_before" ] || log "skip $rev on $failed_before: failed before"
[ "$training_blocked" = 0 ] || err "training skipped: a training refresh did not finish (state/sync-training.incomplete, docs/ops.md 6b)"

# Copy the baked config out of the image and validate it before installing.
# Leftovers of a killed run (staging dirs, config containers) go first.
rm -rf state/staging-*
docker ps -aq --filter label=awt-agent-staging | xargs -r docker rm -f -v >/dev/null 2>&1 || true
staging="state/staging-$rev"
cid=""
need_restore=0
attempted=""
# On any non-zero exit after the install that the run did not handle itself
# (failed command, INT/TERM, systemd timeout), restore exactly once: the
# previous files and the previous revision of every environment attempted.
on_exit() {
  rc=$?
  trap - EXIT INT TERM
  if [ "$rc" != 0 ] && [ "$need_restore" = 1 ]; then
    need_restore=0
    restore_all || true
  fi
  [ -z "$cid" ] || docker rm -f -v "$cid" >/dev/null 2>&1 || true
  rm -rf "$staging"
  exit "$rc"
}
trap on_exit EXIT
trap 'exit 1' INT TERM
mkdir -p "$staging"
cid=$(docker create --label awt-agent-staging=1 "$ref")
docker cp -q "$cid:$BAKED/compose.yaml" "$staging/compose.yaml"
docker cp -q "$cid:$BAKED/Caddyfile" "$staging/Caddyfile"

validate() {
  # Same interpolation as at deploy time (.env); profiles are not checked.
  docker compose -f "$staging/compose.yaml" --project-directory "$root" --env-file .env config -q 2>&1 \
    || { err "compose.yaml of $rev is invalid"; return 1; }
  # Validate with the Caddy image the new compose.yaml runs, given what
  # compose gives Caddy: the two hosts from .env and the certs directory
  # (AWT_TLS_CERT/KEY keep their Caddyfile defaults, as in compose.yaml).
  caddy_image=$(docker compose -f "$staging/compose.yaml" --project-directory "$root" --env-file .env config --images 2>/dev/null \
    | grep -E '^caddy[:@]' | head -1)
  [ -n "$caddy_image" ] || { err "no caddy image in compose.yaml of $rev"; return 1; }
  docker run --rm --network none \
    -e AWT_PROD_HOST="$(env_value AWT_PROD_HOST)" -e AWT_TRAINING_HOST="$(env_value AWT_TRAINING_HOST)" \
    -v "$root/$staging:/etc/caddy/site:ro" -v "$root/certs:/etc/caddy/certs:ro" \
    "$caddy_image" caddy validate --config /etc/caddy/site/Caddyfile --adapter caddyfile >/dev/null 2>"$staging/caddy.err" \
    || { err "Caddyfile of $rev is invalid: $(tail -1 "$staging/caddy.err")"; return 1; }
}
# The config is shared: invalid, it reaches no environment, and the revision
# is recorded as failed on both.
if ! validate >&2; then
  record_failed training prod
  exit 1
fi

# Install: write next to the target, then rename (atomic; Caddy mounts the
# caddy/ directory, so it sees the new inode).
install_file() {
  cmp -s "$1" "$2" && return 0
  cp "$1" "$2.new"
  mv "$2.new" "$2"
  log "installed $2"
}

# Keep what runs now: restore_all() puts back these files and redeploys
# these revisions. state/installed-ok exists only while the installed files
# belong to a deployed revision (written after a deploy or a restore).
# Without it a run died after installing: keep the state/prev saved by that
# run, the installed files are not known good.
mkdir -p state/prev caddy
if [ -f state/installed-ok ] || [ ! -f state/prev/tags ]; then
  rm -rf state/prev; mkdir -p state/prev
  if [ -f compose.yaml ]; then cp compose.yaml state/prev/compose.yaml; fi
  if [ -f caddy/Caddyfile ]; then cp caddy/Caddyfile state/prev/Caddyfile; fi
  echo "$(env_value AWT_TRAINING_TAG) $(env_value AWT_PROD_TAG)" > state/prev/tags
else
  err "previous run did not finish: keeping its saved config and revisions"
fi
read -r prev_training prev_prod < state/prev/tags || true

# rollback_env <env>: back to the revision it ran before this run.
rollback_env() {
  if [ "$1" = training ]; then p=$prev_training; else p=$prev_prod; fi
  case "$p" in none|"$rev"|"") return 0 ;; esac
  if "$root/deploy/deploy.sh" "$1" "$p"; then
    err "rolled back $1 to $p"
  else
    err "ROLLBACK OF $1 TO $p FAILED: check $root by hand (docs/ops.md section 5)"
    return 1
  fi
}
# restore_all: no environment took the revision: the previous files, then
# the previous revision of each environment attempted (under those files).
restore_all() {
  err "restoring the previous compose.yaml and Caddyfile"
  restored=1
  if [ -f state/prev/compose.yaml ]; then install_file state/prev/compose.yaml compose.yaml || restored=0; fi
  if [ -f state/prev/Caddyfile ]; then install_file state/prev/Caddyfile caddy/Caddyfile || restored=0; fi
  for e in $attempted; do rollback_env "$e" || restored=0; done
  if [ "$restored" = 1 ]; then
    : > state/installed-ok
    err "restored training=${prev_training:-none} prod=${prev_prod:-none}"
  else
    err "RESTORE FAILED: check $root by hand (docs/ops.md section 5)"
  fi
}

rm -f state/installed-ok
need_restore=1
install_file "$staging/compose.yaml" compose.yaml
# Test hook (scripts/agent-test.sh): die between the two installs.
case "${AWT_TEST_KILL_AFTER_INSTALL:-}" in TERM|KILL) kill -s "$AWT_TEST_KILL_AFTER_INSTALL" $$ ;; esac
install_file "$staging/Caddyfile" caddy/Caddyfile

# deploy.sh pulls $image:$rev only if missing: tag the image just pulled so
# the deploy needs no second registry round-trip.
docker tag "$ref" "$image:$rev"

# Each pending environment in turn, whatever happened to the other. Exit 3
# from deploy.sh = pull failed, nothing changed: not the revision's fault,
# not recorded, retried next run. Exit 4 = training refresh incomplete
# (started meanwhile): skipped. Any other failure (unhealthy app, Caddy
# check) records the revision on that environment.
ok="" bad=""
for e in $pending; do
  attempted="$attempted $e"
  rc=0
  "$root/deploy/deploy.sh" "$e" "$rev" || rc=$?
  case "$rc" in
    0) ok="$ok $e" ;;
    3) err "$e: pull of $image:$rev failed; will retry"; bad="$bad $e" ;;
    4) err "$e: skipped, a training refresh did not finish"
       attempted=$(echo "$attempted" | sed "s/ $e\$//") ;;
    *) record_failed "$e"; bad="$bad $e" ;;
  esac
done
need_restore=0
if [ -z "$ok" ]; then
  restore_all
  exit 1
fi
# The revision runs somewhere: its files stay; failed environments go back
# to their previous revision under them.
for e in $bad; do rollback_env "$e" || true; done
: > state/installed-ok
if [ -z "$bad" ]; then
  log "deployed $rev to$ok"
else
  log "deployed $rev to$ok; failed on$bad"
fi

# Disk: drop local images of $image older than AWT_PRUNE_DAYS (0-9999, no
# leading zero: dash rejects "08" or overlong numbers in arithmetic; default 30), never the new revision, the previous training and prod
# revisions or the moving tag; then dangling images of this project (source
# label) as old. Best effort: never fails the run.
prune_images() {
  days=${AWT_PRUNE_DAYS:-30}
  case "$days" in
    0|[1-9]|[1-9][0-9]|[1-9][0-9][0-9]|[1-9][0-9][0-9][0-9]) ;;
    *) err "AWT_PRUNE_DAYS='$days' is not a number of days (0-9999), using 30"; days=30 ;;
  esac
  cutoff=$(($(date +%s) - days * 86400))
  docker image ls "$image" --format '{{.Tag}}' | while read -r t; do
    case " $rev $prev_training $prev_prod $channel <none> " in *" $t "*) continue ;; esac
    # .Created is RFC 3339 with a zone offset: compare as epoch seconds.
    raw=$(docker image inspect -f '{{.Created}}' "$image:$t" 2>/dev/null) || continue
    created=$(date -d "$raw" +%s 2>/dev/null) || continue
    if [ "$created" -lt "$cutoff" ]; then
      if docker image rm "$image:$t" >/dev/null 2>&1; then log "removed old image $image:$t"; fi
    fi
  done
  docker image prune -f --filter "until=$((days * 24))h" \
    --filter "label=org.opencontainers.image.source=$SOURCE" >/dev/null 2>&1 || true
}
prune_images || true
[ -z "$bad" ] || exit 1
exit 0
