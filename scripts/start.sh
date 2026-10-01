#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/runtime.sh"
load_runtime_config
require_runtime_config
MIAO_BINARY="$MIAO_INSTALL_DIR/bin/miao"
[[ -x "$MIAO_BINARY" ]] || { echo "Run $SCRIPT_DIR/install.sh first." >&2; exit 1; }
command -v curl >/dev/null || { echo "curl is required to check readiness." >&2; exit 1; }
prepare_pm2_environment
# Retire the former standalone process before the embedded app opens its data.
pm2_command delete miao-platform >/dev/null 2>&1 || true
pm2_command delete miao-pocketbase >/dev/null 2>&1 || true
pm2_command start "$MIAO_ROOT/ecosystem.config.cjs" --only miao-platform --update-env
ready=0
for attempt in $(seq 1 60); do
  if curl --noproxy '*' --fail --silent "http://127.0.0.1:${MIAO_PORT}/api/health" | grep -q '"service":"miao"'; then ready=1; break; fi
  sleep 0.5
done
if [[ "$ready" != 1 ]]; then
  echo "MIAO did not become ready. Check: $SCRIPT_DIR/logs.sh" >&2
  exit 1
fi
pm2_command save
echo "MIAO: http://127.0.0.1:${MIAO_PORT} (embedded PocketBase; one process)"
echo "Data: $MIAO_DATA_DIR"
