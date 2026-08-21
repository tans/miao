const headingSection = (source, titles) => {
  const pattern = titles.map((title) => title.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|');
  const match = new RegExp(`^##\\s+(?:${pattern})\\s*$`, 'mi').exec(source);
  if (!match) return '';
  const rest = source.slice(match.index + match[0].length);
  const next = /^##\s+/mi.exec(rest);
  return rest.slice(0, next ? next.index : rest.length).trim();
};

const bullets = (text) => text.split(/\r?\n/).map((line) => line.match(/^[-*]\s+(.+)/)?.[1]?.trim()).filter(Boolean);
export const slug = (text) => String(text).toLowerCase().replace(/[^a-z0-9\u4e00-\u9fa5]+/g, '_').replace(/^_|_$/g, '').slice(0, 48) || 'records';

const splitSubsections = (text) => {
  const matches = [...text.matchAll(/^###\s+(.+)$/gim)];
  return matches.map((match, index) => ({ name: match[1].trim(), body: text.slice(match.index + match[0].length, matches[index + 1]?.index ?? text.length).trim() }));
};

const block = (text, title) => {
  const match = new RegExp(`^${title}:?\\s*$`, 'mi').exec(text);
  if (!match) return '';
  const rest = text.slice(match.index + match[0].length);
  const next = /^(?:Properties|Input|Rules|Mutations|Process|Effect):?\s*$/mi.exec(rest);
  return rest.slice(0, next ? next.index : rest.length).trim();
};

const parseType = (spec) => {
  const value = String(spec || '').trim();
  const enumMatch = value.match(/^enum\s*\[([^\]]+)\]/i);
  if (enumMatch) return { type: 'enum', values: enumMatch[1].split(',').map((item) => item.trim()).filter(Boolean) };
  const refMatch = value.match(/^(?:ref|object_ref)\s+([\w\u4e00-\u9fa5_-]+)/i);
  if (refMatch) return { type: 'object_ref', object: slug(refMatch[1]) };
  return { type: value.replace(/\s+required\b/i, '').trim() || 'string' };
};

const parseProperties = (body) => {
  const source = block(body, 'Properties') || body;
  return Object.fromEntries(bullets(source).map((line) => {
    const [rawName, ...rest] = line.split(':');
    const name = rawName.trim();
    const spec = rest.join(':').trim();
    const required = /\brequired\b/i.test(spec);
    return [slug(name), { name, ...parseType(spec), required }];
  }).filter(([name]) => name));
};

const parseInput = (body) => parseProperties(block(body, 'Input'));

const parseRules = (body) => bullets(block(body, 'Rules')).map((line) => {
  const required = line.match(/^required\s*:\s*(.+)$/i);
  if (required) return { op: 'required', property: slug(required[1]) };
  const greater = line.match(/^greater_than\s*:\s*([^\s]+)\s+(.+)$/i);
  if (greater) return { op: 'greater_than', property: slug(greater[1]), value: Number.isNaN(Number(greater[2])) ? greater[2] : Number(greater[2]) };
  const state = line.match(/^state\s*:\s*([^\s]+)\s+(.+)$/i);
  if (state) return { op: 'state', property: slug(state[1]), value: state[2].trim() };
  return { op: 'description', description: line };
});

const parseMutations = (body) => bullets(block(body, 'Mutations') || block(body, 'Effect')).map((line) => {
  const set = line.match(/^set\s*:\s*([^\.\s]+)\.([^=\s]+)\s*=\s*(.+)$/i);
  if (set) return { op: 'set', object: slug(set[1]), property: slug(set[2]), value: set[3].trim() };
  const link = line.match(/^(?:link|create_link)\s*:\s*(\S+)\s+from\s+(\S+)\s+to\s+(\S+)$/i);
  if (link) return { op: 'link', link: slug(link[1]), from: link[2], to: link[3] };
  return { op: 'description', description: line };
});

const parseObjectSection = (source, fallback = []) => {
  const strict = splitSubsections(headingSection(source, ['Objects', '对象']));
  if (strict.length) return strict.map(({ name, body }) => ({ name, slug: slug(name), properties: parseProperties(body) }));
  const legacy = splitSubsections(headingSection(source, ['业务概念', 'Business concepts']));
  if (legacy.length) return legacy.map(({ name, body }) => ({ name, slug: slug(name), properties: Object.fromEntries(bullets(body).map((field) => [slug(field), { name: field, type: 'string', required: false }])) }));
  return fallback.map((name) => ({ name, slug: slug(name), properties: {} }));
};

const parseLinks = (source) => {
  const text = headingSection(source, ['Links', '关系']);
  return bullets(text).map((line) => {
    const strict = line.match(/^(\S+)\s+(?:has|contains|matches|from|to|->)\s+(\S+)$/i);
    if (strict) return { slug: slug(`${strict[1]}_${strict[2]}`), name: line, from: slug(strict[1]), to: slug(strict[2]), cardinality: 'many' };
    const parts = line.split(/\s*(?:->|:)\s*/);
    return parts.length === 2 ? { slug: slug(parts[0]), name: line, from: slug(parts[0]), to: slug(parts[1]), cardinality: 'many' } : null;
  }).filter(Boolean);
};

const parseActions = (source) => {
  const strict = splitSubsections(headingSection(source, ['Actions', '动作']));
  if (strict.length) return strict.map(({ name, body }) => ({ name, slug: slug(name), description: name, input: parseInput(body), rules: parseRules(body), mutations: parseMutations(body) }));
  return bullets(headingSection(source, ['可以做的事情', 'Actions'])).map((name) => ({ name, slug: slug(name), description: name, input: {}, rules: [], mutations: [] }));
};

export function compileSource(source, fallback = {}) {
  const objects = parseObjectSection(source, fallback.concepts || ['业务记录']);
  const actions = parseActions(source);
  const links = parseLinks(source);
  const pages = bullets(headingSection(source, ['页面', 'Pages'])).map((name) => ({ name, slug: slug(name), type: 'list' }));
  const diagnostics = [];
  const objectSlugs = new Set(objects.map((item) => item.slug));
  for (const link of links) if (!objectSlugs.has(link.from) || !objectSlugs.has(link.to)) diagnostics.push({ level: 'error', code: 'unknown_link_object', message: `关系 ${link.name} 引用了未定义对象` });
  for (const action of actions) if (action.mutations.some((item) => item.op === 'description')) diagnostics.push({ level: 'warning', code: 'uncompiled_mutation', message: `动作 ${action.name} 包含未编译的变更描述` });
  return {
    schema_version: 2,
    version: 2,
    description: fallback.description || headingSection(source, ['目标', 'Goal']).split(/\r?\n/).find((line) => line.trim()) || '面向 Agent 的业务应用',
    objects,
    links,
    actions,
    files: [{ name: 'File', slug: 'file', type: 'reference' }],
    rules: bullets(headingSection(source, ['业务规则', 'Rules'])),
    pages: pages.length ? pages : [{ name: '工作台', slug: 'home', type: 'home' }, { name: '对象列表', slug: 'objects', type: 'list' }],
    collections: objects.map((item) => ({ name: item.name, slug: item.slug, fields: Object.values(item.properties).map((field) => field.name) })),
    diagnostics,
    compiled_at: new Date().toISOString()
  };
}

export function starterSource({ name, goal, concepts = [] }) {
  const names = concepts.length ? concepts : ['客户', '跟进'];
  const objectBlocks = names.map((item) => `### ${item}\nProperties:\n- name: string required\n- status: string\n- owner: string`).join('\n\n');
  return `# ${name}\n\n## 目标\n${goal || '让团队用自然语言管理业务数据。'}\n\n## Objects\n${objectBlocks}\n\n## Links\n\n## Actions\n\n### activate_${slug(names[0])}\nInput:\n- object_id: object_ref ${slug(names[0])} required\nMutations:\n- set: ${slug(names[0])}.status = active\n\n## Files\n- 原始文件保留，并通过 file_ref 关联到业务对象。\n\n## 页面\n- 工作台\n- 对象列表\n- 文件中心\n- 历史记录\n`;
}
