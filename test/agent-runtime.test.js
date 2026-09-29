import test from 'node:test';
import assert from 'node:assert/strict';
import Fastify from 'fastify';
import { registerRuntimeRoutes } from '../src/routes/runtime.js';
import { registerOperationRoutes } from '../src/routes/operations.js';
import { processRecordAutomation } from '../src/routes/automation.js';

const fixture = () => {
  let sequence = 0;
  const rows = {
    apps: [{ id: 'app1', tenant_id: 't1', name: '客户', published_version_id: '', draft_version_id: '' }],
    app_members: [], app_collections: [{ id: 'table1', tenant_id: 't1', app_id: 'app1', slug: 'customers', pb_collection: 'app_app1_customers', name: '客户', fields: [
      { name: 'name', label: '姓名', type: 'text', required: true }, { name: 'status', label: '状态', type: 'select', options: ['新建', '跟进中'] }
    ] }],
    app_app1_customers: [{ id: 'row1', tenant_id: 't1', app_id: 'app1', name: '张三', status: '新建', updated: '2026-09-29T10:00:00Z', created: '2026-09-29T10:00:00Z' }],
    app_versions: [], batch_jobs: [], tenant_members: [{ id: 'membership1', tenant_id: 't1', user_id: 'u1', role: 'owner' }], automation_rules: [], automation_runs: [], automation_notifications: []
  };
  const pb = {
    filter: (source, params) => ({ source, params }),
    collection: (name) => ({
      getOne: async (id) => { const item = rows[name]?.find((row) => row.id === id); if (!item) throw new Error('not found'); return { ...item }; },
      getFirstListItem: async (filter) => {
        const item = rows[name]?.find((row) => Object.entries(filter.params || {}).every(([key, value]) => {
          const property = { tenantId: 'tenant_id', appId: 'app_id', slug: 'slug', userId: 'user_id', ruleId: 'rule_id', key: 'event_key' }[key] || key;
          return row[property] === value;
        }));
        if (!item) throw new Error('not found'); return { ...item };
      },
      getFullList: async () => (rows[name] || []).map((item) => ({ ...item })),
      getList: async (page, perPage) => {
        const all = rows[name] || [];
        return { items: all.slice((page - 1) * perPage, page * perPage).map((item) => ({ ...item })), totalItems: all.length, page, totalPages: Math.ceil(all.length / perPage) };
      },
      create: async (body) => { const item = { id: `generated${++sequence}`, created: new Date().toISOString(), updated: new Date().toISOString(), ...body }; rows[name].push(item); return { ...item }; },
      update: async (id, body) => { const item = rows[name].find((row) => row.id === id); Object.assign(item, body, { updated: new Date().toISOString() }); return { ...item }; }
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
  registerRuntimeRoutes(app, { auth, pocketbase: pb });
  registerOperationRoutes(app, { auth, pocketbase: pb });
  await app.ready();
  return { app, rows };
};

const definition = { pages: [{ id: 'customers', title: '客户', table: 'customers', view: 'list', fields: ['name', 'status'] }] };

test('draft is isolated until explicit publish, and stale base cannot replace the live version', async (t) => {
  const { app } = await setup(); t.after(() => app.close());
  const draft = await app.inject({ method: 'POST', url: '/api/apps/app1/versions', payload: { definition, summary: '客户列表', base_version_id: '' } });
  assert.equal(draft.statusCode, 201);
  assert.equal((await app.inject('/api/apps/app1/runtime')).json().status, 'unpublished');
  const id = draft.json().version.id;
  const denied = await app.inject({ method: 'POST', url: `/api/apps/app1/versions/${id}/publish`, payload: { confirm: false, version_id: id } });
  assert.equal(denied.statusCode, 400);
  const published = await app.inject({ method: 'POST', url: `/api/apps/app1/versions/${id}/publish`, payload: { confirm: true, version_id: id } });
  assert.equal(published.statusCode, 200);
  assert.equal((await app.inject('/api/apps/app1/runtime')).json().definition.pages[0].id, 'customers');
  const stale = await app.inject({ method: 'POST', url: '/api/apps/app1/versions', payload: { definition, summary: '旧草稿', base_version_id: '' } });
  assert.equal(stale.statusCode, 409);
});

test('viewer cannot create drafts or write through batch plans', async (t) => {
  const { app } = await setup('viewer'); t.after(() => app.close());
  const draft = await app.inject({ method: 'POST', url: '/api/apps/app1/versions', payload: { definition, base_version_id: '' } });
  assert.equal(draft.statusCode, 403);
  const batch = await app.inject({ method: 'POST', url: '/api/apps/app1/batch-plans', payload: { table: 'customers', conditions: [], change: { field: 'status', value: '跟进中' } } });
  assert.equal(batch.statusCode, 403);
});

test('schema changes invalidate a draft at publish time', async (t) => {
  const { app, rows } = await setup(); t.after(() => app.close());
  const draft = await app.inject({ method: 'POST', url: '/api/apps/app1/versions', payload: { definition, base_version_id: '' } });
  rows.app_collections[0].fields = [{ name: 'name', label: '姓名', type: 'text' }];
  const id = draft.json().version.id;
  const published = await app.inject({ method: 'POST', url: `/api/apps/app1/versions/${id}/publish`, payload: { confirm: true, version_id: id } });
  assert.equal(published.statusCode, 409);
  assert.equal((await app.inject('/api/apps/app1/runtime')).json().status, 'unpublished');
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
