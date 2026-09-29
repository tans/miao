export const validateAppUiDefinition = (definition, tables) => {
  if (!definition || typeof definition !== 'object' || Array.isArray(definition)) return { error: '界面定义必须是对象' };
  const allowedKeys = new Set(['schema_version', 'title', 'collection', 'fields']);
  const unknownKey = Object.keys(definition).find((key) => !allowedKeys.has(key));
  if (unknownKey) return { error: `界面定义不支持「${unknownKey}」；只接受受限的列表界面配置` };
  if (definition.schema_version !== 1) return { error: '界面定义版本不受支持' };

  const title = String(definition.title || '').trim();
  if (!title || title.length > 120) return { error: '界面标题必须为 1 到 120 个字符' };
  const collectionSlug = String(definition.collection || '').trim();
  const table = tables.find((item) => item.slug === collectionSlug);
  if (!table) return { error: '界面引用的数据表不存在于当前应用' };
  if (!Array.isArray(definition.fields) || definition.fields.length < 1 || definition.fields.length > 12) {
    return { error: '列表界面需要 1 到 12 个字段' };
  }

  const fieldByName = new Map((table.fields || []).map((field) => [field.name, field]));
  const selectedFields = [];
  const seen = new Set();
  for (const rawName of definition.fields) {
    const name = String(rawName || '');
    if (seen.has(name)) return { error: `界面字段「${name}」重复` };
    const field = fieldByName.get(name);
    if (!field || field.type === 'file') return { error: `界面字段「${name}」不存在或暂不支持` };
    seen.add(name);
    selectedFields.push({ name: field.name, label: field.label || field.name, type: field.type });
  }

  return {
    definition: { schema_version: 1, title, collection: table.slug, fields: selectedFields.map((field) => field.name) },
    table,
    fields: selectedFields,
  };
};
