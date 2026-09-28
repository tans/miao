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
  for key in MIAO_DATA_DIR MIAO_PM2_HOME POCKETBASE_PORT MIAO_PORT HOST POCKETBASE_SUPERUSER_EMAIL POCKETBASE_SUPERUSER_PASSWORD AI_GATEWAY_API_KEY; do
    if [[ -z "${!key:-}" ]]; then
      value="$(read_config_value "$key")"
      [[ -z "$value" ]] || printf -v "$key" '%s' "$value"
    fi
  done
  MIAO_DATA_DIR="${MIAO_DATA_DIR:-$(runtime_default_data_dir)}"
  MIAO_PM2_HOME="${MIAO_PM2_HOME:-${HOME}/.pm2-miao}"
  POCKETBASE_PORT="${POCKETBASE_PORT:-8090}"
  MIAO_PORT="${MIAO_PORT:-41874}"
  HOST="${HOST:-0.0.0.0}"
  POCKETBASE_SUPERUSER_EMAIL="${POCKETBASE_SUPERUSER_EMAIL:-}"
  POCKETBASE_SUPERUSER_PASSWORD="${POCKETBASE_SUPERUSER_PASSWORD:-}"
  AI_GATEWAY_API_KEY="${AI_GATEWAY_API_KEY:-}"
}

prepare_pm2_environment() {
  export MIAO_ROOT MIAO_INSTALL_DIR MIAO_CONFIG_FILE MIAO_DATA_DIR
  export MIAO_BUN_BIN="$(command -v bun)"
  if [[ "$MIAO_PM2_HOME" == *" "* ]]; then
    echo "MIAO_PM2_HOME must not contain spaces; PM2 requires a path without spaces." >&2
    return 1
  fi
  export MIAO_PM2_HOME PM2_HOME="$MIAO_PM2_HOME"
  mkdir -p "$PM2_HOME" "$MIAO_DATA_DIR/logs"
}

pm2_command() {
  bun "$MIAO_ROOT/node_modules/pm2/bin/pm2" "$@"
}

require_runtime_config() {
  if [[ -z "$POCKETBASE_SUPERUSER_EMAIL" || -z "$POCKETBASE_SUPERUSER_PASSWORD" ]]; then
    echo "Set POCKETBASE_SUPERUSER_EMAIL and POCKETBASE_SUPERUSER_PASSWORD in $MIAO_CONFIG_FILE" >&2
    return 1
  fi
  if [[ ! "$POCKETBASE_PORT" =~ ^[0-9]+$ || ! "$MIAO_PORT" =~ ^[0-9]+$ ]]; then
    echo "POCKETBASE_PORT and MIAO_PORT must be numeric ports" >&2
    return 1
  fi
}
