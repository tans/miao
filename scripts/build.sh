#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIAO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUTPUT="${1:-$MIAO_ROOT/dist/miao}"
mkdir -p "$(dirname "$OUTPUT")"
version="${MIAO_VERSION:-$(git -C "$MIAO_ROOT" rev-parse --short HEAD 2>/dev/null || echo dev)}"
[[ "$version" =~ ^[a-zA-Z0-9][a-zA-Z0-9._-]*$ ]] || { echo "Invalid MIAO_VERSION" >&2; exit 1; }
if [[ "${MIAO_SKIP_ASSETS:-}" != "1" ]]; then
  if [[ -x "$MIAO_ROOT/node_modules/.bin/tailwindcss" && -x "$MIAO_ROOT/node_modules/.bin/esbuild" ]]; then
    bash "$SCRIPT_DIR/build-assets.sh"
  else
    echo "Node asset toolchain not found; using committed frontend assets" >&2
  fi
fi
(cd "$MIAO_ROOT" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$OUTPUT" .)
chmod 0755 "$OUTPUT"
echo "Built MIAO Go server: $OUTPUT"
