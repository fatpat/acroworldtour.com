#!/bin/sh
# Backup/restore test: replicate to a local MinIO, wipe the app volume, start
# again, and check the data was restored from the replica. Run via
# `make backup-restore-test` (it builds awt:dev first).
set -eu
cd "$(dirname "$0")/.."

export COMPOSE_PROJECT_NAME=awt-restore-test
# compose.yaml interpolates these for every service, even unused ones.
export AWT_PROD_HOST=prod.invalid AWT_TRAINING_HOST=training.invalid
export AWT_TEST_IMAGE="${AWT_TEST_IMAGE:-awt:dev}"
DC="docker compose --profile test"
EMAIL=restore@example.org
PASS='correct-horse-battery-staple-42'

cleanup() { $DC down -v --remove-orphans >/dev/null 2>&1 || true; }
trap cleanup EXIT
fail() { echo "FAIL: $*" >&2; $DC logs awt-test 2>&1 | tail -30 >&2; exit 1; }

wait_healthy() {
  i=0
  while [ "$i" -lt 60 ]; do
    cid=$($DC ps -q awt-test)
    [ -n "$cid" ] && [ "$(docker inspect -f '{{.State.Health.Status}}' "$cid")" = healthy ] && return 0
    i=$((i + 1)); sleep 2
  done
  fail "awt-test did not become healthy"
}

cleanup
echo "== start minio + awt (empty volume, empty replica)"
$DC up -d minio awt-test
wait_healthy

echo "== write data"
printf '%s\n' "$PASS" | $DC exec -T awt-test awt user add --email "$EMAIL" --name Restore --role admin \
  || fail "user add"
sleep 5 # litestream syncs every second; leave margin for the WAL to be shipped

echo "== stop, delete the volume"
$DC stop awt-test
$DC rm -f awt-test >/dev/null
docker volume rm "${COMPOSE_PROJECT_NAME}_test_data" >/dev/null

echo "== start again, expect restore from the replica"
$DC up -d awt-test
wait_healthy
$DC logs awt-test 2>&1 | grep -qi "restoring snapshot" || fail "no restore in the logs"

printf '%s\n' "$PASS" | $DC exec -T awt-test awt user passwd --email "$EMAIL" >/dev/null \
  || fail "user $EMAIL not found after restore"

echo "PASS: data restored from the replica"
