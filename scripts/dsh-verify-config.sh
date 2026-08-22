#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
MIAOZAO_MCP_TOKEN="${MIAOZAO_MCP_TOKEN:?Set MIAOZAO_MCP_TOKEN to an app-scoped token}"
export MIAOZAO_MCP_TOKEN

output="$(docker compose -f "$COMPOSE_FILE" --profile agent run --rm --no-deps --entrypoint dsh dsh --profile web --dump-config)"
printf '%s\n' "$output"
if ! grep -q 'mcp-miaozao' <<<"$output"; then
  echo "DSH config validation failed: mcp-miaozao is missing from --dump-config" >&2
  exit 1
fi
echo "DSH config validation passed: mcp-miaozao is present"

