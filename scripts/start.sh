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
  echo "curl is required to check service readiness." >&2
  exit 1
fi

PB_DATA_DIR="$MIAO_DATA_DIR/pb_data"
PB_PUBLIC_DIR="$MIAO_DATA_DIR/pb_public"
PB_MIGRATIONS_DIR="$MIAO_DATA_DIR/pb_migrations"
mkdir -p "$PB_DATA_DIR" "$PB_PUBLIC_DIR" "$PB_MIGRATIONS_DIR" "$MIAO_DATA_DIR/logs"

# Keep PM2-generated schema snapshots with user data; copy only checked-in
# versioned migrations into the persistent runtime migration directory.
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

prepare_pm2_environment
pm2_command --version >/dev/null
pm2_command delete miao-platform >/dev/null 2>&1 || true
pm2_command delete miao-pocketbase >/dev/null 2>&1 || true
pm2_command start "$MIAO_ROOT/ecosystem.config.cjs" --only miao-pocketbase --update-env

ready=0
for attempt in $(seq 1 40); do
  if curl --noproxy '*' --fail --silent "http://127.0.0.1:${POCKETBASE_PORT}/api/health" >/dev/null; then
    ready=1
    break
  fi
  sleep 0.5
done
if [[ "$ready" != 1 ]]; then
  echo "PocketBase did not become ready. Check: $SCRIPT_DIR/logs.sh miao-pocketbase" >&2
  exit 1
fi

pm2_command start "$MIAO_ROOT/ecosystem.config.cjs" --only miao-platform --update-env

ready=0
for attempt in $(seq 1 40); do
  if curl --noproxy '*' --fail --silent "http://127.0.0.1:${MIAO_PORT}/api/health" | grep -q '"service":"miao"'; then
    ready=1
    break
  fi
  sleep 0.5
done
if [[ "$ready" != 1 ]]; then
  echo "MIAO did not become ready. Check: $SCRIPT_DIR/logs.sh miao-platform" >&2
  exit 1
fi

pm2_command save
echo "MIAO is running under PM2."
echo "MIAO:       http://127.0.0.1:${MIAO_PORT}"
echo "PocketBase: http://127.0.0.1:${POCKETBASE_PORT} (localhost only)"
echo "Data:       $MIAO_DATA_DIR"
echo "Status:     $SCRIPT_DIR/status.sh"
echo "Logs:       $SCRIPT_DIR/logs.sh"
