const fieldTypes = new Set(['text', 'number', 'bool', 'date', 'email', 'url']);
const reservedFields = new Set(['id', 'created', 'updated', 'collectionid', 'collectionname', 'app_id', 'tenant_id']);
const cleanSlug = (value) => String(value || '').toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 20);
const publicApp = ({ id, name, description, created, updated }) => ({ id, name, description, created_at: created, updated_at: updated });
const publicCollection = ({ id, name, slug, fields, created }) => ({ id, name, slug, fields, created_at: created });
const publicRecord = (row) => ({ id: row.id, data: Object.fromEntries(Object.entries(row).filter(([key]) => !['id', 'collectionId', 'collectionName', 'created', 'updated', 'app_id', 'tenant_id'].includes(key))), created_at: row.created, updated_at: row.updated });

export const registerAppRoutes = (app, { auth, body, pocketbase }) => {
  const getApp = async (request, reply) => {
    const record = await pocketbase.collection('apps').getOne(request.params.id).catch(() => null);
    if (!record || record.tenant_id !== request.tenant.id) {
      reply.code(404).send({ error: '应用不存在' });
      return null;
    }
    return record;
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

  app.get('/api/apps', { preHandler: auth }, async (request) => {
    const rows = await pocketbase.collection('apps').getFullList({
      filter: pocketbase.filter('tenant_id = {:tenantId}', { tenantId: request.tenant.id }),
      sort: '-updated'
    });
    return rows.map(publicApp);
  });

  app.post('/api/apps', { preHandler: auth }, async (request, reply) => {
    const { name, description } = body(request);
    if (!String(name || '').trim()) return reply.code(400).send({ error: '请输入应用名称' });
    const record = await pocketbase.collection('apps').create({
      tenant_id: request.tenant.id,
      name: String(name).trim().slice(0, 160),
      description: String(description || '').trim().slice(0, 4000)
    });
    return reply.code(201).send(publicApp(record));
  });

  app.get('/api/apps/:id', { preHandler: auth }, async (request, reply) => {
    const record = await getApp(request, reply);
    return record && publicApp(record);
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

  app.post('/api/apps/:id/collections', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
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
      normalizedFields.push({ name: fieldName, label: String(field.label || field.name || fieldName).trim().slice(0, 120), type, required: Boolean(field.required) });
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
          { name: 'app_id', type: 'text', required: true, max: 64 },
          { name: 'tenant_id', type: 'text', required: true, max: 64 },
          ...normalizedFields.map(({ name: fieldName, type, required }) => ({ name: fieldName, type, required, max: type === 'text' ? 10000 : undefined }))
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

  app.get('/api/apps/:id/collections/:slug/records', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    const metadata = await getAppCollection(request, reply, appRecord);
    if (!metadata) return;
    const rows = await pocketbase.collection(metadata.pb_collection).getFullList({
      filter: pocketbase.filter('app_id = {:appId} && tenant_id = {:tenantId}', { appId: appRecord.id, tenantId: request.tenant.id }),
      sort: '-created'
    });
    return rows.map(publicRecord);
  });

  app.post('/api/apps/:id/collections/:slug/records', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    const metadata = await getAppCollection(request, reply, appRecord);
    if (!metadata) return;
    const values = body(request).data;
    if (!values || typeof values !== 'object' || Array.isArray(values)) return reply.code(400).send({ error: '记录内容必须是对象' });
    const allowed = new Set((metadata.fields || []).map((field) => field.name));
    const invalid = Object.keys(values).find((key) => !allowed.has(key));
    if (invalid) return reply.code(400).send({ error: `数据表中没有「${invalid}」字段` });
    const record = await pocketbase.collection(metadata.pb_collection).create({ ...values, app_id: appRecord.id, tenant_id: request.tenant.id });
    return reply.code(201).send(publicRecord(record));
  });

  app.patch('/api/apps/:id/collections/:slug/records/:recordId', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    const metadata = await getAppCollection(request, reply, appRecord);
    if (!metadata) return;
    const collection = pocketbase.collection(metadata.pb_collection);
    const record = await collection.getOne(request.params.recordId).catch(() => null);
    if (!record || record.app_id !== appRecord.id || record.tenant_id !== request.tenant.id) return reply.code(404).send({ error: '记录不存在' });
    const values = body(request).data;
    if (!values || typeof values !== 'object' || Array.isArray(values)) return reply.code(400).send({ error: '记录内容必须是对象' });
    const allowed = new Set((metadata.fields || []).map((field) => field.name));
    const invalid = Object.keys(values).find((key) => !allowed.has(key));
    if (invalid) return reply.code(400).send({ error: `数据表中没有「${invalid}」字段` });
    const updated = await collection.update(record.id, { ...values, app_id: appRecord.id, tenant_id: request.tenant.id });
    return publicRecord(updated);
  });

  app.delete('/api/apps/:id/collections/:slug/records/:recordId', { preHandler: auth }, async (request, reply) => {
    const appRecord = await getApp(request, reply);
    if (!appRecord) return;
    const metadata = await getAppCollection(request, reply, appRecord);
    if (!metadata) return;
    const collection = pocketbase.collection(metadata.pb_collection);
    const record = await collection.getOne(request.params.recordId).catch(() => null);
    if (!record || record.app_id !== appRecord.id || record.tenant_id !== request.tenant.id) return reply.code(404).send({ error: '记录不存在' });
    await collection.delete(record.id);
    return { ok: true };
  });
};
