import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { resolve, join } from 'node:path';
import { spawn, execFileSync } from 'node:child_process';
import { once } from 'node:events';
import Fastify from 'fastify';
import PocketBase from 'pocketbase';
import ExcelJS from 'exceljs';
import { validateAppUiDefinition, diffAppUi } from '../src/app-ui.js';
import { readSpreadsheet } from '../src/routes/files.js';
import { registerAppRoutes } from '../src/routes/apps.js';
import { registerEnvironmentRoutes, conversationScope } from '../src/routes/environment.js';
import { registerFileRoutes } from '../src/routes/files.js';
import { registerImportRoutes } from '../src/routes/imports.js';
import { registerTaskRoutes } from '../src/routes/tasks.js';
import { createTaskWorker } from '../src/runtime/worker.js';

const tables = [{ slug: 'customers', fields: [{ name: 'name', type: 'text', required: true }, { name: 'status', type: 'select', options: ['new', 'done'] }, { name: 'file', type: 'file' }] }];
const definition = { schema_version: 2, title: '业务', pages: [{ id: 'customers', title: '客户', collection: 'customers', fields: ['name', 'status', 'file'], actions: [{ id: 'complete', label: '完成', set: { status: 'done' } }] }] };

test('v2 UI rejects code, duplicate pages, missing fields and invalid actions', () => {
  assert.equal(validateAppUiDefinition(definition, tables).error, undefined);
  assert.ok(validateAppUiDefinition({ ...definition, script: 'alert(1)' }, tables).error);
  assert.ok(validateAppUiDefinition({ ...definition, pages: [definition.pages[0], definition.pages[0]] }, tables).error);
  assert.ok(validateAppUiDefinition({ ...definition, pages: [{ ...definition.pages[0], fields: ['missing'] }] }, tables).error);
  assert.ok(validateAppUiDefinition({ ...definition, pages: [{ ...definition.pages[0], actions: [{ id: 'bad', label: '坏', set: { status: 'unknown' } }] }] }, tables).error);
  assert.equal(diffAppUi(null, definition).filter((change) => change.type === 'add_page').length, 1);
});

test('CSV and XLSX preserve quoted values, false and zero; duplicate headers are rejected', async () => {
  const csv = await readSpreadsheet(Buffer.from('姓名,金额\r\n"张,三",0\r\n'), '客户.csv');
  assert.deepEqual(csv.rows, [{ 姓名: '张,三', 金额: '0' }]);
  await assert.rejects(() => readSpreadsheet(Buffer.from('姓名,姓名\na,b'), 'bad.csv'), /表头/);
  const workbook = new ExcelJS.Workbook(); const sheet = workbook.addWorksheet('客户');
  sheet.addRow(['name', 'value', 'active']); sheet.addRow(['张三', 0, false]);
  const parsed = await readSpreadsheet(Buffer.from(await workbook.xlsx.writeBuffer()), 'test.xlsx', '客户');
  assert.deepEqual(parsed.rows, [{ name: '张三', value: 0, active: false }]);
  assert.deepEqual(parsed.sheets, ['客户']);
});

