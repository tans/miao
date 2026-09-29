import { availablePlatformAdminCount, isPlatformAdmin, readAIConfig, useEnvironmentAIKey, writeAIKey } from '../ai-settings.js';
import { isMailConfigured } from '../mailer.js';
import { pocketbase } from '../store.js';

const normalizedEmail = (value) => String(value || '').trim().toLowerCase();
const clampInteger = (value, fallback, min, max) => {
  const parsed = Number.parseInt(value, 10);
  return Number.isInteger(parsed) ? Math.min(max, Math.max(min, parsed)) : fallback;
};
const asIso = (date) => date.toISOString();

const pageOptions = (request) => ({
  page: clampInteger(request.query?.page, 1, 1, 1_000_000),
  perPage: clampInteger(request.query?.perPage, 25, 1, 100),
});

const filtersFrom = (filters, params) => filters.length ? pocketbase.filter(filters.join(' && '), params) : '';

const writePlatformAudit = async (request, { action, targetType, targetId = '', reason = '', status = 200 }) => {
  try {
    return await pocketbase.collection('platform_audit_logs').create({
      actor_id: request.user.id,
      actor_email: request.user.email,
      action,
      target_type: targetType,
      target_id: String(targetId).slice(0, 64),
      reason: String(reason).trim().slice(0, 500),
      status,
    });
  } catch (error) {
    request.log.error({ err: error, action, targetType, targetId }, 'failed to save platform audit log');
    return null;
  }
};

const finishPlatformAudit = async (request, record, { action, status }) => {
  if (!record) return;
  await pocketbase.collection('platform_audit_logs').update(record.id, { action, status })
    .catch((error) => request.log.error({ err: error, action, auditId: record.id }, 'failed to complete platform audit log'));
};

const requirePlatformAdmin = async (request, reply) => {
  if (isPlatformAdmin(request.user?.email)) return;
  if (request.user) {
    const target = String(request.routeOptions?.url || request.url.split('?')[0]).slice(0, 64);
    await writePlatformAudit(request, { action: 'access.denied', targetType: 'endpoint', targetId: target, status: 403 });
  }
  return reply.code(403).send({ error: '无权访问平台管理功能' });
};

const loadAggregate = async (from, to) => {
  const filter = pocketbase.filter('created >= {:from} && created <= {:to}', { from, to });
  const perPage = 200;
  const totals = { requests: 0, successes: 0, errors: 0, pending: 0, input_tokens: 0, output_tokens: 0 };
  const byTenant = new Map();
  let page = 1;
  let totalPages = 1;

  while (page <= totalPages) {
    const result = await pocketbase.collection('ai_usage').getList(page, perPage, { filter, sort: 'created,id' });
    totalPages = result.totalPages;
    for (const row of result.items) {
      const current = byTenant.get(row.tenant_id) || { tenant_id: row.tenant_id, requests: 0, successes: 0, errors: 0, pending: 0, input_tokens: 0, output_tokens: 0 };
      const status = Number(row.status || 0);
      const inputTokens = Number(row.input_tokens || 0);
      const outputTokens = Number(row.output_tokens || 0);

      totals.requests += 1;
      current.requests += 1;
      if (status >= 400) { totals.errors += 1; current.errors += 1; }
      else if (status >= 200 && status < 400) { totals.successes += 1; current.successes += 1; }
      else { totals.pending += 1; current.pending += 1; }
      totals.input_tokens += inputTokens;
      totals.output_tokens += outputTokens;
      current.input_tokens += inputTokens;
      current.output_tokens += outputTokens;
      byTenant.set(row.tenant_id, current);
    }
    page += 1;
  }

  return { totals, byTenant: [...byTenant.values()] };
};

const count = async (collectionName, filter = '') => {
  const result = await pocketbase.collection(collectionName).getList(1, 1, { filter });
  return result.totalItems;
};

const addQueryFilter = (filters, params, expression, key, value) => {
  if (value === undefined || value === '') return;
  filters.push(expression);
  params[key] = value;
};

