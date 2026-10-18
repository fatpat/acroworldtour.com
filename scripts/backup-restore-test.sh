#!/bin/sh
# Backup/restore test: replicate to a local MinIO, wipe the app volume, start
# again, and check the data was restored from the replica. Run via
# `make backup-restore-test` (it builds awt:dev first).
set -eu
cd "$(dirname "$0")/.."

export COMPOSE_PROJECT_NAME=awt-restore-test
# compose.yaml interpolates these for every service, even unused ones.
export AWT_PROD_HOST=prod.invalid AWT_TRAINING_HOST=training.invalid
export AWT_PROD_TAG=unused AWT_TRAINING_TAG=unused AWT_IMAGE=unused
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

# Number of WAL segments litestream has shipped to the replica.
wal_count() {
  $DC exec -T awt-test litestream wal -config /etc/litestream.yml /data/awt.db 2>/dev/null | tail -n +2 | wc -l
}

# Wait until the replica stops changing (initial snapshot and migrations shipped).
wait_replica_idle() {
  last=-1; i=0
  while [ "$i" -lt 30 ]; do
    n=$(wal_count)
    [ "$n" -ge 1 ] && [ "$n" = "$last" ] && return 0
    last=$n; i=$((i + 1)); sleep 2
  done
  fail "replica never became idle"
}

cleanup
echo "== start minio + awt (empty volume, empty replica)"
$DC up -d minio awt-test
wait_healthy
wait_replica_idle
base=$(wal_count)

echo "== write data"
printf '%s\n' "$PASS" | $DC exec -T awt-test awt user add --email "$EMAIL" --name Restore --role admin \
  || fail "user add"
i=0
until [ "$(wal_count)" -gt "$base" ]; do
  i=$((i + 1)); [ "$i" -lt 30 ] || fail "write was not replicated"
  sleep 1
done

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
