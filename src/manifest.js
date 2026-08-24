import YAML from 'yaml';

const text = (value) => String(value ?? '').trim();
export const slug = (value) => text(value).toLowerCase().replace(/[^a-z0-9\u4e00-\u9fa5]+/g, '_').replace(/^_|_$/g, '').slice(0, 48) || 'records';

const parseYaml = (value, filename, diagnostics) => {
  if (value && typeof value === 'object') return value;
  try {
    return YAML.parse(text(value)) || {};
  } catch (error) {
    diagnostics.push({ level: 'error', code: 'invalid_yaml', file: filename, message: `${filename} YAML 无法解析：${error.message}` });
    return {};
  }
};

const asEntries = (value) => {
  if (Array.isArray(value)) return value.map((item) => [item.name || item.slug, item]).filter(([name]) => name);
  return Object.entries(value || {});
};

const normalizeField = (field, fallbackName) => {
  if (typeof field === 'string') return { name: fallbackName, type: field, required: false };
  const value = field || {};
  const type = typeof value.type === 'object' ? value.type.type : value.type || 'string';
  return {
    name: value.label || value.name || fallbackName,
    type,
    values: value.values || (typeof value.type === 'object' ? value.type.values : undefined),
    object: value.object || (typeof value.type === 'object' ? value.type.object : undefined),
    required: Boolean(value.required)
  };
};

const normalizeProperties = (fields) => Object.fromEntries(asEntries(fields).map(([name, field]) => [slug(name), normalizeField(field, name)]));

const normalizeObject = ([name, value]) => ({
  name: value?.label || value?.name || name,
  slug: slug(value?.slug || name),
  description: text(value?.description),
  properties: normalizeProperties(value?.fields || value?.properties)
});

const normalizeRelation = ([name, value]) => {
  const relation = typeof value === 'string' ? { to: value } : value || {};
  return {
    name: relation.label || relation.name || name,
    slug: slug(relation.slug || name),
    from: slug(relation.from),
    to: slug(relation.to),
    type: relation.type || 'related_to',
    cardinality: relation.cardinality || 'many',
    description: text(relation.description)
  };
};

const normalizeAction = ([name, value]) => {
  const action = value || {};
  const input = normalizeProperties(action.input);
  const rules = (action.rules || []).map((rule) => {
    if (rule.state) return { op: 'state', property: slug(rule.state.property), value: rule.state.value };
    if (rule.required) return { op: 'required', property: slug(rule.required) };
    if (rule.greater_than) return { op: 'greater_than', property: slug(rule.greater_than.property), value: rule.greater_than.value };
    return rule;
  });
  const mutations = (action.mutations || action.effects || []).map((mutation) => {
    if (mutation.set) {
      const [object, property] = text(mutation.set).split('.');
      return { op: 'set', object: slug(object), property: slug(property), value: mutation.value ?? '' };
    }
    if (mutation.link) return { op: 'link', link: slug(mutation.link), from: slug(mutation.from), to: slug(mutation.to) };
    return mutation;
  });
  return {
    name: action.label || action.name || name,
    slug: slug(action.slug || name),
    description: text(action.description || action.label || name),
    input,
    output: action.output || {},
    rules,
    mutations
  };
};

const normalizeWorkflow = ([name, value]) => {
  const workflow = value || {};
  return { name: workflow.label || workflow.name || name, slug: slug(workflow.slug || name), description: text(workflow.description), steps: workflow.steps || [] };
};

const definitionFiles = (definition = {}) => ({
  appMd: text(definition.appMd ?? definition['app.md']),
  appYaml: definition.appYaml ?? definition['app.yaml'] ?? {},
  ontologyYaml: definition.ontologyYaml ?? definition['ontology.yaml'] ?? {},
  workflowYaml: definition.workflowYaml ?? definition['workflow.yaml'] ?? {},
  actionsYaml: definition.actionsYaml ?? definition['actions.yaml'] ?? {}
});

export const parseDefinition = (definition = {}) => {
  const diagnostics = [];
  const files = definitionFiles(definition);
  return {
    files,
    app: parseYaml(files.appYaml, 'app.yaml', diagnostics),
    ontology: parseYaml(files.ontologyYaml, 'ontology.yaml', diagnostics),
    workflow: parseYaml(files.workflowYaml, 'workflow.yaml', diagnostics),
    actions: parseYaml(files.actionsYaml, 'actions.yaml', diagnostics),
    diagnostics
  };
};

