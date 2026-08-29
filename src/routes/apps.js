export const registerAppRoutes = (app, { auth, requireApp, body, c, id, now, hashPassword, verifyPassword, addEvent, publicUser, publicApp, starterDefinition, compileDefinition, serializeDefinition, manifestOf, blockingPublishDiagnostics, publishSnapshot, rollbackSnapshot, issueAppToken }) => {
  app.post('/api/auth/register', async (request, reply) => {
    const { email, password, name } = body(request); const normalizedEmail = String(email || '').toLowerCase();
    if (!/^\S+@\S+\.\S+$/.test(normalizedEmail)) return reply.code(400).send({ error: '请输入有效邮箱' });
    if (!password || String(password).length < 8) return reply.code(400).send({ error: '密码至少 8 位' });
    if (!name?.trim()) return reply.code(400).send({ error: '请输入姓名' });
    if (await c('users').findOne({ email: normalizedEmail })) return reply.code(409).send({ error: '该邮箱已注册' });
    const userId = id(); const tenantId = id(); const timestamp = now(); const tenantName = `${name.trim()} 的工作区`; const slugBase = name.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'workspace'; const tenantSlug = `${slugBase}-${userId.slice(0, 6)}`;
    await c('users').insertOne({ id: userId, email: normalizedEmail, password_hash: hashPassword(password), name: name.trim(), created_at: timestamp });
    await c('tenants').insertOne({ id: tenantId, name: tenantName, slug: tenantSlug, owner_id: userId, created_at: timestamp });
    const token = id(); await c('sessions').insertOne({ token, user_id: userId, created_at: timestamp, expires_at: new Date(Date.now() + 30 * 86400000).toISOString() });
    await addEvent({ tenantId, type: 'tenant.created', message: '工作区已创建', actor: 'human', payload: { email: normalizedEmail } });
    return reply.code(201).send({ token, user: { id: userId, email: normalizedEmail, name: name.trim() }, tenant: { id: tenantId, name: tenantName, slug: tenantSlug }, needs_onboarding: true });
  });
  app.post('/api/auth/login', async (request, reply) => {
    const { email, password } = body(request); const user = await c('users').findOne({ email: String(email || '').toLowerCase() });
    if (!user || !verifyPassword(String(password || ''), user.password_hash)) return reply.code(401).send({ error: '邮箱或密码不正确' });
    const tenant = await c('tenants').findOne({ owner_id: user.id }, { sort: { created_at: 1 } }); const token = id();
    await c('sessions').insertOne({ token, user_id: user.id, created_at: now(), expires_at: new Date(Date.now() + 30 * 86400000).toISOString() });
    return { token, user: publicUser(user), tenant: { id: tenant.id, name: tenant.name, slug: tenant.slug }, needs_onboarding: !(await c('apps').findOne({ tenant_id: tenant.id })) };
  });
  app.post('/api/auth/logout', { preHandler: auth }, async (request) => { await c('sessions').deleteOne({ token: request.token }); return { ok: true }; });
  app.get('/api/me', { preHandler: auth }, async (request) => ({ user: request.user, tenant: request.tenant, apps: (await c('apps').find({ tenant_id: request.tenant.id }).sort({ updated_at: -1 }).toArray()).map(publicApp) }));
  
  app.post('/api/onboard', { preHandler: auth }, async (request, reply) => {
    const { name, goal, concepts } = body(request); if (!name?.trim() || !goal?.trim()) return reply.code(400).send({ error: '请填写应用名称和目标' });
    const definition = starterDefinition({ name: name.trim(), goal: goal.trim(), concepts: Array.isArray(concepts) ? concepts.filter(Boolean).slice(0, 12) : [] }); const manifest = compileDefinition(definition); const appId = id(); const timestamp = now();
    await c('apps').insertOne({ id: appId, tenant_id: request.tenant.id, name: name.trim(), description: goal.trim(), published_definition: definition, published_manifest_json: manifest, draft_definition: definition, draft_manifest_json: manifest, published_version: 1, draft_version: 1, created_at: timestamp, updated_at: timestamp });
    await c('app_versions').insertOne({ id: id(), app_id: appId, version: 1, definition, manifest_json: manifest, status: 'published', created_at: timestamp, published_at: timestamp, previous_version: null });
    await addEvent({ tenantId: request.tenant.id, appId, type: 'app.published', message: `应用「${name.trim()}」已创建并发布 v1`, actor: 'human' });
    return reply.code(201).send({ app: publicApp(await c('apps').findOne({ id: appId })) });
  });
  app.get('/api/apps', { preHandler: auth }, async (request) => (await c('apps').find({ tenant_id: request.tenant.id }).sort({ updated_at: -1 }).toArray()).map(publicApp));
  app.get('/api/apps/:id', { preHandler: [auth, requireApp] }, async (request) => publicApp(request.appRecord));
  app.get('/api/apps/:id/definition', { preHandler: [auth, requireApp] }, async (request) => ({ definition: request.appRecord.draft_definition ?? request.appRecord.definition, files: serializeDefinition(request.appRecord.draft_definition ?? request.appRecord.definition), manifest: manifestOf(request.appRecord, 'draft'), version: request.appRecord.draft_version }));
  app.put('/api/apps/:id/definition', { preHandler: [auth, requireApp] }, async (request, reply) => {
    const definition = body(request).definition; if (!definition || typeof definition !== 'object') return reply.code(400).send({ error: 'definition 至少需要 app.md 和 app.yaml，ontology.yaml、workflow.yaml、actions.yaml 可选' }); const current = request.appRecord; const nextVersion = Math.max(current.published_version, current.draft_version) + 1; const manifest = compileDefinition(definition); if (manifest.diagnostics.some((item) => item.level === 'error')) return reply.code(422).send({ error: manifest.diagnostics.map((item) => item.message).join('；'), diagnostics: manifest.diagnostics }); const timestamp = now();
    await c('apps').updateOne({ id: current.id }, { $set: { draft_definition: definition, draft_manifest_json: manifest, draft_version: nextVersion, updated_at: timestamp } }); await c('app_versions').insertOne({ id: id(), app_id: current.id, version: nextVersion, definition, manifest_json: manifest, status: 'draft', created_at: timestamp, published_at: null, previous_version: current.published_version }); await addEvent({ tenantId: request.tenant.id, appId: current.id, type: 'app.draft', message: `已生成定义 v${nextVersion} 草稿`, actor: 'builder' }); return { version: nextVersion, definition, files: serializeDefinition(definition), manifest };
  });
  app.post('/api/apps/:id/compile', { preHandler: [auth, requireApp] }, async (request) => { const manifest = compileDefinition(request.appRecord.draft_definition ?? request.appRecord.definition); return { ok: !manifest.diagnostics.some((item) => item.level === 'error'), manifest, draft_version: request.appRecord.draft_version }; });
  app.post('/api/apps/:id/publish', { preHandler: [auth, requireApp] }, async (request, reply) => { const current = request.appRecord; const manifest = manifestOf(current, 'draft'); const diagnostics = blockingPublishDiagnostics(manifest); if (diagnostics.length) return reply.code(422).send({ error: '当前 Ontology 不满足发布条件', diagnostics }); if (!current.draft_version || current.draft_version <= current.published_version) return reply.code(400).send({ error: '没有待发布草稿' }); const timestamp = now(); const update = await c('apps').updateOne({ id: current.id, draft_version: current.draft_version, published_version: current.published_version }, { $set: publishSnapshot(current, timestamp) }); if (!update.modifiedCount) return reply.code(409).send({ error: '应用版本已变化，请重新读取后发布' }); await c('app_versions').updateMany({ app_id: current.id, status: 'published' }, { $set: { status: 'archived' } }); await c('app_versions').updateOne({ app_id: current.id, version: current.draft_version }, { $set: { status: 'published', published_at: timestamp } }); await addEvent({ tenantId: request.tenant.id, appId: current.id, type: 'app.published', message: `已发布 v${current.draft_version}`, actor: 'builder' }); return { ok: true, version: current.draft_version }; });
  app.post('/api/apps/:id/rollback', { preHandler: [auth, requireApp] }, async (request, reply) => { const target = Number(body(request).version); const version = await c('app_versions').findOne({ app_id: request.appRecord.id, version: target }); if (!version) return reply.code(404).send({ error: '版本不存在' }); const timestamp = now(); await c('app_versions').updateMany({ app_id: request.appRecord.id, status: 'published' }, { $set: { status: 'archived' } }); await c('app_versions').updateOne({ id: version.id }, { $set: { status: 'published', published_at: timestamp } }); await c('apps').updateOne({ id: request.appRecord.id }, { $set: rollbackSnapshot(version, timestamp) }); await addEvent({ tenantId: request.tenant.id, appId: request.appRecord.id, type: 'app.rollback', message: `已回滚发布版本到 v${target}`, actor: 'builder' }); return { ok: true, version: target }; });
  app.get('/api/apps/:id/versions', { preHandler: [auth, requireApp] }, async (request) => c('app_versions').find({ app_id: request.appRecord.id }, { projection: { _id: 0, version: 1, status: 1, created_at: 1, published_at: 1, previous_version: 1 } }).sort({ version: -1 }).toArray());
  app.post('/api/apps/:id/tokens', { preHandler: [auth, requireApp] }, async (request, reply) => {
    const scope = body(request).scope;
    if (!['builder', 'user'].includes(scope)) return reply.code(400).send({ error: 'scope 必须是 builder 或 user' });
    const result = await issueAppToken({ tenantId: request.tenant.id, appId: request.appRecord.id, scope, userId: request.user.id, agentId: body(request).agent_id || 'external-agent', permissions: body(request).permissions || [], expiresInDays: body(request).expires_in_days });
    await addEvent({ tenantId: request.tenant.id, appId: request.appRecord.id, type: 'mcp.token.created', message: `已签发长期 ${scope} MCP Token`, actor: 'human', payload: { token_id: result.id, scope, expires_at: result.expires_at } });
    return reply.code(201).send(result);
  });
  app.delete('/api/apps/:id/tokens/:tokenId', { preHandler: [auth, requireApp] }, async (request, reply) => {
    const result = await c('app_tokens').updateOne({ id: request.params.tokenId, tenant_id: request.tenant.id, app_id: request.appRecord.id, revoked_at: null }, { $set: { revoked_at: now() } });
    if (!result.modifiedCount) return reply.code(404).send({ error: 'MCP Token 不存在或已撤销' });
    return { ok: true };
  });
};

