#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/runtime.sh
source "$SCRIPT_DIR/runtime.sh"
load_runtime_config
require_runtime_config

POCKETBASE="$MIAO_INSTALL_DIR/bin/pocketbase"
if [[ ! -x "$POCKETBASE" ]]; then
  echo "PocketBase is not installed. Run $SCRIPT_DIR/install.sh first." >&2
  exit 1
fi
if ! command -v bun >/dev/null 2>&1; then
  echo "Bun is required. Install Bun, then run $SCRIPT_DIR/install.sh." >&2
  exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required to check PocketBase readiness." >&2
  exit 1
fi

PB_DATA_DIR="$MIAO_DATA_DIR/pb_data"
PB_PUBLIC_DIR="$MIAO_DATA_DIR/pb_public"
PB_MIGRATIONS_DIR="$MIAO_DATA_DIR/pb_migrations"
LOG_DIR="$MIAO_DATA_DIR/logs"
mkdir -p "$PB_DATA_DIR" "$PB_PUBLIC_DIR" "$PB_MIGRATIONS_DIR" "$LOG_DIR"

# Keep generated PocketBase collection snapshots with persistent app data,
# while copying the checked-in, versioned migrations on first start.
for migration in "$MIAO_ROOT"/pb_migrations/*.js; do
  [[ -f "$migration" ]] || continue
  target="$PB_MIGRATIONS_DIR/$(basename "$migration")"
  if [[ ! -e "$target" ]]; then
    cp "$migration" "$target"
  elif ! cmp -s "$migration" "$target"; then
    echo "Migration differs from the installed version: $target" >&2
    echo "Do not edit applied migration files; add a new migration instead." >&2
    exit 1
  fi
done

"$POCKETBASE" superuser upsert "$POCKETBASE_SUPERUSER_EMAIL" "$POCKETBASE_SUPERUSER_PASSWORD" \
  --dir "$PB_DATA_DIR" --migrationsDir "$PB_MIGRATIONS_DIR" --publicDir "$PB_PUBLIC_DIR"

"$POCKETBASE" serve --http="127.0.0.1:${POCKETBASE_PORT}" \
  --dir "$PB_DATA_DIR" --migrationsDir "$PB_MIGRATIONS_DIR" --publicDir "$PB_PUBLIC_DIR" \
  >> "$LOG_DIR/pocketbase.log" 2>&1 &
POCKETBASE_PID=$!

cleanup() {
  trap - EXIT INT TERM
  if kill -0 "$POCKETBASE_PID" 2>/dev/null; then
    kill -TERM "$POCKETBASE_PID" 2>/dev/null || true
    wait "$POCKETBASE_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

ready=0
for attempt in $(seq 1 40); do
  if ! kill -0 "$POCKETBASE_PID" 2>/dev/null; then
    echo "PocketBase exited during startup. See $LOG_DIR/pocketbase.log" >&2
    exit 1
  fi
  if curl --fail --silent "http://127.0.0.1:${POCKETBASE_PORT}/api/health" >/dev/null; then
    ready=1
    break
  fi
  sleep 0.5
done
if [[ "$ready" != 1 ]]; then
  echo "PocketBase did not become ready. See $LOG_DIR/pocketbase.log" >&2
  exit 1
fi

echo "Starting MIAO on ${HOST}:${MIAO_PORT}; PocketBase data: $PB_DATA_DIR"
cd "$MIAO_ROOT"
MIAO_DATA_DIR="$MIAO_DATA_DIR" POCKETBASE_URL="http://127.0.0.1:${POCKETBASE_PORT}" \
  HOST="$HOST" PORT="$MIAO_PORT" \
  bun --env-file="$MIAO_CONFIG_FILE" run start
