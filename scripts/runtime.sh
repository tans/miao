#!/usr/bin/env bash

set -euo pipefail

runtime_default_install_dir() {
  case "$(uname -s)" in
    Darwin) printf '%s\n' "${HOME}/Library/Application Support/Miao" ;;
    Linux) printf '%s\n' "${XDG_DATA_HOME:-${HOME}/.local/share}/miao" ;;
    *) echo "Unsupported operating system: $(uname -s)" >&2; return 1 ;;
  esac
}

MIAO_INSTALL_DIR="${MIAO_INSTALL_DIR:-$(runtime_default_install_dir)}"
MIAO_CONFIG_FILE="${MIAO_CONFIG_FILE:-${MIAO_INSTALL_DIR}/miao.env}"
MIAO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

runtime_default_data_dir() {
  case "$(uname -s)" in
    Darwin) printf '%s\n' "${HOME}/Library/Application Support/Miao/data" ;;
    Linux) printf '%s\n' "${XDG_DATA_HOME:-${HOME}/.local/share}/miao" ;;
    *) echo "Unsupported operating system: $(uname -s)" >&2; return 1 ;;
  esac
}

read_config_value() {
  local key="$1" line value
  [[ -f "$MIAO_CONFIG_FILE" ]] || return 0
  line="$(grep -m 1 -E "^[[:space:]]*${key}=" "$MIAO_CONFIG_FILE" || true)"
  [[ -n "$line" ]] || return 0
  value="${line#*=}"
  value="${value%$'\r'}"
  if [[ "$value" == \"*\" && "$value" == *\" ]]; then value="${value:1:${#value}-2}"; fi
  if [[ "$value" == \'*\' && "$value" == *\' ]]; then value="${value:1:${#value}-2}"; fi
  printf '%s' "$value"
}

load_runtime_config() {
  local key value
  for key in MIAO_DATA_DIR MIAO_BACKUP_DIR MIAO_BACKUP_RETENTION_DAYS MIAO_PORT HOST AI_GATEWAY_API_KEY MIAO_AI_PROVIDER MIAO_AI_BASE_URL MIAO_AI_MODEL MIAO_ADMIN_EMAILS MIAO_SETTINGS_ENCRYPTION_KEY MIAO_PUBLIC_URL MIAO_MAIL_FROM RESEND_API_KEY MIAO_REQUIRE_EMAIL_VERIFICATION MIAO_REGISTRATION_MODE MIAO_ALLOWED_EMAIL_DOMAINS; do
    if [[ -z "${!key:-}" ]]; then
      value="$(read_config_value "$key")"
      [[ -z "$value" ]] || printf -v "$key" '%s' "$value"
    fi
  done
  MIAO_DATA_DIR="${MIAO_DATA_DIR:-$(runtime_default_data_dir)}"
  MIAO_PORT="${MIAO_PORT:-41874}"
  HOST="${HOST:-0.0.0.0}"
  AI_GATEWAY_API_KEY="${AI_GATEWAY_API_KEY:-}"
  MIAO_BACKUP_DIR="${MIAO_BACKUP_DIR:-}"
  MIAO_BACKUP_RETENTION_DAYS="${MIAO_BACKUP_RETENTION_DAYS:-30}"
}

prepare_pm2_environment() {
  export MIAO_ROOT MIAO_INSTALL_DIR MIAO_CONFIG_FILE MIAO_DATA_DIR
  export MIAO_BIN="$MIAO_INSTALL_DIR/bin/miao"
  if ! command -v pm2 >/dev/null 2>&1; then
    echo "System PM2 is required. Install PM2 globally and ensure pm2 is on PATH." >&2
    return 1
  fi
  mkdir -p "$MIAO_DATA_DIR/logs"
}

pm2_command() {
  command pm2 "$@"
}

require_runtime_config() {
  if [[ ! "$MIAO_PORT" =~ ^[0-9]+$ ]]; then
    echo "MIAO_PORT must be a numeric port" >&2
    return 1
  fi
}
