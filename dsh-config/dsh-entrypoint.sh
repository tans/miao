#!/usr/bin/env sh
set -eu

mkdir -p "${DSH_HOME}/profiles"
cp -a /opt/miaozao/dsh-profiles/. "${DSH_HOME}/profiles/"
if [ ! -f "${DSH_HOME}/cordis.patch.yml" ]; then
  cp /opt/miaozao/cordis.patch.yml "${DSH_HOME}/cordis.patch.yml"
fi

if [ -z "${MIAOZAO_SESSION_TOKEN:-}" ] && [ -n "${MIAOZAO_SESSION_APP_ID:-}" ]; then
  export MIAOZAO_SESSION_TOKEN
  MIAOZAO_SESSION_TOKEN="$(MIAOZAO_INTERNAL_TOKEN="${MIAOZAO_INTERNAL_TOKEN:-${MIAOZAO_MCP_TOKEN:-}}" node --input-type=module <<'NODE'
const url = process.env.MIAOZAO_SESSION_CREATE_URL || 'http://runtime:41874/api/mcp/session/create';
const token = process.env.MIAOZAO_INTERNAL_TOKEN;
if (!token) throw new Error('MIAOZAO_INTERNAL_TOKEN is required to bootstrap DSH');
const response = await fetch(url, {
  method: 'POST',
  headers: { authorization: `Bearer ${token}`, 'content-type': 'application/json' },
  body: JSON.stringify({
    app_id: process.env.MIAOZAO_SESSION_APP_ID,
    mode: process.env.MIAOZAO_SESSION_MODE || 'user',
    agent_id: process.env.MIAOZAO_AGENT_ID || 'dsh',
    user_id: process.env.MIAOZAO_SESSION_USER_ID || null,
    permissions: (process.env.MIAOZAO_SESSION_PERMISSIONS || '').split(',').map((item) => item.trim()).filter(Boolean)
  })
});
const result = await response.json().catch(() => ({}));
if (!response.ok || !result.token) throw new Error(result.error || `DSH MCP token bootstrap failed (${response.status})`);
process.stdout.write(result.token);
NODE
)"
fi
: "${MIAOZAO_SESSION_TOKEN:?Set MIAOZAO_SESSION_TOKEN or MIAOZAO_SESSION_APP_ID plus MIAOZAO_INTERNAL_TOKEN}"

dsh --profile web --no-open --host 127.0.0.1 --port "${DSH_INTERNAL_PORT:-3080}" &
dsh_pid=$!
trap 'kill "$dsh_pid" 2>/dev/null || true' INT TERM EXIT
node /opt/miaozao/dsh-proxy.mjs &
proxy_pid=$!
while kill -0 "$dsh_pid" 2>/dev/null && kill -0 "$proxy_pid" 2>/dev/null; do
  sleep 1
done
kill "$dsh_pid" "$proxy_pid" 2>/dev/null || true
