#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIAO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

command -v node >/dev/null || { echo "node is required to bundle the UI renderer" >&2; exit 1; }
[[ -x "$MIAO_ROOT/node_modules/.bin/esbuild" ]] || { echo "Run npm install first; esbuild is required" >&2; exit 1; }

"$MIAO_ROOT/node_modules/.bin/esbuild" \
  "$MIAO_ROOT/public/modules/ui-renderer.jsx" \
  --bundle \
  --format=esm \
  --jsx=automatic \
  --define:process.env.NODE_ENV='"production"' \
  --minify \
  --outfile="$MIAO_ROOT/public/modules/ui-renderer.bundle.js"

echo "Rebuilt public/modules/ui-renderer.bundle.js"
