#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/runtime.sh"
load_runtime_config
POCKETBASE_URL="${POCKETBASE_URL:-http://127.0.0.1:${POCKETBASE_PORT}}"
export MIAO_DATA_DIR MIAO_BACKUP_DIR MIAO_BACKUP_RETENTION_DAYS POCKETBASE_URL POCKETBASE_SUPERUSER_EMAIL POCKETBASE_SUPERUSER_PASSWORD
MIAO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
if [[ "${2:-}" != "--confirm" ]]; then
  echo "Restore requires an archive and --confirm; no service was stopped." >&2
  exit 1
fi
if [[ ! -f "${1:-}" ]]; then
  echo "Backup archive does not exist; no service was stopped." >&2
  exit 1
fi
prepare_pm2_environment
pm2_command stop miao-platform
export MIAO_RESTORE_PLATFORM_STOPPED=true
exec node "$MIAO_ROOT/scripts/restore.js" "$@"
