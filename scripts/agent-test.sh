#!/bin/sh
# Deploy agent test: a local registry stands in for GHCR, a temp directory for
# /opt/awt, and deploy/agent.sh runs against them through the real deploy.sh
# and compose.yaml. Cases: lock held, happy path, no-op, bad revision (app
# never healthy), bad config (invalid baked Caddyfile or compose.yaml), label
# "unknown" or missing, registry down then back (retry), pull failing inside
# deploy.sh, :<rev> only local, prod failing after training, image pruning,
# run interrupted (TERM) or killed (KILL) after installing the config.
# Run via `make agent-test` (needs Docker, not root; takes several minutes).
set -eu
cd "$(dirname "$0")/.."
repo=$(pwd)

# registry 2.8.3, pinned by digest.
REGISTRY_IMAGE=registry:2.8.3@sha256:a3d8aaa63ed8681a604f1dea0aa03f100d5895b6a58ace528858a7b332415373
export COMPOSE_PROJECT_NAME=awt-agent-test
HEALTH_TIMEOUT=${HEALTH_TIMEOUT:-45}
LOCAL=awt-agent-test
REG_NAME=awt-agent-test-registry
G1=1111111111111111111111111111111111111111
G2=2222222222222222222222222222222222222222
BAD=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
BADCFG=cccccccccccccccccccccccccccccccccccccccc
G3=3333333333333333333333333333333333333333
PRODFAIL=dddddddddddddddddddddddddddddddddddddddd
BADCOMPOSE=eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee
G4=4444444444444444444444444444444444444444
BAD2=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb2
BAD3=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb3
G5=5555555555555555555555555555555555555555
G6=6666666666666666666666666666666666666666

root=$(mktemp -d)
port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')
IMAGE=localhost:$port/awt
failures=0

dc() { (cd "$root" && docker compose "$@"); }
cleanup() {
  dc down -v --remove-orphans >/dev/null 2>&1 || true
  docker rm -f "$REG_NAME" >/dev/null 2>&1 || true
  docker image ls --format '{{.Repository}}:{{.Tag}}' \
    | grep -E "^($LOCAL|$IMAGE):" | xargs -r docker image rm -f >/dev/null 2>&1 || true
  rm -rf "$root"
}
trap cleanup EXIT

ok() { echo "  ok: $*"; }
bad() { echo "  FAIL: $*" >&2; failures=$((failures + 1)); }
check() { # check <description> <command...>
  d=$1; shift
  if "$@"; then ok "$d"; else bad "$d"; fi
}

tag_of() { sed -n "s/^$1=//p" "$root/.env" | tail -1; }
# Id and start time: a restart or a recreate changes one of them.
ctr() { cid=$(dc ps -q "$1" 2>/dev/null || true); [ -n "$cid" ] && docker inspect -f '{{.Id}} {{.State.StartedAt}}' "$cid" || echo none; }
ctr_image() { cid=$(dc ps -q "$1" 2>/dev/null || true); [ -n "$cid" ] && docker inspect -f '{{.Config.Image}}' "$cid" || echo none; }
files_sum() { cat "$root/compose.yaml" "$root/caddy/Caddyfile" | sha256sum; }
failed_list() { cat "$root/state/failed-revisions" 2>/dev/null || true; }
has_marker() { grep -q "# agent-test revision $1" "$root/compose.yaml" && grep -q "# agent-test revision $1" "$root/caddy/Caddyfile"; }
no_staging() { ! ls -d "$root"/state/staging-* >/dev/null 2>&1; }

run_agent() {
  set +e
  out=$(AWT_ROOT="$root" AWT_IMAGE="$IMAGE" AWT_CHANNEL=test AWT_LOCK="$root/agent.lock" \
    HEALTH_TIMEOUT="$HEALTH_TIMEOUT" AWT_PRUNE_DAYS="${PRUNE_DAYS:-30}" \
    AWT_TEST_KILL_AFTER_INSTALL="${KILL_AFTER_INSTALL:-}" "$root/deploy/agent.sh" 2>&1)
  rc=$?
  set -e
  [ -z "$out" ] || printf '%s\n' "$out" | sed 's/^/    | /'
  echo "    (exit $rc)"
}