export const registerAdminRoutes = (app, { auth, body }) => {
  const platformOnly = () => ({ preHandler: [auth, requirePlatformAdmin] });

  app.get('/api/admin/overview', platformOnly(), async (request, reply) => {
    const start = new Date();
    start.setUTCHours(0, 0, 0, 0);
    const end = new Date();
    const from = asIso(start);
    const to = asIso(end);

    try {
      const [users, disabledUsers, workspaces, apps, ai] = await Promise.all([
        count('users'),
        count('users', 'disabled = true'),
        count('tenants'),
        count('apps'),
        loadAggregate(from, to),
      ]);
      return {
        generated_at: to,
        users: { total: users, active: users - disabledUsers, disabled: disabledUsers },
        workspaces,
        apps,
        ai_today: ai.totals,
      };
    } catch (error) {
      request.log.error({ err: error }, 'failed to load platform overview');
      return reply.code(503).send({ error: '平台总览暂时不可用' });
    }
  });

  app.get('/api/admin/runtime', platformOnly(), async (request, reply) => {
    try {
      const ai = await readAIConfig();
      const registrationMode = process.env.MIAO_REGISTRATION_MODE || 'open';
      const allowedDomains = String(process.env.MIAO_ALLOWED_EMAIL_DOMAINS || '').split(',').map((domain) => domain.trim().toLowerCase()).filter(Boolean);
      return {
        registration: {
          mode: ['open', 'invite', 'closed'].includes(registrationMode) ? registrationMode : 'invalid',
          email_verification_required: process.env.MIAO_REQUIRE_EMAIL_VERIFICATION === 'true',
          allowed_email_domains: allowedDomains,
        },
        mail: { configured: isMailConfigured(), public_url_configured: Boolean(process.env.MIAO_PUBLIC_URL) },
        ai: {
          provider: ai.provider,
          model: ai.model,
          configured: Boolean(ai.key),
          source: ai.source,
          encryption_key_ready: String(process.env.MIAO_SETTINGS_ENCRYPTION_KEY || '').length >= 32,
        },
      };
    } catch (error) {
      request.log.error({ err: error }, 'failed to read platform runtime status');
      return reply.code(503).send({ error: '运行配置状态暂时不可用' });
    }
  });

  app.get('/api/admin/users', platformOnly(), async (request, reply) => {
    const { page, perPage } = pageOptions(request);
    const filters = [];
    const params = {};
    const query = String(request.query?.q || '').trim().slice(0, 120);
    if (query) {
      filters.push('(email ~ {:query} || name ~ {:query})');
      params.query = query;
    }
    if (request.query?.status === 'active') filters.push('disabled = false');
    if (request.query?.status === 'disabled') filters.push('disabled = true');

    try {
      const result = await pocketbase.collection('users').getList(page, perPage, {
        filter: filtersFrom(filters, params), sort: '-created', fields: 'id,email,name,verified,disabled,created,updated'
      });
      return {
        items: result.items.map(({ id, email, name, verified, disabled, created, updated }) => ({
          id, email, name, verified: Boolean(verified), disabled: Boolean(disabled), created_at: created, updated_at: updated,
        })),
        page: result.page, perPage: result.perPage, totalItems: result.totalItems, totalPages: result.totalPages,
      };
    } catch (error) {
      request.log.error({ err: error }, 'failed to list platform users');
      return reply.code(503).send({ error: '用户列表暂时不可用' });
    }
  });

  app.patch('/api/admin/users/:id/status', platformOnly(), async (request, reply) => {
    const disabled = body(request).disabled;
    const reason = String(body(request).reason || '').trim();
    const targetId = String(request.params.id || '');
    if (typeof disabled !== 'boolean') {
      await writePlatformAudit(request, { action: 'user.status.rejected', targetType: 'user', targetId, reason, status: 400 });
      return reply.code(400).send({ error: '账号状态无效' });
    }
    if (reason.length < 5 || reason.length > 500) {
      await writePlatformAudit(request, { action: 'user.status.rejected', targetType: 'user', targetId, status: 400 });
      return reply.code(400).send({ error: '请填写 5 到 500 个字符的操作原因' });
    }
    if (disabled && targetId === request.user.id) {
      await writePlatformAudit(request, { action: 'user.status.rejected', targetType: 'user', targetId, reason, status: 409 });
      return reply.code(409).send({ error: '不能停用当前登录的平台管理员账号' });
    }

    const user = await pocketbase.collection('users').getOne(targetId).catch(() => null);
    if (!user) {
      await writePlatformAudit(request, { action: 'user.status.rejected', targetType: 'user', targetId, reason, status: 404 });
      return reply.code(404).send({ error: '用户不存在' });
    }

    if (disabled) {
      const platformEmails = [...new Set(String(process.env.MIAO_ADMIN_EMAILS || '').split(',').map(normalizedEmail).filter(Boolean))];
      if (platformEmails.includes(normalizedEmail(user.email))) {
        if (await availablePlatformAdminCount() <= 1) {
          await writePlatformAudit(request, { action: 'user.status.rejected', targetType: 'user', targetId, reason, status: 409 });
          return reply.code(409).send({ error: '必须至少保留一个可用的平台管理员账号' });
        }
      }
    }

    const audit = await writePlatformAudit(request, { action: 'user.status.change_requested', targetType: 'user', targetId, reason, status: 102 });
    if (!audit) return reply.code(503).send({ error: '平台审计服务暂不可用，未执行账号状态变更' });

    try {
      await pocketbase.collection('users').update(targetId, { disabled });
      await finishPlatformAudit(request, audit, { action: `user.${disabled ? 'disabled' : 'enabled'}`, status: 200 });
      return { ok: true, id: targetId, disabled };
    } catch (error) {
      request.log.error({ err: error, targetId }, 'failed to update platform user status');
      await finishPlatformAudit(request, audit, { action: 'user.status.failed', status: 503 });
      return reply.code(503).send({ error: '账号状态更新失败' });
    }
  });

  app.get('/api/admin/workspaces', platformOnly(), async (request, reply) => {
    const { page, perPage } = pageOptions(request);
    const filters = [];
    const params = {};
    const query = String(request.query?.q || '').trim().slice(0, 120);
    addQueryFilter(filters, params, 'name ~ {:query} || slug ~ {:query}', 'query', query);

    try {
      const result = await pocketbase.collection('tenants').getList(page, perPage, { filter: filtersFrom(filters, params), sort: '-created' });
      const items = await Promise.all(result.items.map(async (tenant) => {
        const [owner, memberCount, appCount] = await Promise.all([
          pocketbase.collection('users').getOne(tenant.owner_id).catch(() => null),
          count('tenant_members', pocketbase.filter('tenant_id = {:tenantId}', { tenantId: tenant.id })),
          count('apps', pocketbase.filter('tenant_id = {:tenantId}', { tenantId: tenant.id })),
        ]);
        return {
          id: tenant.id, name: tenant.name, slug: tenant.slug,
          owner: owner ? { id: owner.id, name: owner.name, email: owner.email, disabled: Boolean(owner.disabled) } : null,
          member_count: memberCount, app_count: appCount, created_at: tenant.created,
        };
      }));
      return { items, page: result.page, perPage: result.perPage, totalItems: result.totalItems, totalPages: result.totalPages };
    } catch (error) {
      request.log.error({ err: error }, 'failed to list platform workspaces');
      return reply.code(503).send({ error: '工作区列表暂时不可用' });
    }
  });

  app.get('/api/admin/apps', platformOnly(), async (request, reply) => {
    const { page, perPage } = pageOptions(request);
    const filters = [];
    const params = {};
    const query = String(request.query?.q || '').trim().slice(0, 120);
    addQueryFilter(filters, params, 'name ~ {:query}', 'query', query);
    if (request.query?.archived === 'true') filters.push('archived = true');
    else if (request.query?.archived === 'false') filters.push('archived = false');

    try {
      const result = await pocketbase.collection('apps').getList(page, perPage, { filter: filtersFrom(filters, params), sort: '-updated' });
      const items = await Promise.all(result.items.map(async (record) => {
        const [tenant, tableCount] = await Promise.all([
          pocketbase.collection('tenants').getOne(record.tenant_id).catch(() => null),
          count('app_collections', pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId}', { tenantId: record.tenant_id, appId: record.id })),
        ]);
        return {
          id: record.id, name: record.name, tenant: tenant ? { id: tenant.id, name: tenant.name } : null,
          archived: Boolean(record.archived), restricted: Boolean(record.restricted), table_count: tableCount,
          created_at: record.created, updated_at: record.updated,
        };
      }));
      return { items, page: result.page, perPage: result.perPage, totalItems: result.totalItems, totalPages: result.totalPages };
    } catch (error) {
      request.log.error({ err: error }, 'failed to list platform apps');
      return reply.code(503).send({ error: '应用目录暂时不可用' });
    }
  });

  app.get('/api/admin/usage', platformOnly(), async (request, reply) => {
    const end = request.query?.to ? new Date(String(request.query.to)) : new Date();
    const start = request.query?.from ? new Date(String(request.query.from)) : new Date(end.getTime() - 6 * 24 * 60 * 60 * 1000);
    if (!Number.isFinite(start.getTime()) || !Number.isFinite(end.getTime()) || start > end) {
      return reply.code(400).send({ error: '用量查询时间范围无效' });
    }
    if (end.getTime() - start.getTime() > 31 * 24 * 60 * 60 * 1000) {
      return reply.code(400).send({ error: '单次用量查询范围最多为 31 天' });
    }
    const { page, perPage } = pageOptions(request);

    try {
      const aggregate = await loadAggregate(asIso(start), asIso(end));
      const sorted = aggregate.byTenant.sort((a, b) => b.requests - a.requests);
      const pageRows = sorted.slice((page - 1) * perPage, page * perPage);
      const items = await Promise.all(pageRows.map(async (row) => {
        const tenant = await pocketbase.collection('tenants').getOne(row.tenant_id).catch(() => null);
        return { ...row, tenant: tenant ? { id: tenant.id, name: tenant.name } : null };
      }));
      return {
        from: asIso(start), to: asIso(end), totals: aggregate.totals,
        items, page, perPage, totalItems: sorted.length, totalPages: Math.ceil(sorted.length / perPage),
      };
    } catch (error) {
      request.log.error({ err: error }, 'failed to load platform AI usage');
      return reply.code(503).send({ error: '平台用量暂时不可用' });
    }
  });

  app.get('/api/admin/audit', platformOnly(), async (request, reply) => {
    const { page, perPage } = pageOptions(request);
    const filters = [];
    const params = {};
    addQueryFilter(filters, params, 'target_type = {:targetType}', 'targetType', request.query?.targetType);
    addQueryFilter(filters, params, 'target_id = {:targetId}', 'targetId', request.query?.targetId);

    try {
      const result = await pocketbase.collection('platform_audit_logs').getList(page, perPage, {
        filter: filtersFrom(filters, params), sort: '-created',
      });
      return {
        items: result.items.map(({ id, actor_id, actor_email, action, target_type, target_id, reason, status, created }) => ({
          id, actor_id, actor_email, action, target_type, target_id, reason, status, created_at: created,
        })),
        page: result.page, perPage: result.perPage, totalItems: result.totalItems, totalPages: result.totalPages,
      };
    } catch (error) {
      request.log.error({ err: error }, 'failed to list platform audit logs');
      return reply.code(503).send({ error: '平台审计暂时不可用' });
    }
  });

  app.get('/api/admin/ai', platformOnly(), async (request, reply) => {
    try {
      const { key, source, provider, model } = await readAIConfig();
      return { configured: Boolean(key), source, provider, model, key_hint: key ? `••••••${key.slice(-4)}` : '', encryption_ready: String(process.env.MIAO_SETTINGS_ENCRYPTION_KEY || '').length >= 32 };
    } catch (error) {
      request.log.error({ err: error }, 'failed to read platform AI configuration');
      return reply.code(503).send({ error: 'AI 服务配置暂时不可用' });
    }
  });

  app.put('/api/admin/ai', platformOnly(), async (request, reply) => {
    const key = String(body(request).api_key || '').trim();
    if (key.length < 16 || key.length > 2000 || /[\r\n]/.test(key)) {
      await writePlatformAudit(request, { action: 'ai_key.rotation.rejected', targetType: 'setting', targetId: 'ai_gateway_api_key', status: 400 });
      return reply.code(400).send({ error: 'AI 服务密钥格式无效' });
    }
    const audit = await writePlatformAudit(request, { action: 'ai_key.rotation.requested', targetType: 'setting', targetId: 'ai_gateway_api_key', status: 102 });
    if (!audit) return reply.code(503).send({ error: '平台审计服务暂不可用，未修改 AI 配置' });
    try {
      await writeAIKey(key, request.user.id);
      await finishPlatformAudit(request, audit, { action: 'ai_key.rotated', status: 200 });
      return { ok: true, configured: true };
    } catch (error) {
      request.log.error({ err: error }, 'failed to update platform AI key');
      await finishPlatformAudit(request, audit, { action: 'ai_key.rotation.failed', status: 503 });
      return reply.code(503).send({ error: '请先配置 MIAO_SETTINGS_ENCRYPTION_KEY（至少 32 个字符）' });
    }
  });

  app.delete('/api/admin/ai', platformOnly(), async (request, reply) => {
    const audit = await writePlatformAudit(request, { action: 'ai_key.environment_selection.requested', targetType: 'setting', targetId: 'ai_gateway_api_key', status: 102 });
    if (!audit) return reply.code(503).send({ error: '平台审计服务暂不可用，未修改 AI 配置' });
    try {
      await useEnvironmentAIKey();
      const source = process.env.AI_GATEWAY_API_KEY ? 'environment' : 'none';
      await finishPlatformAudit(request, audit, { action: 'ai_key.environment_selected', status: 200 });
      return { ok: true, source };
    } catch (error) {
      request.log.error({ err: error }, 'failed to switch to environment AI key');
      await finishPlatformAudit(request, audit, { action: 'ai_key.environment_selection.failed', status: 503 });
      return reply.code(503).send({ error: '无法切换到服务器环境中的 AI 密钥' });
    }
  });
};
