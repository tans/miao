import { canManageApp, canPublishApp, resolveAppAccess } from './apps.js';

const locks = new Map();
const withAppLock = async (key, run) => {
  const previous = locks.get(key) || Promise.resolve();
  let release;
  const next = new Promise((resolve) => { release = resolve; });
  locks.set(key, next);
  await previous;
  try { return await run(); }
  finally { release(); if (locks.get(key) === next) locks.delete(key); }
};

const recordData = (row) => ({
  id: row.id, created_at: row.created, updated_at: row.updated,
  data: Object.fromEntries(Object.entries(row).filter(([key]) => !['id', 'collectionId', 'collectionName', 'created', 'updated', 'app_id', 'tenant_id'].includes(key)))
});

const safeId = (value) => /^[a-z][a-z0-9_]{0,31}$/.test(value);
const views = new Set(['list', 'board', 'form']);
const filterOps = new Set(['eq', 'contains', 'before', 'after', 'empty', 'this_week']);

const inspectDefinition = async (definition, appRecord, tenantId, pocketbase) => {
  if (!definition || !Array.isArray(definition.pages) || definition.pages.length < 1 || definition.pages.length > 12) throw new Error('界面需要 1 到 12 个页面');
  const tables = await pocketbase.collection('app_collections').getFullList({
    filter: pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId}', { tenantId, appId: appRecord.id })
  });
  const tableMap = new Map(tables.map((item) => [item.slug, item]));
  const ids = new Set();
  const pages = [];
  for (const item of definition.pages) {
    const id = String(item.id || '');
    const table = tableMap.get(item.table);
    const view = String(item.view || 'list');
    if (!safeId(id) || ids.has(id) || !table || !views.has(view)) throw new Error(`页面「${id}」的标识、数据表或视图无效`);
    ids.add(id);
    const available = new Map((table.fields || []).map((field) => [field.name, field]));
    const fields = Array.isArray(item.fields) ? item.fields : [];
    if (!fields.length || fields.length > 24 || fields.some((field) => !available.has(field)) || new Set(fields).size !== fields.length) throw new Error(`页面「${id}」的字段无效`);
    let filter = null;
    if (item.filter) {
      const { field, op, value } = item.filter;
      const target = available.get(field);
      if (!target || !filterOps.has(op) || (op === 'this_week' && target.type !== 'date') || (['before', 'after'].includes(op) && target.type !== 'date') || (['eq', 'contains', 'before', 'after'].includes(op) && typeof value !== 'string')) throw new Error(`页面「${id}」的筛选条件无效`);
      const timezone = String(item.filter.timezone || 'Asia/Shanghai');
      try { new Intl.DateTimeFormat('en-US', { timeZone: timezone }); } catch { throw new Error(`页面「${id}」的时区无效`); }
      filter = { field, op, timezone, ...(value !== undefined ? { value: String(value).slice(0, 200) } : {}) };
    }
    const groupBy = view === 'board' && available.get(item.group_by)?.type === 'select' ? item.group_by : '';
    if (view === 'board' && !groupBy) throw new Error(`页面「${id}」需要选项类型的分组字段`);
    pages.push({ id, title: String(item.title || table.name).trim().slice(0, 80), table: table.slug, view, fields, ...(filter ? { filter } : {}), ...(groupBy ? { group_by: groupBy } : {}) });
  }
  return { schema_version: 1, pages };
};

const accessibleVersion = async (request, reply, pocketbase, appRecord, id) => {
  if (!id) return null;
  const row = await pocketbase.collection('app_versions').getOne(id).catch(() => null);
  if (!row || row.tenant_id !== request.tenant.id || row.app_id !== appRecord.id) {
    reply.code(404).send({ error: '版本不存在' });
    return null;
  }
  return row;
};

const versionSummary = (row) => ({ id: row.id, base_version_id: row.base_version_id || '', summary: row.summary, created_by: row.created_by, created_at: row.created, published_at: row.published_at || '' });

