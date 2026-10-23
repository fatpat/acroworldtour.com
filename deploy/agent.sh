#!/bin/sh
# Deploy agent, installed at /opt/awt/deploy/agent.sh and run every 60 s by
# awt-agent.timer as the deploy user (docs/ops.md, decision 0016).
#
# Pulls the moving tag $AWT_IMAGE:$AWT_CHANNEL (public image, no login), reads
# the commit from its org.opencontainers.image.revision label, installs the
# compose.yaml and Caddyfile baked into the image once they validate, then
# runs deploy.sh for training and, only if training is healthy, for prod.
#
# A revision that fails validation or a deploy is appended to
# state/failed-revisions and skipped from then on (delete the line to retry).
# A failed pull is not recorded: the next run retries. Nothing new = no output.
#
# Environment (all optional): AWT_ROOT (default /opt/awt); AWT_IMAGE and
# AWT_CHANNEL (default: from $AWT_ROOT/.env; channel rewrite-go); AWT_LOCK
# (default /run/lock/awt-agent.lock); HEALTH_TIMEOUT (passed to deploy.sh).
set -eu

root=${AWT_ROOT:-/opt/awt}
lock=${AWT_LOCK:-/run/lock/awt-agent.lock}
LABEL=org.opencontainers.image.revision
BAKED=/usr/share/awt/deploy

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

[ "$(env_value AWT_TRAINING_TAG)" = "$rev" ] && [ "$(env_value AWT_PROD_TAG)" = "$rev" ] && exit 0

mkdir -p state
failed=state/failed-revisions
if [ -f "$failed" ] && grep -qx "$rev" "$failed"; then
  log "skip $rev: failed before"
  exit 0
fi
record_failed() { echo "$rev" >> "$failed"; err "$rev recorded as failed in $root/$failed"; }

log "new revision $rev on $ref"

# Copy the baked config out of the image and validate it before installing.
staging="state/staging-$rev"
cid=""
cleanup() {
  [ -z "$cid" ] || docker rm -f "$cid" >/dev/null 2>&1 || true
  rm -rf "$staging"
}
trap cleanup EXIT
rm -rf "$staging"; mkdir -p "$staging"
cid=$(docker create "$ref")
docker cp -q "$cid:$BAKED/compose.yaml" "$staging/compose.yaml"
docker cp -q "$cid:$BAKED/Caddyfile" "$staging/Caddyfile"

validate() {
  # Same interpolation as at deploy time (.env); profiles are not checked.
  docker compose -f "$staging/compose.yaml" --env-file .env config -q 2>&1 \
    || { err "compose.yaml of $rev is invalid"; return 1; }
  # Validate with the Caddy image the new compose.yaml runs, given what
  # compose gives Caddy: the two hosts from .env and the certs directory
  # (AWT_TLS_CERT/KEY keep their Caddyfile defaults, as in compose.yaml).
  caddy_image=$(docker compose -f "$staging/compose.yaml" --env-file .env config --images 2>/dev/null \
    | grep -E '^caddy[:@]' | head -1)
  [ -n "$caddy_image" ] || { err "no caddy image in compose.yaml of $rev"; return 1; }
  docker run --rm --network none \
    -e AWT_PROD_HOST="$(env_value AWT_PROD_HOST)" -e AWT_TRAINING_HOST="$(env_value AWT_TRAINING_HOST)" \
    -v "$root/$staging:/etc/caddy/site:ro" -v "$root/certs:/etc/caddy/certs:ro" \
    "$caddy_image" caddy validate --config /etc/caddy/site/Caddyfile --adapter caddyfile >/dev/null 2>"$staging/caddy.err" \
    || { err "Caddyfile of $rev is invalid: $(tail -1 "$staging/caddy.err")"; return 1; }
}
if ! validate >&2; then
  record_failed
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

# Keep what runs now: on any failure from here on, restore() puts back these
# files and redeploys these revisions, so both apps and Caddy run the
# previous revision with its own config.
prev_training=$(env_value AWT_TRAINING_TAG)
prev_prod=$(env_value AWT_PROD_TAG)
rm -rf state/prev; mkdir -p state/prev caddy
if [ -f compose.yaml ]; then cp compose.yaml state/prev/compose.yaml; fi
if [ -f caddy/Caddyfile ]; then cp caddy/Caddyfile state/prev/Caddyfile; fi

# restore <prod attempted: 0|1>
restore() {
  err "restoring the previous compose.yaml, Caddyfile and revisions"
  if [ -f state/prev/compose.yaml ]; then install_file state/prev/compose.yaml compose.yaml; fi
  if [ -f state/prev/Caddyfile ]; then install_file state/prev/Caddyfile caddy/Caddyfile; fi
  restored=1
  case "$prev_training" in none|"$rev"|"") ;; *)
    "$root/deploy/deploy.sh" training "$prev_training" || restored=0 ;;
  esac
  if [ "$1" = 1 ]; then
    case "$prev_prod" in none|"$rev"|"") ;; *)
      "$root/deploy/deploy.sh" prod "$prev_prod" || restored=0 ;;
    esac
  fi
  if [ "$restored" = 1 ]; then
    err "restored training=${prev_training:-none} prod=${prev_prod:-none}"
  else
    err "RESTORE FAILED: check $root by hand (docs/ops.md section 5)"
  fi
}

install_file "$staging/compose.yaml" compose.yaml
install_file "$staging/Caddyfile" caddy/Caddyfile

# deploy.sh pulls $image:$rev only if missing: tag the image just pulled so
# the deploy needs no second registry round-trip.
docker tag "$ref" "$image:$rev"

# deploy_env <env>: exit 3 from deploy.sh = pull failed, nothing changed:
# not the revision's fault, so it is not recorded and the next run retries.
# Any other failure (unhealthy app, Caddy check) records the revision. Either
# way the previous config and revisions are restored.
deploy_env() {
  rc=0
  "$root/deploy/deploy.sh" "$1" "$rev" || rc=$?
  [ "$rc" = 0 ] && return 0
  if [ "$rc" = 3 ]; then
    err "$1: pull of $image:$rev failed; will retry"
  else
    record_failed
  fi
  if [ "$1" = prod ]; then restore 1; else restore 0; fi
  exit 1
}
if [ "$(env_value AWT_TRAINING_TAG)" != "$rev" ]; then
  deploy_env training
fi
deploy_env prod
log "deployed $rev to training and prod"
