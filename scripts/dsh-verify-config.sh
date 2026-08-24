#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
: "${MIAOZAO_SESSION_TOKEN:?Set MIAOZAO_SESSION_TOKEN to the DSH session-bound MCP token}"
export MIAOZAO_SESSION_TOKEN

output="$(docker compose -f "$COMPOSE_FILE" --profile agent run --rm --no-deps --entrypoint dsh dsh --profile web --dump-config)"
printf '%s\n' "$output"
if grep -qF 'entry "mcp-miaozao" not found' <<<"$output"; then
  echo "DSH config validation failed: mcp-miaozao was declared as a patch over a non-existent row" >&2
  exit 1
fi
if ! grep -qE '^\s*- id: mcp-miaozao\s*$' <<<"$output"; then
  echo "DSH config validation failed: mcp-miaozao is missing from --dump-config" >&2
  exit 1
fi
echo "DSH config validation passed: mcp-miaozao is registered"
