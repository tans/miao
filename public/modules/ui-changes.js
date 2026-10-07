// Shared renderer for controlled UI definition diffs (preview and dialogue review).
import { t } from '/modules/i18n.js';

export function formatUIChanges(changes, esc) {
  const labels = { title: t('应用标题'), page_order: t('页面顺序'), add_page: t('新增页面'), remove_page: t('删除页面'), page_title: t('页面标题'), add_source: t('新增数据绑定'), remove_source: t('删除数据绑定'), source_collection: t('绑定数据表'), source_fields: t('展示字段与顺序'), source_form_fields: t('表单字段与顺序'), source_actions: t('记录动作'), source_query: t('筛选与排序'), source_context: t('当前详情关联范围'), root: t('根组件'), add_component: t('新增组件'), remove_component: t('删除组件'), component_type: t('替换组件'), component_props: t('组件内容与绑定'), component_order: t('组件顺序与归属') };
  return changes.length ? `<ol class="ui-change-list">${changes.map((change) => `<li><strong>${esc(labels[change.type] || change.type)}</strong><span>${esc([change.page, change.resource].filter(Boolean).join(' / '))}</span><div><span>${esc(t('修改前'))}</span><pre>${esc(JSON.stringify(change.before ?? null, null, 2))}</pre><span>${esc(t('修改后'))}</span><pre>${esc(JSON.stringify(change.after ?? null, null, 2))}</pre></div></li>`).join('')}</ol>` : `<p>${esc(t('没有界面定义差异。'))}</p>`;
}

