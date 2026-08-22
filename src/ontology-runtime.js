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

export function buildObjectRecord({ manifest, tenantId, appId, objectType, data, provenance = { type: 'agent' }, timestamp = now() }) {
  const definition = findObjectDefinition(manifest, objectType);
  if (!definition) throw new Error(`未定义对象类型：${objectType}`);
  const errors = validateProperties(definition, data);
  if (errors.length) throw new Error(errors.join('；'));
  return { id: id(), tenant_id: tenantId, app_id: appId, object_type: definition.slug, data_json: data, created_at: timestamp, updated_at: timestamp, deleted_at: null, provenance_json: provenance };
}

export async function createObject({ collections, manifest, tenantId, appId, objectType, data, provenance = { type: 'agent' } }) {
  const row = buildObjectRecord({ manifest, tenantId, appId, objectType, data, provenance });
  await collections.records.insertOne(row);
  return recordData(row);
}

export async function createObjects({ collections, manifest, tenantId, appId, objectType, rows, provenanceFor = () => ({ type: 'bulk' }) }) {
  const timestamp = now();
  const records = rows.map((data, index) => buildObjectRecord({ manifest, tenantId, appId, objectType, data, provenance: provenanceFor(index, data), timestamp }));
  if (records.length) await collections.records.insertMany(records);
  return records.map(recordData);
}

export async function updateObject({ collections, manifest, appId, objectId, data, provenance = { type: 'human' } }) {
  const row = await collections.records.findOne({ id: objectId, app_id: appId, deleted_at: null });
  if (!row) throw new Error('对象不存在');
  const definition = findObjectDefinition(manifest, row.object_type || row.collection); const properties = { ...row.data_json, ...data }; const errors = validateProperties(definition, properties);
  if (errors.length) throw new Error(errors.join('；'));
  const timestamp = now(); await collections.records.updateOne({ id: row.id, app_id: appId }, { $set: { data_json: properties, updated_at: timestamp, provenance_json: provenance } });
  return { ...recordData(row), properties, updated_at: timestamp, provenance };
}

export async function deleteObject({ collections, appId, objectId, provenance = { type: 'human' } }) {
  const timestamp = now(); const result = await collections.records.updateOne({ id: objectId, app_id: appId, deleted_at: null }, { $set: { deleted_at: timestamp, updated_at: timestamp, provenance_json: provenance } });
  if (!result.modifiedCount) throw new Error('对象不存在');
  return { id: objectId, deleted_at: timestamp };
}

export const mapImportRows = (rows, fieldMapping) => rows.map((row) => Object.fromEntries(Object.entries(fieldMapping).map(([sourceField, property]) => [property, row[sourceField]])));

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

const argumentValue = (args, key, field) => args[key] ?? args[field.name];
const actionInputErrors = (definition, args) => Object.entries(definition.input || {}).filter(([key, field]) => field.required && argumentValue(args, key, field) === undefined).map(([key]) => `动作参数 ${key} 必填`);

