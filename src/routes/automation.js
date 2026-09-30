import { appPermission } from '../business/access.js';
import { canPublishApp, resolveAppAccess } from './apps.js';
import { updateBusinessRecord } from '../business/records.js';
import { taskAuthority } from '../business/access.js';

const localDate = (timezone, date = new Date()) => new Intl.DateTimeFormat('en-CA', { timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(date);

const deliver = async (pocketbase, rule, key, message, sourceRecord) => {
  const definition = rule.definition;
  const member = definition.action?.type === 'set_field' ? true : await pocketbase.collection('tenant_members').getFirstListItem(
    pocketbase.filter('tenant_id = {:tenantId} && user_id = {:userId}', { tenantId: rule.tenant_id, userId: definition.recipient_id })
  ).catch(() => null);
  const creator = await pocketbase.collection('tenant_members').getFirstListItem(
    pocketbase.filter('tenant_id = {:tenantId} && user_id = {:userId}', { tenantId: rule.tenant_id, userId: rule.created_by })
  ).catch(() => null);
  const application = await pocketbase.collection('apps').getOne(rule.app_id).catch(() => null);
  if (!member || !creator || !application || application.archived) {
    await pocketbase.collection('automation_rules').update(rule.id, { enabled: false, pause_reason: !creator ? '规则创建者已离开工作区' : !member ? '接收人已离开工作区' : '应用已归档或删除' });
    return;
  }
  if (creator.role !== 'owner') {
    const creatorAccess = await pocketbase.collection('app_members').getFirstListItem(pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && user_id = {:userId}', { tenantId: rule.tenant_id, appId: rule.app_id, userId: rule.created_by })).catch(() => null);
    if (!['publisher'].includes(creatorAccess?.role)) { await pocketbase.collection('automation_rules').update(rule.id, { enabled: false, pause_reason: '创建者已失去规则管理权限' }); return; }
  }
  if (definition.action?.type !== 'set_field' && application.restricted && member.role !== 'owner') {
    const access = await pocketbase.collection('app_members').getFirstListItem(
      pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && user_id = {:userId}', { tenantId: rule.tenant_id, appId: rule.app_id, userId: definition.recipient_id })
    ).catch(() => null);
    if (!access) { await pocketbase.collection('automation_rules').update(rule.id, { enabled: false, pause_reason: '接收人已失去应用权限' }); return; }
  }
  const existing = await pocketbase.collection('automation_runs').getFirstListItem(
    pocketbase.filter('rule_id = {:ruleId} && event_key = {:key}', { ruleId: rule.id, key })
  ).catch(() => null);
  if (existing?.status === 'delivered') return;
  const run = existing || await pocketbase.collection('automation_runs').create({ tenant_id: rule.tenant_id, app_id: rule.app_id, rule_id: rule.id, event_key: key, status: 'pending' }).catch(() => null);
  if (!run) return;
  if (definition.action?.type === 'set_field') {
    const metadata = await pocketbase.collection('app_collections').getFirstListItem(pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && slug = {:slug}', { tenantId: rule.tenant_id, appId: rule.app_id, slug: definition.table }));
    const row = await pocketbase.collection(metadata.pb_collection).getOne(sourceRecord.id);
    if (row.tenant_id !== rule.tenant_id || row.app_id !== rule.app_id) return;
    await updateBusinessRecord({ pocketbase, table: metadata, tenantId: rule.tenant_id, appId: rule.app_id, recordId: row.id, data: { [definition.action.field]: definition.action.value }, expectedUpdated: row.updated, eventSource: 'background', authorize: () => taskAuthority(pocketbase, rule) });
    await pocketbase.collection('automation_runs').update(run.id, { status: 'delivered', result: { record_id: row.id, action: 'set_field' } });
    return;
  }
  try { await pocketbase.collection('automation_notifications').create({ tenant_id: rule.tenant_id, app_id: rule.app_id, rule_id: rule.id, event_key: key, user_id: definition.recipient_id, message }); }
  catch (error) {
    const duplicate = await pocketbase.collection('automation_notifications').getFirstListItem(pocketbase.filter('rule_id = {:ruleId} && event_key = {:key} && user_id = {:userId}', { ruleId: rule.id, key, userId: definition.recipient_id })).catch(() => null);
    if (!duplicate) throw error;
  }
  await pocketbase.collection('automation_runs').update(run.id, { status: 'delivered', result: { recipient_id: definition.recipient_id } });
};

export const processRecordAutomation = async (pocketbase, { tenantId, appId, table, event, before, after }) => {
  const rules = await pocketbase.collection('automation_rules').getFullList({
    filter: pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && enabled = true', { tenantId, appId })
  });
  for (const rule of rules) {
    const definition = rule.definition;
    if (definition.table !== table) continue;
    const key = `${event}:${after.id}:${after.updated}`.slice(0, 160);
    if (definition.trigger === 'record_created' && event === 'created') await deliver(pocketbase, rule, key, `${rule.name}：新增了一条记录`, after);
    if (definition.trigger === 'status_changed' && event === 'updated' && before?.[definition.field] === definition.from && after[definition.field] === definition.to) await deliver(pocketbase, rule, key, `${rule.name}：记录状态已更新`, after);
  }
};

export const scanDueAutomation = async (pocketbase) => {
  const rules = await pocketbase.collection('automation_rules').getFullList({ filter: 'enabled = true' });
  for (const rule of rules.filter((item) => item.definition?.trigger === 'due')) {
    try {
      const { table, field, timezone = 'Asia/Shanghai', offset_days: offset = 0 } = rule.definition;
      const metadata = await pocketbase.collection('app_collections').getFirstListItem(pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && slug = {:slug}', { tenantId: rule.tenant_id, appId: rule.app_id, slug: table }));
      const today = localDate(timezone);
      const limitDate = new Date(`${today}T00:00:00Z`);
      limitDate.setUTCDate(limitDate.getUTCDate() + offset + 1);
      const endExclusive = limitDate.toISOString().slice(0, 10);
      let page = 1;
      while (page <= 100) {
        const result = await pocketbase.collection(metadata.pb_collection).getList(page, 100, {
          filter: pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && ' + field + ' >= {:today} && ' + field + ' < {:end}', { tenantId: rule.tenant_id, appId: rule.app_id, today, end: endExclusive }), sort: field
        });
        for (const row of result.items) await deliver(pocketbase, rule, `due:${row.id}:${String(row[field]).slice(0, 10)}`, `${rule.name}：有一项任务即将到期`, row);
        if (page >= result.totalPages) break;
        page++;
      }
    } catch (error) {
      await pocketbase.collection('automation_rules').update(rule.id, { enabled: false, pause_reason: '目标字段或数据表已失效' }).catch(() => {});
      console.error('automation scan failed', rule.id, error);
    }
  }
};

export const registerAutomationRoutes = (app, { auth, pocketbase }) => {
  const ownRule = async (request, reply) => {
    const record = await resolveAppAccess(request, reply, pocketbase);
    if (!record) return null;
    const rule = await pocketbase.collection('automation_rules').getOne(request.params.ruleId).catch(() => null);
    if (!rule || rule.tenant_id !== request.tenant.id || rule.app_id !== record.id) { reply.code(404).send({ error: '规则不存在' }); return null; }
    return rule;
  };
  app.get('/api/apps/:id/automations', { preHandler: auth }, async (request, reply) => {
    const record = await resolveAppAccess(request, reply, pocketbase);
    if (!record) return;
    const rules = await pocketbase.collection('automation_rules').getFullList({ filter: pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId}', { tenantId: request.tenant.id, appId: record.id }), sort: '-created' });
    return rules.map((item) => ({ id: item.id, name: item.name, definition: item.definition, enabled: item.enabled, pause_reason: item.pause_reason || '' }));
  });
  app.post('/api/apps/:id/automations', { preHandler: auth }, async (request, reply) => {
    const record = await resolveAppAccess(request, reply, pocketbase);
    if (!record) return;
    if (!canPublishApp(request)) return reply.code(403).send({ error: '没有配置自动化的权限' });
    const definition = request.body?.definition || {};
    const action = definition.action || { type: 'notify' };
    if (!['record_created', 'status_changed', 'due'].includes(definition.trigger) || !['notify', 'set_field'].includes(action.type) || (action.type === 'notify' && !definition.recipient_id) || (action.type === 'set_field' && definition.trigger !== 'record_created')) return reply.code(400).send({ error: '触发条件或动作无效' });
    const table = await pocketbase.collection('app_collections').getFirstListItem(pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && slug = {:slug}', { tenantId: request.tenant.id, appId: record.id, slug: definition.table })).catch(() => null);
    if (!table) return reply.code(400).send({ error: '目标表不存在' });
    if (action.type === 'notify') {
      const member = await pocketbase.collection('tenant_members').getFirstListItem(pocketbase.filter('tenant_id = {:tenantId} && user_id = {:userId}', { tenantId: request.tenant.id, userId: definition.recipient_id })).catch(() => null);
      if (!member) return reply.code(400).send({ error: '接收人不是工作区成员' });
    }
    const field = (table.fields || []).find((item) => item.name === definition.field);
    if (definition.trigger === 'status_changed' && (!field || field.type !== 'select' || !field.options?.includes(definition.from) || !field.options?.includes(definition.to))) return reply.code(400).send({ error: '状态条件无效' });
    if (definition.trigger === 'due' && (!field || field.type !== 'date' || !Number.isInteger(definition.offset_days) || definition.offset_days < 0 || definition.offset_days > 30)) return reply.code(400).send({ error: '到期条件无效' });
    const timezone = String(definition.timezone || 'Asia/Shanghai');
    try { localDate(timezone); } catch { return reply.code(400).send({ error: '时区无效' }); }
    let safeAction = { type: 'notify' };
    if (action.type === 'set_field') {
      const target = (table.fields || []).find((item) => item.name === action.field);
      const valid = target && (target.type === 'bool' ? typeof action.value === 'boolean' : target.type === 'number' ? typeof action.value === 'number' && Number.isFinite(action.value) : ['text', 'select', 'email', 'url', 'date'].includes(target.type) && typeof action.value === 'string' && (target.type !== 'select' || target.options?.includes(action.value)));
      if (!valid) return reply.code(400).send({ error: '表单提交后的动作字段或值无效' });
      safeAction = { type: 'set_field', field: target.name, value: action.value };
    }
    const safe = { trigger: definition.trigger, table: table.slug, ...(action.type === 'notify' ? { recipient_id: definition.recipient_id } : {}), action: safeAction, ...(field ? { field: field.name } : {}), ...(definition.trigger === 'status_changed' ? { from: definition.from, to: definition.to } : {}), ...(definition.trigger === 'due' ? { offset_days: definition.offset_days, timezone } : {}) };
    const rule = await pocketbase.collection('automation_rules').create({ tenant_id: request.tenant.id, app_id: record.id, created_by: request.user.id, name: String(request.body?.name || '提醒').slice(0, 160), definition: safe, enabled: false });
    return reply.code(201).send({ id: rule.id, name: rule.name, definition: safe, enabled: false });
  });
  app.post('/api/apps/:id/automations/:ruleId/enable', { preHandler: auth }, async (request, reply) => {
    const rule = await ownRule(request, reply);
    if (!rule) return;
    if (!canPublishApp(request) || request.body?.confirm !== true) return reply.code(403).send({ error: '需要有权限的用户确认启用' });
    const updated = await pocketbase.collection('automation_rules').update(rule.id, { enabled: Boolean(request.body?.enabled), pause_reason: '' });
    return { id: updated.id, enabled: updated.enabled };
  });
  app.get('/api/notifications', { preHandler: auth }, async (request) => {
    const result = await pocketbase.collection('automation_notifications').getList(Math.max(1, Math.min(10000, Number.parseInt(request.query.page, 10) || 1)), 30, { filter: pocketbase.filter('tenant_id = {:tenantId} && user_id = {:userId}', { tenantId: request.tenant.id, userId: request.user.id }), sort: '-created' });
    const visible = [];
    for (const item of result.items) {
      const application = await pocketbase.collection('apps').getOne(item.app_id).catch((error) => { if (error.status === 404) return null; throw error; });
      if (!application || application.tenant_id !== request.tenant.id) continue;
      if (!await appPermission(pocketbase, { app: application, tenant: request.tenant, user: request.user, membership: request.membership })) continue;
      visible.push({ id: item.id, app_id: item.app_id, run_id: item.run_id || '', message: item.message, read: Boolean(item.read), created_at: item.created });
    }
    return request.query.page === undefined ? visible : { ...result, items: visible };
  });
  app.post('/api/notifications/:notificationId/read', { preHandler: auth }, async (request, reply) => {
    const item = await pocketbase.collection('automation_notifications').getOne(request.params.notificationId).catch((error) => { if (error.status === 404) return null; throw error; });
    if (!item || item.tenant_id !== request.tenant.id || item.user_id !== request.user.id) return reply.code(404).send({ error: '通知不存在' });
    const application = await pocketbase.collection('apps').getOne(item.app_id).catch((error) => { if (error.status === 404) return null; throw error; });
    if (!application || application.tenant_id !== request.tenant.id || !await appPermission(pocketbase, { app: application, tenant: request.tenant, user: request.user, membership: request.membership })) return reply.code(404).send({ error: '通知不存在或权限已变化' });
    await pocketbase.collection('automation_notifications').update(item.id, { read: true });
    return { id: item.id, read: true };
  });
};
