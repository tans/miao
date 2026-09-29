import { registerAppVersionRoutes } from './app-versions.js';
import { processRecordAutomation } from './automation.js';

const fieldTypes = new Set(['text', 'number', 'bool', 'date', 'email', 'url', 'select', 'relation', 'file']);
const reservedFields = new Set(['id', 'created', 'updated', 'collectionid', 'collectionname', 'app_id', 'tenant_id']);
const cleanSlug = (value) => String(value || '').toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 20);
const publicApp = ({ id, name, description, archived, restricted, published_version_id, permission, created, updated }) => ({ id, name, description, archived: Boolean(archived), restricted: Boolean(restricted), has_published_version: Boolean(published_version_id), permission, created_at: created, updated_at: updated });
const publicCollection = ({ id, name, slug, fields, created }) => ({ id, name, slug, fields, created_at: created });
const publicRecord = (row) => ({ id: row.id, data: Object.fromEntries(Object.entries(row).filter(([key]) => !['id', 'collectionId', 'collectionName', 'created', 'updated', 'app_id', 'tenant_id'].includes(key))), created_at: row.created, updated_at: row.updated });
const stringFieldTypes = new Set(['text', 'date', 'email', 'url', 'select', 'relation']);

const validateRecordData = (values, fields, { partial = false } = {}) => {
  const fieldByName = new Map((fields || []).map((field) => [field.name, field]));
  const unknownField = Object.keys(values).find((name) => !fieldByName.has(name));
  if (unknownField) return `数据表中没有「${unknownField}」字段`;

  for (const field of fields || []) {
    const present = Object.prototype.hasOwnProperty.call(values, field.name);
    if (!present) {
      if (!partial && field.required) return `字段「${field.label || field.name}」不能为空`;
      continue;
    }

    const value = values[field.name];
    if (field.required && (value === null || value === '')) return `字段「${field.label || field.name}」不能为空`;
    if (value === null) return `字段「${field.label || field.name}」的值类型无效`;

    const validType = stringFieldTypes.has(field.type)
      ? typeof value === 'string'
      : field.type === 'number'
        ? typeof value === 'number' && Number.isFinite(value)
        : field.type === 'bool'
          ? typeof value === 'boolean'
          : field.type === 'file' && value instanceof File;
    if (field.type === 'select' && value !== '' && Array.isArray(field.options) && !field.options.includes(value)) return `字段「${field.label || field.name}」的选项无效`;
    if (!validType) return `字段「${field.label || field.name}」的值类型无效`;
  }

  return null;
};

