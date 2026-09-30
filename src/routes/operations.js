import { buildRecordFilter, updateBusinessRecord } from '../business/records.js';
import { resolveAppAccess } from './apps.js';
import { processRecordAutomation } from './automation.js';

const publicRecord = (row) => ({ id: row.id, updated_at: row.updated, data: Object.fromEntries(Object.entries(row).filter(([key]) => !['id', 'collectionId', 'collectionName', 'created', 'updated', 'app_id', 'tenant_id'].includes(key))) });
const jobLocks = new Map();
const withJobLock = async (id, run) => {
  const prior = jobLocks.get(id) || Promise.resolve();
  let release;
  const next = new Promise((resolve) => { release = resolve; });
  jobLocks.set(id, next);
  await prior;
  try { return await run(); }
  finally { release(); if (jobLocks.get(id) === next) jobLocks.delete(id); }
};

export const registerOperationRoutes = (app, { auth, pocketbase }) => {
  const context = async (request, reply) => {
    const appRecord = await resolveAppAccess(request, reply, pocketbase);
    if (!appRecord) return null;
    const table = await pocketbase.collection('app_collections').getFirstListItem(
      pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && slug = {:slug}', { tenantId: request.tenant.id, appId: appRecord.id, slug: request.body?.table || request.query?.table })
    ).catch(() => null);
    if (!table) { reply.code(404).send({ error: '数据表不存在' }); return null; }
    return { appRecord, table };
  };

  app.post('/api/apps/:id/query', { preHandler: auth }, async (request, reply) => {
    const ctx = await context(request, reply);
    if (!ctx) return;
    let filter;
    try { filter = buildRecordFilter(request, ctx.table, request.body?.conditions || [], pocketbase); }
    catch (error) { return reply.code(400).send({ error: error.message }); }
    const page = Math.max(1, Math.min(100000, Number.parseInt(request.body?.page, 10) || 1));
    const result = await pocketbase.collection(ctx.table.pb_collection).getList(page, 25, { filter, sort: '-created' });
    return { items: result.items.map(publicRecord), totalItems: result.totalItems, page: result.page, totalPages: result.totalPages, conditions: request.body?.conditions || [] };
  });

  app.post('/api/apps/:id/batch-plans', { preHandler: auth }, async (request, reply) => {
    const ctx = await context(request, reply);
    if (!ctx) return;
    if (!request.appCanBatch) return reply.code(403).send({ error: '没有批量修改权限' });
    const { conditions = [], change } = request.body || {};
    const field = (ctx.table.fields || []).find((item) => item.name === change?.field);
    if (!field || field.type === 'file' || field.type === 'relation') return reply.code(400).send({ error: '批量修改字段无效' });
    const value = change.value;
    const valid = field.type === 'number' ? typeof value === 'number' && Number.isFinite(value)
      : field.type === 'bool' ? typeof value === 'boolean'
        : typeof value === 'string' && (field.type !== 'select' || field.options?.includes(value));
    if (!valid || (field.required && value === '')) return reply.code(400).send({ error: '批量修改值无效' });
    let filter;
    try { filter = buildRecordFilter(request, ctx.table, conditions, pocketbase); }
    catch (error) { return reply.code(400).send({ error: error.message }); }
    const result = await pocketbase.collection(ctx.table.pb_collection).getList(1, 101, { filter, sort: 'created' });
    if (!result.totalItems || result.totalItems > 100) return reply.code(400).send({ error: '请把目标范围缩小到 1–100 条记录' });
    const targets = result.items.map((item) => ({ id: item.id, updated_at: item.updated }));
    const job = await pocketbase.collection('batch_jobs').create({ tenant_id: request.tenant.id, app_id: ctx.appRecord.id, user_id: request.user.id, status: 'planned', plan: { table: ctx.table.slug, conditions, change, targets } });
    return reply.code(201).send({ plan_id: job.id, table: ctx.table.slug, count: targets.length, change, sample: result.items.slice(0, 10).map(publicRecord), status: 'planned' });
  });

  const ownJob = async (request, reply) => {
    const record = await resolveAppAccess(request, reply, pocketbase);
    if (!record) return null;
    const job = await pocketbase.collection('batch_jobs').getOne(request.params.jobId).catch(() => null);
    if (!job || job.tenant_id !== request.tenant.id || job.app_id !== record.id || job.user_id !== request.user.id) {
      reply.code(404).send({ error: '批量计划不存在' }); return null;
    }
    return job;
  };

  app.get('/api/apps/:id/batch-plans/:jobId', { preHandler: auth }, async (request, reply) => {
    const job = await ownJob(request, reply);
    if (!job) return;
    return { id: job.id, status: job.status, count: job.plan.targets.length, change: job.plan.change, result: job.result || null };
  });

  app.post('/api/apps/:id/batch-plans/:jobId/commit', { preHandler: auth }, async (request, reply) => {
    let job = await ownJob(request, reply);
    if (!job) return;
    if (!request.appCanBatch) return reply.code(403).send({ error: '没有批量修改权限' });
    if (request.body?.confirm !== true || request.body?.plan_id !== job.id) return reply.code(400).send({ error: '请确认当前批量计划' });
    return withJobLock(job.id, async () => {
    job = await pocketbase.collection('batch_jobs').getOne(job.id);
    if (job.status === 'completed' || job.status === 'partial') return { status: job.status, result: job.result };
    if (job.status === 'running' && Date.now() - new Date(job.updated).getTime() < 2 * 60 * 1000) return reply.code(409).send({ error: '计划正在执行' });
    if (!['planned', 'running'].includes(job.status)) return reply.code(409).send({ error: '计划已经失效' });
    if (job.status === 'planned' && Date.now() - new Date(job.created).getTime() > 15 * 60 * 1000) return reply.code(409).send({ error: '计划已过期，请重新预览' });
    await pocketbase.collection('batch_jobs').update(job.id, { status: 'running' });
    const table = await pocketbase.collection('app_collections').getFirstListItem(pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && slug = {:slug}', { tenantId: request.tenant.id, appId: request.params.id, slug: job.plan.table })).catch(() => null);
    if (!table || !(table.fields || []).some((field) => field.name === job.plan.change.field)) { await pocketbase.collection('batch_jobs').update(job.id, { status: 'partial', result: { error: '表结构已变化', updated: 0 } }); return reply.code(409).send({ error: '表结构已变化' }); }
    const result = { updated: 0, conflicted: 0, failed: 0, items: [] };
    for (const target of job.plan.targets) {
      const fresh = await resolveAppAccess(request, reply, pocketbase);
      if (!fresh || !request.appCanBatch) { result.failed++; result.items.push({ id: target.id, status: 'permission_changed' }); break; }
      const row = await pocketbase.collection(table.pb_collection).getOne(target.id).catch(() => null);
      if (!row || row.tenant_id !== request.tenant.id || row.app_id !== request.params.id || row.updated !== target.updated_at) { result.conflicted++; result.items.push({ id: target.id, status: 'conflict' }); continue; }
      try {
        const updated = await updateBusinessRecord({ pocketbase, table, tenantId: request.tenant.id, appId: request.params.id, recordId: row.id, data: { [job.plan.change.field]: job.plan.change.value }, expectedUpdated: target.updated_at, authorize: async () => {
          const current = await resolveAppAccess(request, reply, pocketbase);
          if (!current || current.archived || !request.appCanBatch) throw Object.assign(new Error('批量权限已变化'), { statusCode: 403 });
        } });
        await processRecordAutomation(pocketbase, { tenantId: request.tenant.id, appId: request.params.id, table: table.slug, event: 'updated', before: row, after: updated }).catch((error) => request.log.error({ err: error }, 'automation failed'));
        result.updated++; result.items.push({ id: target.id, status: 'updated' });
      }
      catch (error) {
        if (error.statusCode === 409) { result.conflicted++; result.items.push({ id: target.id, status: 'conflict' }); }
        else { result.failed++; result.items.push({ id: target.id, status: 'failed' }); }
      }
    }
    const status = result.failed || result.conflicted ? 'partial' : 'completed';
    await pocketbase.collection('batch_jobs').update(job.id, { status, result });
    return { status, result };
    });
  });
};
