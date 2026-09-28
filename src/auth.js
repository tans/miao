import crypto from 'node:crypto';
import { connectPocketBase, createPocketBaseClient, id, pocketbase } from './store.js';

const publicUser = (user) => ({ id: user.id, email: user.email, name: user.name, created_at: user.created });
const publicTenant = (tenant, role) => ({ id: tenant.id, name: tenant.name, slug: tenant.slug, role });
const publicApp = (app) => ({ id: app.id, name: app.name, description: app.description, created_at: app.created, updated_at: app.updated });
const normalizeEmail = (email) => String(email || '').trim().toLowerCase();
const tokenHash = (token) => crypto.createHash('sha256').update(token).digest('hex');

async function listWorkspaces(userId) {
  const memberships = await pocketbase.collection('tenant_members').getFullList({
    filter: pocketbase.filter('user_id = {:userId}', { userId }),
    sort: 'created'
  });
  const workspaces = [];
  for (const membership of memberships) {
    const tenant = await pocketbase.collection('tenants').getOne(membership.tenant_id).catch(() => null);
    if (tenant) workspaces.push(publicTenant(tenant, membership.role));
  }
  return workspaces.sort((a, b) => Number(b.role === 'owner') - Number(a.role === 'owner'));
}

export const createAuth = () => {
  const auth = async (request, reply) => {
    const token = request.headers.authorization?.replace(/^Bearer\s+/i, '');
    if (!token) return reply.code(401).send({ error: '请先登录' });
    try {
      const client = createPocketBaseClient(token);
      const data = await client.collection('users').authRefresh();
      const workspaces = await listWorkspaces(data.record.id);
      if (!workspaces.length) return reply.code(403).send({ error: '账号没有可访问的工作区' });
      const requestedId = request.headers['x-miao-tenant-id'];
      const selected = requestedId
        ? workspaces.find((workspace) => workspace.id === requestedId)
        : workspaces.find((workspace) => workspace.role === 'owner') || workspaces[0];
      if (!selected) return reply.code(403).send({ error: '你没有权限访问这个工作区' });
      const tenant = await pocketbase.collection('tenants').getOne(selected.id);
      request.user = publicUser(data.record);
      request.tenant = tenant;
      request.membership = { role: selected.role };
      request.workspaces = workspaces;
      request.token = data.token;
      reply.header('X-PocketBase-Token', data.token);
    } catch {
      return reply.code(401).send({ error: '请先登录' });
    }
  };

  const requireOwner = async (request, reply) => {
    if (request.membership?.role !== 'owner' || request.tenant.owner_id !== request.user.id) {
      return reply.code(403).send({ error: '只有工作区所有者可以管理成员' });
    }
  };

  const registerRoutes = (app, { body }) => {
    app.post('/api/auth/register', async (request, reply) => {
      const { email, password, name } = body(request);
      const normalizedEmail = normalizeEmail(email);
      const displayName = String(name || '').trim();
      if (!/^\S+@\S+\.\S+$/.test(normalizedEmail)) return reply.code(400).send({ error: '请输入有效邮箱' });
      if (String(password || '').length < 8) return reply.code(400).send({ error: '密码至少 8 位' });
      if (!displayName) return reply.code(400).send({ error: '请输入姓名' });

      let userId;
      let tenantId;
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
        tenantId = tenant.id;
        await pocketbase.collection('tenant_members').create({ tenant_id: tenant.id, user_id: user.id, role: 'owner' });
        const client = createPocketBaseClient();
        const authData = await client.collection('users').authWithPassword(normalizedEmail, password);
        return reply.code(201).send({ token: authData.token, user: publicUser(authData.record), tenant: publicTenant(tenant, 'owner'), needs_onboarding: true });
      } catch (error) {
        if (tenantId) {
          const memberships = await pocketbase.collection('tenant_members').getFullList({
            filter: pocketbase.filter('tenant_id = {:tenantId}', { tenantId })
          }).catch(() => []);
          await Promise.all(memberships.map((membership) => pocketbase.collection('tenant_members').delete(membership.id).catch(() => {})));
        }
        if (tenantId) await pocketbase.collection('tenants').delete(tenantId).catch(() => {});
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
        const authData = await client.collection('users').authWithPassword(normalizeEmail(email), password);
        const workspaces = await listWorkspaces(authData.record.id);
        const tenant = workspaces.find((workspace) => workspace.role === 'owner') || workspaces[0];
        if (!tenant) return reply.code(403).send({ error: '账号没有可访问的工作区' });
        const apps = await pocketbase.collection('apps').getFullList({
          filter: pocketbase.filter('tenant_id = {:tenantId} && archived = false', { tenantId: tenant.id }),
          sort: '-updated'
        });
        return { token: authData.token, user: publicUser(authData.record), tenant, workspaces, needs_onboarding: apps.length === 0 };
      } catch {
        return reply.code(401).send({ error: '邮箱或密码不正确' });
      }
    });

    app.post('/api/auth/logout', { preHandler: auth }, async () => ({ ok: true }));

    app.get('/api/me', { preHandler: auth }, async (request) => {
      const apps = await pocketbase.collection('apps').getFullList({
        filter: pocketbase.filter('tenant_id = {:tenantId} && archived = false', { tenantId: request.tenant.id }),
        sort: '-updated'
      });
      return {
        user: request.user,
        tenant: publicTenant(request.tenant, request.membership.role),
        workspaces: request.workspaces,
        apps: apps.map(publicApp),
        ai_configured: Boolean(process.env.AI_GATEWAY_API_KEY),
      };
    });

    app.get('/api/workspace/members', { preHandler: auth }, async (request) => {
      const memberships = await pocketbase.collection('tenant_members').getFullList({
        filter: pocketbase.filter('tenant_id = {:tenantId}', { tenantId: request.tenant.id }),
        sort: 'created'
      });
      const members = [];
      for (const membership of memberships) {
        const user = await pocketbase.collection('users').getOne(membership.user_id).catch(() => null);
        if (user) members.push({ ...publicUser(user), role: membership.role, membership_id: membership.id });
      }
      return { members, can_manage: request.membership.role === 'owner' };
    });

    app.get('/api/workspace/invites', { preHandler: auth }, async (request, reply) => {
      await requireOwner(request, reply);
      if (reply.sent) return;
      const invites = await pocketbase.collection('tenant_invites').getFullList({
        filter: pocketbase.filter('tenant_id = {:tenantId} && status = {:status}', { tenantId: request.tenant.id, status: 'pending' }),
        sort: '-created'
      });
      return invites.map(({ id: inviteId, email, expires_at, created }) => ({ id: inviteId, email, expires_at, created_at: created }));
    });

    app.post('/api/workspace/invites', { preHandler: auth }, async (request, reply) => {
      await requireOwner(request, reply);
      if (reply.sent) return;
      const email = normalizeEmail(body(request).email);
      if (!/^\S+@\S+\.\S+$/.test(email)) return reply.code(400).send({ error: '请输入有效邮箱' });
      if (email === request.user.email.toLowerCase()) return reply.code(400).send({ error: '你已经是此工作区的所有者' });

      const existingUser = await pocketbase.collection('users').getFirstListItem(
        pocketbase.filter('email = {:email}', { email })
      ).catch(() => null);
      if (existingUser) {
        const existingMembership = await pocketbase.collection('tenant_members').getFirstListItem(
          pocketbase.filter('tenant_id = {:tenantId} && user_id = {:userId}', { tenantId: request.tenant.id, userId: existingUser.id })
        ).catch(() => null);
        if (existingMembership) return reply.code(409).send({ error: '此用户已在工作区中' });
      }

      const pendingInvites = await pocketbase.collection('tenant_invites').getFullList({
        filter: pocketbase.filter('tenant_id = {:tenantId} && status = {:status}', { tenantId: request.tenant.id, status: 'pending' })
      });
      const duplicateInvite = pendingInvites.find((invite) => invite.email.toLowerCase() === email && Date.parse(invite.expires_at) > Date.now());
      if (duplicateInvite) return reply.code(409).send({ error: '此邮箱已有待接受的邀请' });
      if (pendingInvites.filter((invite) => Date.parse(invite.expires_at) > Date.now()).length >= 50) {
        return reply.code(429).send({ error: '待接受邀请已达上限，请先撤销或等待现有邀请过期' });
      }

      const token = crypto.randomBytes(32).toString('base64url');
      const expiresAt = new Date(Date.now() + 72 * 60 * 60 * 1000).toISOString();
      const invite = await pocketbase.collection('tenant_invites').create({
        tenant_id: request.tenant.id,
        email,
        token_hash: tokenHash(token),
        expires_at: expiresAt,
        invited_by: request.user.id,
        status: 'pending'
      });
      return reply.code(201).send({ id: invite.id, email, expires_at: expiresAt, invite_url: `/?invite=${encodeURIComponent(token)}` });
    });

    app.delete('/api/workspace/invites/:id', { preHandler: auth }, async (request, reply) => {
      await requireOwner(request, reply);
      if (reply.sent) return;
      const invite = await pocketbase.collection('tenant_invites').getOne(request.params.id).catch(() => null);
      if (!invite || invite.tenant_id !== request.tenant.id || invite.status !== 'pending') return reply.code(404).send({ error: '邀请不存在' });
      await pocketbase.collection('tenant_invites').update(invite.id, { status: 'revoked' });
      return { ok: true };
    });

    app.delete('/api/workspace/members/:id', { preHandler: auth }, async (request, reply) => {
      await requireOwner(request, reply);
      if (reply.sent) return;
      const membership = await pocketbase.collection('tenant_members').getOne(request.params.id).catch(() => null);
      if (!membership || membership.tenant_id !== request.tenant.id || membership.role === 'owner') return reply.code(404).send({ error: '成员不存在' });
      await pocketbase.collection('tenant_members').delete(membership.id);
      return { ok: true };
    });

    app.post('/api/invites/accept', { preHandler: auth }, async (request, reply) => {
      const token = String(body(request).token || '');
      if (!token || token.length > 128) return reply.code(400).send({ error: '邀请链接无效' });
      const invite = await pocketbase.collection('tenant_invites').getFirstListItem(
        pocketbase.filter('token_hash = {:tokenHash}', { tokenHash: tokenHash(token) })
      ).catch(() => null);
      if (!invite || invite.status !== 'pending' || Date.parse(invite.expires_at) <= Date.now()) {
        return reply.code(400).send({ error: '邀请已失效或已被撤销' });
      }
      if (invite.email.toLowerCase() !== request.user.email.toLowerCase()) {
        return reply.code(403).send({ error: `请使用 ${invite.email} 登录或注册后接受邀请` });
      }

      let membership = await pocketbase.collection('tenant_members').getFirstListItem(
        pocketbase.filter('tenant_id = {:tenantId} && user_id = {:userId}', { tenantId: invite.tenant_id, userId: request.user.id })
      ).catch(() => null);
      if (!membership) {
        membership = await pocketbase.collection('tenant_members').create({ tenant_id: invite.tenant_id, user_id: request.user.id, role: 'member' });
      }
      await pocketbase.collection('tenant_invites').update(invite.id, { status: 'accepted' });
      const tenant = await pocketbase.collection('tenants').getOne(invite.tenant_id);
      return { tenant: publicTenant(tenant, membership.role) };
    });
  };

  return { auth, registerRoutes };
};
