#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIAO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TAILWIND="$MIAO_ROOT/node_modules/.bin/tailwindcss"
ESBUILD="$MIAO_ROOT/node_modules/.bin/esbuild"

[[ -x "$TAILWIND" ]] || { echo "Run npm install first; Tailwind CLI is required" >&2; exit 1; }
[[ -x "$ESBUILD" ]] || { echo "Run npm install first; esbuild is required" >&2; exit 1; }

"$TAILWIND" -i "$MIAO_ROOT/scripts/ui-src.css" -o "$MIAO_ROOT/public/vendor/ui.css" --minify
bash "$SCRIPT_DIR/build-ui-renderer.sh"
