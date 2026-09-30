import test from 'node:test';
import assert from 'node:assert/strict';
import Fastify from 'fastify';
import { registerOperationRoutes } from '../src/routes/operations.js';
import { registerAppRoutes } from '../src/routes/apps.js';
import { processRecordAutomation } from '../src/routes/automation.js';
import { registerTaskRoutes } from '../src/routes/tasks.js';

const fixture = () => {
  let sequence = 0;
  const rows = {
    users: [{ id: 'u1', email: 'one@example.invalid', verified: true, disabled: false }],
    tenants: [{ id: 't1', owner_id: 'u1' }],
    apps: [{ id: 'app1', tenant_id: 't1', name: '客户', published_version_id: '', draft_version_id: '' }],
    app_members: [], app_collections: [{ id: 'table1', tenant_id: 't1', app_id: 'app1', slug: 'customers', pb_collection: 'app_app1_customers', name: '客户', fields: [
      { name: 'name', label: '姓名', type: 'text', required: true }, { name: 'status', label: '状态', type: 'select', options: ['新建', '跟进中'] }
    ] }],
    app_app1_customers: [{ id: 'row1', tenant_id: 't1', app_id: 'app1', name: '张三', status: '新建', updated: '2026-09-29T10:00:00Z', created: '2026-09-29T10:00:00Z' }],
    app_versions: [], batch_jobs: [], tenant_members: [{ id: 'membership1', tenant_id: 't1', user_id: 'u1', role: 'owner' }], automation_rules: [], automation_runs: [], automation_notifications: [],
    miao_tasks: [], miao_runs: [], miao_run_attempts: [], miao_actions: [], agent_threads: [], agent_messages: []
  };
  const filtered = (name, filter) => (rows[name] || []).filter((row) => Object.entries(filter?.params || {}).every(([key, value]) => {
    const property = filter.source.match(new RegExp(`(\\w+) = \\{:${key}\\}`))?.[1] || key;
    return row[property] === value;
  }));
  const pb = {
    filter: (source, params) => ({ source, params }),
    collections: { delete: async (name) => { delete rows[name]; } },
    collection: (name) => ({
      getOne: async (id) => { const item = rows[name]?.find((row) => row.id === id); if (!item) throw new Error('not found'); return { ...item }; },
      getFirstListItem: async (filter) => {
        const item = rows[name]?.find((row) => Object.entries(filter.params || {}).every(([key, value]) => {
          const property = { tenantId: 'tenant_id', appId: 'app_id', slug: 'slug', userId: 'user_id', ruleId: 'rule_id', key: 'event_key' }[key] || key;
          return row[property] === value;
        }));
        if (!item) throw new Error('not found'); return { ...item };
      },
      getFullList: async ({ filter } = {}) => filtered(name, filter).map((item) => ({ ...item })),
      getList: async (page, perPage, { filter } = {}) => {
        const all = filtered(name, filter);
        return { items: all.slice((page - 1) * perPage, page * perPage).map((item) => ({ ...item })), totalItems: all.length, page, totalPages: Math.ceil(all.length / perPage) };
      },
      create: async (body) => { const item = { id: `generated${++sequence}`, created: new Date().toISOString(), updated: new Date().toISOString(), ...body }; rows[name].push(item); return { ...item }; },
      update: async (id, body) => { const item = rows[name].find((row) => row.id === id); Object.assign(item, body, { updated: new Date().toISOString() }); return { ...item }; },
      delete: async (id) => { rows[name] = rows[name].filter((row) => row.id !== id); }
    })
  };
  return { pb, rows };
};

const setup = async (role = 'owner') => {
  const { pb, rows } = fixture();
  const app = Fastify();
  const auth = async (request) => {
    request.user = { id: 'u1', email: 'one@example.invalid' };
    request.tenant = { id: 't1', owner_id: 'u1' };
    request.membership = { role };
  };
  if (role !== 'owner') rows.app_members.push({ id: 'member1', tenant_id: 't1', app_id: 'app1', user_id: 'u1', role, can_batch: false });
  registerAppRoutes(app, { auth, pocketbase: pb, body: (request) => request.body || {} });
  registerOperationRoutes(app, { auth, pocketbase: pb });
  registerTaskRoutes(app, { auth, pocketbase: pb, worker: { cancel() {} } });
  await app.ready();
  return { app, rows };
};

const definition = { schema_version: 1, title: '客户列表', collection: 'customers', fields: ['name', 'status'] };

