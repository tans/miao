import crypto from 'node:crypto';
import { resolveAppAccess, canPublishApp } from './apps.js';
import { taskAuthority } from '../business/access.js';
import { normalizeTaskDefinition, nextScheduledRun } from '../runtime/task-definition.js';
import { enqueueRun, missing, publicRun, serialized } from '../runtime/repository.js';

export function registerTaskRoutes(app, { auth, pocketbase, worker }) {
  const context = async (request, reply, managing = false) => {
    const application = await resolveAppAccess(request, reply, pocketbase);
    if (!application) return null;
    if (managing && (!canPublishApp(request) || application.archived)) { reply.code(403).send({ error: '需要当前应用的发布权限，且应用未归档' }); return null; }
    return application;
  };
  const owned = async (collection, request, reply, managing = false) => {
    if (!await context(request, reply, managing)) return null;
    const id = collection === 'miao_tasks' ? request.params.taskId : request.params.runId;
    const record = await pocketbase.collection(collection).getOne(id).catch(missing);
    if (!record || record.tenant_id !== request.tenant.id || record.app_id !== request.params.id) { reply.code(404).send({ error: '任务或运行不存在' }); return null; }
    if (managing && record.created_by !== request.user.id && request.appPermission !== 'owner') { reply.code(403).send({ error: '由任务负责人或工作区所有者处理' }); return null; }
    return record;
  };
  app.get('/api/apps/:id/tasks', { preHandler: auth }, async (request, reply) => {
    if (!await context(request, reply)) return;
    const result = await pocketbase.collection('miao_tasks').getList(Math.max(1, Math.min(10000, Number.parseInt(request.query.page, 10) || 1)), 25, { filter: pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId}', { tenantId: request.tenant.id, appId: request.params.id }), sort: '-created' });
    return result;
  });
  app.post('/api/apps/:id/tasks', { preHandler: auth }, async (request, reply) => {
    if (!await context(request, reply, true)) return;
    const name = String(request.body?.name || '').trim();
    if (!name || name.length > 160) return reply.code(400).send({ error: '任务名称不能为空，最多 160 字' });
    const definition = await normalizeTaskDefinition(pocketbase, { tenantId: request.tenant.id, appId: request.params.id }, request.body.definition);
    const task = await pocketbase.collection('miao_tasks').create({ tenant_id: request.tenant.id, app_id: request.params.id, created_by: request.user.id, name, definition, revision: 1, status: 'draft' });
    return reply.code(201).send(task);
  });
  app.patch('/api/apps/:id/tasks/:taskId', { preHandler: auth }, async (request, reply) => serialized(`task:${request.params.taskId}`, async () => {
    const task = await owned('miao_tasks', request, reply, true);
    if (!task) return;
    if (!['draft', 'paused'].includes(task.status) || request.body?.expected_revision !== task.revision) return reply.code(409).send({ error: '先暂停任务，并读取最新版本后修改' });
    const definition = await normalizeTaskDefinition(pocketbase, { tenantId: task.tenant_id, appId: task.app_id }, request.body.definition);
    const name = String(request.body.name || task.name).trim().slice(0, 160);
    if (!name) return reply.code(400).send({ error: '任务名称不能为空' });
    return pocketbase.collection('miao_tasks').update(task.id, { name, definition, revision: task.revision + 1, status: 'draft', next_run_at: '', pause_reason: '' });
  }));
  app.post('/api/apps/:id/tasks/:taskId/enable', { preHandler: auth }, async (request, reply) => serialized(`task:${request.params.taskId}`, async () => {
    const task = await owned('miao_tasks', request, reply, true);
    if (!task) return;
    if (request.body?.confirm !== true || request.body.expected_revision !== task.revision) return reply.code(409).send({ error: '请审阅并确认任务的最新具体版本' });
    await taskAuthority(pocketbase, task);
    await normalizeTaskDefinition(pocketbase, { tenantId: task.tenant_id, appId: task.app_id }, task.definition);
    if (task.status === 'archived') return reply.code(409).send({ error: '请先恢复为草稿并重新审阅' });
    const next = nextScheduledRun(task.definition.trigger);
    if (task.definition.trigger.type === 'once' && !next) return reply.code(400).send({ error: '一次性任务的时间已过，请修改时间后重新确认' });
    return pocketbase.collection('miao_tasks').update(task.id, { status: 'enabled', next_run_at: next, pause_reason: '' });
  }));
  app.post('/api/apps/:id/tasks/:taskId/pause', { preHandler: auth }, async (request, reply) => serialized(`task:${request.params.taskId}`, async () => {
    const task = await owned('miao_tasks', request, reply, true);
    if (!task) return;
    return pocketbase.collection('miao_tasks').update(task.id, { status: 'paused', next_run_at: '', pause_reason: '用户暂停；已创建的运行可单独取消' });
  }));
  app.post('/api/apps/:id/tasks/:taskId/preview', { preHandler: auth }, async (request, reply) => serialized(`task:${request.params.taskId}`, async () => {
    const task = await owned('miao_tasks', request, reply, true);
    if (!task) return;
    if (task.status === 'archived' || request.body?.expected_revision !== task.revision) return reply.code(409).send({ error: '读取当前版本后再试运行' });
    await taskAuthority(pocketbase, task);
    const definition = { ...task.definition, mode: 'preview', scope: { ...task.definition.scope, recipient_ids: [], tables: task.definition.scope.tables.map((grant) => ({ ...grant, write_fields: [] })) }, limits: { ...task.definition.limits, max_writes: 0 } };
    const key = String(request.body.request_id || crypto.randomUUID());
    if (!/^[\w-]{1,100}$/.test(key)) return reply.code(400).send({ error: '请求标识无效' });
    const run = await enqueueRun(pocketbase, { ...task, definition }, `preview:${task.revision}:${key}`);
    return reply.code(202).send(publicRun(run));
  }));
  app.post('/api/apps/:id/tasks/:taskId/archive', { preHandler: auth }, async (request, reply) => serialized(`task:${request.params.taskId}`, async () => {
    const task = await owned('miao_tasks', request, reply, true);
    if (!task) return;
    if (request.body?.confirm !== true || request.body.expected_revision !== task.revision) return reply.code(409).send({ error: '请确认当前任务版本归档，剩余运行将取消' });
    const updated = await pocketbase.collection('miao_tasks').update(task.id, { status: 'archived', next_run_at: '', pause_reason: '用户归档' });
    const runs = await pocketbase.collection('miao_runs').getFullList({ filter: pocketbase.filter('task_id = {:id} && (status = "queued" || status = "running" || status = "waiting")', { id: task.id }) });
    for (const run of runs) await serialized(`run:${run.id}`, async () => {
      const current = await pocketbase.collection('miao_runs').getOne(run.id);
      if (!['queued', 'running', 'waiting'].includes(current.status)) return;
      await pocketbase.collection('miao_runs').update(run.id, { cancel_requested: true, ...(current.status === 'running' ? {} : { status: 'cancelled', error: '任务已归档', finished_at: new Date().toISOString() }) });
      worker.cancel(run.id);
    });
    return updated;
  }));
  app.post('/api/apps/:id/tasks/:taskId/restore', { preHandler: auth }, async (request, reply) => serialized(`task:${request.params.taskId}`, async () => {
    const task = await owned('miao_tasks', request, reply, true);
    if (!task) return;
    if (task.status !== 'archived' || request.body?.expected_revision !== task.revision) return reply.code(409).send({ error: '请读取当前归档版本' });
    return pocketbase.collection('miao_tasks').update(task.id, { status: 'draft', revision: task.revision + 1, next_run_at: '', pause_reason: '' });
  }));
  app.post('/api/apps/:id/tasks/:taskId/transfer', { preHandler: auth }, async (request, reply) => serialized(`task:${request.params.taskId}`, async () => {
    const task = await owned('miao_tasks', request, reply, true);
    if (!task) return;
    if (task.status === 'archived' || request.body?.confirm !== true || request.body.expected_revision !== task.revision) return reply.code(409).send({ error: '确认当前版本的负责人转交，转交后需新负责人重新启用' });
    const unsettled = await pocketbase.collection('miao_runs').getList(1, 1, { filter: pocketbase.filter('task_id = {:id} && (status = "queued" || status = "running" || status = "waiting")', { id: task.id }) });
    if (unsettled.totalItems) return reply.code(409).send({ error: '先处理或取消原负责人尚未结束的运行' });
    const userId = String(request.body.user_id || '');
    await taskAuthority(pocketbase, { ...task, created_by: userId });
    return pocketbase.collection('miao_tasks').update(task.id, { created_by: userId, status: 'draft', revision: task.revision + 1, next_run_at: '', pause_reason: '负责人已转交，等待新负责人重新审阅授权' });
  }));
  app.post('/api/apps/:id/tasks/:taskId/run', { preHandler: auth }, async (request, reply) => serialized(`task:${request.params.taskId}`, async () => {
    const task = await owned('miao_tasks', request, reply, true);
    if (!task) return;
    if (task.status !== 'enabled' || request.body?.expected_revision !== task.revision) return reply.code(409).send({ error: '先确认启用任务的当前版本' });
    await taskAuthority(pocketbase, task);
    const key = String(request.body.request_id || crypto.randomUUID());
    if (!/^[\w-]{1,100}$/.test(key)) return reply.code(400).send({ error: '请求标识无效' });
    const run = await enqueueRun(pocketbase, task, `manual:${task.revision}:${key}`);
    return reply.code(202).send(publicRun(run));
  }));
  app.get('/api/apps/:id/runs', { preHandler: auth }, async (request, reply) => {
    if (!await context(request, reply)) return;
    const page = Math.max(1, Math.min(10000, Number.parseInt(request.query.page, 10) || 1));
    const result = await pocketbase.collection('miao_runs').getList(page, 25, { filter: pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId}', { tenantId: request.tenant.id, appId: request.params.id }), sort: '-created' });
    return { ...result, items: result.items.map(publicRun) };
  });
  app.get('/api/apps/:id/runs/:runId', { preHandler: auth }, async (request, reply) => {
    const run = await owned('miao_runs', request, reply);
    if (!run) return;
    const actions = await pocketbase.collection('miao_actions').getFullList({ filter: pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && run_id = {:runId}', { tenantId: request.tenant.id, appId: request.params.id, runId: run.id }), sort: 'created' });
    const attempts = await pocketbase.collection('miao_run_attempts').getFullList({ filter: pocketbase.filter('run_id = {:id}', { id: run.id }), sort: 'sequence' });
    return { ...publicRun(run), actions, attempt_history: attempts };
  });
  app.post('/api/apps/:id/runs/:runId/cancel', { preHandler: auth }, async (request, reply) => serialized(`run:${request.params.runId}`, async () => {
    const run = await owned('miao_runs', request, reply, true);
    if (!run) return;
    if (!['queued', 'running', 'waiting'].includes(run.status)) return reply.code(409).send({ error: '此运行已经结束' });
    const updated = await pocketbase.collection('miao_runs').update(run.id, { cancel_requested: true, ...(run.status === 'running' ? {} : { status: 'cancelled', finished_at: new Date().toISOString() }) });
    worker.cancel(run.id);
    return publicRun(updated);
  }));
  app.post('/api/apps/:id/runs/:runId/retry', { preHandler: auth }, async (request, reply) => serialized(`run:${request.params.runId}`, async () => {
    const run = await owned('miao_runs', request, reply, true);
    if (!run) return;
    if (!['failed', 'partial'].includes(run.status) || Number(run.retry_count || 0) >= 2 || request.body?.confirm !== true) return reply.code(409).send({ error: '只有失败或部分完成的运行可确认重试，最多三次尝试' });
    await taskAuthority(pocketbase, run);
    const uncertain = await pocketbase.collection('miao_actions').getList(1, 1, { filter: pocketbase.filter('run_id = {:id} && (status = "executing" || status = "unknown")', { id: run.id }) });
    if (uncertain.totalItems) return reply.code(409).send({ error: '存在待核实写入，不能直接重试' });
    return publicRun(await pocketbase.collection('miao_runs').update(run.id, { status: 'queued', retry_count: Number(run.retry_count || 0) + 1, error: '', finished_at: '', delivery_status: run.snapshot.mode === 'preview' ? 'suppressed' : 'pending', cancel_requested: false }));
  }));
  app.post('/api/apps/:id/runs/:runId/resolve', { preHandler: auth }, async (request, reply) => serialized(`run:${request.params.runId}`, async () => {
    const run = await owned('miao_runs', request, reply, true);
    if (!run) return;
    if (run.status !== 'waiting' || request.body?.expected_updated_at !== run.updated || request.body.confirm !== true) return reply.code(409).send({ error: '待处理内容已变化，请刷新后重新确认' });
    await taskAuthority(pocketbase, run);
    const pending = run.pending;
    if (!pending || !['approval', 'information', 'uncertain'].includes(pending.kind)) return reply.code(409).send({ error: '此待处理事项已失效' });
    if (Date.parse(pending.expires_at) <= Date.now()) return reply.code(409).send({ error: '确认已过期，不能继续执行' });
    if (request.body.decision === 'reject') return publicRun(await pocketbase.collection('miao_runs').update(run.id, { status: 'cancelled', error: '用户拒绝本次待处理事项', finished_at: new Date().toISOString() }));
    if (request.body.decision !== 'approve') return reply.code(400).send({ error: '请选择批准或拒绝' });
    if (pending.kind === 'information') {
      const answer = String(request.body.answer || '').trim();
      if (!answer || answer.length > 6000) return reply.code(400).send({ error: '请提供不超过 6000 字的补充信息' });
      return publicRun(await pocketbase.collection('miao_runs').update(run.id, { status: 'queued', pending: { ...pending, answer, answered_by: request.user.id }, error: '' }));
    }
    const action = await pocketbase.collection('miao_actions').getOne(pending.action_id);
    if (action.run_id !== run.id || action.app_id !== run.app_id || action.tenant_id !== run.tenant_id) return reply.code(409).send({ error: '动作与运行不匹配' });
    const table = await pocketbase.collection('app_collections').getFirstListItem(pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && slug = {:slug}', { tenantId: run.tenant_id, appId: run.app_id, slug: action.input.table }));
    const row = await pocketbase.collection(table.pb_collection).getOne(action.input.record_id);
    if (row.tenant_id !== run.tenant_id || row.app_id !== run.app_id) return reply.code(404).send({ error: '记录已失效' });
    if (pending.kind === 'uncertain') {
      if (!Object.entries(action.input.data).every(([key, value]) => row[key] === value)) return reply.code(409).send({ error: '当前记录与预期写入不一致，请拒绝本次运行并根据当前记录建立新任务，不能盲目重试' });
      await pocketbase.collection('miao_actions').update(action.id, { status: 'done', result: { id: row.id, updated_at: row.updated, data: action.input.data, verified_by: request.user.id } });
    } else {
      if (request.body.decision !== 'approve') return reply.code(400).send({ error: '请选择批准或拒绝' });
      if (row.updated !== action.input.expected_updated_at) return reply.code(409).send({ error: '目标记录已变化，此预览不可批准；请拒绝并基于当前数据重新处理' });
      await pocketbase.collection('miao_actions').update(action.id, { status: 'approved', approved_by: request.user.id, approved_at: new Date().toISOString() });
    }
    return publicRun(await pocketbase.collection('miao_runs').update(run.id, { status: 'queued', pending: { ...pending, resolved_by: request.user.id, decision: 'approved' }, error: '' }));
  }));
}
