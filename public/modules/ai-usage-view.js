import { t, fmtDateTime, fmtNumber } from '/modules/i18n.js';

export function renderUsageBreakdown(byKind, esc) {
  const rows = ['llm', 'jev', 'unclassified'].map((kind) => {
    const value = byKind?.[kind] || {};
    const label = kind === 'unclassified' ? t('未分类') : kind.toUpperCase();
    const tokens = (name) => `${fmtNumber(value[`${name}_tokens`] || 0)}${value[`${name}_unknown`] ? t(' + {count} 次未知', { count: value[`${name}_unknown`] }) : ''}`;
    return `<tr><th>${esc(label)}</th><td>${value.requests || 0}</td><td>${value.successes || 0} / ${value.errors || 0} / ${value.pending || 0}</td><td>${tokens('input')}</td><td>${tokens('output')}</td></tr>`;
  }).join('');
  return `<div class="overflow-x-auto"><table class="table table-sm admin-table"><thead><tr><th>${esc(t('类型'))}</th><th>${esc(t('请求'))}</th><th>${esc(t('成功 / 失败 / 未完成'))}</th><th>${esc(t('输入 tokens'))}</th><th>${esc(t('输出 tokens'))}</th></tr></thead><tbody>${rows}</tbody></table></div>`;
}

export function renderUsageRequestRows(items, esc, platform = false) {
  return items.length ? items.map((row) => `<tr><td>${esc(fmtDateTime(row.created_at.replace(' ', 'T')))}${platform ? `<small class="admin-cell-secondary">${esc(row.tenant?.name || t('工作区已删除'))}</small>` : ''}</td><td>${row.kind === 'unclassified' ? esc(t('未分类')) : esc(row.kind.toUpperCase())}</td><td>${esc(row.provider || t('未知'))}<small class="admin-cell-secondary">${esc(row.model || t('未知'))}</small></td><td>${row.status >= 300 ? esc(t('失败')) : row.status >= 200 ? esc(t('成功')) : esc(t('未完成'))} · ${Number(row.status)}</td><td>${row.input_known ? fmtNumber(row.input_tokens || 0) : t('未知')}</td><td>${row.output_known ? fmtNumber(row.output_tokens || 0) : t('未知')}</td><td>${row.latency_ms ? `${fmtNumber(row.latency_ms)} ms` : '—'}</td></tr>`).join('') : `<tr><td colspan="7">${esc(t('此范围没有调用记录。'))}</td></tr>`;
}
