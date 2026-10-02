#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIAO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
for program in node pm2 curl openssl; do
  command -v "$program" >/dev/null || { echo "Required command not found: $program" >&2; exit 1; }
done
[[ -x "$MIAO_ROOT/dist/miao" ]] || { echo "Run npm run build first." >&2; exit 1; }

# Scope all service operations to an isolated PM2 daemon and temporary data.
smoke_root="$(mktemp -d "${TMPDIR:-/tmp}/miao-smoke.XXXXXX")"
export PM2_HOME="$smoke_root/pm2" MIAO_INSTALL_DIR="$smoke_root/install"
unset PM2_DAEMON_RPC_PORT PM2_DAEMON_PUB_PORT PM2_INTERACTOR_RPC_PORT OVER_HOME
export MIAO_CONFIG_FILE="$smoke_root/miao.env" MIAO_DATA_DIR="$smoke_root/data"
export MIAO_BACKUP_DIR="$smoke_root/backups" HOST=127.0.0.1
export MIAO_REGISTRATION_MODE=open MIAO_REQUIRE_EMAIL_VERIFICATION=false
export MIAO_ALLOWED_EMAIL_DOMAINS= MIAO_ADMIN_EMAILS= RESEND_API_KEY= AI_GATEWAY_API_KEY=
export MIAO_AI_BASE_URL=http://127.0.0.1:1
export MIAO_SETTINGS_ENCRYPTION_KEY="$(openssl rand -base64 24)"
export MIAO_PORT="$(node -e 'const net=require("node:net");const s=net.createServer();s.listen(0,"127.0.0.1",()=>{console.log(s.address().port);s.close()})')"
export MIAO_SMOKE_ROOT="$smoke_root"
cleanup() { command pm2 kill >/dev/null 2>&1 || true; rm -rf "$smoke_root"; }
trap cleanup EXIT
bash "$SCRIPT_DIR/install.sh" --binary "$MIAO_ROOT/dist/miao"
# Use the installed commands so removing the source/archive directory is safe.
SCRIPT_DIR="$MIAO_INSTALL_DIR/runtime/scripts"
cd "$MIAO_ROOT"
bash "$SCRIPT_DIR/start.sh"
bash "$SCRIPT_DIR/status.sh"
pm2 jlist | node -e 'let s="";process.stdin.on("data",d=>s+=d);process.stdin.on("end",()=>{const p=JSON.parse(s);if(p.length!==1||p[0].name!=="miao-platform"||p[0].pm2_env.status!=="online")process.exit(1)})'

node --input-type=module <<'JS'
import assert from 'node:assert/strict';
import { writeFile } from 'node:fs/promises';
const origin = `http://127.0.0.1:${process.env.MIAO_PORT}`;
let token;
async function api(method, path, body, status = 200) {
  const response = await fetch(origin + path, { method, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) }, ...(body ? { body: JSON.stringify(body) } : {}) });
  const result = await response.json();
  assert.equal(response.status, status, JSON.stringify(result));
  return result;
}
assert.equal((await api('GET', '/api/health')).runtime, 'go');
const wasm = await fetch(origin + '/vendor/fx/fx-core.wasm');
assert.equal(wasm.status, 200);
assert.ok((await wasm.arrayBuffer()).byteLength > 1000000);
assert.equal((await fetch(origin + '/admin/users', { redirect: 'manual' })).status, 200);
assert.equal((await fetch(origin + '/api/collections/users/records')).status, 404);
token = (await api('POST', '/api/auth/register', { name: 'Smoke Owner', email: 'smoke@example.invalid', password: 'Smoke-Password-2026' }, 201)).token;
const app = await api('POST', '/api/apps', { name: 'Runtime smoke' }, 201);
const base = '/api/apps/' + app.id;
await api('POST', base + '/collections', { name: 'Rows', slug: 'rows', fields: [{ name: 'name', type: 'text', required: true }] }, 201);