export function compileDefinition(definition = {}) {
  const parsed = parseDefinition(definition);
  const app = parsed.app || {};
  const ontology = parsed.ontology || {};
  const workflow = parsed.workflow || {};
  const actionsConfig = parsed.actions || {};
  const objects = asEntries(ontology.objects).map(normalizeObject);
  const links = asEntries(ontology.relations || ontology.links).map(normalizeRelation);
  const actions = asEntries(actionsConfig.actions || actionsConfig).map(normalizeAction);
  const workflows = asEntries(workflow.workflows || workflow).map(normalizeWorkflow);
  const objectSlugs = new Set(objects.map((item) => item.slug));
  for (const link of links) if (!link.from || !link.to || !objectSlugs.has(link.from) || !objectSlugs.has(link.to)) parsed.diagnostics.push({ level: 'error', code: 'unknown_relation_object', message: `关系 ${link.name} 引用了未定义对象` });
  for (const action of actions) for (const mutation of action.mutations) {
    if (mutation.op === 'set' && (!mutation.object || !objects.find((item) => item.slug === mutation.object))) parsed.diagnostics.push({ level: 'error', code: 'unknown_action_object', message: `动作 ${action.name} 引用了未定义对象` });
    if (mutation.op === 'link' && !links.find((item) => item.slug === mutation.link)) parsed.diagnostics.push({ level: 'error', code: 'unknown_action_relation', message: `动作 ${action.name} 引用了未定义关系` });
    if (!['set', 'link'].includes(mutation.op)) parsed.diagnostics.push({ level: 'error', code: 'unknown_mutation', message: `动作 ${action.name} 含有未知变更类型` });
  }
  const files = app.modules?.includes('file') || app.file ? [{ name: 'File', slug: 'file', type: 'reference' }] : [];
  const description = text(app.description) || text(parsed.files.appMd.split(/\r?\n/).find((line) => line && !line.startsWith('#')));
  return {
    schema_version: 3,
    version: Number(app.version) || 1,
    name: text(app.name),
    description: description || '面向 Agent 的业务应用',
    app,
    ontology: { objects, relations: links },
    workflow: { workflows },
    actions: { actions },
    objects,
    links,
    actions,
    workflows,
    files,
    rules: Array.isArray(ontology.rules) ? ontology.rules : [],
    modules: app.modules || ['ontology', 'actions', 'workflow', 'template', 'static-resource', 'file', 'knowledge'],
    permissions: app.permissions || [],
    mcp: app.mcp || { enabled: true },
    diagnostics: parsed.diagnostics,
    compiled_at: new Date().toISOString()
  };
}

export const serializeDefinition = (definition) => {
  const files = definitionFiles(definition);
  return {
    'app.md': files.appMd,
    'app.yaml': typeof files.appYaml === 'string' ? files.appYaml : YAML.stringify(files.appYaml),
    'ontology.yaml': typeof files.ontologyYaml === 'string' ? files.ontologyYaml : YAML.stringify(files.ontologyYaml),
    'workflow.yaml': typeof files.workflowYaml === 'string' ? files.workflowYaml : YAML.stringify(files.workflowYaml),
    'actions.yaml': typeof files.actionsYaml === 'string' ? files.actionsYaml : YAML.stringify(files.actionsYaml)
  };
};

export function starterDefinition({ name, goal, concepts = [] }) {
  const names = concepts.length ? concepts : ['客户', '跟进'];
  const app = { name: slug(name), version: 1, description: goal || '让团队用自然语言管理业务数据。', runtime: { type: 'business-application' }, modules: ['ontology', 'actions', 'workflow', 'template', 'static-resource', 'file', 'knowledge'], mcp: { enabled: true }, permissions: [] };
  const objects = Object.fromEntries(names.map((item) => [slug(item), { label: item, description: `${item}业务对象`, fields: { name: { type: 'string', required: true }, status: { type: 'string' }, owner: { type: 'string' } } }]));
  return {
    appMd: `# ${name}\n\n## 简介\n${goal || '让团队用自然语言管理业务数据。'}\n\n## 核心流程\n请在 workflow.yaml 中定义业务流程，在 actions.yaml 中定义可执行的业务动作。\n`,
    appYaml: app,
    ontologyYaml: { objects, relations: {}, rules: [] },
    workflowYaml: { workflows: {} },
    actionsYaml: { actions: {} }
  };
}
