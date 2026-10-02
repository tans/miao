#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/runtime.sh
source "$SCRIPT_DIR/runtime.sh"
load_runtime_config
prepare_pm2_environment

pm2_command stop miao-platform || true
pm2_command save
