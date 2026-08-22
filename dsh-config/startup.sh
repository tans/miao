#!/usr/bin/env sh
set -eu

: "${MIAOZAO_MCP_URL:?MIAOZAO_MCP_URL is required}"
: "${MIAOZAO_MCP_TOKEN:?MIAOZAO_MCP_TOKEN is required}"
exec "$@"
