#!/bin/sh
# Weekly backup check, run from cron on the VPS (see docs/ops.md): restore the
# latest prod replica into a throwaway container and open it with awt (which
# applies migrations, so a corrupt or unreadable database fails loudly).
# On failure: exit non-zero (cron mails it) and ping $RESTORE_TEST_PING_URL/fail
# if set (healthchecks.io style); on success ping $RESTORE_TEST_PING_URL.
set -eu
cd "$(dirname "$0")"
img="$(sed -n 's/^AWT_IMAGE=//p' .env):$(sed -n 's/^AWT_PROD_TAG=//p' .env)"
ping_url=${RESTORE_TEST_PING_URL:-}

if docker run --rm --env-file .env.prod --entrypoint sh "$img" -c '
    set -eu
    export LITESTREAM_S3_REGION="${LITESTREAM_S3_REGION:-us-east-1}"
    litestream restore -config /etc/litestream.yml -o /tmp/restored.db "$AWT_DB_PATH"
    AWT_DB_PATH=/tmp/restored.db awt migrate
    ls -l /tmp/restored.db
  '; then
  echo "restore test OK"
  [ -z "$ping_url" ] || wget -q -O /dev/null "$ping_url" || true
else
  echo "restore test FAILED" >&2
  [ -z "$ping_url" ] || wget -q -O /dev/null "$ping_url/fail" || true
  exit 1
fi
