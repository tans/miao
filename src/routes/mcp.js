export const registerMcpSessionRoutes = (app, { internalAuth, body, c, issueMcpSession, addEvent, now }) => {
  app.post('/api/mcp/session/create', { preHandler: internalAuth }, async (request, reply) => {
    const { app_id: appId, mode = 'user', user_id: userId = null, agent_id: agentId = 'dsh', permissions = [] } = body(request);
    if (!appId || !['builder', 'user'].includes(mode)) return reply.code(400).send({ error: 'app_id 必填，mode 必须是 builder 或 user' });
    const appRecord = await c('apps').findOne({ id: appId });
    if (!appRecord) return reply.code(404).send({ error: '应用不存在' });
    const result = await issueMcpSession({ tenantId: appRecord.tenant_id, appId, scope: mode, userId, agentId, permissions, source: 'internal-bootstrap' });
    await addEvent({ tenantId: appRecord.tenant_id, appId, type: 'mcp.session.created', message: '内部 DSH 会话已绑定 MCP Token', actor: 'system', payload: { session_id: result.session_id, agent_id: agentId, scope: mode } });
    return reply.code(201).send(result);
  });
  app.get('/api/mcp/sessions', { preHandler: internalAuth }, async (request) => {
    const query = {}; if (request.query?.app_id) query.app_id = request.query.app_id; if (request.query?.agent_id) query.agent_id = request.query.agent_id;
    return c('mcp_sessions').find(query, { projection: { _id: 0, token_hash: 0 } }).sort({ created_at: -1 }).limit(100).toArray();
  });
  app.post('/api/mcp/sessions/:sessionId/revoke', { preHandler: internalAuth }, async (request, reply) => {
    const result = await c('mcp_sessions').updateOne({ id: request.params.sessionId, revoked_at: null }, { $set: { revoked_at: now() } });
    if (!result.modifiedCount) return reply.code(404).send({ error: 'MCP Session 不存在或已撤销' });
    return { ok: true };
  });
  app.post('/api/mcp/session/revoke', { preHandler: internalAuth }, async (request, reply) => {
    const sessionId = body(request).session_id;
    if (!sessionId) return reply.code(400).send({ error: 'session_id 必填' });
    const result = await c('mcp_sessions').updateOne({ id: sessionId, revoked_at: null }, { $set: { revoked_at: now() } });
    if (!result.modifiedCount) return reply.code(404).send({ error: 'MCP Session 不存在或已撤销' });
    return { ok: true };
  });
};

