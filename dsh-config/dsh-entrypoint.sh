#!/usr/bin/env sh
set -eu

mkdir -p "${DSH_HOME}/profiles"
cp -a /opt/miaozao/dsh-profiles/. "${DSH_HOME}/profiles/"
if [ ! -f "${DSH_HOME}/cordis.patch.yml" ]; then
  cp /opt/miaozao/cordis.patch.yml "${DSH_HOME}/cordis.patch.yml"
fi

dsh --profile web --no-open --host 127.0.0.1 --port "${DSH_INTERNAL_PORT:-3080}" &
dsh_pid=$!
trap 'kill "$dsh_pid" 2>/dev/null || true' INT TERM EXIT
node /opt/miaozao/dsh-proxy.mjs &
proxy_pid=$!
while kill -0 "$dsh_pid" 2>/dev/null && kill -0 "$proxy_pid" 2>/dev/null; do
  sleep 1
done
kill "$dsh_pid" "$proxy_pid" 2>/dev/null || true
