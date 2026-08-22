import { id, now, parseJson } from './db.js';
import { slug } from './manifest.js';

export const manifestOf = (app, channel = 'published') => {
  const value = channel === 'draft' ? (app.draft_manifest_json ?? app.manifest_json) : (app.published_manifest_json ?? app.manifest_json);
  return parseJson(value, {});
};

export const findObjectDefinition = (manifest, objectType) => {
  const key = slug(objectType);
  return (manifest.objects || []).find((item) => item.slug === key || slug(item.name) === key);
};

export const findActionDefinition = (manifest, actionName) => {
  const key = slug(actionName);
  return (manifest.actions || []).find((item) => item.slug === key || slug(item.name) === key);
};

export const validateProperties = (definition, data, { partial = false } = {}) => {
  if (!definition) return [`未定义对象类型：${data?.object_type || 'unknown'}`];
  const errors = [];
  for (const [key, field] of Object.entries(definition.properties || {})) {
    const value = data?.[key];
    if (!partial && field.required && (value === undefined || value === null || value === '')) errors.push(`字段 ${field.name || key} 必填`);
    if (value === undefined || value === null || value === '') continue;
    if (field.type === 'number' && typeof value !== 'number' && Number.isNaN(Number(value))) errors.push(`字段 ${field.name || key} 必须是数字`);
    if (field.type === 'enum' && !field.values.includes(value)) errors.push(`字段 ${field.name || key} 必须是：${field.values.join('、')}`);
  }
  return errors;
};

const recordData = (row) => ({ id: row.id, object_type: row.object_type || row.collection, properties: row.data_json || {}, created_at: row.created_at, updated_at: row.updated_at, provenance: row.provenance_json });

export async function createObject({ collections, manifest, tenantId, appId, objectType, data, provenance = { type: 'agent' } }) {
  const definition = findObjectDefinition(manifest, objectType);
  if (!definition) throw new Error(`未定义对象类型：${objectType}`);
  const errors = validateProperties(definition, data);
  if (errors.length) throw new Error(errors.join('；'));
  const timestamp = now();
  const row = { id: id(), tenant_id: tenantId, app_id: appId, object_type: definition.slug, collection: definition.slug, data_json: data, created_at: timestamp, updated_at: timestamp, deleted_at: null, provenance_json: provenance };
  await collections.records.insertOne(row);
  return recordData(row);
}

export async function searchObjects({ collections, appId, objectType, q, limit = 100 }) {
  const query = { app_id: appId, deleted_at: null };
  if (objectType) query.$or = [{ object_type: slug(objectType) }, { collection: slug(objectType) }];
  const rows = await collections.records.find(query).sort({ updated_at: -1 }).limit(Math.min(Number(limit) || 100, 500)).toArray();
  const needle = String(q || '').toLowerCase();
  return rows.map(recordData).filter((row) => !needle || JSON.stringify(row.properties).toLowerCase().includes(needle));
}

export async function getObject({ collections, appId, objectId }) {
  const row = await collections.records.findOne({ id: objectId, app_id: appId, deleted_at: null });
  return row ? recordData(row) : null;
}

export async function relatedObjects({ collections, appId, objectId, linkType, direction = 'out' }) {
  const query = { app_id: appId, [direction === 'in' ? 'to_object_id' : 'from_object_id']: objectId };
  if (linkType) query.link_type = slug(linkType);
  const links = await collections.links.find(query).limit(500).toArray();
  const ids = links.map((link) => direction === 'in' ? link.from_object_id : link.to_object_id);
  if (!ids.length) return [];
  const rows = await collections.records.find({ app_id: appId, id: { $in: ids }, deleted_at: null }).toArray();
  return rows.map(recordData);
}

const actionInputErrors = (definition, args) => Object.entries(definition.input || {}).filter(([, field]) => field.required && (args[field.name] === undefined && args[slug(field.name)] === undefined)).map(([key]) => `动作参数 ${key} 必填`);

export async function applyAction({ collections, tenantId, appId, manifest, actionName, args = {}, actor = 'agent', addEvent }) {
  const definition = findActionDefinition(manifest, actionName);
  if (!definition) throw new Error(`未知业务动作：${actionName}`);
  const inputErrors = actionInputErrors(definition, args);
  if (inputErrors.length) throw new Error(inputErrors.join('；'));
  const objectId = args.object_id || args.target_id || args.id || args.quote_id;
  const target = objectId ? await collections.records.findOne({ id: objectId, app_id: appId, deleted_at: null }) : null;
  const errors = [];
  for (const rule of definition.rules || []) {
    if (rule.op === 'required' && (target?.data_json?.[rule.property] === undefined || target.data_json[rule.property] === '')) errors.push(`字段 ${rule.property} 必填`);
    if (rule.op === 'greater_than' && !(Number(target?.data_json?.[rule.property]) > Number(rule.value))) errors.push(`${rule.property} 必须大于 ${rule.value}`);
    if (rule.op === 'state' && target?.data_json?.[rule.property] !== rule.value) errors.push(`${rule.property} 必须是 ${rule.value}`);
  }
  if (errors.length) throw new Error(errors.join('；'));
  if (!target && definition.mutations?.some((mutation) => mutation.op === 'set')) throw new Error('该动作需要 object_id 或 target_id');
  const changed = [];
  for (const mutation of definition.mutations || []) {
    if (mutation.op === 'set') {
      if (mutation.object && target && mutation.object !== (target.object_type || target.collection)) continue;
      const value = typeof mutation.value === 'string' ? mutation.value.replace(/\{([^}]+)\}/g, (_, key) => args[key] ?? target.data_json[key] ?? '') : mutation.value;
      await collections.records.updateOne({ id: target.id }, { $set: { [`data_json.${mutation.property}`]: value, updated_at: now(), provenance_json: { type: 'action', action: definition.slug, actor } } });
      changed.push({ object_id: target.id, property: mutation.property, value });
    } else if (mutation.op === 'link') {
      const from = args.from_object_id || target?.id;
      const to = args.to_object_id;
      if (!from || !to) throw new Error(`动作 ${definition.name} 缺少关系对象参数`);
      await collections.links.updateOne({ tenant_id: tenantId, app_id: appId, link_type: mutation.link, from_object_id: from, to_object_id: to }, { $setOnInsert: { id: id(), tenant_id: tenantId, app_id: appId, link_type: mutation.link, from_object_id: from, to_object_id: to, created_at: now(), provenance_json: { type: 'action', action: definition.slug, actor } } }, { upsert: true });
      changed.push({ link_type: mutation.link, from_object_id: from, to_object_id: to });
    } else if (mutation.op === 'description') throw new Error(`动作 ${definition.name} 含有未编译变更：${mutation.description}`);
  }
  await addEvent({ tenantId, appId, type: 'action.applied', message: `已执行动作 ${definition.name}`, actor, payload: { action: definition.slug, changed } });
  return { action: definition.slug, changed, object: target ? await getObject({ collections, appId, objectId: target.id }) : null };
}
