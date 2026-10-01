#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/runtime.sh"
for program in go npm node pm2 openssl; do
  command -v "$program" >/dev/null || { echo "Required command not found: $program" >&2; exit 1; }
done
load_runtime_config
(cd "$MIAO_ROOT" && npm ci --omit=dev)
mkdir -p "$MIAO_INSTALL_DIR/bin" "$MIAO_DATA_DIR" "$(dirname "$MIAO_CONFIG_FILE")"
# go.mod pins the PocketBase-compatible Go toolchain; Go downloads it if needed.
bash "$MIAO_ROOT/scripts/build.sh" "$MIAO_INSTALL_DIR/bin/miao"
if [[ ! -f "$MIAO_CONFIG_FILE" ]]; then
  settings_key="$(openssl rand -base64 24)"
  umask 077
  cat > "$MIAO_CONFIG_FILE" <<CONFIG
# MIAO configuration. Keep this file private (mode 600).
MIAO_DATA_DIR=$MIAO_DATA_DIR
MIAO_PORT=41874
HOST=0.0.0.0
AI_GATEWAY_API_KEY=
MIAO_AI_PROVIDER=vercel
MIAO_AI_BASE_URL=http://127.0.0.1:3210/api/v1
MIAO_AI_MODEL=gpt-5.2
MIAO_ADMIN_EMAILS=
MIAO_SETTINGS_ENCRYPTION_KEY=$settings_key
MIAO_PUBLIC_URL=
MIAO_MAIL_FROM=
RESEND_API_KEY=
MIAO_REGISTRATION_MODE=open
MIAO_REQUIRE_EMAIL_VERIFICATION=false
MIAO_ALLOWED_EMAIL_DOMAINS=
CONFIG
  chmod 600 "$MIAO_CONFIG_FILE"
  echo "Created configuration: $MIAO_CONFIG_FILE"
fi
echo "Single MIAO binary installed: $MIAO_INSTALL_DIR/bin/miao"
echo "Existing PocketBase data is reused at: $MIAO_DATA_DIR/pb_data"
echo "Run: $SCRIPT_DIR/start.sh"
