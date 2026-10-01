import { resolveAppAccess } from './apps.js';
import { serialized } from '../business/locks.js';
import { validateRecordData, validateRelations } from '../business/records.js';

export function registerImportRoutes(app, { auth, pocketbase }) {
  const context = async (request, reply) => {
    const application = await resolveAppAccess(request, reply, pocketbase);
    if (!application) return null;
    if (!request.appCanBatch || application.archived) { reply.code(403).send({ error: '导入需要批量写入权限' }); return null; }
    return application;
  };
  app.post('/api/apps/:id/import-plans', { preHandler: auth }, async (request, reply) => {
    const application = await context(request, reply); if (!application) return;
    const { table: slug, rows } = request.body || {};
    const table = await pocketbase.collection('app_collections').getFirstListItem(pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && slug = {:slug}', { tenantId: request.tenant.id, appId: application.id, slug })).catch(() => null);
    if (!table || !Array.isArray(rows) || !rows.length || rows.length > 100 || JSON.stringify(rows).length > 500000) return reply.code(400).send({ error: '导入需要真实数据表及 1–100 行已映射的数据' });
    const errors = [];
    for (const [index, row] of rows.entries()) {
      const error = !row || typeof row !== 'object' || Array.isArray(row) ? '每行必须是对象' : validateRecordData(row, table.fields) || await validateRelations(row, table.fields, { pocketbase, appId: application.id, tenantId: request.tenant.id });
      if (error) errors.push({ row: index + 1, error });
    }
    if (errors.length) return reply.code(400).send({ error: '导入校验失败，没有写入记录', errors });
    const plan = await pocketbase.collection('batch_jobs').create({ tenant_id: request.tenant.id, app_id: application.id, user_id: request.user.id, status: 'planned', plan: { kind: 'import', table: slug, fields: table.fields, rows, targets: [] } });
    return reply.code(201).send({ plan_id: plan.id, table: slug, count: rows.length, sample: rows.slice(0, 10), status: 'planned', note: '只新增，不自动合并；请审阅后确认。计划 15 分钟内有效。' });
  });
  app.post('/api/apps/:id/import-plans/:planId/commit', { preHandler: auth }, async (request, reply) => serialized(`import:${request.params.planId}`, async () => {
    if (!await context(request, reply)) return;
    const job = await pocketbase.collection('batch_jobs').getOne(request.params.planId).catch(() => null);
    if (!job || job.tenant_id !== request.tenant.id || job.app_id !== request.params.id || job.user_id !== request.user.id || job.plan?.kind !== 'import') return reply.code(404).send({ error: '导入计划不存在' });
    if (request.body?.confirm !== true || request.body.plan_id !== job.id) return reply.code(400).send({ error: '请确认这个具体导入计划' });
    if (['completed', 'partial'].includes(job.status)) return { status: job.status, result: job.result };
    if (job.status !== 'planned' || Date.now() - Date.parse(job.created) > 15 * 60 * 1000) return reply.code(409).send({ error: '计划已过期或执行中；不要重复导入，请检查已有结果' });
    const table = await pocketbase.collection('app_collections').getFirstListItem(pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && slug = {:slug}', { tenantId: request.tenant.id, appId: request.params.id, slug: job.plan.table })).catch(() => null);
    if (!table || JSON.stringify(table.fields) !== JSON.stringify(job.plan.fields)) return reply.code(409).send({ error: '表结构已变化，请重新预览' });
    await pocketbase.collection('batch_jobs').update(job.id, { status: 'running' });
    const result = { created: 0, failed: 0, items: [] };
    for (const [index, data] of job.plan.rows.entries()) {
      if (!await context(request, reply)) { result.failed += job.plan.rows.length - index; break; }
      const response = await app.inject({ method: 'POST', url: `/api/apps/${encodeURIComponent(job.app_id)}/collections/${encodeURIComponent(job.plan.table)}/records`, headers: { authorization: request.headers.authorization || '', 'x-miao-tenant-id': request.tenant.id }, payload: { data } });
      const payload = response.json();
      if (response.statusCode === 201) { result.created++; result.items.push({ row: index + 1, id: payload.id, status: 'created' }); }
      else { result.failed++; result.items.push({ row: index + 1, status: 'failed', error: payload.error }); }
      await pocketbase.collection('batch_jobs').update(job.id, { result });
    }
    const status = result.failed ? 'partial' : 'completed';
    await pocketbase.collection('batch_jobs').update(job.id, { status, result });
    if (!reply.sent) return { status, result };
  }));
  app.get('/api/apps/:id/import-plans/:planId', { preHandler: auth }, async (request, reply) => {
    if (!await context(request, reply)) return;
    const job = await pocketbase.collection('batch_jobs').getOne(request.params.planId).catch(() => null);
    if (!job || job.tenant_id !== request.tenant.id || job.app_id !== request.params.id || job.user_id !== request.user.id || job.plan?.kind !== 'import') return reply.code(404).send({ error: '导入计划不存在' });
    return { id: job.id, status: job.status, count: job.plan.rows.length, result: job.result || null };
  });
}
