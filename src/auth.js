import crypto from 'node:crypto';
import { connectPocketBase, createPocketBaseClient, id, pocketbase } from './store.js';
import { isMailConfigured, publicUrl, sendMail } from './mailer.js';

const publicUser = (user) => ({ id: user.id, email: user.email, name: user.name, created_at: user.created });
const publicTenant = (tenant, role) => ({ id: tenant.id, name: tenant.name, slug: tenant.slug, role });
const publicApp = (app) => ({ id: app.id, name: app.name, description: app.description, created_at: app.created, updated_at: app.updated });
const normalizeEmail = (email) => String(email || '').trim().toLowerCase();
const tokenHash = (token) => crypto.createHash('sha256').update(token).digest('hex');
const verificationRequired = () => process.env.MIAO_REQUIRE_EMAIL_VERIFICATION === 'true';

async function issueAccountToken(user, kind) {
  const token = crypto.randomBytes(32).toString('base64url');
  const expiresAt = new Date(Date.now() + (kind === 'verify' ? 24 : 1) * 60 * 60 * 1000).toISOString();
  await pocketbase.collection('account_tokens').create({ user_id: user.id, token_hash: tokenHash(token), kind, expires_at: expiresAt });
  const href = publicUrl(kind === 'verify' ? `/verify-email?token=${encodeURIComponent(token)}` : `/reset-password?token=${encodeURIComponent(token)}`);
  const subject = kind === 'verify' ? '验证你的 MIAO 邮箱' : '重置你的 MIAO 密码';
  const label = kind === 'verify' ? '验证邮箱' : '重置密码';
  try {
    await sendMail({ to: user.email, subject, html: `<p>你好 ${String(user.name || '').replace(/[<>&"']/g, '')}，</p><p>请在 ${kind === 'verify' ? '24 小时' : '1 小时'}内使用以下链接${label}：</p><p><a href="${href}">${label}</a></p><p>如果这不是你的操作，请忽略此邮件。</p>` });
  } catch (error) {
    await pocketbase.collection('account_tokens').getFirstListItem(pocketbase.filter('token_hash = {:hash}', { hash: tokenHash(token) })).then((record) => pocketbase.collection('account_tokens').delete(record.id)).catch(() => {});
    throw error;
  }
  return { sent: isMailConfigured(), href: isMailConfigured() ? undefined : href };
}

