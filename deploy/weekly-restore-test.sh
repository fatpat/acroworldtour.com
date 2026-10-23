#!/bin/sh
# Weekly backup check, run from cron on the VPS (see docs/ops.md): restore the
# latest prod replica into a throwaway container, run SQLite's integrity check,
# require at least one user, and open it with awt (applies migrations).
# On failure: exit non-zero (cron mails it) and ping $RESTORE_TEST_PING_URL/fail
# if set (healthchecks.io style); on success ping $RESTORE_TEST_PING_URL.
set -eu
cd "$(dirname "$0")/.."
img="$(sed -n 's/^AWT_IMAGE=//p' .env):$(sed -n 's/^AWT_PROD_TAG=//p' .env)"
ping_url=${RESTORE_TEST_PING_URL:-}

if docker run --rm --env-file .env.prod --entrypoint sh "$img" -c '
    set -eu
    export LITESTREAM_S3_REGION="${LITESTREAM_S3_REGION:-us-east-1}"
    LITESTREAM_REPLICA_PATH=$(awt bucket-path litestream)
    export LITESTREAM_REPLICA_PATH
    litestream restore -config /etc/litestream.yml -o /tmp/restored.db "$AWT_DB_PATH"
    [ "$(sqlite3 /tmp/restored.db "PRAGMA integrity_check")" = ok ] || { echo "integrity_check failed" >&2; exit 1; }
    users=$(sqlite3 /tmp/restored.db "SELECT count(*) FROM users")
    [ "$users" -gt 0 ] || { echo "no users in the restored database" >&2; exit 1; }
    AWT_DB_PATH=/tmp/restored.db awt migrate
    echo "restored $(stat -c %s /tmp/restored.db) bytes, $users users"
  '; then
  echo "restore test OK"
  [ -z "$ping_url" ] || wget -q -O /dev/null "$ping_url" || true
else
  echo "restore test FAILED" >&2
  [ -z "$ping_url" ] || wget -q -O /dev/null "$ping_url/fail" || true
  exit 1
fi
