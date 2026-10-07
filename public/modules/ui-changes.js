// Shared renderer for controlled UI definition diffs (preview and dialogue review).
export function formatUIChanges(changes, esc) {
  const labels = { title: '应用标题', page_order: '页面顺序', add_page: '新增页面', remove_page: '删除页面', page_title: '页面标题', add_source: '新增数据绑定', remove_source: '删除数据绑定', source_collection: '绑定数据表', source_fields: '展示字段与顺序', source_form_fields: '表单字段与顺序', source_actions: '记录动作', source_query: '筛选与排序', source_context: '当前详情关联范围', root: '根组件', add_component: '新增组件', remove_component: '删除组件', component_type: '替换组件', component_props: '组件内容与绑定', component_order: '组件顺序与归属' };
  return changes.length ? `<ol class="ui-change-list">${changes.map((change) => `<li><strong>${esc(labels[change.type] || change.type)}</strong><span>${esc([change.page, change.resource].filter(Boolean).join(' / '))}</span><div><span>修改前</span><pre>${esc(JSON.stringify(change.before ?? null, null, 2))}</pre><span>修改后</span><pre>${esc(JSON.stringify(change.after ?? null, null, 2))}</pre></div></li>`).join('')}</ol>` : '<p>没有界面定义差异。</p>';
}

