#!/bin/sh
# Container entrypoint. With an S3 bucket configured: restore the database from
# the Litestream replica if the volume has none, then run the app under
# `litestream replicate -exec` (litestream supervises awt and forwards signals).
# Without a bucket (local docker run, dev): just run awt.
set -eu

if [ "${1:-}" != "serve" ] || [ -z "${AWT_S3_BUCKET:-}" ]; then
  exec awt "$@"
fi

export AWT_DB_PATH="${AWT_DB_PATH:-/data/awt.db}"
export AWT_ENV="${AWT_ENV:-prod}"
export LITESTREAM_S3_REGION="${LITESTREAM_S3_REGION:-us-east-1}"
# <AWT_BUCKET_PREFIX>litestream, normalised by awt itself (decision 0017).
# An invalid prefix stops the container here, before any restore.
LITESTREAM_REPLICA_PATH=$(awt bucket-path litestream) || exit 1
export LITESTREAM_REPLICA_PATH
CFG=/etc/litestream.yml

litestream restore -config "$CFG" -if-db-not-exists -if-replica-exists "$AWT_DB_PATH"
exec litestream replicate -config "$CFG" -exec "awt serve"
