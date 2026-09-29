import { validateAppUiDefinition } from '../app-ui.js';

const appLocks = new Map();
const withAppLock = async (appId, operation) => {
  const previous = appLocks.get(appId) || Promise.resolve();
  let release;
  const current = new Promise((resolve) => { release = resolve; });
  appLocks.set(appId, current);
  await previous;
  try { return await operation(); }
  finally {
    release();
    if (appLocks.get(appId) === current) appLocks.delete(appId);
  }
};

const publicVersion = (record, publishedVersionId = '') => ({
  id: record.id,
  version: record.version,
  summary: record.summary || '',
  status: record.id === publishedVersionId ? 'published' : record.published_at ? 'superseded' : 'draft',
  based_on_version_id: record.based_on_version_id || null,
  created_at: record.created,
  updated_at: record.updated,
  published_at: record.published_at || null,
});

const runtimeFormFieldTypes = new Set(['text', 'number', 'bool', 'date', 'email', 'url', 'select']);

const appTables = (pocketbase, appRecord, tenantId) => pocketbase.collection('app_collections').getFullList({
  filter: pocketbase.filter('app_id = {:appId} && tenant_id = {:tenantId}', { appId: appRecord.id, tenantId })
});

const latestAppVersion = async (pocketbase, appId, tenantId) => {
  const result = await pocketbase.collection('app_versions').getList(1, 1, {
    filter: pocketbase.filter('app_id = {:appId} && tenant_id = {:tenantId}', { appId, tenantId }),
    sort: '-version'
  });
  return result.items[0] || null;
};

const saveForwardVersion = async ({ pocketbase, request, reply, appRecord, tenantId, definition, summary, basedOnVersion }) => {
  const latest = await latestAppVersion(pocketbase, appRecord.id, tenantId);
  const nextVersionNumber = (latest?.version || 0) + 1;
  try {
    const version = await pocketbase.collection('app_versions').create({
      tenant_id: tenantId,
      app_id: appRecord.id,
      version: nextVersionNumber,
      definition,
      summary: String(summary || '').trim().slice(0, 1000),
      based_on_version_id: basedOnVersion?.id || '',
      created_by: request.user.id,
    });
    return reply.code(201).send({ ...publicVersion(version, appRecord.published_version_id), definition });
  } catch (error) {
    const latestAfterFailure = await latestAppVersion(pocketbase, appRecord.id, tenantId).catch(() => null);
    if (error?.status === 409 || (latestAfterFailure && Number(latestAfterFailure.version || 0) >= nextVersionNumber)) {
      return reply.code(409).send({ error: '版本序号刚发生变化，请刷新版本列表后重试；原草稿和已发布界面未更改' });
    }
    request.log.error({ err: error, app_id: appRecord.id }, 'failed to save app UI draft');
    return reply.code(503).send({ error: '草稿保存暂时失败；原草稿和已发布界面未更改，请重试' });
  }
};

const publishedRuntime = async ({ pocketbase, appRecord, tenantId, query = {} }) => {
  const versionId = appRecord.published_version_id;
  if (!versionId) return { status: 'not_published' };
  const version = await pocketbase.collection('app_versions').getOne(versionId).catch(() => null);
  if (!version || version.tenant_id !== tenantId || version.app_id !== appRecord.id) return { status: 'unavailable' };
  const tables = await appTables(pocketbase, appRecord, tenantId);
  const validated = validateAppUiDefinition(version.definition, tables);
  if (validated.error) return { status: 'unavailable' };

  const page = Math.max(1, Math.min(1_000_000, Number.parseInt(query.page, 10) || 1));
  const perPage = Math.max(1, Math.min(50, Number.parseInt(query.perPage, 10) || 25));
  const selectedFields = validated.fields;
  const formFields = selectedFields.flatMap((field) => {
    if (!runtimeFormFieldTypes.has(field.type)) return [];
    const source = (validated.table.fields || []).find((candidate) => candidate.name === field.name);
    if (!source) return [];
    return [{
      ...field,
      required: Boolean(source.required),
      ...(field.type === 'select' ? { options: Array.isArray(source.options) ? source.options : [] } : {}),
    }];
  });
  const formFieldNames = new Set(formFields.map((field) => field.name));
  const requiredFields = (validated.table.fields || []).filter((field) => field.required);
  const createFormAvailable = formFields.length > 0 && requiredFields.every((field) => formFieldNames.has(field.name));
  const searchable = selectedFields.filter((field) => ['text', 'email', 'url'].includes(field.type));
  const params = { appId: appRecord.id, tenantId };
  const filters = ['app_id = {:appId}', 'tenant_id = {:tenantId}'];
  const search = String(query.search || '').trim().slice(0, 120);
  if (search && searchable.length) {
    const alternatives = searchable.map((field, index) => {
      const key = `runtimeSearch${index}`;
      params[key] = search;
      return `${field.name} ~ {:${key}}`;
    });
    filters.push(`(${alternatives.join(' || ')})`);
  }
  const result = await pocketbase.collection(validated.table.pb_collection).getList(page, perPage, {
    filter: pocketbase.filter(filters.join(' && '), params), sort: '-created'
  });
  return {
    status: 'published',
    version: publicVersion(version, versionId),
    title: validated.definition.title,
    collection: validated.table.slug,
    fields: selectedFields,
    create_form_available: createFormAvailable,
    create_form_fields: createFormAvailable ? formFields : [],
    search_supported: searchable.length > 0,
    page: result.page,
    per_page: result.perPage,
    total_items: result.totalItems,
    total_pages: result.totalPages,
    items: result.items.map((record) => ({
      id: record.id,
      data: Object.fromEntries(selectedFields.map(({ name }) => [name, record[name] ?? null])),
      created_at: record.created,
      updated_at: record.updated,
    })),
  };
};