const addUploadedFiles = (values, files, fields) => {
  files ||= {};
  if (typeof files !== 'object' || Array.isArray(files)) return '附件内容无效';
  for (const [name, upload] of Object.entries(files)) {
    const field = fields.find((item) => item.name === name && item.type === 'file');
    if (!field || !upload || typeof upload.base64 !== 'string') return `字段「${name}」不是文件字段`;
    const base64 = upload.base64.replace(/^data:[^;,]+;base64,/, '');
    const bytes = Buffer.from(base64, 'base64');
    const mime = String(upload.type || 'application/octet-stream');
    if (!bytes.length || bytes.length > 5 * 1024 * 1024) return `附件「${name}」不能超过 5 MB`;
    if (!['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'application/pdf', 'text/plain'].includes(mime)) return `附件「${name}」类型不支持`;
    const filename = String(upload.name || 'attachment').split(/[\\/]/).pop().replace(/[^\p{L}\p{N}._-]/gu, '_').slice(0, 120) || 'attachment';
    values[name] = new File([bytes], filename, { type: mime });
  }
  return null;
};

const validateRelations = async (values, fields, { pocketbase, appId, tenantId }) => {
  for (const field of fields.filter((item) => item.type === 'relation')) {
    const recordId = values[field.name];
    if (!recordId) continue;
    const target = await pocketbase.collection('app_collections').getFirstListItem(
      pocketbase.filter('app_id = {:appId} && tenant_id = {:tenantId} && slug = {:slug}', { appId, tenantId, slug: field.target })
    ).catch(() => null);
    if (!target) return `关联字段「${field.label}」的数据表不存在`;
    const row = await pocketbase.collection(target.pb_collection).getOne(recordId).catch(() => null);
    if (!row || row.app_id !== appId || row.tenant_id !== tenantId) return `关联字段「${field.label}」的记录无效`;
  }
  return null;
};

export const canEditRecords = (request) => request.appPermission !== 'viewer';
export const canManageApp = (request) => ['owner', 'manager', 'publisher'].includes(request.appPermission);
export const canPublishApp = (request) => ['owner', 'publisher'].includes(request.appPermission);

export const resolveAppAccess = async (request, reply, pocketbase) => {
    const record = await pocketbase.collection('apps').getOne(request.params.id).catch(() => null);
    if (!record || record.tenant_id !== request.tenant.id) {
      reply.code(404).send({ error: '应用不存在' });
      return null;
    }
    if (request.membership.role === 'owner') { request.appPermission = 'owner'; request.appCanBatch = true; }
    else {
      const permission = await pocketbase.collection('app_members').getFirstListItem(
        pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && user_id = {:userId}', { tenantId: request.tenant.id, appId: record.id, userId: request.user.id })
      ).catch(() => null);
      if (record.restricted && !permission) {
        reply.code(404).send({ error: '应用不存在或你没有访问权限' });
        return null;
      }
      request.appPermission = permission?.role || 'editor';
      request.appCanBatch = Boolean(permission?.can_batch) && request.appPermission !== 'viewer';
    }
    return record;
};

export const registerAppRoutes = (app, { auth, body, pocketbase }) => {
  const getApp = (request, reply) => resolveAppAccess(request, reply, pocketbase);

  const requireAppEditor = (request, reply) => {
    if (!canEditRecords(request)) return reply.code(403).send({ error: '你只有查看权限，不能修改此应用' });
  };
  const requireAppManager = (request, reply) => {
    if (!canManageApp(request)) return reply.code(403).send({ error: '你没有管理此应用的权限' });
  };
  const requireAppPublisher = (request, reply) => {
    if (!canPublishApp(request)) return reply.code(403).send({ error: '你没有发布权限' });
  };
  const requireWorkspaceOwner = (request, reply) => {
    if (request.membership.role !== 'owner' || request.tenant.owner_id !== request.user.id) return reply.code(403).send({ error: '只有工作区所有者可以调整应用访问权限' });
  };

  const getAppCollection = async (request, reply, appRecord) => {
    const rows = await pocketbase.collection('app_collections').getFullList({
      filter: pocketbase.filter('app_id = {:appId} && tenant_id = {:tenantId} && slug = {:slug}', {
        appId: appRecord.id, tenantId: request.tenant.id, slug: request.params.slug
      })
    });
    if (!rows.length) {
      reply.code(404).send({ error: '数据表不存在' });
      return null;
    }
    return rows[0];
  };

  const publishedUiUses = async (appRecord, tenantId, slug, removedFields = null) => {
    if (!appRecord.published_version_id) return false;
    const version = await pocketbase.collection('app_versions').getOne(appRecord.published_version_id).catch(() => null);
    if (!version || version.app_id !== appRecord.id || version.tenant_id !== tenantId) return false;
    const definition = version.definition;
    if (definition?.collection !== slug) return false;
    if (!removedFields) return true;
    const used = new Set(Array.isArray(definition.fields) ? definition.fields : []);
    return removedFields.some((name) => used.has(name));
  };

  registerAppVersionRoutes(app, { auth, body, pocketbase, getApp, requireAppManager, requireAppPublisher });

  app.get('/api/apps', { preHandler: auth }, async (request) => {
    const archived = request.query?.archived === 'true';
    const rows = await pocketbase.collection('apps').getFullList({
      filter: pocketbase.filter('tenant_id = {:tenantId} && archived = {:archived}', { tenantId: request.tenant.id, archived }),
      sort: '-updated'
    });
    const visible = [];
    for (const record of rows) {
      if (request.membership.role === 'owner') { visible.push({ ...record, permission: 'owner' }); continue; }
      const permission = await pocketbase.collection('app_members').getFirstListItem(
        pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && user_id = {:userId}', { tenantId: request.tenant.id, appId: record.id, userId: request.user.id })
      ).catch(() => null);
      if (permission || !record.restricted) visible.push({ ...record, permission: permission?.role || 'editor' });
    }
    return visible.map(publicApp);
  });

  app.post('/api/apps', { preHandler: auth }, async (request, reply) => {
    const { name, description } = body(request);
    if (!String(name || '').trim()) return reply.code(400).send({ error: '请输入应用名称' });
    const record = await pocketbase.collection('apps').create({
      tenant_id: request.tenant.id,
      creator_id: request.user.id,
      name: String(name).trim().slice(0, 160),
      description: String(description || '').trim().slice(0, 4000)
    });
    if (request.membership.role !== 'owner') await pocketbase.collection('app_members').create({ tenant_id: request.tenant.id, app_id: record.id, user_id: request.user.id, role: 'publisher' });
    return reply.code(201).send(publicApp({ ...record, permission: request.membership.role === 'owner' ? 'owner' : 'publisher' }));
  });

  app.patch('/api/apps/:id', { preHandler: auth }, async (request, reply) => {
    const record = await getApp(request, reply);
    if (!record) return;
    requireAppManager(request, reply);
    if (reply.sent) return;
    const { name, description, archived } = body(request);
    if (name !== undefined && !String(name || '').trim()) return reply.code(400).send({ error: '请输入应用名称' });
    const updated = await pocketbase.collection('apps').update(record.id, {
      ...(name !== undefined ? { name: String(name).trim().slice(0, 160) } : {}),
      ...(description !== undefined ? { description: String(description || '').trim().slice(0, 4000) } : {}),
      ...(archived !== undefined ? { archived: Boolean(archived) } : {})
    });
    return publicApp({ ...updated, permission: request.appPermission });
  });

  app.delete('/api/apps/:id', { preHandler: auth }, async (request, reply) => {
    const record = await getApp(request, reply);
    if (!record) return;
    requireWorkspaceOwner(request, reply);
    if (reply.sent) return;
    if (body(request).confirm !== true) return reply.code(400).send({ error: '删除应用会永久删除其中所有数据，请明确确认' });
    const tables = await pocketbase.collection('app_collections').getFullList({
      filter: pocketbase.filter('app_id = {:appId} && tenant_id = {:tenantId}', { appId: record.id, tenantId: request.tenant.id })
    });
    const appMembers = await pocketbase.collection('app_members').getFullList({
      filter: pocketbase.filter('app_id = {:appId} && tenant_id = {:tenantId}', { appId: record.id, tenantId: request.tenant.id })
    });
    const versions = await pocketbase.collection('app_versions').getFullList({
      filter: pocketbase.filter('app_id = {:appId} && tenant_id = {:tenantId}', { appId: record.id, tenantId: request.tenant.id })
    }).catch(() => []);
    for (const table of tables) {
      await pocketbase.collections.delete(table.pb_collection);
      await pocketbase.collection('app_collections').delete(table.id);
    }
    await Promise.all(appMembers.map((permission) => pocketbase.collection('app_members').delete(permission.id)));
    await Promise.all(versions.map((version) => pocketbase.collection('app_versions').delete(version.id)));
    for (const name of ['agent_threads', 'batch_jobs', 'automation_notifications', 'automation_runs', 'automation_rules']) {
      const rows = await pocketbase.collection(name).getFullList({ filter: pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId}', { tenantId: request.tenant.id, appId: record.id }) });
      if (name === 'agent_threads') for (const thread of rows) {
        const messages = await pocketbase.collection('agent_messages').getFullList({ filter: pocketbase.filter('tenant_id = {:tenantId} && thread_id = {:threadId}', { tenantId: request.tenant.id, threadId: thread.id }) });
        for (const message of messages) await pocketbase.collection('agent_messages').delete(message.id);
      }
      for (const item of rows) await pocketbase.collection(name).delete(item.id);
    }
    await pocketbase.collection('apps').delete(record.id);
    return { ok: true, deleted_tables: tables.length };
  });

  app.get('/api/apps/:id', { preHandler: auth }, async (request, reply) => {
    const record = await getApp(request, reply);
    if (!record) return;
    return publicApp({ ...record, permission: request.appPermission });
  });

  app.get('/api/apps/:id/collections', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    const rows = await pocketbase.collection('app_collections').getFullList({
      filter: pocketbase.filter('app_id = {:appId} && tenant_id = {:tenantId}', { appId: appRecord.id, tenantId: request.tenant.id }),
      sort: 'created'
    });
    return rows.map(publicCollection);
  });

  app.get('/api/apps/:id/access', { preHandler: auth }, async (request, reply) => {
    requireWorkspaceOwner(request, reply);
    if (reply.sent) return;
    const appRecord = await pocketbase.collection('apps').getOne(request.params.id).catch(() => null);
    if (!appRecord || appRecord.tenant_id !== request.tenant.id) return reply.code(404).send({ error: '应用不存在' });
    const [memberships, permissions] = await Promise.all([
      pocketbase.collection('tenant_members').getFullList({ filter: pocketbase.filter('tenant_id = {:tenantId}', { tenantId: request.tenant.id }) }),
      pocketbase.collection('app_members').getFullList({ filter: pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId}', { tenantId: request.tenant.id, appId: appRecord.id }) })
    ]);
    const assigned = new Map(permissions.map((permission) => [permission.user_id, permission]));
    const members = [];
    for (const membership of memberships) {
      const user = await pocketbase.collection('users').getOne(membership.user_id).catch(() => null);
      if (user && membership.role !== 'owner') members.push({ id: user.id, email: user.email, name: user.name, workspace_role: membership.role, app_role: assigned.get(user.id)?.role || '', can_batch: Boolean(assigned.get(user.id)?.can_batch) });
    }
    return { restricted: Boolean(appRecord.restricted), members };
  });

  app.put('/api/apps/:id/access', { preHandler: auth }, async (request, reply) => {
    requireWorkspaceOwner(request, reply);
    if (reply.sent) return;
    const appRecord = await pocketbase.collection('apps').getOne(request.params.id).catch(() => null);
    if (!appRecord || appRecord.tenant_id !== request.tenant.id) return reply.code(404).send({ error: '应用不存在' });
    const { restricted, permissions } = body(request);
    if (typeof restricted !== 'boolean' || !Array.isArray(permissions) || permissions.length > 500) return reply.code(400).send({ error: '访问权限设置无效' });
    const memberships = await pocketbase.collection('tenant_members').getFullList({ filter: pocketbase.filter('tenant_id = {:tenantId}', { tenantId: request.tenant.id }) });
    const memberIds = new Set(memberships.filter((item) => item.role !== 'owner').map((item) => item.user_id));
    const normalized = new Map();
    for (const permission of permissions) {
      if (!memberIds.has(permission.user_id) || !['viewer', 'editor', 'manager', 'publisher'].includes(permission.role)) return reply.code(400).send({ error: '权限成员或角色无效' });
      normalized.set(permission.user_id, { role: permission.role, can_batch: Boolean(permission.can_batch) && permission.role !== 'viewer' });
    }
    const current = await pocketbase.collection('app_members').getFullList({ filter: pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId}', { tenantId: request.tenant.id, appId: appRecord.id }) });
    await Promise.all(current.map((permission) => pocketbase.collection('app_members').delete(permission.id)));
    for (const [userId, permission] of normalized) await pocketbase.collection('app_members').create({ tenant_id: request.tenant.id, app_id: appRecord.id, user_id: userId, ...permission });
    await pocketbase.collection('apps').update(appRecord.id, { restricted });
    return { ok: true };
  });

  app.post('/api/apps/:id/collections', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    requireAppManager(request, reply);
    if (reply.sent) return;
    const { name, fields } = body(request);
    const displayName = String(name || '').trim();
    if (!displayName) return reply.code(400).send({ error: '请输入数据表名称' });
    if (!Array.isArray(fields) || fields.length < 1 || fields.length > 24) return reply.code(400).send({ error: '数据表需要 1 到 24 个字段' });
    const fieldNames = new Set();
    const normalizedFields = [];
    for (const field of fields) {
      const fieldName = cleanSlug(field.name);
      const type = String(field.type || 'text');
      if (!fieldName || reservedFields.has(fieldName)) return reply.code(400).send({ error: `字段名「${field.name || ''}」无效` });
      if (fieldNames.has(fieldName)) return reply.code(400).send({ error: `字段「${fieldName}」重复` });
      if (!fieldTypes.has(type)) return reply.code(400).send({ error: `暂不支持「${type}」字段` });
      fieldNames.add(fieldName);
      const normalized = { name: fieldName, label: String(field.label || field.name || fieldName).trim().slice(0, 120), type, required: Boolean(field.required) };
      if (type === 'select') {
        const options = Array.isArray(field.options) ? [...new Set(field.options.map((value) => String(value).trim()).filter(Boolean))].slice(0, 40) : [];
        if (options.length < 2 || options.some((value) => value.length > 120)) return reply.code(400).send({ error: `字段「${normalized.label}」至少需要两个有效选项` });
        normalized.options = options;
      }
      if (type === 'relation') {
        const target = String(field.target || '');
        const targetMetadata = await pocketbase.collection('app_collections').getFirstListItem(
          pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && slug = {:slug}', { tenantId: request.tenant.id, appId: appRecord.id, slug: target })
        ).catch(() => null);
        if (!targetMetadata) return reply.code(400).send({ error: `关联字段「${normalized.label}」必须选择当前应用中的数据表` });
        const targetCollection = await pocketbase.collections.getOne(targetMetadata.pb_collection);
        normalized.target = targetMetadata.slug;
        normalized.target_name = targetMetadata.name;
        normalized.target_collection_id = targetCollection.id;
      }
      normalizedFields.push(normalized);
    }

    const slug = cleanSlug(body(request).slug || displayName) || `table_${Math.random().toString(36).slice(2, 7)}`;
    const pbCollection = `app_${appRecord.id}_${slug}`;
    let createdCollection = false;
    try {
      await pocketbase.collections.create({
        type: 'base',
        name: pbCollection,
        listRule: null,
        viewRule: null,
        createRule: null,
        updateRule: null,
        deleteRule: null,
        fields: [
          { name: 'created', type: 'autodate', onCreate: true, system: true },
          { name: 'updated', type: 'autodate', onCreate: true, onUpdate: true, system: true },
          { name: 'app_id', type: 'text', required: true, max: 64 },
          { name: 'tenant_id', type: 'text', required: true, max: 64 },
          ...normalizedFields.map(({ name: fieldName, type, required, options, target_collection_id: targetCollectionId }) => {
            if (type === 'select') return { name: fieldName, type, required, maxSelect: 1, values: options };
            if (type === 'relation') return { name: fieldName, type, required, maxSelect: 1, collectionId: targetCollectionId, cascadeDelete: false };
            if (type === 'file') return { name: fieldName, type, required, maxSelect: 1, maxSize: 5 * 1024 * 1024, mimeTypes: ['image/*', 'application/pdf', 'text/plain'] };
            return { name: fieldName, type, required, max: type === 'text' ? 10000 : undefined };
          })
        ]
      });
      createdCollection = true;
      const metadata = await pocketbase.collection('app_collections').create({
        tenant_id: request.tenant.id,
        app_id: appRecord.id,
        name: displayName.slice(0, 160),
        slug,
        pb_collection: pbCollection,
        fields: normalizedFields
      });
      return reply.code(201).send(publicCollection(metadata));
    } catch (error) {
      if (createdCollection) await pocketbase.collections.delete(pbCollection).catch(() => {});
      request.log.error(error);
      return reply.code(error?.status === 400 || error?.status === 409 ? 409 : 422).send({ error: '数据表创建失败，请检查名称和字段' });
    }
  });

  app.patch('/api/apps/:id/collections/:slug', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    requireAppManager(request, reply);
    if (reply.sent) return;
    const metadata = await getAppCollection(request, reply, appRecord);
    if (!metadata) return;
    const { name, fields, remove_fields: removeFields = [], confirm_data_loss: confirmDataLoss } = body(request);
    const updates = {};
    if (name !== undefined) {
      if (!String(name || '').trim()) return reply.code(400).send({ error: '请输入数据表名称' });
      updates.name = String(name).trim().slice(0, 160);
    }
    if (fields !== undefined) {
      if (!Array.isArray(fields) || fields.length < 1 || fields.length > 24 || !Array.isArray(removeFields)) return reply.code(400).send({ error: '字段设置无效' });
      const oldFields = metadata.fields || [];
      const names = new Set();
      const nextFields = [];
      for (const item of fields) {
        const fieldName = cleanSlug(item.name);
        const type = String(item.type || 'text');
        if (!fieldName || reservedFields.has(fieldName) || names.has(fieldName) || !fieldTypes.has(type)) return reply.code(400).send({ error: `字段「${item.name || ''}」无效或重复` });
        names.add(fieldName);
        const current = oldFields.find((field) => field.name === fieldName);
        if (current && current.type !== type) return reply.code(400).send({ error: `字段「${current.label}」不能直接更改类型；请新增字段并迁移数据` });
        const normalized = { name: fieldName, label: String(item.label || fieldName).trim().slice(0, 120), type, required: Boolean(item.required) };
        if (type === 'select') {
          normalized.options = Array.isArray(item.options) ? [...new Set(item.options.map((value) => String(value).trim()).filter(Boolean))].slice(0, 40) : [];
          if (normalized.options.length < 2) return reply.code(400).send({ error: `字段「${normalized.label}」至少需要两个有效选项` });
        }
        if (type === 'relation') {
          const target = String(item.target || current?.target || '');
          const targetMetadata = await pocketbase.collection('app_collections').getFirstListItem(
            pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && slug = {:slug}', { tenantId: request.tenant.id, appId: appRecord.id, slug: target })
          ).catch(() => null);
          if (!targetMetadata) return reply.code(400).send({ error: `关联字段「${normalized.label}」必须选择当前应用中的数据表` });
          const targetCollection = await pocketbase.collections.getOne(targetMetadata.pb_collection);
          normalized.target = targetMetadata.slug;
          normalized.target_name = targetMetadata.name;
          normalized.target_collection_id = targetCollection.id;
          if (current?.target && current.target !== normalized.target) return reply.code(400).send({ error: `字段「${current.label}」不能直接更改关联数据表` });
        }
        nextFields.push(normalized);
      }
      const removed = oldFields.filter((field) => !names.has(field.name)).map((field) => field.name);
      if (removed.some((field) => !removeFields.includes(field))) return reply.code(400).send({ error: '要删除字段时请明确列出字段名' });
      if (removed.length && confirmDataLoss !== true) return reply.code(400).send({ error: '删除字段会永久清除这些字段中的数据，请明确确认' });
      if (removed.length && await publishedUiUses(appRecord, request.tenant.id, metadata.slug, removed)) {
        return reply.code(409).send({ error: '这些字段正在当前已发布界面中使用。请先为界面创建并发布不再引用它们的新版本，再删除字段' });
      }
      const collection = await pocketbase.collections.getOne(metadata.pb_collection);
      const recordFilter = pocketbase.filter('app_id = {:appId} && tenant_id = {:tenantId}', { appId: appRecord.id, tenantId: request.tenant.id });
      const count = await pocketbase.collection(metadata.pb_collection).getList(1, 1, { filter: recordFilter });
      for (const field of nextFields) {
        const existing = oldFields.find((item) => item.name === field.name);
        if (field.required && !existing && count.totalItems) return reply.code(400).send({ error: `新增必填字段「${field.label}」前，请先确保数据表没有现有记录` });
        if (field.required && existing && !existing.required) {
          const empty = await pocketbase.collection(metadata.pb_collection).getList(1, 1, { filter: pocketbase.filter(`(${field.name} = "") && app_id = {:appId} && tenant_id = {:tenantId}`, { appId: appRecord.id, tenantId: request.tenant.id }) }).catch(() => ({ totalItems: 0 }));
          if (empty.totalItems) return reply.code(400).send({ error: `字段「${field.label}」仍有空值，无法设为必填` });
        }
      }
      const schemaFields = collection.fields.filter((field) => !names.has(field.name) && !oldFields.some((item) => item.name === field.name));
      for (const field of nextFields) {
        const existing = collection.fields.find((item) => item.name === field.name);
        if (existing) {
          schemaFields.push({ ...existing, required: field.required, ...(field.type === 'select' ? { values: field.options } : {}) });
        } else if (field.type === 'select') schemaFields.push({ name: field.name, type: field.type, required: field.required, maxSelect: 1, values: field.options });
        else if (field.type === 'relation') schemaFields.push({ name: field.name, type: field.type, required: field.required, maxSelect: 1, collectionId: field.target_collection_id, cascadeDelete: false });
        else if (field.type === 'file') schemaFields.push({ name: field.name, type: field.type, required: field.required, maxSelect: 1, maxSize: 5 * 1024 * 1024, mimeTypes: ['image/*', 'application/pdf', 'text/plain'] });
        else schemaFields.push({ name: field.name, type: field.type, required: field.required, max: field.type === 'text' ? 10000 : undefined });
      }
      await pocketbase.collections.update(metadata.pb_collection, {
        name: collection.name, type: 'base', listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
        fields: schemaFields, indexes: collection.indexes || []
      });
      updates.fields = nextFields;
    }
    const updated = await pocketbase.collection('app_collections').update(metadata.id, updates);
    return publicCollection(updated);
  });

  app.delete('/api/apps/:id/collections/:slug', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    requireAppManager(request, reply);
    if (reply.sent) return;
    const metadata = await getAppCollection(request, reply, appRecord);
    if (!metadata) return;
    if (body(request).confirm !== true) return reply.code(400).send({ error: '删除数据表会永久删除其中所有记录，请明确确认' });
    if (await publishedUiUses(appRecord, request.tenant.id, metadata.slug)) {
      return reply.code(409).send({ error: '此数据表正在当前已发布界面中使用。请先为界面创建并发布引用其他数据表的新版本，再删除此表' });
    }
    const count = await pocketbase.collection(metadata.pb_collection).getList(1, 1, {
      filter: pocketbase.filter('app_id = {:appId} && tenant_id = {:tenantId}', { appId: appRecord.id, tenantId: request.tenant.id })
    });
    await pocketbase.collections.delete(metadata.pb_collection);
    await pocketbase.collection('app_collections').delete(metadata.id);
    return { ok: true, deleted_records: count.totalItems };
  });

  app.get('/api/apps/:id/collections/:slug/records', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    const metadata = await getAppCollection(request, reply, appRecord);
    if (!metadata) return;
    const query = request.query || {};
    const page = Math.max(1, Math.min(1000000, Number.parseInt(query.page, 10) || 1));
    const perPage = Math.max(1, Math.min(100, Number.parseInt(query.perPage, 10) || 25));
    const fields = metadata.fields || [];
    const filterParts = ['app_id = {:appId}', 'tenant_id = {:tenantId}'];
    const params = { appId: appRecord.id, tenantId: request.tenant.id };
    const search = String(query.search || '').trim().slice(0, 120);
    if (search) {
      const searchable = fields.filter((field) => ['text', 'email', 'url'].includes(field.type));
      if (searchable.length) {
        const alternatives = searchable.map((field, index) => {
          const key = `search${index}`;
          params[key] = search;
          return `${field.name} ~ {:${key}}`;
        });
        filterParts.push(`(${alternatives.join(' || ')})`);
      }
    }
    const fieldName = fields.some((field) => field.name === query.filterField) ? query.filterField : null;
    if (fieldName && query.filterValue !== undefined && query.filterValue !== '') {
      const field = fields.find((item) => item.name === fieldName);
      const filterValue = String(query.filterValue).slice(0, 200);
      params.filterValue = field.type === 'number' ? Number(filterValue) : field.type === 'bool' ? filterValue === 'true' : filterValue;
      if (field.type === 'number' && !Number.isFinite(params.filterValue)) return reply.code(400).send({ error: '筛选值必须是数字' });
      if (field.type === 'bool' && !['true', 'false'].includes(filterValue)) return reply.code(400).send({ error: '筛选值必须是布尔值' });
      filterParts.push(`${fieldName} = {:filterValue}`);
    }
    const requestedSort = String(query.sort || '-created');
    const sortName = requestedSort.replace(/^-/, '');
    const sort = ['created', 'updated'].includes(sortName) || fields.some((field) => field.name === sortName) ? requestedSort : '-created';
    const result = await pocketbase.collection(metadata.pb_collection).getList(page, perPage, {
      filter: pocketbase.filter(filterParts.join(' && '), params), sort
    });
    return { items: result.items.map(publicRecord), page: result.page, perPage: result.perPage, totalItems: result.totalItems, totalPages: result.totalPages };
  });

  app.post('/api/apps/:id/collections/:slug/records', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    requireAppEditor(request, reply);
    if (reply.sent) return;
    const metadata = await getAppCollection(request, reply, appRecord);
    if (!metadata) return;
    const payload = body(request);
    const values = payload.data;
    if (!values || typeof values !== 'object' || Array.isArray(values)) return reply.code(400).send({ error: '记录内容必须是对象' });
    const fileError = addUploadedFiles(values, payload.files, metadata.fields || []);
    if (fileError) return reply.code(400).send({ error: fileError });
    const validationError = validateRecordData(values, metadata.fields);
    if (validationError) return reply.code(400).send({ error: validationError });
    const relationError = await validateRelations(values, metadata.fields || [], { pocketbase, appId: appRecord.id, tenantId: request.tenant.id });
    if (relationError) return reply.code(400).send({ error: relationError });
    let record;
    try {
      record = await pocketbase.collection(metadata.pb_collection).create({ ...values, app_id: appRecord.id, tenant_id: request.tenant.id });
    } catch (error) {
      if (error?.status === 400) return reply.code(400).send({ error: '记录字段值无效' });
      request.log.error(error);
      return reply.code(503).send({ error: '记录保存失败，请稍后重试' });
    }
    await processRecordAutomation(pocketbase, { tenantId: request.tenant.id, appId: appRecord.id, table: metadata.slug, event: 'created', after: record }).catch((error) => request.log.error({ err: error }, 'automation failed'));
    return reply.code(201).send(publicRecord(record));
  });

  app.patch('/api/apps/:id/collections/:slug/records/:recordId', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    requireAppEditor(request, reply);
    if (reply.sent) return;
    const metadata = await getAppCollection(request, reply, appRecord);
    if (!metadata) return;
    const collection = pocketbase.collection(metadata.pb_collection);
    const record = await collection.getOne(request.params.recordId).catch(() => null);
    if (!record || record.app_id !== appRecord.id || record.tenant_id !== request.tenant.id) return reply.code(404).send({ error: '记录不存在' });
    const payload = body(request);
    const values = payload.data;
    if (!values || typeof values !== 'object' || Array.isArray(values)) return reply.code(400).send({ error: '记录内容必须是对象' });
    const fileError = addUploadedFiles(values, payload.files, metadata.fields || []);
    if (fileError) return reply.code(400).send({ error: fileError });
    const validationError = validateRecordData(values, metadata.fields, { partial: true });
    if (validationError) return reply.code(400).send({ error: validationError });
    const relationError = await validateRelations(values, metadata.fields || [], { pocketbase, appId: appRecord.id, tenantId: request.tenant.id });
    if (relationError) return reply.code(400).send({ error: relationError });
    let updated;
    try {
      updated = await collection.update(record.id, { ...values, app_id: appRecord.id, tenant_id: request.tenant.id });
    } catch (error) {
      if (error?.status === 400) return reply.code(400).send({ error: '记录字段值无效' });
      request.log.error(error);
      return reply.code(503).send({ error: '记录保存失败，请稍后重试' });
    }
    await processRecordAutomation(pocketbase, { tenantId: request.tenant.id, appId: appRecord.id, table: metadata.slug, event: 'updated', before: record, after: updated }).catch((error) => request.log.error({ err: error }, 'automation failed'));
    return publicRecord(updated);
  });

  app.delete('/api/apps/:id/collections/:slug/records/:recordId', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    requireAppEditor(request, reply);
    if (reply.sent) return;
    const metadata = await getAppCollection(request, reply, appRecord);
    if (!metadata) return;
    const collection = pocketbase.collection(metadata.pb_collection);
    const record = await collection.getOne(request.params.recordId).catch(() => null);
    if (!record || record.app_id !== appRecord.id || record.tenant_id !== request.tenant.id) return reply.code(404).send({ error: '记录不存在' });
    await collection.delete(record.id);
    return { ok: true };
  });

  app.get('/api/apps/:id/collections/:slug/records/:recordId/files/:fieldName', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    const metadata = await getAppCollection(request, reply, appRecord);
    if (!metadata) return;
    const field = (metadata.fields || []).find((item) => item.name === request.params.fieldName && item.type === 'file');
    if (!field) return reply.code(404).send({ error: '附件不存在' });
    const record = await pocketbase.collection(metadata.pb_collection).getOne(request.params.recordId).catch(() => null);
    const filename = record?.[field.name];
    if (!record || record.app_id !== appRecord.id || record.tenant_id !== request.tenant.id || typeof filename !== 'string' || !filename) return reply.code(404).send({ error: '附件不存在' });
    try {
      const url = `${process.env.POCKETBASE_URL || 'http://127.0.0.1:8090'}/api/files/${encodeURIComponent(metadata.pb_collection)}/${encodeURIComponent(record.id)}/${encodeURIComponent(filename)}`;
      const response = await fetch(url, { headers: { Authorization: pocketbase.authStore.token } });
      if (!response.ok) return reply.code(404).send({ error: '附件不存在' });
      reply.header('content-type', response.headers.get('content-type') || 'application/octet-stream');
      reply.header('content-disposition', `inline; filename*=UTF-8''${encodeURIComponent(filename)}`);
      return reply.send(Buffer.from(await response.arrayBuffer()));
    } catch (error) {
      request.log.error({ err: error }, 'file proxy failed');
      return reply.code(502).send({ error: '附件暂时无法读取' });
    }
  });
};