const binary = process.env.POCKETBASE_BIN;
test('real PocketBase: shared data, files, imports, multipage runtime, actions, recovery, sessions and background events survive restart', { skip: !binary || !existsSync(binary), timeout: 90000 }, async (t) => {
  const directory = mkdtempSync(join(tmpdir(), 'miao-business-'));
  const args = [`--dir=${directory}`, `--migrationsDir=${resolve('pb_migrations')}`, `--hooksDir=${resolve('pb_hooks')}`, '--automigrate=0'];
  const email = 'smoke@example.invalid', password = 'Smoke-Only-Password-2026';
  execFileSync(binary, ['migrate', 'up', ...args], { stdio: 'pipe' });
  execFileSync(binary, ['superuser', 'upsert', email, password, ...args], { stdio: 'pipe' });
  let processHandle;
  let pb;
  const launch = async () => {
    processHandle = spawn(binary, ['serve', '--http=127.0.0.1:18097', ...args], { stdio: 'ignore' });
    for (let count = 0; count < 100; count++) {
      try { const response = await fetch('http://127.0.0.1:18097/api/health'); if (response.ok) break; } catch {}
      await new Promise((resolve) => setTimeout(resolve, 30));
    }
    pb ||= new PocketBase('http://127.0.0.1:18097'); pb.autoCancellation(false);
    await pb.collection('_superusers').authWithPassword(email, password);
  };
  const stop = async () => { if (processHandle && processHandle.exitCode === null) { const exited = once(processHandle, 'exit'); processHandle.kill('SIGTERM'); await exited; } };
  t.after(async () => { await stop(); rmSync(directory, { recursive: true, force: true }); });
  await launch();
  const user = await pb.collection('users').create({ email: 'owner@example.invalid', password, passwordConfirm: password, name: '负责人', verified: true });
  const tenant = await pb.collection('tenants').create({ name: 'Smoke', slug: 'smoke', owner_id: user.id });
  await pb.collection('tenant_members').create({ tenant_id: tenant.id, user_id: user.id, role: 'owner' });
  const editor = await pb.collection('users').create({ email: 'editor@example.invalid', password, passwordConfirm: password, name: '同事', verified: true });
  await pb.collection('tenant_members').create({ tenant_id: tenant.id, user_id: editor.id, role: 'member' });
  const server = Fastify();
  const auth = async (request) => { request.user = request.headers['x-smoke-user'] === 'editor' ? editor : user; request.tenant = tenant; request.membership = { role: request.user.id === user.id ? 'owner' : 'member' }; };
  const options = { auth, pocketbase: pb, body: (request) => request.body || {} };
  registerAppRoutes(server, options); registerEnvironmentRoutes(server, options); registerFileRoutes(server, options); registerImportRoutes(server, options);
  registerTaskRoutes(server, { ...options, worker: { cancel() {} } });
  t.after(() => server.close());
  const request = async (method, url, payload, expected = 200, headers = {}) => { const response = await server.inject({ method, url, payload, headers }); assert.equal(response.statusCode, expected, `${url}: ${response.body}`); return response.json(); };
  const application = await request('POST', '/api/apps', { name: '客户项目' }, 201);
  const base = `/api/apps/${application.id}`;
  await request('POST', `${base}/collections`, { name: '客户', slug: 'customers', fields: [{ name: 'name', type: 'text', required: true }, { name: 'status', type: 'select', options: ['new', 'done'] }, { name: 'file', type: 'file' }] }, 201);
  await request('POST', `${base}/collections`, { name: '项目', slug: 'projects', fields: [{ name: 'name', type: 'text', required: true }, { name: 'customer', type: 'relation', target: 'customers' }] }, 201);
  const metadata = await pb.collection('app_collections').getFirstListItem(pb.filter('app_id = {:id} && slug = "customers"', { id: application.id }));
  const schema = await pb.collections.getOne(metadata.pb_collection);
  await pb.collections.update(schema.id, { fields: schema.fields.map((field) => field.type === 'file' ? { ...field, protected: false } : field) });
  await stop();
  execFileSync(binary, ['migrate', 'down', '1', ...args], { input: 'y\n', stdio: ['pipe', 'pipe', 'pipe'] });
  execFileSync(binary, ['migrate', 'up', ...args], { stdio: 'pipe' });
  await launch();
  assert.equal((await pb.collections.getOne(schema.id)).fields.find((field) => field.name === 'file').protected, true);
  const v2 = { ...definition, pages: [...definition.pages, { id: 'projects', title: '项目', collection: 'projects', fields: ['name', 'customer'] }] };
  const version = await request('POST', `${base}/versions`, { definition: v2, summary: '客户项目' }, 201);
  const preview = await request('GET', `${base}/versions/${version.id}/preview`); assert.equal(preview.pages.length, 2); assert.equal(preview.page_previews.length, 2); assert.equal(preview.changes.length > 0, true);
  await request('POST', `${base}/versions/${version.id}/publish`, { expected_published_version_id: null });
  const file = await request('POST', `${base}/files`, { name: '客户.csv', base64: Buffer.from('姓名,状态\n张三,new\n李四,new').toString('base64') }, 201);
  const storedFile = await pb.collection('app_files').getOne(file.id);
  assert.equal((await fetch(pb.files.getURL(storedFile, storedFile.file))).ok, false);
  const content = await request('GET', `${base}/files/${file.id}/content`); assert.equal(content.rows.length, 2);
  const plan = await request('POST', `${base}/import-plans`, { table: 'customers', rows: content.rows.map((row) => ({ name: row.姓名, status: row.状态 })) }, 201);
  await request('POST', `${base}/import-plans/${plan.plan_id}/commit`, { confirm: false }, 400);
  const imported = await request('POST', `${base}/import-plans/${plan.plan_id}/commit`, { confirm: true, plan_id: plan.plan_id }); assert.equal(imported.result.created, 2);
  const repeated = await request('POST', `${base}/import-plans/${plan.plan_id}/commit`, { confirm: true, plan_id: plan.plan_id }); assert.equal(repeated.result.created, 2);
  let runtime = await request('GET', `${base}/runtime`); assert.equal(runtime.total_items, 2); assert.ok(runtime.create_form_fields.some((field) => field.type === 'file'));
  const row = runtime.items[0];
  await request('POST', `${base}/collections/projects/records`, { data: { name: '交付', customer: row.id } }, 201);
  runtime = await request('GET', `${base}/runtime?ui_page=projects`); assert.equal(runtime.relation_labels.customer[row.id], row.data.name);
  const attachment = await request('POST', `${base}/files`, { name: 'notes.txt', base64: Buffer.from('交付说明').toString('base64') }, 201);
  const attached = await request('POST', `${base}/files/${attachment.id}/attach`, { table: 'customers', record_id: row.id, field: 'file', expected_updated_at: row.updated_at });
  assert.ok(attached.data.file);
  assert.equal((await server.inject({ method: 'GET', url: `${base}/collections/customers/records/${row.id}/files/file` })).statusCode, 200);
  const current = await request('GET', `${base}/collections/customers/records/${row.id}`);
  const completed = await request('POST', `${base}/runtime/actions/complete`, { ui_page: 'customers', record_id: row.id, expected_updated_at: current.updated_at, expected_version_id: version.id, confirm: true }); assert.equal(completed.record.data.status, 'done');
  await request('POST', `${base}/runtime/actions/complete`, { ui_page: 'customers', record_id: row.id, expected_updated_at: current.updated_at, expected_version_id: version.id, confirm: true }, 409);
  const history = await request('GET', `${base}/record-changes?record_id=${row.id}`); assert.equal(history.items[0].actor_id, user.id); assert.equal(history.items[0].before.status, 'new'); assert.equal(history.items[0].after.status, 'done');
  await request('POST', `${base}/record-changes/${history.items[0].id}/restore`, { expected_updated_at: completed.record.updated_at, confirm: true });
  assert.equal((await request('GET', `${base}/collections/customers/records/${row.id}`)).data.status, 'new');
  await request('PUT', `${base}/context`, { content: '客户完成后交付项目', expected_revision: 0 });
  await request('PUT', `${base}/context`, { content: '旧修改', expected_revision: 0 }, 409);
  assert.equal((await request('GET', `${base}/context`, undefined, 200, { 'x-smoke-user': 'editor' })).content, '客户完成后交付项目');
  const scope = await conversationScope(pb, { user, tenant, membership: { role: 'owner' } });
  const saved = await request('PUT', '/api/agent/conversation', { scope, expected_revision: 0, checkpoint: 'AQID', messages: [{ role: 'user', content: '继续客户跟进' }] }); assert.equal(saved.revision, 1);
  assert.equal((await request('PUT', '/api/agent/conversation', { scope, expected_revision: 0, checkpoint: 'AQID', messages: [] })).conflict, true);
  assert.equal((await request('GET', '/api/agent/conversation', undefined, 200, { 'x-smoke-user': 'editor' })).conversation, null);
  const task = await request('POST', `${base}/tasks`, { name: '客户检查', definition: { goal: '读取客户数据', execution: 'report', trigger: { type: 'manual' }, scope: { tables: [{ table: 'customers', read_fields: ['name', 'status'], write_fields: [] }] } } }, 201);
  await request('POST', `${base}/tasks/${task.id}/enable`, { confirm: true, expected_revision: 1 });
  const run = await request('POST', `${base}/tasks/${task.id}/events`, { event_id: 'event-1', expected_revision: 1, input: { source: 'external' } }, 202);
  const duplicate = await request('POST', `${base}/tasks/${task.id}/events`, { event_id: 'event-1', expected_revision: 1, input: { source: 'external' } }, 202); assert.equal(duplicate.id, run.id);
  await stop(); await launch();
  assert.equal((await request('GET', `${base}/runtime`)).total_items, 2);
  assert.equal((await request('GET', '/api/agent/conversation')).conversation.checkpoint, 'AQID');
  assert.equal((await request('GET', `${base}/files/${file.id}/content`)).rows.length, 2);
  const worker = createTaskWorker({ pocketbase: pb, logger: { error() {} } }); worker.start();
  try {
    let finished;
    for (let count = 0; count < 100; count++) { finished = await pb.collection('miao_runs').getOne(run.id); if (['completed', 'failed'].includes(finished.status)) break; await new Promise((resolve) => setTimeout(resolve, 100)); }
    assert.equal(finished.status, 'completed', finished.error);
  } finally { await worker.stop(); }
  await request('PUT', `${base}/access`, { restricted: true, permissions: [{ user_id: editor.id, role: 'viewer' }] });
  await request('POST', `${base}/import-plans`, { table: 'customers', rows: [{ name: '越权' }] }, 403, { 'x-smoke-user': 'editor' });
  await request('POST', `${base}/runtime/actions/complete`, { ui_page: 'customers' }, 403, { 'x-smoke-user': 'editor' });
  await request('GET', `${base}/files/notowned/content`, undefined, 404);
});