export async function planAction({ collections, tenantId, appId, manifest, actionName, args = {} }) {
  const definition = findActionDefinition(manifest, actionName);
  if (!definition) throw new Error(`未知业务动作：${actionName}`);
  const inputErrors = actionInputErrors(definition, args);
  if (inputErrors.length) throw new Error(inputErrors.join('；'));
  const targets = [];
  for (const [key, field] of Object.entries(definition.input || {})) {
    if (field.type !== 'object_ref') continue; const objectId = argumentValue(args, key, field); if (!objectId) continue;
    const target = await collections.records.findOne({ id: objectId, app_id: appId, deleted_at: null });
    if (!target) throw new Error(`动作参数 ${key} 引用的对象不存在`);
    if (field.object && field.object !== (target.object_type || target.collection)) throw new Error(`动作参数 ${key} 必须引用 ${field.object} 对象`);
    targets.push({ input: key, row: target });
  }
  const target = targets[0]?.row || null;
  const errors = [];
  for (const rule of definition.rules || []) {
    if (rule.op === 'required' && (target?.data_json?.[rule.property] === undefined || target.data_json[rule.property] === '')) errors.push(`字段 ${rule.property} 必填`);
    if (rule.op === 'greater_than' && !(Number(target?.data_json?.[rule.property]) > Number(rule.value))) errors.push(`${rule.property} 必须大于 ${rule.value}`);
    if (rule.op === 'state' && target?.data_json?.[rule.property] !== rule.value) errors.push(`${rule.property} 必须是 ${rule.value}`);
  }
  if (errors.length) throw new Error(errors.join('；'));
  const mutations = [];
  for (const mutation of definition.mutations || []) {
    if (mutation.op === 'set') {
      const mutationTarget = targets.find((item) => !mutation.object || mutation.object === (item.row.object_type || item.row.collection))?.row;
      if (!mutationTarget) throw new Error(`动作 ${definition.name} 缺少 ${mutation.object || '目标'} object_ref 参数`);
      const objectDefinition = findObjectDefinition(manifest, mutationTarget.object_type || mutationTarget.collection); if (!objectDefinition?.properties?.[mutation.property]) throw new Error(`动作 ${definition.name} 修改了未定义属性 ${mutation.property}`);
      const value = typeof mutation.value === 'string' ? mutation.value.replace(/\{([^}]+)\}/g, (_, key) => args[key] ?? mutationTarget.data_json[key] ?? '') : mutation.value;
      const nextData = { ...mutationTarget.data_json, [mutation.property]: value }; const validationErrors = validateProperties(objectDefinition, nextData); if (validationErrors.length) throw new Error(validationErrors.join('；'));
      mutations.push({ op: 'set', target: mutationTarget, property: mutation.property, value });
    } else if (mutation.op === 'link') {
      const linkDefinition = (manifest.links || []).find((item) => item.slug === mutation.link); if (!linkDefinition) throw new Error(`动作 ${definition.name} 引用了未定义关系 ${mutation.link}`);
      const fromTarget = targets.find((item) => (item.row.object_type || item.row.collection) === linkDefinition.from)?.row; const toTarget = targets.find((item) => (item.row.object_type || item.row.collection) === linkDefinition.to)?.row;
      const from = args.from_object_id || fromTarget?.id; const to = args.to_object_id || toTarget?.id; if (!from || !to) throw new Error(`动作 ${definition.name} 缺少关系对象参数`);
      const referenced = await collections.records.find({ id: { $in: [from, to] }, app_id: appId, deleted_at: null }).toArray(); if (referenced.length !== new Set([from, to]).size) throw new Error(`动作 ${definition.name} 的关系对象不存在`);
      mutations.push({ op: 'link', link_type: mutation.link, from_object_id: from, to_object_id: to });
    } else if (mutation.op === 'description') throw new Error(`动作 ${definition.name} 含有未编译变更：${mutation.description}`);
    else throw new Error(`动作 ${definition.name} 含有未知 Mutation：${mutation.op}`);
  }
  return { action: definition.slug, targets: targets.map((item) => ({ input: item.input, object_id: item.row.id, object_type: item.row.object_type || item.row.collection })), preconditions: definition.rules || [], mutations };
}

export async function applyAction(context) {
  const { collections, tenantId, appId, actor = 'agent', addEvent } = context; const plan = await planAction(context); const changed = [];
  for (const mutation of plan.mutations) {
    if (mutation.op === 'set') { await collections.records.updateOne({ id: mutation.target.id, app_id: appId }, { $set: { [`data_json.${mutation.property}`]: mutation.value, updated_at: now(), provenance_json: { type: 'action', action: plan.action, actor } } }); changed.push({ object_id: mutation.target.id, property: mutation.property, value: mutation.value }); }
    if (mutation.op === 'link') { await collections.links.updateOne({ tenant_id: tenantId, app_id: appId, link_type: mutation.link_type, from_object_id: mutation.from_object_id, to_object_id: mutation.to_object_id }, { $setOnInsert: { id: id(), tenant_id: tenantId, app_id: appId, link_type: mutation.link_type, from_object_id: mutation.from_object_id, to_object_id: mutation.to_object_id, created_at: now(), provenance_json: { type: 'action', action: plan.action, actor } } }, { upsert: true }); changed.push({ link_type: mutation.link_type, from_object_id: mutation.from_object_id, to_object_id: mutation.to_object_id }); }
  }
  await addEvent({ tenantId, appId, type: 'action.applied', message: `已执行动作 ${plan.action}`, actor, payload: { action: plan.action, changed } });
  return { action: plan.action, changed, plan, object: plan.targets[0] ? await getObject({ collections, appId, objectId: plan.targets[0].object_id }) : null };
}
