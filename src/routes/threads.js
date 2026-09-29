import { resolveAppAccess } from './apps.js';

export const registerThreadRoutes = (app, { auth, pocketbase }) => {
  const scopedApp = async (request, reply, appId) => {
    if (!appId) return true;
    return resolveAppAccess({ tenant: request.tenant, user: request.user, membership: request.membership, params: { id: appId } }, reply, pocketbase);
  };
  const ownThread = async (request, reply) => {
    const thread = await pocketbase.collection('agent_threads').getOne(request.params.threadId).catch(() => null);
    if (!thread || thread.tenant_id !== request.tenant.id || thread.user_id !== request.user.id) {
      reply.code(404).send({ error: '对话不存在' }); return null;
    }
    if (thread.app_id && !await scopedApp(request, reply, thread.app_id)) return null;
    return thread;
  };

  app.get('/api/agent/threads', { preHandler: auth }, async (request, reply) => {
    const appId = String(request.query?.app_id || '');
    if (appId && !await scopedApp(request, reply, appId)) return;
    const result = await pocketbase.collection('agent_threads').getList(1, 20, {
      filter: pocketbase.filter('tenant_id = {:tenantId} && user_id = {:userId} && app_id = {:appId}', { tenantId: request.tenant.id, userId: request.user.id, appId }), sort: '-updated'
    });
    return result.items.map((item) => ({ id: item.id, title: item.title, app_id: item.app_id, updated_at: item.updated }));
  });

  app.post('/api/agent/threads', { preHandler: auth }, async (request, reply) => {
    const appId = String(request.body?.app_id || '');
    if (appId && !await scopedApp(request, reply, appId)) return;
    const thread = await pocketbase.collection('agent_threads').create({ tenant_id: request.tenant.id, app_id: appId, user_id: request.user.id, title: String(request.body?.title || '新对话').slice(0, 160) });
    return reply.code(201).send({ id: thread.id, app_id: appId, title: thread.title });
  });

  app.get('/api/agent/threads/:threadId/messages', { preHandler: auth }, async (request, reply) => {
    if (!await ownThread(request, reply)) return;
    const result = await pocketbase.collection('agent_messages').getList(Math.max(1, Number.parseInt(request.query?.page, 10) || 1), 50, {
      filter: pocketbase.filter('tenant_id = {:tenantId} && thread_id = {:threadId} && user_id = {:userId}', { tenantId: request.tenant.id, threadId: request.params.threadId, userId: request.user.id }), sort: 'created'
    });
    return { items: result.items.map((item) => ({ id: item.id, role: item.role, content: item.content, created_at: item.created })), page: result.page, totalPages: result.totalPages };
  });

  app.post('/api/agent/threads/:threadId/messages', { preHandler: auth }, async (request, reply) => {
    if (!await ownThread(request, reply)) return;
    const { role, content } = request.body || {};
    if (!['user', 'assistant'].includes(role) || typeof content !== 'string' || !content.trim() || content.length > 30000) return reply.code(400).send({ error: '消息内容无效' });
    const message = await pocketbase.collection('agent_messages').create({ tenant_id: request.tenant.id, thread_id: request.params.threadId, user_id: request.user.id, role, content: content.trim() });
    if (role === 'user') await pocketbase.collection('agent_threads').update(request.params.threadId, { title: content.trim().slice(0, 80) });
    return reply.code(201).send({ id: message.id, role, content: message.content });
  });
};