const zonedMidnight = (date, timezone) => {
  const target = Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate());
  let guess = target;
  for (let attempt = 0; attempt < 2; attempt++) {
    const parts = Object.fromEntries(new Intl.DateTimeFormat('en-US', { timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23' }).formatToParts(new Date(guess)).map((part) => [part.type, part.value]));
    const shown = Date.UTC(Number(parts.year), Number(parts.month) - 1, Number(parts.day), Number(parts.hour), Number(parts.minute), Number(parts.second));
    guess += target - shown;
  }
  return new Date(guess);
};
const weekBounds = (timezone) => {
  const parts = Object.fromEntries(new Intl.DateTimeFormat('en-US', { timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(new Date()).map((part) => [part.type, part.value]));
  const today = new Date(Date.UTC(Number(parts.year), Number(parts.month) - 1, Number(parts.day)));
  const monday = new Date(today.getTime() - ((today.getUTCDay() + 6) % 7) * 86400000);
  return [zonedMidnight(monday, timezone), zonedMidnight(new Date(monday.getTime() + 7 * 86400000), timezone)];
};

export const registerRuntimeRoutes = (app, { auth, pocketbase }) => {
  const access = (request, reply) => resolveAppAccess(request, reply, pocketbase);

  app.get('/api/apps/:id/runtime', { preHandler: auth }, async (request, reply) => {
    const record = await access(request, reply);
    if (!record) return;
    if (!record.published_version_id) return { status: 'unpublished', definition: null };
    const version = await accessibleVersion(request, reply, pocketbase, record, record.published_version_id);
    if (!version) return;
    return { status: 'published', version: versionSummary(version), definition: version.definition };
  });

  app.get('/api/apps/:id/versions', { preHandler: auth }, async (request, reply) => {
    const record = await access(request, reply);
    if (!record) return;
    if (!canManageApp(request)) return reply.code(403).send({ error: '你没有管理应用的权限' });
    const rows = await pocketbase.collection('app_versions').getList(1, 30, {
      filter: pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId}', { tenantId: request.tenant.id, appId: record.id }), sort: '-created'
    });
    return { published_version_id: record.published_version_id || '', draft_version_id: record.draft_version_id || '', items: rows.items.map(versionSummary) };
  });

  app.post('/api/apps/:id/versions', { preHandler: auth }, async (request, reply) => {
    const record = await access(request, reply);
    if (!record) return;
    if (!canManageApp(request)) return reply.code(403).send({ error: '你没有管理应用的权限' });
    let definition;
    try { definition = await inspectDefinition(request.body?.definition, record, request.tenant.id, pocketbase); }
    catch (error) { return reply.code(400).send({ error: error.message }); }
    const current = await pocketbase.collection('apps').getOne(record.id);
    const base = String(request.body?.base_version_id || '');
    if (base !== (current.published_version_id || '')) return reply.code(409).send({ error: '正式版已变化，请重新生成草稿', code: 'VERSION_CONFLICT' });
    const version = await pocketbase.collection('app_versions').create({
      tenant_id: request.tenant.id, app_id: record.id, base_version_id: base, definition,
      summary: String(request.body?.summary || '更新应用界面').trim().slice(0, 2000), created_by: request.user.id
    });
    await pocketbase.collection('apps').update(record.id, { draft_version_id: version.id });
    return reply.code(201).send({ version: versionSummary(version), definition });
  });

  app.get('/api/apps/:id/versions/:versionId/preview', { preHandler: auth }, async (request, reply) => {
    const record = await access(request, reply);
    if (!record) return;
    if (!canManageApp(request)) return reply.code(403).send({ error: '你没有管理应用的权限' });
    if (record.draft_version_id !== request.params.versionId) return reply.code(409).send({ error: '草稿已变化，请刷新预览' });
    const version = await accessibleVersion(request, reply, pocketbase, record, request.params.versionId);
    if (!version) return;
    return { status: 'preview', version: versionSummary(version), definition: version.definition };
  });

  app.post('/api/apps/:id/versions/:versionId/publish', { preHandler: auth }, async (request, reply) => {
    const record = await access(request, reply);
    if (!record) return;
    if (!canPublishApp(request)) return reply.code(403).send({ error: '你没有发布权限' });
    if (request.body?.confirm !== true || request.body?.version_id !== request.params.versionId) return reply.code(400).send({ error: '请确认当前草稿版本及改动' });
    return withAppLock(record.id, async () => {
      const current = await pocketbase.collection('apps').getOne(record.id);
      if (current.draft_version_id !== request.params.versionId) return reply.code(409).send({ error: '草稿已变化', code: 'DRAFT_CHANGED' });
      const version = await accessibleVersion(request, reply, pocketbase, current, request.params.versionId);
      if (!version) return;
      if ((version.base_version_id || '') !== (current.published_version_id || '')) return reply.code(409).send({ error: '正式版已变化，请重新生成差异', code: 'VERSION_CONFLICT' });
      try { await inspectDefinition(version.definition, current, request.tenant.id, pocketbase); }
      catch (error) { return reply.code(409).send({ error: error.message, code: 'SCHEMA_CHANGED' }); }
      await pocketbase.collection('app_versions').update(version.id, { published_at: new Date().toISOString() });
      await pocketbase.collection('apps').update(current.id, { published_version_id: version.id, draft_version_id: '' });
      return { status: 'published', version: versionSummary(version) };
    });
  });

  app.post('/api/apps/:id/versions/:versionId/restore', { preHandler: auth }, async (request, reply) => {
    const record = await access(request, reply);
    if (!record) return;
    if (!canPublishApp(request)) return reply.code(403).send({ error: '你没有发布权限' });
    if (request.body?.confirm !== true) return reply.code(400).send({ error: '请确认恢复此界面版本' });
    return withAppLock(record.id, async () => {
      const current = await pocketbase.collection('apps').getOne(record.id);
      const version = await accessibleVersion(request, reply, pocketbase, current, request.params.versionId);
      if (!version || !version.published_at) return reply.code(400).send({ error: '只能恢复已发布的界面版本' });
      try { await inspectDefinition(version.definition, current, request.tenant.id, pocketbase); }
      catch (error) { return reply.code(409).send({ error: error.message, code: 'SCHEMA_CHANGED' }); }
      await pocketbase.collection('apps').update(current.id, { published_version_id: version.id, draft_version_id: '' });
      return { status: 'published', version: versionSummary(version) };
    });
  });

  app.get('/api/apps/:id/runtime/pages/:pageId/records', { preHandler: auth }, async (request, reply) => {
    const record = await access(request, reply);
    if (!record) return;
    const preview = request.query?.preview === 'true';
    if (preview && !canManageApp(request)) return reply.code(403).send({ error: '没有预览权限' });
    const versionId = preview ? record.draft_version_id : record.published_version_id;
    if (!versionId) return reply.code(404).send({ error: '应用尚未发布' });
    const version = await accessibleVersion(request, reply, pocketbase, record, versionId);
    if (!version) return;
    const page = version.definition?.pages?.find((item) => item.id === request.params.pageId);
    if (!page) return reply.code(404).send({ error: '页面不存在' });
    const table = await pocketbase.collection('app_collections').getFirstListItem(pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && slug = {:slug}', { tenantId: request.tenant.id, appId: record.id, slug: page.table })).catch(() => null);
    if (!table) return reply.code(409).send({ error: '页面绑定的数据表已变化' });
    const params = { tenantId: request.tenant.id, appId: record.id };
    const parts = ['tenant_id = {:tenantId}', 'app_id = {:appId}'];
    if (page.filter) {
      const { field, op, value } = page.filter;
      if (!(table.fields || []).some((item) => item.name === field)) return reply.code(409).send({ error: '筛选字段已变化' });
      if (op === 'this_week') { const [start, end] = weekBounds(page.filter.timezone || 'Asia/Shanghai'); params.start = start.toISOString(); params.end = end.toISOString(); parts.push(`${field} >= {:start} && ${field} < {:end}`); }
      else if (op === 'empty') parts.push(`${field} = ""`);
      else { params.value = value; parts.push(`${field} ${op === 'contains' ? '~' : op === 'before' ? '<' : op === 'after' ? '>' : '='} {:value}`); }
    }
    const search = String(request.query?.search || '').trim().slice(0, 120);
    if (search) {
      const fields = (table.fields || []).filter((item) => page.fields.includes(item.name) && ['text', 'email', 'url'].includes(item.type));
      if (fields.length) { params.search = search; parts.push(`(${fields.map((item) => `${item.name} ~ {:search}`).join(' || ')})`); }
    }
    const pageNumber = Math.max(1, Math.min(100000, Number.parseInt(request.query?.page, 10) || 1));
    const result = await pocketbase.collection(table.pb_collection).getList(pageNumber, 25, { filter: pocketbase.filter(parts.join(' && '), params), sort: '-created' });
    return { items: result.items.map(recordData), totalItems: result.totalItems, page: result.page, totalPages: result.totalPages, fields: table.fields.filter((item) => page.fields.includes(item.name)) };
  });
};