# publish <revision>: push :<revision> and move :test to it (what CI does).
publish() {
  docker tag "$LOCAL:$1" "$IMAGE:$1"
  docker tag "$LOCAL:$1" "$IMAGE:test"
  docker push -q "$IMAGE:$1" >/dev/null
  docker push -q "$IMAGE:test" >/dev/null
}

# publish_channel_only <revision>: move :test without pushing :<revision>.
publish_channel_only() {
  docker tag "$LOCAL:$1" "$IMAGE:test"
  docker push -q "$IMAGE:test" >/dev/null
}

# overlay <revision> <dockerfile lines>: an image on top of the base build.
overlay() {
  rev=$1; shift
  printf 'FROM %s:base\n%s\nLABEL org.opencontainers.image.revision=%s\n' "$LOCAL" "$*" "$rev" \
    | docker build -q -t "$LOCAL:$rev" - >/dev/null
}
marker() {
  printf 'USER root\nRUN printf "\\n# agent-test revision %s\\n" | tee -a /usr/share/awt/deploy/compose.yaml >> /usr/share/awt/deploy/Caddyfile\nUSER awt' "$1"
}

echo "== build test images"
# Base: built without REVISION, as outside CI (label "unknown").
docker build -q -t "$LOCAL:base" . >/dev/null
overlay "$G1" "$(marker "$G1")"
overlay "$G2" "$(marker "$G2")"
overlay "$G3" "$(marker "$G3")"
overlay "$G4" "$(marker "$G4")"
overlay "$BAD2" "$(marker "$BAD2")
ENTRYPOINT [\"sleep\", \"3600\"]"
overlay "$BAD3" "$(marker "$BAD3")
ENTRYPOINT [\"sleep\", \"3600\"]"
overlay "$G5" "$(marker "$G5")"
overlay "$G6" "$(marker "$G6")"
# Healthy as training, never healthy as prod (AWT_ENV is set by compose.yaml).
overlay "$PRODFAIL" "$(marker "$PRODFAIL")"'
ENTRYPOINT ["/bin/sh", "-c", "if [ \"$AWT_ENV\" = prod ]; then exec sleep 3600; fi; exec /usr/local/bin/entrypoint.sh \"$@\"", "sh"]
CMD ["serve"]'
# Starts, never answers /healthz.
overlay "$BAD" "$(marker "$BAD")
ENTRYPOINT [\"sleep\", \"3600\"]"
overlay "$BADCOMPOSE" 'USER root
RUN echo "services: [this is not { compose" >> /usr/share/awt/deploy/compose.yaml
USER awt'
# No label at all (the base has org.opencontainers.image.revision=unknown).
printf 'FROM scratch\nENV AWT_AGENT_TEST=1\n' | docker build -q -t "$LOCAL:nolabel" - >/dev/null
overlay "$BADCFG" 'USER root
RUN echo "this is { not a caddyfile" >> /usr/share/awt/deploy/Caddyfile
USER awt'

echo "== start the registry on 127.0.0.1:$port"
docker run -d --name "$REG_NAME" -p "127.0.0.1:$port:5000" "$REGISTRY_IMAGE" >/dev/null
i=0
until curl -fs "http://127.0.0.1:$port/v2/" >/dev/null; do
  i=$((i + 1)); [ "$i" -lt 30 ] || { echo "FAIL: registry did not start" >&2; exit 1; }
  sleep 1
done

echo "== set up $root as /opt/awt"
mkdir -p "$root/deploy" "$root/caddy" "$root/certs" "$root/state"
cp deploy/deploy.sh "$root/deploy/"
cp deploy/agent.sh "$root/deploy/" 2>/dev/null || echo "  (deploy/agent.sh missing)"
cp compose.yaml "$root/"
cp Caddyfile "$root/caddy/"
cat > "$root/.env" <<EOF
AWT_IMAGE=$IMAGE
AWT_CHANNEL=test
AWT_PROD_HOST=prod.invalid
AWT_TRAINING_HOST=training.invalid
AWT_PROD_TAG=none
AWT_TRAINING_TAG=none
EOF
for e in prod training; do
  printf 'AWT_BASE_URL=http://%s.invalid\n' "$e" > "$root/.env.$e"
done
# Test only: no published ports (80/443 may be taken), and a subnet Docker
# picks, so the test never clashes with a local stack.
cat > "$root/compose.override.yaml" <<'EOF'
services:
  caddy:
    ports: !reset []
networks:
  edge:
    ipam: !reset {}
EOF
openssl req -x509 -newkey rsa:2048 -nodes -days 2 -subj /CN=prod.invalid \
  -addext "subjectAltName=DNS:prod.invalid,DNS:training.invalid" \
  -keyout "$root/certs/origin.key" -out "$root/certs/origin.pem" 2>/dev/null
chmod 644 "$root/certs/origin.key"
dc down -v --remove-orphans >/dev/null 2>&1 || true

echo "== case: lock held (another run in progress)"
publish "$G1"
# The holder's sleeps do not inherit the lock, so killing the subshell frees it.
( exec 9>"$root/agent.lock"; flock 9; while :; do sleep 1 9>&-; done ) &
holder=$!
sleep 1
run_agent
kill "$holder" 2>/dev/null || true; wait "$holder" 2>/dev/null || true
check "exit 0" [ "$rc" = 0 ]
check "silent" [ -z "$out" ]
check "nothing deployed" [ "$(tag_of AWT_TRAINING_TAG) $(tag_of AWT_PROD_TAG)" = "none none" ]
check "lock released" flock -n "$root/agent.lock" true

echo "== case: happy path (new revision $G1)"
run_agent
check "exit 0" [ "$rc" = 0 ]
check "training tag = $G1" [ "$(tag_of AWT_TRAINING_TAG)" = "$G1" ]
check "prod tag = $G1" [ "$(tag_of AWT_PROD_TAG)" = "$G1" ]
check "baked compose.yaml and Caddyfile installed" has_marker "$G1"
check "logs deployed" eval 'printf "%s" "$out" | grep -q "deployed $G1"'
check "no failed revision" [ -z "$(failed_list)" ]
check "staging cleaned" no_staging

echo "== case: no-op (moving tag on the deployed revision)"
before="$(ctr awt-training) $(ctr awt-prod) $(ctr caddy)"
sum=$(files_sum)
run_agent
check "exit 0" [ "$rc" = 0 ]
check "silent" [ -z "$out" ]
check "no restart" [ "$(ctr awt-training) $(ctr awt-prod) $(ctr caddy)" = "$before" ]
check "files untouched" [ "$(files_sum)" = "$sum" ]

echo "== case: bad revision $BAD (app never healthy)"
publish "$BAD"
prod_before=$(ctr awt-prod)
run_agent
check "exit non-zero" [ "$rc" != 0 ]
check "training rolled back to $G1" [ "$(tag_of AWT_TRAINING_TAG)" = "$G1" ]
check "prod tag untouched" [ "$(tag_of AWT_PROD_TAG)" = "$G1" ]
check "prod container untouched" [ "$(ctr awt-prod)" = "$prod_before" ]
check "revision recorded as failed" [ "$(failed_list)" = "$BAD" ]
check "previous compose.yaml and Caddyfile restored" has_marker "$G1"
check "training runs $G1" eval '[ "$(ctr_image awt-training)" = "$IMAGE:$G1" ]'
echo "  -- next run"
before="$(ctr awt-training) $(ctr awt-prod) $(ctr caddy)"
run_agent
check "exit 0" [ "$rc" = 0 ]
check "logs skip" [ "$out" = "agent: skip $BAD: failed before" ]
check "no restart" [ "$(ctr awt-training) $(ctr awt-prod) $(ctr caddy)" = "$before" ]
check "recorded once" [ "$(failed_list)" = "$BAD" ]

echo "== case: bad config $BADCFG (invalid baked Caddyfile)"
publish "$BADCFG"
before="$(ctr awt-training) $(ctr awt-prod) $(ctr caddy)"
sum=$(files_sum)
run_agent
check "exit non-zero" [ "$rc" != 0 ]
check "files untouched" [ "$(files_sum)" = "$sum" ]
check "tags untouched" [ "$(tag_of AWT_TRAINING_TAG) $(tag_of AWT_PROD_TAG)" = "$G1 $G1" ]
check "no restart" [ "$(ctr awt-training) $(ctr awt-prod) $(ctr caddy)" = "$before" ]
check "revision recorded as failed" eval '[ "$(failed_list)" = "$(printf "%s\n%s" "$BAD" "$BADCFG")" ]'
check "staging cleaned" no_staging

echo "== case: bad config $BADCOMPOSE (invalid baked compose.yaml)"
publish "$BADCOMPOSE"
before="$(ctr awt-training) $(ctr awt-prod) $(ctr caddy)"
sum=$(files_sum)
failed=$(failed_list)
run_agent
check "exit non-zero" [ "$rc" != 0 ]
check "logs invalid compose.yaml" eval 'printf "%s" "$out" | grep -q "compose.yaml of $BADCOMPOSE is invalid"'
check "files untouched" [ "$(files_sum)" = "$sum" ]
check "tags untouched" [ "$(tag_of AWT_TRAINING_TAG) $(tag_of AWT_PROD_TAG)" = "$G1 $G1" ]
check "no restart" [ "$(ctr awt-training) $(ctr awt-prod) $(ctr caddy)" = "$before" ]
check "revision recorded as failed" eval '[ "$(failed_list)" = "$(printf "%s\n%s" "$failed" "$BADCOMPOSE")" ]'

echo "== case: no label at all"
docker tag "$LOCAL:nolabel" "$IMAGE:test"
docker push -q "$IMAGE:test" >/dev/null
failed=$(failed_list)
run_agent
check "exit non-zero" [ "$rc" != 0 ]
check "logs refused" eval 'printf "%s" "$out" | grep -q "refused"'
check "files untouched" [ "$(files_sum)" = "$sum" ]
check "tags untouched" [ "$(tag_of AWT_TRAINING_TAG) $(tag_of AWT_PROD_TAG)" = "$G1 $G1" ]
check "no restart" [ "$(ctr awt-training) $(ctr awt-prod) $(ctr caddy)" = "$before" ]
check "failed revisions unchanged" [ "$(failed_list)" = "$failed" ]

echo "== case: label 'unknown' (image built outside CI)"
docker tag "$LOCAL:base" "$IMAGE:test"
docker push -q "$IMAGE:test" >/dev/null
before="$(ctr awt-training) $(ctr awt-prod) $(ctr caddy)"
sum=$(files_sum)
failed=$(failed_list)
run_agent
check "exit non-zero" [ "$rc" != 0 ]
check "logs refused" eval 'printf "%s" "$out" | grep -q "refused"'
check "files untouched" [ "$(files_sum)" = "$sum" ]
check "tags untouched" [ "$(tag_of AWT_TRAINING_TAG) $(tag_of AWT_PROD_TAG)" = "$G1 $G1" ]
check "no restart" [ "$(ctr awt-training) $(ctr awt-prod) $(ctr caddy)" = "$before" ]
check "failed revisions unchanged" [ "$(failed_list)" = "$failed" ]

echo "== case: registry down (new revision $G2 published, then the registry stops)"
publish "$G2"
docker stop "$REG_NAME" >/dev/null
run_agent
check "exit non-zero" [ "$rc" != 0 ]
check "one error line" [ "$(printf '%s\n' "$out" | wc -l)" = 1 ]
check "files untouched" [ "$(files_sum)" = "$sum" ]
check "tags untouched" [ "$(tag_of AWT_TRAINING_TAG) $(tag_of AWT_PROD_TAG)" = "$G1 $G1" ]
check "no restart" [ "$(ctr awt-training) $(ctr awt-prod) $(ctr caddy)" = "$before" ]
check "pull failure not recorded" [ "$(failed_list)" = "$failed" ]
echo "  -- registry back, next run"
docker start "$REG_NAME" >/dev/null
i=0
until curl -fs "http://127.0.0.1:$port/v2/" >/dev/null; do
  i=$((i + 1)); [ "$i" -lt 30 ] || break
  sleep 1
done
run_agent
check "exit 0" [ "$rc" = 0 ]
check "training tag = $G2" [ "$(tag_of AWT_TRAINING_TAG)" = "$G2" ]
check "prod tag = $G2" [ "$(tag_of AWT_PROD_TAG)" = "$G2" ]
check "baked files of $G2 installed" has_marker "$G2"

echo "== case: pull inside deploy.sh fails ($G3 only on the moving tag, local tag removed)"
publish_channel_only "$G3"
mv "$root/deploy/deploy.sh" "$root/deploy/deploy-real.sh"
# Drop the local tag the agent just made, so deploy.sh must pull :<rev>,
# which the registry does not have.
printf '#!/bin/sh\ndocker image rm "%s:$2" >/dev/null 2>&1 || true\nexec "%s" "$@"\n' \
  "$IMAGE" "$root/deploy/deploy-real.sh" > "$root/deploy/deploy.sh"
chmod +x "$root/deploy/deploy.sh"
before="$(ctr awt-training) $(ctr awt-prod)"
failed=$(failed_list)
run_agent
mv "$root/deploy/deploy-real.sh" "$root/deploy/deploy.sh"
check "exit non-zero" [ "$rc" != 0 ]
check "logs retry" eval 'printf "%s" "$out" | grep -q "will retry"'
check "not recorded as failed" [ "$(failed_list)" = "$failed" ]
check "tags untouched" [ "$(tag_of AWT_TRAINING_TAG) $(tag_of AWT_PROD_TAG)" = "$G2 $G2" ]
check "apps not restarted" [ "$(ctr awt-training) $(ctr awt-prod)" = "$before" ]
check "previous compose.yaml and Caddyfile restored" has_marker "$G2"

echo "== case: :<rev> missing in the registry, present locally (retry of $G3; prune age 0)"
PRUNE_DAYS=0 run_agent
check "exit 0" [ "$rc" = 0 ]
check "training tag = $G3" [ "$(tag_of AWT_TRAINING_TAG)" = "$G3" ]
check "prod tag = $G3" [ "$(tag_of AWT_PROD_TAG)" = "$G3" ]
check "baked files of $G3 installed" has_marker "$G3"
check "previous revision image kept" eval 'docker image inspect "$IMAGE:$G2" >/dev/null 2>&1'
check "older image pruned" eval '! docker image inspect "$IMAGE:$G1" >/dev/null 2>&1'

echo "== case: prod fails ($PRODFAIL healthy as training, not as prod)"
publish "$PRODFAIL"
run_agent
check "exit non-zero" [ "$rc" != 0 ]
check "revision recorded as failed" eval 'failed_list | grep -qx "$PRODFAIL"'
check "tags back to $G3" [ "$(tag_of AWT_TRAINING_TAG) $(tag_of AWT_PROD_TAG)" = "$G3 $G3" ]
check "previous compose.yaml and Caddyfile restored" has_marker "$G3"
check "training runs $G3 again" eval '[ "$(ctr_image awt-training)" = "$IMAGE:$G3" ]'
check "prod runs $G3" eval '[ "$(ctr_image awt-prod)" = "$IMAGE:$G3" ]'
check "logs restored" eval 'printf "%s" "$out" | grep -q "restored training=$G3 prod=$G3"'

echo "== case: interrupted (TERM) between the two installs ($G4)"
publish "$G4"
failed=$(failed_list)
KILL_AFTER_INSTALL=TERM run_agent
check "exit non-zero" [ "$rc" != 0 ]
check "previous compose.yaml and Caddyfile restored" has_marker "$G3"
check "tags untouched" [ "$(tag_of AWT_TRAINING_TAG) $(tag_of AWT_PROD_TAG)" = "$G3 $G3" ]
check "not recorded as failed" [ "$(failed_list)" = "$failed" ]
check "staging cleaned" no_staging

echo "== case: killed (KILL) between the two installs ($G4), then a failing revision"
KILL_AFTER_INSTALL=KILL run_agent
check "exit non-zero" [ "$rc" != 0 ]
check "compose.yaml of $G4 left behind" grep -q "# agent-test revision $G4" "$root/compose.yaml"
check "tags untouched" [ "$(tag_of AWT_TRAINING_TAG) $(tag_of AWT_PROD_TAG)" = "$G3 $G3" ]
echo "  -- next run: $BAD2 (never healthy) must restore $G3, not the leftovers"
publish "$BAD2"
run_agent
check "exit non-zero" [ "$rc" != 0 ]
check "logs kept saved state" eval 'printf "%s" "$out" | grep -q "previous run did not finish"'
check "files of $G3 restored" has_marker "$G3"
check "tags = $G3" [ "$(tag_of AWT_TRAINING_TAG) $(tag_of AWT_PROD_TAG)" = "$G3 $G3" ]
check "training runs $G3" eval '[ "$(ctr_image awt-training)" = "$IMAGE:$G3" ]'
check "staging and config container of the killed run removed" eval 'no_staging && [ -z "$(docker ps -aq --filter label=awt-agent-staging)" ]'

echo "== case: next run deploys $G4 (AWT_PRUNE_DAYS invalid)"
publish "$G4"
PRUNE_DAYS=abc run_agent
check "exit 0" [ "$rc" = 0 ]
check "tags = $G4" [ "$(tag_of AWT_TRAINING_TAG) $(tag_of AWT_PROD_TAG)" = "$G4 $G4" ]
check "baked files of $G4 installed" has_marker "$G4"
check "logs invalid prune days" eval 'printf "%s" "$out" | grep -q "AWT_PRUNE_DAYS=.abc. is not a number"'

echo "== case: marker lost after a successful deploy (killed before writing it), no-op run"
rm -f "$root/state/installed-ok"
run_agent
check "exit 0" [ "$rc" = 0 ]
check "silent" [ -z "$out" ]
check "marker written back" [ -f "$root/state/installed-ok" ]
echo "  -- next run: $BAD3 (never healthy) must restore $G4, the files of the last deploy"
publish "$BAD3"
run_agent
check "exit non-zero" [ "$rc" != 0 ]
check "saved state refreshed" eval '! printf "%s" "$out" | grep -q "previous run did not finish"'
check "files of $G4 restored" has_marker "$G4"
check "tags = $G4" [ "$(tag_of AWT_TRAINING_TAG) $(tag_of AWT_PROD_TAG)" = "$G4 $G4" ]

for v in 08 12345678901234567890; do
  [ "$v" = 08 ] && r=$G5 || r=$G6
  echo "== case: deploy $r with AWT_PRUNE_DAYS=$v"
  publish "$r"
  PRUNE_DAYS=$v run_agent
  check "exit 0" [ "$rc" = 0 ]
  check "tags = $r" [ "$(tag_of AWT_TRAINING_TAG) $(tag_of AWT_PROD_TAG)" = "$r $r" ]
  check "logs invalid prune days" eval 'printf "%s" "$out" | grep -q "AWT_PRUNE_DAYS=.$v. is not a number"'
done

if [ "$failures" -gt 0 ]; then
  echo "FAIL: $failures check(s) failed" >&2
  exit 1
fi
echo "PASS: deploy agent"
