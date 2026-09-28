import { connectPocketBase, createPocketBaseClient, publicApp, pocketbase } from './store.js';

const publicUser = (user) => ({ id: user.id, email: user.email, name: user.name, created_at: user.created });

export const createAuth = ({ id, addEvent, c }) => {
  const auth = async (request, reply) => {
    const token = request.headers.authorization?.replace(/^Bearer\s+/i, '');
    if (!token) return reply.code(401).send({ error: '请先登录' });
    try {
      const client = createPocketBaseClient(token);
      const { record } = await client.collection('users').authRefresh();
      const tenant = await c('tenants').findOne({ owner_id: record.id });
      if (!tenant) return reply.code(401).send({ error: '账号工作区不存在' });
      request.user = publicUser(record);
      request.tenant = tenant;
      request.token = token;
    } catch {
      return reply.code(401).send({ error: '请先登录' });
    }
  };

  const registerRoutes = (app, { body, c: collection }) => {
    app.post('/api/auth/register', async (request, reply) => {
      const { email, password, name } = body(request);
      const normalizedEmail = String(email || '').trim().toLowerCase();
      const displayName = String(name || '').trim();
      if (!/^\S+@\S+\.\S+$/.test(normalizedEmail)) return reply.code(400).send({ error: '请输入有效邮箱' });
      if (String(password || '').length < 8) return reply.code(400).send({ error: '密码至少 8 位' });
      if (!displayName) return reply.code(400).send({ error: '请输入姓名' });
      try {
        await connectPocketBase();
        const record = await pocketbase.collection('users').create({ email: normalizedEmail, password, passwordConfirm: password, name: displayName });
        const tenantId = id();
        const tenant = { id: tenantId, owner_id: record.id, name: `${displayName} 的工作区`, slug: `${displayName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'workspace'}-${record.id.slice(0, 6)}` };
        await collection('tenants').insertOne(tenant);
        await addEvent({ tenantId, type: 'tenant.created', message: '工作区已创建', actor: 'human', payload: { email: normalizedEmail } });
        const client = createPocketBaseClient();
        const authData = await client.collection('users').authWithPassword(normalizedEmail, password);
        return reply.code(201).send({ token: authData.token, user: publicUser(authData.record), tenant, needs_onboarding: true });
      } catch (error) {
        if (error?.status === 400 || error?.status === 409) return reply.code(409).send({ error: '该邮箱已注册或注册信息无效' });
        request.log.error(error);
        return reply.code(503).send({ error: '账号服务暂不可用' });
      }
    });
    app.post('/api/auth/login', async (request, reply) => {
      const { email, password } = body(request);
      const client = createPocketBaseClient();
      try {
        const authData = await client.collection('users').authWithPassword(String(email || '').trim().toLowerCase(), password);
        const tenant = await collection('tenants').findOne({ owner_id: authData.record.id });
        if (!tenant) return reply.code(401).send({ error: '账号工作区不存在' });
        return { token: authData.token, user: publicUser(authData.record), tenant, needs_onboarding: !(await collection('apps').findOne({ tenant_id: tenant.id })) };
      } catch {
        return reply.code(401).send({ error: '邮箱或密码不正确' });
      }
    });
    app.post('/api/auth/logout', { preHandler: auth }, async () => ({ ok: true }));
    app.get('/api/me', { preHandler: auth }, async (request) => ({ user: request.user, tenant: request.tenant, apps: (await collection('apps').find({ tenant_id: request.tenant.id }).sort({ updated_at: -1 }).toArray()).map(publicApp) }));
  };

  return { auth, registerRoutes };
};