test('app and task routes coexist and keep run checkpoints private', async (t) => {
  const { app, rows } = await setup(); t.after(() => app.close());
  rows.miao_tasks.push({ id: 'task1', tenant_id: 't1', app_id: 'app1', name: '客户报表' });
  rows.miao_tasks.push({ id: 'other-task', tenant_id: 't1', app_id: 'app2', name: '其他应用' });
  rows.miao_runs.push({ id: 'run1', tenant_id: 't1', app_id: 'app1', checkpoint: { bytes: 'private' }, snapshot: { name: '客户报表', revision: 1 }, status: 'completed' });
  rows.miao_run_attempts.push({ id: 'attempt1', tenant_id: 't1', app_id: 'app1', run_id: 'run1', sequence: 1 });
  const application = await app.inject({ url: '/api/apps/app1' });
  assert.equal(application.statusCode, 200);
  assert.equal(application.json().name, '客户');
  const tasks = await app.inject({ url: '/api/apps/app1/tasks' });
  assert.equal(tasks.statusCode, 200);
  assert.deepEqual(tasks.json().items.map((task) => task.id), ['task1']);
  const run = await app.inject({ url: '/api/apps/app1/runs/run1' });
  assert.equal(run.statusCode, 200);
  assert.equal(run.json().task_name, '客户报表');
  assert.equal(run.json().checkpoint, undefined);
  assert.equal(run.json().snapshot, undefined);
  assert.deepEqual(run.json().attempt_history.map((attempt) => attempt.id), ['attempt1']);
  rows.miao_runs[0].tenant_id = 'other-tenant';
  const denied = await app.inject({ url: '/api/apps/app1/runs/run1' });
  assert.equal(denied.statusCode, 404);
});

test('deleting an app clears task history and existing app data within its tenant', async (t) => {
  const { app, rows } = await setup(); t.after(() => app.close());
  const collections = ['app_members', 'app_versions', 'miao_run_attempts', 'miao_actions', 'miao_runs', 'miao_tasks', 'agent_threads', 'batch_jobs', 'automation_notifications', 'automation_runs', 'automation_rules'];
  for (const name of collections) rows[name].push(
    { id: `${name}-target`, tenant_id: 't1', app_id: 'app1' },
    { id: `${name}-other-app`, tenant_id: 't1', app_id: 'app2' },
    { id: `${name}-other-tenant`, tenant_id: 't2', app_id: 'app1' }
  );
  rows.agent_messages.push(
    { id: 'target-message', tenant_id: 't1', thread_id: 'agent_threads-target' },
    { id: 'other-app-message', tenant_id: 't1', thread_id: 'agent_threads-other-app' },
    { id: 'other-tenant-message', tenant_id: 't2', thread_id: 'agent_threads-target' }
  );
  const unconfirmed = await app.inject({ method: 'DELETE', url: '/api/apps/app1', payload: { confirm: false } });
  assert.equal(unconfirmed.statusCode, 400);
  assert.equal(rows.miao_run_attempts.length, 3);
  assert.equal(rows.apps.length, 1);
  const removed = await app.inject({ method: 'DELETE', url: '/api/apps/app1', payload: { confirm: true } });
  assert.equal(removed.statusCode, 200);
  assert.deepEqual(removed.json(), { ok: true, deleted_tables: 1 });
  assert.equal(rows.apps.length, 0);
  assert.equal(rows.app_collections.length, 0);
  assert.equal(rows.app_app1_customers, undefined);
  for (const name of collections) assert.deepEqual(rows[name].map((row) => row.id), [`${name}-other-app`, `${name}-other-tenant`], name);
  assert.deepEqual(rows.agent_messages.map((row) => row.id), ['other-app-message', 'other-tenant-message']);
});

test('an app publisher cannot permanently delete an app or its tasks', async (t) => {
  const { app, rows } = await setup('publisher'); t.after(() => app.close());
  rows.miao_tasks.push({ id: 'task1', tenant_id: 't1', app_id: 'app1' });
  const denied = await app.inject({ method: 'DELETE', url: '/api/apps/app1', payload: { confirm: true } });
  assert.equal(denied.statusCode, 403);
  assert.equal(rows.apps.length, 1);
  assert.equal(rows.miao_tasks.length, 1);
});

