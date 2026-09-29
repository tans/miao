#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/runtime.sh
source "$SCRIPT_DIR/runtime.sh"

POCKETBASE_VERSION='0.40.4'

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Required command not found: $1" >&2
    return 1
  fi
}

platform_asset() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch='amd64' ;;
    arm64|aarch64) arch='arm64' ;;
    *) echo "Unsupported CPU architecture: $arch" >&2; return 1 ;;
  esac
  case "$os" in
    darwin|linux) printf 'pocketbase_%s_%s_%s.zip\n' "$POCKETBASE_VERSION" "$os" "$arch" ;;
    *) echo "Unsupported operating system: $os" >&2; return 1 ;;
  esac
}

require_command curl
require_command unzip
require_command openssl
require_command bun
require_command pm2

load_runtime_config
(cd "$MIAO_ROOT" && bun install --frozen-lockfile)
(cd "$MIAO_ROOT" && bun run prepare:fx)

mkdir -p "$MIAO_INSTALL_DIR/bin" "$MIAO_DATA_DIR" "$(dirname "$MIAO_CONFIG_FILE")"

archive="$(platform_asset)"
pocketbase="$MIAO_INSTALL_DIR/bin/pocketbase"
installed_version=''
if [[ -x "$pocketbase" ]]; then
  installed_version="$("$pocketbase" --version | awk '{print $NF}')"
fi
if [[ "$installed_version" != "$POCKETBASE_VERSION" ]]; then
  temp_dir="$(mktemp -d)"
  trap 'rm -rf "$temp_dir"' EXIT
  curl --fail --location --silent --show-error \
    "https://github.com/pocketbase/pocketbase/releases/download/v${POCKETBASE_VERSION}/${archive}" \
    --output "$temp_dir/pocketbase.zip"
  unzip -q "$temp_dir/pocketbase.zip" -d "$temp_dir/unpacked"
  install -m 0755 "$temp_dir/unpacked/pocketbase" "$pocketbase"
  [[ "$("$pocketbase" --version | awk '{print $NF}')" == "$POCKETBASE_VERSION" ]] || {
    echo "Downloaded PocketBase version did not match $POCKETBASE_VERSION" >&2
    exit 1
  }
  rm -rf "$temp_dir"
  trap - EXIT
fi

if [[ ! -f "$MIAO_CONFIG_FILE" ]]; then
  admin_password="$(openssl rand -hex 24)"
  umask 077
  cat > "$MIAO_CONFIG_FILE" <<EOF
# MIAO server configuration. Keep this file private (mode 600).
MIAO_DATA_DIR=$MIAO_DATA_DIR
POCKETBASE_PORT=8090
MIAO_PORT=41874
HOST=0.0.0.0
POCKETBASE_SUPERUSER_EMAIL=admin@example.com
POCKETBASE_SUPERUSER_PASSWORD=$admin_password
AI_GATEWAY_API_KEY=
MIAO_AI_PROVIDER=vercel
MIAO_AI_BASE_URL=http://127.0.0.1:3210/api/v1
MIAO_AI_MODEL=gpt-5.2
MIAO_ADMIN_EMAILS=
MIAO_SETTINGS_ENCRYPTION_KEY=
EOF
  chmod 600 "$MIAO_CONFIG_FILE"
  echo "Created server credentials in: $MIAO_CONFIG_FILE"
  echo "Change the generated PocketBase password before exposing this server publicly."
fi

echo "PocketBase $POCKETBASE_VERSION installed at: $pocketbase"
echo "Persistent data directory: $MIAO_DATA_DIR"
echo "System PM2 $(pm2 --version) will manage MIAO services"
echo "Run: $SCRIPT_DIR/start.sh"
