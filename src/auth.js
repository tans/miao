import { connectPocketBase, createPocketBaseClient, id, pocketbase } from './store.js';

const publicUser = (user) => ({ id: user.id, email: user.email, name: user.name, created_at: user.created });
const publicTenant = (tenant) => ({ id: tenant.id, name: tenant.name, slug: tenant.slug });
const publicApp = (app) => ({ id: app.id, name: app.name, description: app.description, created_at: app.created, updated_at: app.updated });

export const createAuth = () => {
  const auth = async (request, reply) => {
    const token = request.headers.authorization?.replace(/^Bearer\s+/i, '');
    if (!token) return reply.code(401).send({ error: '请先登录' });
    try {
      const client = createPocketBaseClient(token);
      const data = await client.collection('users').authRefresh();
      const tenant = await pocketbase.collection('tenants').getFirstListItem(
        pocketbase.filter('owner_id = {:ownerId}', { ownerId: data.record.id })
      );
      request.user = publicUser(data.record);
      request.tenant = tenant;
      request.token = data.token;
      reply.header('X-PocketBase-Token', data.token);
    } catch {
      return reply.code(401).send({ error: '请先登录' });
    }
  };

  const registerRoutes = (app, { body }) => {
    app.post('/api/auth/register', async (request, reply) => {
      const { email, password, name } = body(request);
      const normalizedEmail = String(email || '').trim().toLowerCase();
      const displayName = String(name || '').trim();
      if (!/^\S+@\S+\.\S+$/.test(normalizedEmail)) return reply.code(400).send({ error: '请输入有效邮箱' });
      if (String(password || '').length < 8) return reply.code(400).send({ error: '密码至少 8 位' });
      if (!displayName) return reply.code(400).send({ error: '请输入姓名' });

      let userId;
      try {
        await connectPocketBase();
        const user = await pocketbase.collection('users').create({ email: normalizedEmail, password, passwordConfirm: password, name: displayName });
        userId = user.id;
        const tenant = await pocketbase.collection('tenants').create({
          id: id(),
          owner_id: user.id,
          name: `${displayName} 的工作区`,
          slug: `${displayName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'workspace'}-${user.id.slice(0, 6)}`
        });
        const client = createPocketBaseClient();
        const authData = await client.collection('users').authWithPassword(normalizedEmail, password);
        return reply.code(201).send({ token: authData.token, user: publicUser(authData.record), tenant: publicTenant(tenant), needs_onboarding: true });
      } catch (error) {
        if (userId) await pocketbase.collection('users').delete(userId).catch(() => {});
        if (error?.status === 400 || error?.status === 409) return reply.code(409).send({ error: '该邮箱已注册或注册信息无效' });
        request.log.error(error);
        return reply.code(503).send({ error: '账号服务暂不可用' });
      }
    });

    app.post('/api/auth/login', async (request, reply) => {
      const { email, password } = body(request);
      const client = createPocketBaseClient();
      try {
        await connectPocketBase();
        const authData = await client.collection('users').authWithPassword(String(email || '').trim().toLowerCase(), password);
        const tenant = await pocketbase.collection('tenants').getFirstListItem(
          pocketbase.filter('owner_id = {:ownerId}', { ownerId: authData.record.id })
        );
        const apps = await pocketbase.collection('apps').getFullList({
          filter: pocketbase.filter('tenant_id = {:tenantId}', { tenantId: tenant.id }),
          sort: '-updated'
        });
        return { token: authData.token, user: publicUser(authData.record), tenant: publicTenant(tenant), needs_onboarding: apps.length === 0 };
      } catch {
        return reply.code(401).send({ error: '邮箱或密码不正确' });
      }
    });

    app.post('/api/auth/logout', { preHandler: auth }, async () => ({ ok: true }));
    app.get('/api/me', { preHandler: auth }, async (request) => {
      const apps = await pocketbase.collection('apps').getFullList({
        filter: pocketbase.filter('tenant_id = {:tenantId}', { tenantId: request.tenant.id }),
        sort: '-updated'
      });
      return { user: request.user, tenant: publicTenant(request.tenant), apps: apps.map(publicApp) };
    });
  };

  return { auth, registerRoutes };
};