test('viewer cannot create drafts or write through batch plans', async (t) => {
  const { app } = await setup('viewer'); t.after(() => app.close());
  const draft = await app.inject({ method: 'POST', url: '/api/apps/app1/versions', payload: { definition, base_version_id: '' } });
  assert.equal(draft.statusCode, 403);
  const batch = await app.inject({ method: 'POST', url: '/api/apps/app1/batch-plans', payload: { table: 'customers', conditions: [], change: { field: 'status', value: '跟进中' } } });
  assert.equal(batch.statusCode, 403);
});

test('record editor cannot publish, publisher can save a draft', async (t) => {
  const { app } = await setup('editor'); t.after(() => app.close());
  const denied = await app.inject({ method: 'POST', url: '/api/apps/app1/versions', payload: { definition } });
  assert.equal(denied.statusCode, 403);
  const { app: publisher } = await setup('publisher'); t.after(() => publisher.close());
  const draft = await publisher.inject({ method: 'POST', url: '/api/apps/app1/versions', payload: { definition } });
  assert.equal(draft.statusCode, 201);
});

test('batch change requires confirmation and repeated submission does not write twice', async (t) => {
  const { app, rows } = await setup(); t.after(() => app.close());
  const plan = await app.inject({ method: 'POST', url: '/api/apps/app1/batch-plans', payload: { table: 'customers', conditions: [], change: { field: 'status', value: '跟进中' } } });
  assert.equal(plan.statusCode, 201);
  const id = plan.json().plan_id;
  assert.equal(rows.app_app1_customers[0].status, '新建');
  const denied = await app.inject({ method: 'POST', url: `/api/apps/app1/batch-plans/${id}/commit`, payload: { confirm: false, plan_id: id } });
  assert.equal(denied.statusCode, 400);
  const done = await app.inject({ method: 'POST', url: `/api/apps/app1/batch-plans/${id}/commit`, payload: { confirm: true, plan_id: id } });
  assert.equal(done.json().result.updated, 1);
  const retry = await app.inject({ method: 'POST', url: `/api/apps/app1/batch-plans/${id}/commit`, payload: { confirm: true, plan_id: id } });
  assert.equal(retry.json().result.updated, 1);
  assert.equal(rows.app_app1_customers[0].status, '跟进中');
});

test('revoking batch permission invalidates a pending plan', async (t) => {
  const { app, rows } = await setup('publisher'); t.after(() => app.close());
  rows.app_members[0].can_batch = true;
  const plan = await app.inject({ method: 'POST', url: '/api/apps/app1/batch-plans', payload: { table: 'customers', conditions: [], change: { field: 'status', value: '跟进中' } } });
  assert.equal(plan.statusCode, 201);
  rows.app_members[0].can_batch = false;
  const commit = await app.inject({ method: 'POST', url: `/api/apps/app1/batch-plans/${plan.json().plan_id}/commit`, payload: { confirm: true, plan_id: plan.json().plan_id } });
  assert.equal(commit.statusCode, 403);
  assert.equal(rows.app_app1_customers[0].status, '新建');
});

test('a successful record event produces one notification across retries', async () => {
  const { pb, rows } = fixture();
  rows.automation_rules.push({ id: 'rule1', tenant_id: 't1', app_id: 'app1', created_by: 'u1', name: '新增客户', enabled: true, definition: { trigger: 'record_created', table: 'customers', recipient_id: 'u1' } });
  const event = { tenantId: 't1', appId: 'app1', table: 'customers', event: 'created', after: rows.app_app1_customers[0] };
  await processRecordAutomation(pb, event);
  await processRecordAutomation(pb, event);
  assert.equal(rows.automation_notifications.length, 1);
  assert.equal(rows.automation_runs.length, 1);
  assert.equal(rows.automation_runs[0].status, 'delivered');
});

test('a confirmed form action sets only the configured field once', async () => {
  const { pb, rows } = fixture();
  rows.automation_rules.push({ id: 'rule2', tenant_id: 't1', app_id: 'app1', created_by: 'u1', name: '初始状态', enabled: true, definition: { trigger: 'record_created', table: 'customers', action: { type: 'set_field', field: 'status', value: '跟进中' } } });
  const event = { tenantId: 't1', appId: 'app1', table: 'customers', event: 'created', after: { ...rows.app_app1_customers[0] } };
  await processRecordAutomation(pb, event);
  await processRecordAutomation(pb, event);
  assert.equal(rows.app_app1_customers[0].status, '跟进中');
  assert.equal(rows.automation_runs.length, 1);
  assert.equal(rows.automation_notifications.length, 0);
});
