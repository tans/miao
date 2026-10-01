#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/runtime.sh"
load_runtime_config
[[ $# == 2 && "$2" == --confirm && -f "$1" ]] || { echo "Usage: restore.sh ARCHIVE --confirm; no service was stopped." >&2; exit 1; }
prepare_pm2_environment
[[ -x "$MIAO_BIN" ]] || { echo "MIAO binary is not installed; no service was stopped." >&2; exit 1; }
pm2_command stop miao-platform >/dev/null 2>&1 || true
pm2_command delete miao-pocketbase >/dev/null 2>&1 || true
export MIAO_DATA_DIR MIAO_SETTINGS_ENCRYPTION_KEY
"$MIAO_BIN" restore "$1" --confirm
pm2_command save
