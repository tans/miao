#!/usr/bin/env bash
# Build the React authentication entry into a browser bundle.
# Mirrors build-ui-renderer.sh: esbuild, ESM, production, minified.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIAO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

command -v node >/dev/null || { echo "node is required to bundle the auth screen" >&2; exit 1; }
[[ -x "$MIAO_ROOT/node_modules/.bin/esbuild" ]] || { echo "Run npm install first; esbuild is required" >&2; exit 1; }

"$MIAO_ROOT/node_modules/.bin/esbuild" \
  "$MIAO_ROOT/public/modules/auth-react.jsx" \
  --bundle \
  --format=esm \
  --jsx=automatic \
  --define:process.env.NODE_ENV='"production"' \
  --minify \
  --outfile="$MIAO_ROOT/public/modules/auth-react.bundle.js"

echo "Rebuilt public/modules/auth-react.bundle.js"
