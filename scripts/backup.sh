#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/runtime.sh"
load_runtime_config
prepare_pm2_environment
[[ -x "$MIAO_BIN" ]] || { echo "MIAO binary is not installed." >&2; exit 1; }
was_online="$(pm2_command jlist | node -e 'let s="";process.stdin.on("data",d=>s+=d);process.stdin.on("end",()=>console.log(JSON.parse(s).some(p=>p.name==="miao-platform"&&p.pm2_env.status==="online")?"yes":"no"))')"
pm2_command stop miao-platform >/dev/null 2>&1 || true
# Backups briefly stop writes. Restore keeps the service stopped for review.
resume() { if [[ "$was_online" == yes ]]; then bash "$SCRIPT_DIR/start.sh"; fi; }
trap resume EXIT
export MIAO_DATA_DIR MIAO_BACKUP_DIR MIAO_BACKUP_RETENTION_DAYS MIAO_SETTINGS_ENCRYPTION_KEY
"$MIAO_BIN" backup
