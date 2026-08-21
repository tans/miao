const section = (source, title) => {
  const match = source.match(new RegExp(`^##\\s+${title}\\s*$([\\s\\S]*?)(?=^##\\s|$)`, 'mi'));
  return match?.[1]?.trim() || '';
};
const bullets = (text) => text.split(/\r?\n/).map((line) => line.match(/^[-*]\s+(.+)/)?.[1]?.trim()).filter(Boolean);
const slug = (text) => text.toLowerCase().replace(/[^a-z0-9\u4e00-\u9fa5]+/g, '_').replace(/^_|_$/g, '').slice(0, 48) || 'records';

export function compileSource(source, fallback = {}) {
  const concepts = [];
  const conceptSection = section(source, '业务概念') || section(source, 'Business concepts');
  const headings = conceptSection.match(/^###\s+(.+)$/gim) || [];
  for (const line of headings) concepts.push(line.replace(/^###\s+/, '').trim());
  for (const item of bullets(conceptSection)) if (!concepts.includes(item) && item.length < 40) concepts.push(item);
  const collections = [...new Set((concepts.length ? concepts : (fallback.concepts || ['业务记录'])).map((name) => ({ name, slug: slug(name), fields: ['名称', '状态', '负责人', '备注'] }))];
  const actionSection = section(source, '可以做的事情') || section(source, 'Actions');
  const actions = bullets(actionSection).map((name) => ({ name, slug: slug(name), description: name }));
  const pageSection = section(source, '页面') || section(source, 'Pages');
  const pages = bullets(pageSection).map((name) => ({ name, slug: slug(name), type: 'list' }));
  return {
    version: 1,
    description: fallback.description || source.split(/\r?\n/).find((line) => line && !line.startsWith('#')) || '面向 Agent 的业务应用',
    collections: collections.length ? collections : [{ name: '业务记录', slug: 'records', fields: ['名称', '状态', '负责人', '备注'] }],
    actions: actions.length ? actions : [{ name: '新增记录', slug: 'create_record', description: '创建一条业务记录' }, { name: '搜索记录', slug: 'search_records', description: '按关键词查询业务记录' }],
    pages: pages.length ? pages : [{ name: '工作台', slug: 'home', type: 'home' }, { name: '数据列表', slug: 'records', type: 'list' }],
    rules: bullets(section(source, '业务规则')),
    compiled_at: new Date().toISOString()
  };
}

export function starterSource({ name, goal, concepts = [] }) {
  const conceptLines = (concepts.length ? concepts : ['客户', '跟进']).map((item) => `### ${item}\n- 名称\n- 状态\n- 负责人\n- 备注`).join('\n\n');
  return `# ${name}\n\n## 目标\n${goal || '让团队用自然语言管理业务数据。'}\n\n## 业务概念\n${conceptLines}\n\n## 数据规则\n- 重要记录保留来源和更新时间。\n- 删除默认使用软删除。\n\n## 文件\n- 支持上传 CSV、Excel、PDF、DOCX、图片和 Markdown。\n- 导入前展示预览，导入后保留原始文件。\n\n## 业务规则\n- 所有关键变更记录到历史。\n\n## 可以做的事情\n- 新增记录\n- 搜索记录\n- 导入文件\n- 导出数据\n\n## 页面\n- 工作台\n- 数据列表\n- 文件中心\n- 历史记录\n\n## 历史与追溯\n所有操作记录操作者、时间和来源。\n\n## 不允许破坏的规则\n- 不跨租户读取数据。\n- 不物理删除原始文件。\n`;
}