async function findAccountToken(token, kind) {
  if (!token || token.length > 128) return null;
  const record = await pocketbase.collection('account_tokens').getFirstListItem(
    pocketbase.filter('token_hash = {:hash} && kind = {:kind}', { hash: tokenHash(token), kind })
  ).catch(() => null);
  return record && Date.parse(record.expires_at) > Date.now() ? record : null;
}

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
      if (data.record.disabled) return reply.code(403).send({ error: '账号已停用，请联系管理员' });
      if (verificationRequired() && !data.record.verified) return reply.code(403).send({ error: '请先验证邮箱后再登录' });
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
      const { email, password, name, invite_token: inviteToken } = body(request);
      const normalizedEmail = normalizeEmail(email);
      const displayName = String(name || '').trim();
      if (!/^\S+@\S+\.\S+$/.test(normalizedEmail)) return reply.code(400).send({ error: '请输入有效邮箱' });
      if (String(password || '').length < 8) return reply.code(400).send({ error: '密码至少 8 位' });
      if (!displayName) return reply.code(400).send({ error: '请输入姓名' });
      const registrationMode = process.env.MIAO_REGISTRATION_MODE || 'open';
      if (!['open', 'invite', 'closed'].includes(registrationMode)) return reply.code(503).send({ error: '注册策略配置无效' });
      const validInvite = inviteToken ? await pocketbase.collection('tenant_invites').getFirstListItem(
        pocketbase.filter('token_hash = {:tokenHash} && status = {:status}', { tokenHash: tokenHash(inviteToken), status: 'pending' })
      ).catch(() => null) : null;
      if (registrationMode === 'closed' || (registrationMode === 'invite' && (!validInvite || validInvite.email.toLowerCase() !== normalizedEmail || Date.parse(validInvite.expires_at) <= Date.now()))) {
        return reply.code(403).send({ error: '当前仅允许受邀邮箱注册' });
      }
      const allowedDomains = String(process.env.MIAO_ALLOWED_EMAIL_DOMAINS || '').split(',').map((domain) => domain.trim().toLowerCase()).filter(Boolean);
      if (allowedDomains.length && !allowedDomains.includes(normalizedEmail.split('@')[1])) return reply.code(403).send({ error: '此邮箱域名暂不允许注册' });
      if (verificationRequired() && !isMailConfigured()) return reply.code(503).send({ error: '邮箱验证已开启，但邮件服务尚未配置' });

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
        if (verificationRequired()) {
          await issueAccountToken(user, 'verify');
          return reply.code(201).send({ requires_verification: true, email: normalizedEmail });
        }
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
        if (authData.record.disabled) return reply.code(403).send({ error: '账号已停用，请联系管理员' });
        if (verificationRequired() && !authData.record.verified) return reply.code(403).send({ error: '请先验证邮箱后再登录' });
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

    app.post('/api/auth/verify-email', async (request, reply) => {
      const token = String(body(request).token || '');
      const stored = await findAccountToken(token, 'verify');
      if (!stored) return reply.code(400).send({ error: '验证链接已失效或已使用' });
      await pocketbase.collection('users').update(stored.user_id, { verified: true });
      await pocketbase.collection('account_tokens').delete(stored.id);
      return { ok: true };
    });

    app.post('/api/auth/password-reset/request', async (request, reply) => {
      const email = normalizeEmail(body(request).email);
      const user = await pocketbase.collection('users').getFirstListItem(pocketbase.filter('email = {:email}', { email })).catch(() => null);
      if (user && !user.disabled && isMailConfigured()) await issueAccountToken(user, 'reset');
      return { ok: true, message: '如果该邮箱已注册，密码重置邮件将发送到邮箱。' };
    });

    app.post('/api/auth/password-reset/confirm', async (request, reply) => {
      const { token, password } = body(request);
      if (String(password || '').length < 8) return reply.code(400).send({ error: '密码至少 8 位' });
      const stored = await findAccountToken(String(token || ''), 'reset');
      if (!stored) return reply.code(400).send({ error: '重置链接已失效或已使用' });
      await pocketbase.collection('users').update(stored.user_id, { password, passwordConfirm: password });
      const tokens = await pocketbase.collection('account_tokens').getFullList({ filter: pocketbase.filter('user_id = {:userId} && kind = {:kind}', { userId: stored.user_id, kind: 'reset' }) });
      await Promise.all(tokens.map((record) => pocketbase.collection('account_tokens').delete(record.id)));
      return { ok: true };
    });

    app.post('/api/auth/logout', { preHandler: auth }, async () => ({ ok: true }));

    app.delete('/api/me', { preHandler: auth }, async (request, reply) => {
      const { password, confirm } = body(request);
      if (confirm !== request.user.email) return reply.code(400).send({ error: '请准确输入账号邮箱以确认删除' });
      try {
        const verifier = createPocketBaseClient();
        await verifier.collection('users').authWithPassword(request.user.email, password);
      } catch { return reply.code(401).send({ error: '密码不正确' }); }
      const ownedTenants = await pocketbase.collection('tenants').getFullList({ filter: pocketbase.filter('owner_id = {:userId}', { userId: request.user.id }) });
      for (const tenant of ownedTenants) {
        const tables = await pocketbase.collection('app_collections').getFullList({ filter: pocketbase.filter('tenant_id = {:tenantId}', { tenantId: tenant.id }) });
        for (const table of tables) await pocketbase.collections.delete(table.pb_collection).catch(() => {});
        const apps = await pocketbase.collection('apps').getFullList({ filter: pocketbase.filter('tenant_id = {:tenantId}', { tenantId: tenant.id }) });
        const invites = await pocketbase.collection('tenant_invites').getFullList({ filter: pocketbase.filter('tenant_id = {:tenantId}', { tenantId: tenant.id }) });
        const memberships = await pocketbase.collection('tenant_members').getFullList({ filter: pocketbase.filter('tenant_id = {:tenantId}', { tenantId: tenant.id }) });
        await Promise.all([...tables, ...apps, ...invites, ...memberships].map((record) => pocketbase.collection(record.collectionName).delete(record.id).catch(() => {})));
        await pocketbase.collection('tenants').delete(tenant.id);
      }
      const memberships = await pocketbase.collection('tenant_members').getFullList({ filter: pocketbase.filter('user_id = {:userId}', { userId: request.user.id }) });
      const tokens = await pocketbase.collection('account_tokens').getFullList({ filter: pocketbase.filter('user_id = {:userId}', { userId: request.user.id }) });
      await Promise.all([...memberships, ...tokens].map((record) => pocketbase.collection(record.collectionName).delete(record.id).catch(() => {})));
      await pocketbase.collection('users').delete(request.user.id);
      return { ok: true };
    });

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
      const inviteUrl = publicUrl(`/?invite=${encodeURIComponent(token)}`);
      let emailed = false;
      if (isMailConfigured()) {
        try {
          emailed = await sendMail({ to: email, subject: `加入 ${request.tenant.name} 工作区`, html: `<p>${request.user.name} 邀请你加入「${request.tenant.name}」工作区。</p><p><a href="${inviteUrl}">接受邀请</a></p><p>链接 72 小时内有效。</p>` });
        } catch (error) {
          request.log.error({ err: error }, 'invite email delivery failed');
        }
      }
      return reply.code(201).send({ id: invite.id, email, expires_at: expiresAt, invite_url: inviteUrl, emailed });
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