const previewUiVersion = async ({ pocketbase, appRecord, tenantId, version }) => {
  const tables = await appTables(pocketbase, appRecord, tenantId);
  const validated = validateAppUiDefinition(version.definition, tables);
  if (validated.error) return { error: `此版本当前无法预览：${validated.error}` };

  const result = await pocketbase.collection(validated.table.pb_collection).getList(1, 5, {
    filter: pocketbase.filter('app_id = {:appId} && tenant_id = {:tenantId}', { appId: appRecord.id, tenantId }),
    sort: '-created'
  });
  return {
    status: 'preview',
    version: publicVersion(version, appRecord.published_version_id),
    title: validated.definition.title,
    collection: validated.table.slug,
    fields: validated.fields,
    total_items: result.totalItems,
    items: result.items.map((record) => ({
      data: Object.fromEntries(validated.fields.map(({ name }) => [name, record[name] ?? null])),
    })),
  };
};

export const registerAppVersionRoutes = (app, { auth, body, pocketbase, getApp, requireAppEditor }) => {
  app.get('/api/apps/:id/runtime', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    return publishedRuntime({ pocketbase, appRecord, tenantId: request.tenant.id, query: request.query });
  });

  app.get('/api/apps/:id/versions', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    const page = Math.max(1, Math.min(1_000_000, Number.parseInt(request.query?.page, 10) || 1));
    const result = await pocketbase.collection('app_versions').getList(page, 50, {
      filter: pocketbase.filter('app_id = {:appId} && tenant_id = {:tenantId}', { appId: appRecord.id, tenantId: request.tenant.id }),
      sort: '-version'
    });
    return { published_version_id: appRecord.published_version_id || null, page: result.page, per_page: result.perPage, total_items: result.totalItems, items: result.items.map((item) => publicVersion(item, appRecord.published_version_id)) };
  });

  app.get('/api/apps/:id/versions/:versionId', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    const version = await pocketbase.collection('app_versions').getOne(request.params.versionId).catch(() => null);
    if (!version || version.app_id !== appRecord.id || version.tenant_id !== request.tenant.id) return reply.code(404).send({ error: '应用界面版本不存在' });
    return { ...publicVersion(version, appRecord.published_version_id), definition: version.definition };
  });

  app.get('/api/apps/:id/versions/:versionId/preview', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    const version = await pocketbase.collection('app_versions').getOne(request.params.versionId).catch(() => null);
    if (!version || version.app_id !== appRecord.id || version.tenant_id !== request.tenant.id) return reply.code(404).send({ error: '应用界面版本不存在' });
    const preview = await previewUiVersion({ pocketbase, appRecord, tenantId: request.tenant.id, version });
    if (preview.error) return reply.code(409).send({ error: preview.error });
    return preview;
  });

  app.post('/api/apps/:id/versions', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    requireAppEditor(request, reply);
    if (reply.sent) return;
    const { definition, summary, based_on_version_id: basedOnVersionId = null } = body(request);
    return withAppLock(appRecord.id, async () => {
      let basedOnVersion = null;
      if (basedOnVersionId !== null) {
        if (typeof basedOnVersionId !== 'string' || !basedOnVersionId.trim()) return reply.code(400).send({ error: '修订来源版本无效' });
        basedOnVersion = await pocketbase.collection('app_versions').getOne(basedOnVersionId).catch(() => null);
        if (!basedOnVersion || basedOnVersion.app_id !== appRecord.id || basedOnVersion.tenant_id !== request.tenant.id) {
          return reply.code(404).send({ error: '修订来源版本不存在' });
        }
        if (basedOnVersion.published_at || basedOnVersion.id === appRecord.published_version_id) {
          return reply.code(409).send({ error: '只能从尚未发布的草稿创建修订。请刷新版本列表后选择草稿。' });
        }
      }
      const tables = await appTables(pocketbase, appRecord, request.tenant.id);
      const validated = validateAppUiDefinition(definition, tables);
      if (validated.error) return reply.code(400).send({ error: validated.error });
      return saveForwardVersion({
        pocketbase,
        request,
        reply,
        appRecord,
        tenantId: request.tenant.id,
        definition: validated.definition,
        summary,
        basedOnVersion,
      });
    });
  });

  app.post('/api/apps/:id/versions/:versionId/restore', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    requireAppEditor(request, reply);
    if (reply.sent) return;
    return withAppLock(appRecord.id, async () => {
      const sourceVersion = await pocketbase.collection('app_versions').getOne(request.params.versionId).catch(() => null);
      if (!sourceVersion || sourceVersion.app_id !== appRecord.id || sourceVersion.tenant_id !== request.tenant.id) {
        return reply.code(404).send({ error: '应用界面版本不存在' });
      }
      if (sourceVersion.id === appRecord.published_version_id) {
        return reply.code(409).send({ error: '该版本已经是当前正式界面，无需恢复' });
      }
      if (!sourceVersion.published_at) {
        return reply.code(409).send({ error: '只能从已发布过的历史界面创建恢复草稿；未发布草稿请使用修订流程' });
      }
      if (appRecord.published_version_id) {
        const currentVersion = await pocketbase.collection('app_versions').getOne(appRecord.published_version_id).catch(() => null);
        if (currentVersion && sourceVersion.version >= currentVersion.version) {
          return reply.code(409).send({ error: '目标版本不是早于当前正式界面的历史版本，请刷新版本列表后重试' });
        }
      }

      const tables = await appTables(pocketbase, appRecord, request.tenant.id);
      const validated = validateAppUiDefinition(sourceVersion.definition, tables);
      if (validated.error) {
        return reply.code(409).send({ error: `无法从 v${sourceVersion.version} 创建恢复草稿：${validated.error}。当前正式界面未更改，也没有创建草稿。` });
      }
      return saveForwardVersion({
        pocketbase,
        request,
        reply,
        appRecord,
        tenantId: request.tenant.id,
        definition: validated.definition,
        summary: `恢复自 v${sourceVersion.version}`,
        basedOnVersion: sourceVersion,
      });
    });
  });

  app.post('/api/apps/:id/versions/:versionId/publish', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    requireAppEditor(request, reply);
    if (reply.sent) return;
    const payload = body(request);
    if (!Object.prototype.hasOwnProperty.call(payload, 'expected_published_version_id')) {
      return reply.code(400).send({ error: '发布时必须提供当前已发布版本，用于检测并发变更' });
    }
    return withAppLock(appRecord.id, async () => {
      const latestApp = await pocketbase.collection('apps').getOne(appRecord.id);
      const currentVersionId = latestApp.published_version_id || null;
      if ((payload.expected_published_version_id || null) !== currentVersionId) {
        return reply.code(409).send({ error: '应用已被其他操作发布了新版本，请刷新版本列表后重试', current_version_id: currentVersionId });
      }
      const version = await pocketbase.collection('app_versions').getOne(request.params.versionId).catch(() => null);
      if (!version || version.app_id !== appRecord.id || version.tenant_id !== request.tenant.id) return reply.code(404).send({ error: '草稿版本不存在' });
      if (currentVersionId === version.id) return publishedRuntime({ pocketbase, appRecord: latestApp, tenantId: request.tenant.id });
      if (version.published_at) return reply.code(409).send({ error: '此版本已经发布或已被替代；如需恢复，请先创建一个新版本' });
      if (currentVersionId) {
        const currentVersion = await pocketbase.collection('app_versions').getOne(currentVersionId).catch(() => null);
        if (currentVersion && version.version <= currentVersion.version) return reply.code(409).send({ error: '不能发布早于或等于当前版本的草稿；如需回退，请基于当前版本创建新草稿' });
      }
      const tables = await appTables(pocketbase, appRecord, request.tenant.id);
      const validated = validateAppUiDefinition(version.definition, tables);
      if (validated.error) return reply.code(400).send({ error: `草稿无法发布：${validated.error}` });
      const nextRuntime = await publishedRuntime({ pocketbase, appRecord: { ...latestApp, published_version_id: version.id }, tenantId: request.tenant.id });
      if (nextRuntime.status !== 'published') return reply.code(409).send({ error: '草稿运行检查失败，当前发布版未更改' });
      await pocketbase.collection('apps').update(appRecord.id, { published_version_id: version.id });
      await pocketbase.collection('app_versions').update(version.id, { published_at: new Date().toISOString() }).catch((error) => {
        request.log.error({ err: error, version_id: version.id }, 'failed to record app version publication timestamp');
      });
      return nextRuntime;
    });
  });
};