const file = await api('POST', base + '/files', { name: 'rows.csv', base64: Buffer.from('name\nbefore backup').toString('base64') }, 201);
const content = await api('GET', base + '/files/' + file.id + '/content');
const plan = await api('POST', base + '/import-plans', { table: 'rows', rows: content.rows }, 201);
await api('POST', base + '/import-plans/' + plan.plan_id + '/commit', { confirm: false }, 400);
for (let i = 0; i < 2; i++) {
  const committed = await api('POST', base + '/import-plans/' + plan.plan_id + '/commit', { confirm: true, plan_id: plan.plan_id });
  assert.equal(committed.result.created, 1);
}
const version = await api('POST', base + '/versions', { definition: { schema_version: 2, title: 'Rows', pages: [{ id: 'rows', title: 'Rows', collection: 'rows', fields: ['name'], actions: [] }] } }, 201);
assert.equal((await api('GET', base + '/versions/' + version.id + '/preview')).status, 'preview');
await api('POST', base + '/versions/' + version.id + '/publish', { expected_published_version_id: null });
const published = await api('GET', base + '/runtime');
assert.equal(published.total_items, 1);
const rowPath = base + '/collections/rows/records/' + published.items[0].id;
const row = await api('GET', rowPath);
await api('PATCH', rowPath, { data: { name: 'before backup' }, expected_updated_at: row.updated_at });
const task = await api('POST', base + '/tasks', { name: 'Read rows', definition: { goal: 'Read rows', execution: 'report', trigger: { type: 'manual' }, scope: { tables: [{ table: 'rows', read_fields: ['name'], write_fields: [] }] } } }, 201);
await api('POST', base + '/tasks/' + task.id + '/enable', { confirm: true, expected_revision: 1 });
const queued = await api('POST', base + '/tasks/' + task.id + '/run', { expected_revision: 1, request_id: 'smoke-report' }, 202);
const repeated = await api('POST', base + '/tasks/' + task.id + '/run', { expected_revision: 1, request_id: 'smoke-report' }, 202);
assert.equal(repeated.id, queued.id);
let completed;
for (let i = 0; i < 90; i++) {
  completed = await api('GET', base + '/runs/' + queued.id);
  if (completed.status === 'completed') break;
  assert.ok(['queued', 'running'].includes(completed.status), JSON.stringify(completed));
  await new Promise(resolve => setTimeout(resolve, 500));
}
assert.equal(completed.status, 'completed');
assert.match(completed.output, /before backup/);
console.log('Core smoke passed: login, app, reviewed import, preview, publish, edit, detached task and result.');
await writeFile(process.env.MIAO_SMOKE_ROOT + '/fixture.json', JSON.stringify({ token, base, fileID: file.id, taskID: task.id }), { mode: 0o600 });
JS

if "$MIAO_INSTALL_DIR/bin/miao" backup > "$smoke_root/locked.log" 2>&1; then
  echo "Offline backup accepted an active data directory" >&2; exit 1
fi
grep -q 'data directory is in use' "$smoke_root/locked.log"
bash "$SCRIPT_DIR/backup.sh"
archive="$(find "$MIAO_BACKUP_DIR" -maxdepth 1 -name 'miao_backup_*.zip' -type f -print -quit)"
[[ -n "$archive" ]] || { echo "Backup archive missing" >&2; exit 1; }
curl --noproxy '*' --fail --silent "http://127.0.0.1:$MIAO_PORT/api/health" > /dev/null
if bash "$SCRIPT_DIR/restore.sh" "$archive" > "$smoke_root/unconfirmed.log" 2>&1; then
  echo "Restore accepted missing confirmation" >&2; exit 1
fi
curl --noproxy '*' --fail --silent "http://127.0.0.1:$MIAO_PORT/api/health" > /dev/null
node --input-type=module <<'JS'
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
const f = JSON.parse(await readFile(process.env.MIAO_SMOKE_ROOT + '/fixture.json'));
const response = await fetch(`http://127.0.0.1:${process.env.MIAO_PORT}${f.base}/collections/rows/records`, { method: 'POST', headers: { Authorization: `Bearer ${f.token}`, 'Content-Type': 'application/json' }, body: JSON.stringify({ data: { name: 'after backup' } }) });
assert.equal(response.status, 201, await response.text());
JS
bash "$SCRIPT_DIR/restore.sh" "$archive" --confirm
pm2 jlist | node -e 'let s="";process.stdin.on("data",d=>s+=d);process.stdin.on("end",()=>{if(JSON.parse(s).some(p=>p.pm2_env.status==="online"))process.exit(1)})'
bash "$SCRIPT_DIR/start.sh"
node --input-type=module <<'JS'
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
const f = JSON.parse(await readFile(process.env.MIAO_SMOKE_ROOT + '/fixture.json'));
async function get(path) {
  const response = await fetch(`http://127.0.0.1:${process.env.MIAO_PORT}` + path, { headers: { Authorization: `Bearer ${f.token}` } });
  assert.equal(response.status, 200);
  return response.json();
}
assert.equal((await get(f.base + '/collections/rows/records')).totalItems, 1);
assert.equal((await get(f.base + '/tasks')).items.find(task => task.id === f.taskID).status, 'paused');
assert.equal((await get(f.base + '/files/' + f.fileID + '/content')).rows[0].name, 'before backup');
console.log('Runtime smoke passed: one process, embedded assets, data lock, backup/resume, confirmed restore, persisted files, paused historical task.');
JS
