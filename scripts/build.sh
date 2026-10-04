#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIAO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUTPUT="${1:-$MIAO_ROOT/dist/miao}"
mkdir -p "$(dirname "$OUTPUT")"
version="${MIAO_VERSION:-$(git -C "$MIAO_ROOT" rev-parse --short HEAD 2>/dev/null || echo dev)}"
[[ "$version" =~ ^[a-zA-Z0-9][a-zA-Z0-9._-]*$ ]] || { echo "Invalid MIAO_VERSION" >&2; exit 1; }
(cd "$MIAO_ROOT" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$OUTPUT" .)
chmod 0755 "$OUTPUT"
echo "Built MIAO Go server: $OUTPUT"
