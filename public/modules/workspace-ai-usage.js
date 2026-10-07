import { renderUsageBreakdown, renderUsageRequestRows } from '/modules/ai-usage-view.js';
import { t } from '/modules/i18n.js';

export function createWorkspaceAIUsage({ state, api, $, esc, toast, renderWorkspace }) {
  let query = { page: 1, from: '', to: '', kind: '' };
  let generation = 0;
  async function open() {
    state.workspaceView = 'management';
    state.workspaceManagementPage = 'settings';
    state.app = null;
    state.table = null;
    await renderWorkspace();
    await load(true);
    $('#workspace-ai-usage').scrollIntoView({ block: 'start', behavior: 'smooth' });
  }

  async function load(reset = false) {
    const tenant = state.tenant?.id;
    const current = ++generation;
    const root = $('#workspace-ai-usage');
    if (reset || !query.from) {
      const today = new Date().toISOString().slice(0, 10);
      query = { page: 1, from: today, to: today, kind: '' };
    }
    root.classList.remove('hidden');
    root.textContent = t('正在读取分类用量…');
    const params = new URLSearchParams({ from: `${query.from}T00:00:00Z`, to: `${query.to}T23:59:59.999Z`, kind: query.kind, page: String(query.page), perPage: '25' });
    try {
      const [usage, details] = await Promise.all([api(`/api/workspace/ai-usage?${params}`), api(`/api/workspace/ai-usage/requests?${params}`)]);
      if (tenant !== state.tenant?.id || current !== generation) return;
      root.innerHTML = `<h2>${esc(t('AI 用量与预算'))}</h2><p class="admin-page-description">${esc(t('UTC 日期统计 · 用量不等同于账单；未返回 token 显示未知。'))}</p>
        <form data-workspace-usage-filter class="admin-filter-form"><label>${esc(t('开始日期'))}<input class="input input-sm" name="from" type="date" value="${esc(query.from)}" required /></label><label>${esc(t('结束日期'))}<input class="input input-sm" name="to" type="date" value="${esc(query.to)}" required /></label><label>${esc(t('服务类型'))}<select class="select select-sm" name="kind">${[['', t('全部')], ['llm', 'LLM'], ['jev', 'JEV'], ['unclassified', t('未分类')]].map(([kind, label]) => `<option value="${kind}" ${kind === query.kind ? 'selected' : ''}>${esc(label)}</option>`).join('')}</select></label><button class="btn btn-sm" type="submit">${esc(t('查询'))}</button></form>
        ${renderUsageBreakdown(usage.by_kind, esc)}
        <h3 class="ai-section-title">${esc(t('调用明细'))}</h3><div class="overflow-x-auto"><table class="table table-sm admin-table"><thead><tr><th>${esc(t('时间'))}</th><th>${esc(t('类型'))}</th><th>${esc(t('提供商 / 模型'))}</th><th>${esc(t('状态'))}</th><th>${esc(t('输入 tokens'))}</th><th>${esc(t('输出 tokens'))}</th><th>${esc(t('延迟'))}</th></tr></thead><tbody>${renderUsageRequestRows(details.items, esc)}</tbody></table></div>
        <div class="admin-pagination"><span>${t('第 {page} / {pages} 页 · 共 {total} 条', { page: details.page, pages: Math.max(1, details.totalPages), total: details.totalItems })}</span><div class="join"><button class="btn btn-sm join-item" data-workspace-usage-delta="-1" ${details.page <= 1 ? 'disabled' : ''}>${esc(t('上一页'))}</button><button class="btn btn-sm join-item" data-workspace-usage-delta="1" ${details.page >= details.totalPages ? 'disabled' : ''}>${esc(t('下一页'))}</button></div></div>
        <h3 class="ai-section-title">${esc(t('每日请求预算'))}</h3><p class="admin-page-description">${esc(t('0 不限制。总上限和分类上限同时生效；失败请求和连接检查也计数。'))}</p>
        <form data-workspace-ai-budget class="ai-budget-grid">${[['daily_limit', t('总请求上限')], ['llm_daily_limit', t('LLM 上限')], ['jev_daily_limit', t('JEV 上限')]].map(([name, label]) => `<label>${esc(label)}<input class="input" name="${name}" type="number" min="0" max="100000" step="1" required value="${usage[name]}" ${usage.can_manage ? '' : 'disabled'} /></label>`).join('')}<button class="btn btn-primary btn-sm" type="submit" ${usage.can_manage ? '' : 'hidden'}>${esc(t('保存预算'))}</button></form>`;
    } catch (error) {
      if (tenant !== state.tenant?.id || current !== generation) return;
      root.innerHTML = `<div class="alert alert-error">${esc(error.message)}</div><button class="btn btn-sm" data-workspace-usage-retry>${esc(t('重新读取'))}</button>`;
    }
  }

  function bind() {
    const root = $('#workspace-ai-usage');
    root.addEventListener('submit', async (event) => {
      event.preventDefault();
      const form = event.target;
      const values = Object.fromEntries(new FormData(form));
      if (form.matches('[data-workspace-usage-filter]')) { query = { ...values, page: 1 }; await load(); return; }
      if (!form.matches('[data-workspace-ai-budget]')) return;
      const button = form.querySelector('[type="submit"]');
      button.disabled = true;
      try {
        await api('/api/workspace/ai-budget', { method: 'PATCH', body: JSON.stringify(Object.fromEntries(Object.entries(values).map(([name, value]) => [name, Number(value)]))) });
        toast(t('分类预算已保存'));
        await load();
      } catch (error) { toast(error.message, true); button.disabled = false; }
    });
    root.addEventListener('click', (event) => {
      const pager = event.target.closest('[data-workspace-usage-delta]');
      if (pager && !pager.disabled) { query.page = Math.max(1, query.page + Number(pager.dataset.workspaceUsageDelta)); load(); }
      if (event.target.closest('[data-workspace-usage-retry]')) load();
    });
  }

  function reset() { generation++; $('#workspace-ai-usage').classList.add('hidden'); $('#workspace-ai-usage').replaceChildren(); }
  return { open, bind, reset };
}
