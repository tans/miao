#!/usr/bin/env bash

set -euo pipefail

POCKETBASE="${MIAO_INSTALL_DIR:?MIAO_INSTALL_DIR is required}/bin/pocketbase"
DATA_DIR="${MIAO_DATA_DIR:?MIAO_DATA_DIR is required}"
PORT="${POCKETBASE_PORT:-8090}"

exec "$POCKETBASE" serve --http="127.0.0.1:${PORT}" \
  --dir "$DATA_DIR/pb_data" \
  --migrationsDir "$DATA_DIR/pb_migrations" \
  --publicDir "$DATA_DIR/pb_public" \
  --hooksDir "${MIAO_ROOT:?MIAO_ROOT is required}/pb_hooks"
