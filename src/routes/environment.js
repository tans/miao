import { resolveAppAccess, canManageApp, canEditRecords } from './apps.js';
import { appPermission } from '../business/access.js';
import { serialized } from '../business/locks.js';
import { updateBusinessRecord, publicRecord } from '../business/records.js';

export async function conversationScope(pocketbase, request) {
  const apps = await pocketbase.collection('apps').getFullList({ filter: pocketbase.filter('tenant_id = {:tenantId} && archived = false', { tenantId: request.tenant.id }) });
  const permissions = [];
  for (const app of apps) {
    const permission = await appPermission(pocketbase, { app, tenant: request.tenant, user: request.user, membership: request.membership });
    if (permission) permissions.push([String(app.id), String(permission.role)]);
  }
  permissions.sort(([a], [b]) => a.localeCompare(b));
  return JSON.stringify({ role: request.membership.role, apps: permissions });
}

export function registerEnvironmentRoutes(app, { auth, pocketbase }) {
  const session = (request) => pocketbase.collection('agent_sessions').getFirstListItem(pocketbase.filter('tenant_id = {:tenantId} && user_id = {:userId}', { tenantId: request.tenant.id, userId: request.user.id })).catch((error) => { if (error.status === 404) return null; throw error; });
  const key = (request) => `session:${request.tenant.id}:${request.user.id}`;
  app.get('/api/agent/conversation', { preHandler: auth }, async (request) => serialized(key(request), async () => {
    const saved = await session(request);
    if (!saved) return { conversation: null };
    if (saved.scope !== await conversationScope(pocketbase, request)) { await pocketbase.collection('agent_sessions').delete(saved.id); return { conversation: null }; }
    return { conversation: { scope: saved.scope, revision: saved.revision, checkpoint: saved.checkpoint, messages: saved.messages || [] } };
  }));
  app.put('/api/agent/conversation', { preHandler: auth }, async (request, reply) => serialized(key(request), async () => {
    const payload = request.body || {};
    const scope = await conversationScope(pocketbase, request);
    if (payload.scope !== scope) return { changed: true, saved: false };
    const saved = await session(request);
    if (saved && saved.scope !== scope) { await pocketbase.collection('agent_sessions').delete(saved.id); return { changed: true, saved: false }; }
    if (!Number.isInteger(payload.expected_revision) || payload.expected_revision !== (saved?.revision || 0)) return { conflict: true, saved: false };
    const checkpoint = payload.checkpoint;
    const messages = payload.messages;
    if (typeof checkpoint !== 'string' || !checkpoint.length || checkpoint.length > 5592408 || !/^[A-Za-z0-9+/]+={0,2}$/.test(checkpoint) || Buffer.from(checkpoint, 'base64').length > 4 * 1024 * 1024 || !Array.isArray(messages) || messages.length > 240 || JSON.stringify(messages).length > 262144 || messages.some((item) => !['user', 'assistant'].includes(item.role) || typeof item.content !== 'string')) return reply.code(400).send({ error: '会话内容无效或超过保存限制' });
    const data = { tenant_id: request.tenant.id, user_id: request.user.id, scope, revision: (saved?.revision || 0) + 1, checkpoint, messages };
    if (saved) await pocketbase.collection('agent_sessions').update(saved.id, data);
    else await pocketbase.collection('agent_sessions').create(data);
    return { saved: true, revision: data.revision };
  }));
  app.delete('/api/agent/conversation', { preHandler: auth }, async (request) => serialized(key(request), async () => {
    const saved = await session(request); if (saved) await pocketbase.collection('agent_sessions').delete(saved.id); return { ok: true };
  }));
  app.get('/api/apps/:id/context', { preHandler: auth }, async (request, reply) => {
    const application = await resolveAppAccess(request, reply, pocketbase);
    if (application) return { content: application.business_context || '', revision: application.context_revision || 0 };
  });
  app.put('/api/apps/:id/context', { preHandler: auth }, async (request, reply) => serialized(`context:${request.params.id}`, async () => {
    const application = await resolveAppAccess(request, reply, pocketbase);
    if (!application) return;
    if (!canManageApp(request) || application.archived) return reply.code(403).send({ error: '没有应用管理权限' });
    const { content, expected_revision } = request.body || {};
    if (typeof content !== 'string' || content.length > 16000) return reply.code(400).send({ error: '业务说明最多 16000 字' });
    if (expected_revision !== (application.context_revision || 0)) return reply.code(409).send({ error: '业务说明已变化，请读取后重试' });
    const revision = (application.context_revision || 0) + 1;
    await pocketbase.collection('apps').update(application.id, { business_context: content, context_revision: revision });
    return { content, revision };
  }));
  app.get('/api/apps/:id/record-changes', { preHandler: auth }, async (request, reply) => {
    if (!await resolveAppAccess(request, reply, pocketbase)) return;
    const params = { tenantId: request.tenant.id, appId: request.params.id };
    let filter = 'tenant_id = {:tenantId} && app_id = {:appId}';
    if (request.query.record_id) { filter += ' && record_id = {:recordId}'; params.recordId = String(request.query.record_id); }
    return pocketbase.collection('miao_record_changes').getList(Math.max(1, Number.parseInt(request.query.page, 10) || 1), 25, { filter: pocketbase.filter(filter, params), sort: '-created' });
  });
  app.post('/api/apps/:id/record-changes/:changeId/restore', { preHandler: auth }, async (request, reply) => {
    const application = await resolveAppAccess(request, reply, pocketbase);
    if (!application) return;
    if (!canEditRecords(request) || application.archived) return reply.code(403).send({ error: '没有修改权限' });
    if (request.body?.confirm !== true || !request.body.expected_updated_at) return reply.code(400).send({ error: '先读取当前记录并确认恢复' });
    const change = await pocketbase.collection('miao_record_changes').getOne(request.params.changeId).catch(() => null);
    if (!change || change.tenant_id !== request.tenant.id || change.app_id !== application.id) return reply.code(404).send({ error: '修改记录不存在' });
    const table = await pocketbase.collection('app_collections').getFirstListItem(pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && slug = {:slug}', { tenantId: request.tenant.id, appId: application.id, slug: change.table }));
    const row = await pocketbase.collection(table.pb_collection).getOne(change.record_id);
    const fields = table.fields.filter((field) => field.type !== 'file');
    const names = fields.filter((field) => JSON.stringify(change.before?.[field.name]) !== JSON.stringify(change.after?.[field.name])).map((field) => field.name);
    if (!names.length || names.some((name) => JSON.stringify(row[name]) !== JSON.stringify(change.after?.[name]))) return reply.code(409).send({ error: '目标字段已有后续变化或无可恢复字段，请重新检查' });
    const data = Object.fromEntries(names.map((name) => [name, change.before[name]]));
    const saved = await updateBusinessRecord({ pocketbase, table, tenantId: request.tenant.id, appId: application.id, recordId: row.id, data, expectedUpdated: request.body.expected_updated_at, actorId: request.user.id, eventSource: 'restore', authorize: async () => {
      const current = await resolveAppAccess(request, reply, pocketbase);
      if (!current || current.archived || !canEditRecords(request)) throw Object.assign(new Error('权限已变化'), { statusCode: 403 });
    } });
    return { status: 'restored', record: publicRecord(saved), note: '产生新的修改记录；附件、删除与表结构不回滚' };
  });
}
