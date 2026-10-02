#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/runtime.sh"
binary=""
case "${1:-}" in
  --binary) [[ $# == 2 ]] || { echo "Usage: install.sh [--binary PATH]" >&2; exit 1; }; binary="$2" ;;
  "") [[ ! -f "$MIAO_ROOT/miao" ]] || binary="$MIAO_ROOT/miao" ;;
  *) echo "Usage: install.sh [--binary PATH]" >&2; exit 1 ;;
esac
for program in node pm2 openssl curl; do
  command -v "$program" >/dev/null || { echo "Required command not found: $program" >&2; exit 1; }
done
load_runtime_config
mkdir -p "$MIAO_INSTALL_DIR/bin" "$MIAO_INSTALL_DIR/runtime/scripts" "$MIAO_DATA_DIR" "$(dirname "$MIAO_CONFIG_FILE")"
if [[ -n "$binary" ]]; then
  [[ -x "$binary" ]] || { echo "Binary must be executable: $binary" >&2; exit 1; }
  "$binary" version
  if [[ "$binary" != "$MIAO_INSTALL_DIR/bin/miao" ]]; then
    cp "$binary" "$MIAO_INSTALL_DIR/bin/miao.next"
    mv "$MIAO_INSTALL_DIR/bin/miao.next" "$MIAO_INSTALL_DIR/bin/miao"
  fi
else
  for program in go npm; do
    command -v "$program" >/dev/null || { echo "Required build command not found: $program" >&2; exit 1; }
  done
  (cd "$MIAO_ROOT" && npm ci --omit=dev)
  bash "$MIAO_ROOT/scripts/build.sh" "$MIAO_INSTALL_DIR/bin/miao.next"
  mv "$MIAO_INSTALL_DIR/bin/miao.next" "$MIAO_INSTALL_DIR/bin/miao"
fi
# Keep operational commands independent of the source/archive directory.
if [[ "$MIAO_ROOT" != "$MIAO_INSTALL_DIR/runtime" ]]; then
  cp "$MIAO_ROOT/ecosystem.config.cjs" "$MIAO_INSTALL_DIR/runtime/"
  for command_script in runtime install start stop status logs backup restore run-miao; do
    cp "$SCRIPT_DIR/$command_script.sh" "$MIAO_INSTALL_DIR/runtime/scripts/"
  done
fi
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
echo "Data directory: $MIAO_DATA_DIR/pb_data"
echo "Run: MIAO_INSTALL_DIR=\"$MIAO_INSTALL_DIR\" bash \"$MIAO_INSTALL_DIR/runtime/scripts/start.sh\""
