#!/bin/sh
# Deploy script, installed at /opt/awt/deploy/deploy.sh next to /opt/awt/compose.yaml,
# .env, .env.prod, .env.training.
#
#   deploy.sh training|prod <image-tag>   deploy one environment
#   deploy.sh sync                        install compose.yaml + Caddyfile from a tar on stdin
#
# It is also the SSH forced command of the CI deploy key (see docs/ops.md):
#   restrict,command="/opt/awt/deploy/deploy.sh" ssh-ed25519 AAAA... awt-deploy
# In that mode the request comes from SSH_ORIGINAL_COMMAND and is validated
# against a strict pattern, and stdin may carry "<registry-user> <registry-token>"
# on its first line (docker login ghcr.io) for `training|prod`.
#
# Deploying pulls the tag, restarts the service, waits for the container health
# check, then checks /healthz inside the stack (app, then through Caddy). If the
# new container is not healthy it restarts the previous tag and exits non-zero.
set -eu
cd "$(dirname "$0")/.."

die() { echo "deploy: $*" >&2; exit 2; }

forced=0
if [ -n "${SSH_ORIGINAL_COMMAND:-}" ]; then
  forced=1
  case "$SSH_ORIGINAL_COMMAND" in
    *[!A-Za-z0-9\ ]*) die "refused: unexpected characters" ;;
  esac
  # shellcheck disable=SC2086 # split is safe: only [A-Za-z0-9 ] got through
  set -- $SSH_ORIGINAL_COMMAND
fi

if [ "${1:-}" = sync ] && [ "$#" -eq 1 ]; then
  tmp=$(mktemp)
  trap 'rm -f "$tmp"' EXIT
  head -c 1048576 > "$tmp"
  [ "$(tar -tf "$tmp" | sort | tr '\n' ' ')" = "Caddyfile compose.yaml " ] || die "sync: archive must contain exactly compose.yaml and Caddyfile"
  tar -tvf "$tmp" | cut -c1 | grep -qv '^-' && die "sync: only regular files allowed"
  tar -xf "$tmp" --no-same-owner --no-same-permissions -C . compose.yaml Caddyfile
  echo "sync: compose.yaml and Caddyfile installed"
  exit 0
fi

env_name=${1:-}; tag=${2:-}
[ "$#" -eq 2 ] || die "usage: deploy.sh training|prod <tag> | sync"
case "$env_name" in training|prod) ;; *) die "unknown environment" ;; esac
printf '%s' "$tag" | grep -Eq '^[0-9a-f]{7,40}$' || die "tag must be a lowercase hex commit id"

if [ "$forced" = 1 ]; then
  login_user=""; login_token=""
  read -r login_user login_token || true
  if [ -n "$login_user" ] && [ -n "$login_token" ]; then
    printf '%s' "$login_user" | grep -Eq '^[A-Za-z0-9-]+$' || die "bad registry user"
    printf '%s' "$login_token" | docker login ghcr.io -u "$login_user" --password-stdin >/dev/null
  fi
fi

svc="awt-$env_name"
upper=$(echo "$env_name" | tr '[:lower:]' '[:upper:]')
var="AWT_${upper}_TAG"
timeout_s=${HEALTH_TIMEOUT:-120}

set_tag() {
  if grep -q "^$var=" .env; then
    sed -i "s|^$var=.*|$var=$1|" .env
  else
    echo "$var=$1" >> .env
  fi
}

wait_healthy() {
  waited=0
  while [ "$waited" -lt "$timeout_s" ]; do
    cid=$(docker compose ps -q "$svc" 2>/dev/null || true)
    if [ -n "$cid" ] && [ "$(docker inspect -f '{{.State.Health.Status}}' "$cid")" = healthy ] \
      && docker compose exec -T "$svc" wget -q -O /dev/null http://127.0.0.1:8080/healthz; then
      return 0
    fi
    waited=$((waited + 3)); sleep 3
  done
  return 1
}

# Caddy: start if needed, validate the Caddyfile, reload, then request
# /healthz through it on its loopback-only health listener (:8081 inside the
# container); the public sites keep the strict Cloudflare-only abort rule.
caddy_up() {
  docker compose up -d --no-deps caddy || return 1
  docker compose exec -T caddy caddy validate --config /etc/caddy/Caddyfile >/dev/null 2>&1 \
    || { echo "deploy: Caddyfile validation failed" >&2; docker compose exec -T caddy caddy validate --config /etc/caddy/Caddyfile >&2 || true; return 1; }
  docker compose exec -T caddy caddy reload --config /etc/caddy/Caddyfile >/dev/null \
    || { echo "deploy: caddy reload failed" >&2; return 1; }
  n=0
  while [ "$n" -lt 10 ]; do
    if docker compose exec -T caddy wget -q -O /dev/null "http://127.0.0.1:8081/$env_name/healthz"; then
      return 0
    fi
    n=$((n + 1)); sleep 2
  done
  echo "deploy: /healthz through caddy failed" >&2
  return 1
}

prev=$(sed -n "s/^$var=//p" .env | tail -1)
case "$prev" in none) prev="" ;; esac
echo "deploy $svc: ${prev:-none} -> $tag"
set_tag "$tag"
if ! docker compose pull "$svc"; then
  echo "deploy $svc: pull of $tag failed, nothing changed" >&2
  set_tag "${prev:-none}"
  exit 1
fi
docker compose up -d "$svc"
if wait_healthy; then
  caddy_up || exit 1
  echo "deploy $svc: healthy on $tag"
  exit 0
fi

echo "deploy $svc: $tag is not healthy after ${timeout_s}s" >&2
docker compose logs --tail 50 "$svc" >&2 || true
if [ -n "$prev" ] && [ "$prev" != "$tag" ]; then
  echo "rolling back to $prev" >&2
  set_tag "$prev"
  docker compose up -d "$svc"
  if wait_healthy; then echo "rolled back to $prev" >&2; else echo "ROLLBACK ALSO UNHEALTHY" >&2; fi
else
  echo "no previous tag to roll back to" >&2
fi
exit 1
