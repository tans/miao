export function renderUsageBreakdown(byKind, esc) {
  const rows = ['llm', 'jev', 'unclassified'].map((kind) => {
    const value = byKind?.[kind] || {};
    const label = kind === 'unclassified' ? '未分类' : kind.toUpperCase();
    const tokens = (name) => `${Number(value[`${name}_tokens`] || 0).toLocaleString()}${value[`${name}_unknown`] ? ` + ${value[`${name}_unknown`]} 次未知` : ''}`;
    return `<tr><th>${esc(label)}</th><td>${value.requests || 0}</td><td>${value.successes || 0} / ${value.errors || 0} / ${value.pending || 0}</td><td>${tokens('input')}</td><td>${tokens('output')}</td></tr>`;
  }).join('');
  return `<div class="overflow-x-auto"><table class="table table-sm admin-table"><thead><tr><th>类型</th><th>请求</th><th>成功 / 失败 / 未完成</th><th>输入 tokens</th><th>输出 tokens</th></tr></thead><tbody>${rows}</tbody></table></div>`;
}

export function renderUsageRequestRows(items, esc, platform = false) {
  return items.length ? items.map((row) => `<tr><td>${esc(new Date(row.created_at.replace(' ', 'T')).toLocaleString())}${platform ? `<small class="admin-cell-secondary">${esc(row.tenant?.name || '空间已删除')}</small>` : ''}</td><td>${row.kind === 'unclassified' ? '未分类' : esc(row.kind.toUpperCase())}</td><td>${esc(row.provider || '未知')}<small class="admin-cell-secondary">${esc(row.model || '未知')}</small></td><td>${row.status >= 300 ? '失败' : row.status >= 200 ? '成功' : '未完成'} · ${Number(row.status)}</td><td>${row.input_known ? Number(row.input_tokens || 0).toLocaleString() : '未知'}</td><td>${row.output_known ? Number(row.output_tokens || 0).toLocaleString() : '未知'}</td><td>${row.latency_ms ? `${Number(row.latency_ms).toLocaleString()} ms` : '—'}</td></tr>`).join('') : '<tr><td colspan="7">此范围没有调用记录。</td></tr>';
}
