#!/bin/sh
# Deploy script, installed at /opt/awt/deploy/deploy.sh next to /opt/awt/compose.yaml,
# .env, .env.prod, .env.training.
#
#   deploy.sh training|prod <image-tag>   deploy one environment
#
# Called by the deploy agent (deploy/agent.sh) for each new revision, or by an
# admin for a manual rollback (docs/ops.md). The image is public: no login.
#
# Deploying pulls the tag (only if it is not already local: the agent tags
# the image it pulled), restarts the service, waits for the container health
# check, then checks /healthz inside the stack (app, then through Caddy). If the
# new container is not healthy it restarts the previous tag and exits non-zero.
#
# Exit codes: 0 deployed; 1 deploy failed (unhealthy, rolled back; or Caddy
# check failed); 2 bad usage; 3 pull failed, nothing changed (the agent retries
# instead of recording the revision as failed).
set -eu
cd "$(dirname "$0")/.."

die() { echo "deploy: $*" >&2; exit 2; }

env_name=${1:-}; tag=${2:-}
[ "$#" -eq 2 ] || die "usage: deploy.sh training|prod <tag>"
case "$env_name" in training|prod) ;; *) die "unknown environment" ;; esac
printf '%s' "$tag" | grep -Eq '^[0-9a-f]{7,40}$' || die "tag must be a lowercase hex commit id"

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
  docker compose exec -T caddy caddy validate --config /etc/caddy/site/Caddyfile >/dev/null 2>&1 \
    || { echo "deploy: Caddyfile validation failed" >&2; docker compose exec -T caddy caddy validate --config /etc/caddy/site/Caddyfile >&2 || true; return 1; }
  docker compose exec -T caddy caddy reload --config /etc/caddy/site/Caddyfile >/dev/null \
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
if ! docker compose pull --policy missing "$svc"; then
  echo "deploy $svc: pull of $tag failed, nothing changed" >&2
  set_tag "${prev:-none}"
  exit 3
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
