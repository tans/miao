const validateList = (definition, tables) => {
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

export const appUiPages = (definition) => definition?.schema_version === 2 ? definition.pages || [] : [{ id: 'main', title: definition?.title, collection: definition?.collection, fields: definition?.fields || [], actions: [] }];

export const validateAppUiDefinition = (definition, tables) => {
  if (definition?.schema_version !== 2) return validateList(definition, tables);
  const fail = (error) => ({ error });
  if (Object.keys(definition).some((key) => !['schema_version', 'title', 'pages'].includes(key))) return fail('界面定义包含不支持的配置');
  if (typeof definition.title !== 'string' || !definition.title.trim() || definition.title.length > 120) return fail('界面标题必须为 1 到 120 个字符');
  if (!Array.isArray(definition.pages) || !definition.pages.length || definition.pages.length > 12) return fail('应用需要 1–12 个页面');
  const seen = new Set();
  const pages = [];
  for (const page of definition.pages) {
    if (!page || typeof page !== 'object' || Object.keys(page).some((key) => !['id', 'title', 'collection', 'fields', 'actions'].includes(key))) return fail('页面配置无效');
    if (!/^[a-z][a-z0-9_]{0,39}$/.test(page.id) || seen.has(page.id)) return fail('页面标识无效或重复');
    seen.add(page.id);
    if (typeof page.title !== 'string' || !page.title.trim() || page.title.length > 120) return fail('页面标题无效');
    const table = tables.find((item) => item.slug === page.collection);
    if (!table) return fail('页面引用的数据表不存在于当前应用');
    if (!Array.isArray(page.fields) || !page.fields.length || page.fields.length > 24 || new Set(page.fields).size !== page.fields.length || page.fields.some((name) => !table.fields.some((field) => field.name === name))) return fail('页面字段无效或重复');
    if (page.actions !== undefined && (!Array.isArray(page.actions) || page.actions.length > 8)) return fail('页面最多支持 8 个动作');
    const actionIds = new Set();
    const actions = [];
    for (const action of page.actions || []) {
      if (!action || Object.keys(action).some((key) => !['id', 'label', 'set'].includes(key)) || !/^[a-z][a-z0-9_]{0,39}$/.test(action.id) || actionIds.has(action.id) || typeof action.label !== 'string' || !action.label.trim() || action.label.length > 60) return fail('业务动作无效或重复');
      actionIds.add(action.id);
      if (!action.set || typeof action.set !== 'object' || Array.isArray(action.set) || !Object.keys(action.set).length) return fail('动作需要具体字段赋值');
      for (const [name, value] of Object.entries(action.set)) {
        const field = table.fields.find((field) => field.name === name);
        if (!field || ['file', 'relation'].includes(field.type) || (field.required && value === '')) return fail('动作字段无效');
        const valid = field.type === 'number' ? typeof value === 'number' && Number.isFinite(value) : field.type === 'bool' ? typeof value === 'boolean' : typeof value === 'string';
        if (!valid || (field.type === 'select' && !field.options.includes(value) && !(!field.required && value === ''))) return fail('动作赋值无效');
      }
      actions.push({ id: action.id, label: action.label.trim(), set: action.set });
    }
    pages.push({ id: page.id, title: page.title.trim(), collection: page.collection, fields: [...page.fields], actions });
  }
  const first = pages[0];
  const table = tables.find((item) => item.slug === first.collection);
  return { definition: { schema_version: 2, title: definition.title.trim(), pages }, table, fields: first.fields.map((name) => table.fields.find((field) => field.name === name)) };
};

export const diffAppUi = (before, after) => {
  const oldPages = appUiPages(before).filter((page) => page.collection);
  const newPages = appUiPages(after);
  const changes = [];
  if (before?.title !== after.title) changes.push({ type: 'title', before: before?.title || '', after: after.title });
  for (const page of oldPages) if (!newPages.some((item) => item.id === page.id)) changes.push({ type: 'remove_page', page: page.id, before: page });
  for (const page of newPages) {
    const old = oldPages.find((item) => item.id === page.id);
    if (!old) changes.push({ type: 'add_page', page: page.id, after: page });
    else if (JSON.stringify(old) !== JSON.stringify(page)) changes.push({ type: 'change_page', page: page.id, before: old, after: page });
  }
  return changes;
};
