export const registerAgentRoutes = (app, { auth, requireApp, body, c, id, now, addEvent, issueMcpSession, toolsForManifest, manifestOf }) => {
  const dshUrl = () => process.env.DSH_PUBLIC_URL || process.env.DSH_URL || null;
  const dshLaunchUrl = ({ sessionId, appId }) => {
    const template = process.env.DSH_LAUNCH_URL_TEMPLATE || '';
    if (!template || !template.includes('{session_id}') || !template.includes('{app_id}')) return null;
    return template.replaceAll('{session_id}', encodeURIComponent(sessionId)).replaceAll('{app_id}', encodeURIComponent(appId));
  };
  const agentProfile = (mode, appRecord, request, capabilityToken = null, sessionId = null) => ({
    name: 'miaozao-' + mode, runtime: 'deepseek-harness', model: process.env.DSH_MODEL || 'deepseek-chat',
    system_prompt: '你是秒造企业助手。优先使用秒造业务能力，禁止访问系统文件，所有业务数据通过秒造工具获取。',
    app_id: appRecord.id, session_id: sessionId,
    mcp_url: request.protocol + '://' + request.host + '/api/mcp/' + mode + '?app_id=' + encodeURIComponent(appRecord.id),
    mcp_headers: capabilityToken ? { Authorization: 'Bearer ' + capabilityToken } : null,
    dsh_url: dshUrl(), dsh_launch_supported: Boolean(process.env.DSH_LAUNCH_URL_TEMPLATE), capabilities: ['file', 'ontology', 'action', 'code']
  });
  app.post('/api/apps/:id/agent/sessions', { preHandler: [auth, requireApp] }, async (request, reply) => {
    const mode = body(request).mode === 'builder' ? 'builder' : 'user'; const sessionId = id(); const timestamp = now();
    const capability = await issueMcpSession({ tenantId: request.tenant.id, appId: request.appRecord.id, scope: mode, userId: request.user.id, agentId: body(request).agent_id || 'dsh', permissions: body(request).permissions || [], source: 'agent-session', agentSessionId: sessionId });
    const session = { id: sessionId, mcp_session_id: capability.id, tenant_id: request.tenant.id, app_id: request.appRecord.id, user_id: request.user.id, agent_id: capability.agent_id, mode, runtime: 'deepseek-harness', status: 'ready', workspace_id: 'session-' + sessionId, created_at: timestamp, last_used_at: timestamp };
    await c('agent_sessions').insertOne(session);
    await addEvent({ tenantId: request.tenant.id, appId: request.appRecord.id, type: 'agent.session.created', message: '已创建 ' + (mode === 'builder' ? 'Builder' : 'User') + ' Agent 会话', actor: 'human', payload: { session_id: sessionId, mode } });
    return reply.code(201).send({ session: { id: sessionId, mode, runtime: session.runtime, status: session.status, workspace_id: session.workspace_id, created_at: timestamp }, profile: agentProfile(mode, request.appRecord, request, capability.token, sessionId), launch_url: dshLaunchUrl({ sessionId, appId: request.appRecord.id }) });
  });
  app.get('/api/apps/:id/agent/sessions', { preHandler: [auth, requireApp] }, async (request) => c('agent_sessions').find({ tenant_id: request.tenant.id, app_id: request.appRecord.id }, { projection: { _id: 0 } }).sort({ created_at: -1 }).limit(50).toArray());
  app.get('/api/apps/:id/agent/profile', { preHandler: [auth, requireApp] }, async (request) => agentProfile(request.query?.mode === 'builder' ? 'builder' : 'user', request.appRecord, request));
  app.get('/api/apps/:id/capabilities', { preHandler: [auth, requireApp] }, async (request) => ({ capabilities: toolsForManifest('user', manifestOf(request.appRecord)).map((tool) => tool.name), code_runtime: Boolean(process.env.CODE_EXECUTOR_URL), dsh_runtime: Boolean(dshUrl()) }));
};
