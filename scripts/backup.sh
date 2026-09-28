#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/runtime.sh"
load_runtime_config
POCKETBASE_URL="${POCKETBASE_URL:-http://127.0.0.1:${POCKETBASE_PORT}}"
export MIAO_DATA_DIR MIAO_BACKUP_DIR MIAO_BACKUP_RETENTION_DAYS POCKETBASE_URL POCKETBASE_SUPERUSER_EMAIL POCKETBASE_SUPERUSER_PASSWORD
MIAO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
exec bun "$MIAO_ROOT/scripts/backup.js"
