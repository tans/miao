import crypto from 'node:crypto';

const users = new Map();
const usersByEmail = new Map();
const tenants = new Map();
const sessions = new Map();

const now = () => new Date().toISOString();
const publicUser = (user) => ({ id: user.id, email: user.email, name: user.name, created_at: user.created_at });
const hashPassword = (password) => {
  const salt = crypto.randomBytes(16).toString('hex');
  return `${salt}:${crypto.scryptSync(String(password), salt, 64).toString('hex')}`;
};
const verifyPassword = (password, stored) => {
  const [salt, hash] = String(stored || '').split(':');
  if (!salt || !hash || hash.length !== 128) return false;
  const candidate = crypto.scryptSync(String(password || ''), salt, 64).toString('hex');
  return crypto.timingSafeEqual(Buffer.from(candidate, 'hex'), Buffer.from(hash, 'hex'));
};

export const createAuth = ({ id, addEvent }) => {
  const auth = async (request, reply) => {
    const token = request.headers.authorization?.replace(/^Bearer\s+/i, '');
    const session = token && sessions.get(token);
    if (!session || session.expires_at <= now()) return reply.code(401).send({ error: '请先登录' });
    const user = users.get(session.user_id); const tenant = user && tenants.get(user.id);
    if (!user || !tenant) return reply.code(401).send({ error: '账号工作区不存在' });
    request.user = publicUser(user);
    request.tenant = tenant;
    request.token = token;
  };

  const issueSession = (userId) => {
    const token = id();
    sessions.set(token, { user_id: userId, expires_at: new Date(Date.now() + 30 * 86400000).toISOString() });
    return token;
  };

  const registerRoutes = (app, { body, c, publicApp }) => {
    app.post('/api/auth/register', async (request, reply) => {
      const { email, password, name } = body(request); const normalizedEmail = String(email || '').trim().toLowerCase();
      if (!/^\S+@\S+\.\S+$/.test(normalizedEmail)) return reply.code(400).send({ error: '请输入有效邮箱' });
      if (String(password || '').length < 8) return reply.code(400).send({ error: '密码至少 8 位' });
      if (!String(name || '').trim()) return reply.code(400).send({ error: '请输入姓名' });
      if (usersByEmail.has(normalizedEmail)) return reply.code(409).send({ error: '该邮箱已注册' });
      const userId = id(); const tenantId = id(); const timestamp = now(); const displayName = String(name).trim();
      const tenant = { id: tenantId, name: `${displayName} 的工作区`, slug: `${displayName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'workspace'}-${userId.slice(0, 6)}` };
      const user = { id: userId, email: normalizedEmail, password_hash: hashPassword(password), name: displayName, created_at: timestamp };
      users.set(userId, user); usersByEmail.set(normalizedEmail, user); tenants.set(userId, tenant);
      await addEvent({ tenantId, type: 'tenant.created', message: '工作区已创建', actor: 'human', payload: { email: normalizedEmail } });
      return reply.code(201).send({ token: issueSession(userId), user: publicUser(user), tenant, needs_onboarding: true });
    });
    app.post('/api/auth/login', async (request, reply) => {
      const { email, password } = body(request); const user = usersByEmail.get(String(email || '').trim().toLowerCase());
      if (!user || !verifyPassword(password, user.password_hash)) return reply.code(401).send({ error: '邮箱或密码不正确' });
      const tenant = tenants.get(user.id);
      return { token: issueSession(user.id), user: publicUser(user), tenant, needs_onboarding: !(await c('apps').findOne({ tenant_id: tenant.id })) };
    });
    app.post('/api/auth/logout', { preHandler: auth }, async (request) => { sessions.delete(request.token); return { ok: true }; });
    app.get('/api/me', { preHandler: auth }, async (request) => ({ user: request.user, tenant: request.tenant, apps: (await c('apps').find({ tenant_id: request.tenant.id }).sort({ updated_at: -1 }).toArray()).map(publicApp) }));
  };

  return { auth, registerRoutes };
};
