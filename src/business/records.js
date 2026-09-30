import { serialized } from './locks.js';

const stringFieldTypes = new Set(['text', 'date', 'email', 'url', 'select', 'relation']);
export const validateRecordData = (values, fields, { partial = false } = {}) => {
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
    if (field.type === 'select' && Array.isArray(field.options) && !field.options.includes(value) && !(!field.required && value === '')) return `字段「${field.label || field.name}」的选项无效`;
    if (!validType) return `字段「${field.label || field.name}」的值类型无效`;
  }

  return null;
};

export const validateRelations = async (values, fields, { pocketbase, appId, tenantId }) => {
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


export const publicRecord = (row) => ({ id: row.id, created_at: row.created, updated_at: row.updated, data: Object.fromEntries(Object.entries(row).filter(([key]) => !['id', 'collectionId', 'collectionName', 'created', 'updated', 'app_id', 'tenant_id'].includes(key))) });

// All MIAO record updates share this gate. Direct PocketBase administrative writes remain outside it.
export const updateBusinessRecord = ({ pocketbase, table, tenantId, appId, recordId, data, expectedUpdated, eventSource = 'interactive', authorize = async () => {} }) => serialized(`record:${table.pb_collection}:${recordId}`, async () => {
  await authorize();
  const metadata = await pocketbase.collection('app_collections').getOne(table.id);
  if (metadata.tenant_id !== tenantId || metadata.app_id !== appId) throw Object.assign(new Error('数据表不属于此应用'), { statusCode: 404 });
  const row = await pocketbase.collection(metadata.pb_collection).getOne(recordId);
  if (row.tenant_id !== tenantId || row.app_id !== appId) throw Object.assign(new Error('记录不属于此应用'), { statusCode: 404 });
  if (expectedUpdated && row.updated !== expectedUpdated) throw Object.assign(new Error('记录已变化，请重新读取后操作'), { statusCode: 409 });
  const validation = validateRecordData(data, metadata.fields || [], { partial: true });
  const relation = await validateRelations(data, metadata.fields || [], { pocketbase, appId, tenantId });
  if (validation || relation) throw Object.assign(new Error(validation || relation), { statusCode: 400 });
  await authorize();
  return pocketbase.collection(metadata.pb_collection).update(row.id, data, { headers: { 'X-Miao-Expected-Updated': expectedUpdated || row.updated, 'X-Miao-Event-Source': eventSource } }).catch((error) => {
    if (error.status === 409) throw Object.assign(new Error('记录已变化，请重新读取后操作'), { statusCode: 409 });
    throw error;
  });
});

const operations = new Set(['eq', 'contains', 'before', 'after', 'empty']);
export const buildRecordFilter = (request, table, conditions, pocketbase) => {
    if (!Array.isArray(conditions) || conditions.length > 8) throw new Error('查询条件最多 8 项');
    const parts = ['tenant_id = {:tenantId}', 'app_id = {:appId}'];
    const params = { tenantId: request.tenant.id, appId: request.params.id };
    for (const [index, condition] of conditions.entries()) {
      const field = (table.fields || []).find((item) => item.name === condition.field);
      if (!field || !operations.has(condition.op)) throw new Error('查询字段或操作符无效');
      if (['before', 'after'].includes(condition.op) && field.type !== 'date') throw new Error('日期条件必须使用日期字段');
      if (condition.op === 'contains' && !['text', 'email', 'url'].includes(field.type)) throw new Error('包含查询只能用于文本字段');
      if (condition.op === 'empty') { parts.push(`${field.name} = ""`); continue; }
      let value = condition.value;
      if (field.type === 'number') value = Number(value);
      if (field.type === 'bool') value = value === true || value === 'true';
      if ((field.type === 'number' && !Number.isFinite(value)) || (field.type !== 'number' && field.type !== 'bool' && typeof value !== 'string')) throw new Error('查询值类型无效');
      params[`value${index}`] = value;
      parts.push(`${field.name} ${condition.op === 'contains' ? '~' : condition.op === 'before' ? '<' : condition.op === 'after' ? '>' : '='} {:value${index}}`);
    }
    return pocketbase.filter(parts.join(' && '), params);
  };
