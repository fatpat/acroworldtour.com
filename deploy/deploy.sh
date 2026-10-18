#!/bin/sh
# Deploy one environment on the VPS: deploy.sh training|prod <image-tag>
# Lives in /opt/awt next to compose.yaml, .env, .env.prod and .env.training.
# Pulls the tag, restarts the service, waits for the container health check
# and rolls back to the previous tag if it never becomes healthy.
set -eu
cd "$(dirname "$0")"

env_name=${1:?usage: deploy.sh training|prod <tag>}
tag=${2:?usage: deploy.sh training|prod <tag>}
case "$env_name" in
  training|prod) ;;
  *) echo "unknown environment $env_name" >&2; exit 2 ;;
esac
svc="awt-$env_name"
var="AWT_$(echo "$env_name" | tr '[:lower:]' '[:upper:]')_TAG"
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
    if [ -n "$cid" ] && [ "$(docker inspect -f '{{.State.Health.Status}}' "$cid")" = healthy ]; then
      return 0
    fi
    waited=$((waited + 3)); sleep 3
  done
  return 1
}

prev=$(sed -n "s/^$var=//p" .env | tail -1)
echo "deploy $svc: ${prev:-none} -> $tag"
set_tag "$tag"
if ! docker compose pull "$svc"; then
  echo "deploy $svc: pull of $tag failed, nothing changed" >&2
  [ -z "$prev" ] || set_tag "$prev"
  exit 1
fi
docker compose up -d "$svc"
if wait_healthy; then
  docker compose up -d --no-deps caddy
  docker compose exec -T caddy caddy reload --config /etc/caddy/Caddyfile >/dev/null 2>&1 || true
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
fi
exit 1
