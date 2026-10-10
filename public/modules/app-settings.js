import { createCollectionResults } from "/modules/collection-results.js";
import { t } from "/modules/i18n.js";

export function createAppSettings({ state, api, $, esc, toast }) {
  const appURL = (path = '', requested = context) => `/api/apps/${encodeURIComponent(requested.appId)}${path}`;
  const results = createCollectionResults({ esc });
  const current = (requested) => requested && state.user?.id === requested.userId && state.app?.id === requested.appId && state.tenant?.id === requested.tenantId && state.appPanel === 'settings';
  let loadRevision = 0;
  let context = null;
  let busy = false;

  const pretty = (value) => JSON.stringify(value ?? {}, null, 2);

  async function load(renderAfter = true) {
    const root = $('#app-settings-root');
    if (!root) return false;
    root.innerHTML = `<p class="runtime-loading">${esc(t('正在读取发布与采集配置…'))}</p>`;
    const appId = state.app.id, tenantId = state.tenant.id, userId = state.user.id;
    const revision = ++loadRevision;
    const requested = { appId, tenantId, userId };
    const url = (path) => appURL(path, requested);
    const canManage = ["owner", "manager", "publisher"].includes(state.app.permission) || state.tenant.role === "owner";
    const [publication, scripts, tables, members, actions, workflows] = await Promise.all([
      canManage ? api(url('/publication')) : {}, api(url('/collection-scripts')),
      api(url('/collections')), api(url('/members')),
      api(url('/actions')), api(url('/workflows'))
    ]);
    if (!current(requested) || revision !== loadRevision) return false;
    context = { appId, tenantId, userId, publication, scripts, tables, members: members.members || [], actions, workflows };
    if (renderAfter) render();
    return true;
  }

  function render() {
    const { scripts } = context;
    $('#app-settings-root').innerHTML = `<header class="settings-heading"><h2>${esc(t('业务与采集配置'))}</h2><p>${esc(t('配置由模型自动处理，人类仅查看。'))}</p></header>
      ${renderBusinessSettings()}
      <section class="settings-block"><div class="settings-section-heading"><div><h3>${esc(t('采集脚本'))}</h3></div></div>
      <div class="settings-list">${scripts.map(renderScript).join('') || `<p class="settings-muted">${esc(t('还没有采集脚本。'))}</p>`}</div></section>`;
  }

  function renderScript(item) {
    return `<details class="settings-item" data-collection-script="${esc(item.id)}"><summary><strong>${esc(item.name)}</strong><span class="badge badge-ghost">${esc({ enabled: t('已启用'), paused: t('已暂停'), draft: t('草稿') }[item.status] || item.status)} · v${esc(item.revision)}</span></summary>${results.summary(item, context.members)}<pre class="settings-definition">${esc(pretty(item.definition))}</pre><div class="settings-actions"><button class="btn btn-ghost btn-sm" type="button" data-settings-action="runs-script" data-id="${esc(item.id)}">${esc(t('运行记录'))}</button></div><div data-script-output="${esc(item.id)}"></div></details>`;
  }

  function renderBusinessSettings() {
    return [['action', t('业务动作'), context.actions], ['workflow', t('状态流程'), context.workflows]].map(([kind, label, rows]) => `<section class="settings-block"><div class="settings-section-heading"><div><h3>${esc(label)}</h3></div></div>
      <div class="settings-list">${rows.map((item) => `<details class="settings-item"><summary><strong>${esc(item.name)}</strong><span class="badge badge-ghost">${esc({ enabled: t('已启用'), paused: t('已暂停'), draft: t('草稿') }[item.status] || item.status)} · v${esc(item.revision)}</span></summary>${item.description ? `<p class="settings-muted">${esc(item.description)}</p>` : ''}<pre class="settings-definition">${esc(pretty(item.definition))}</pre>${item.pause_reason ? `<p class="settings-muted">${esc(item.pause_reason)}</p>` : ''}</details>`).join('') || `<p class="settings-muted">${esc(t('还没有{label}。', { label }))}</p>`}</div></section>`).join('');
  }

  async function open() {
    if (!await load(false)) return;
    render();
  }

  async function click(event, requested) {
    const context = requested;
    const appURL = (path) => `/api/apps/${encodeURIComponent(requested.appId)}${path}`;
    const button = event.target.closest('[data-settings-action]');
    if (!button) return false;
    const id = button.dataset.id, revision = Number(button.dataset.revision);
    switch (button.dataset.settingsAction) {
      case 'runs-script': {
        const page = Math.max(1, Number(button.dataset.page) || 1);
        const result = await api(appURL(`/collection-scripts/${encodeURIComponent(id)}/runs?page=${page}`));
        if (current(requested)) showResult(id, results.history(id, result));
        break;
      }
      case 'view-script-run':
        await showCollectionRun(id, button.dataset.run, requested); break;
    }
    return true;
  }

  const once = async (event, operation) => {
    if (!event.target.closest(event.type === 'submit' ? '[data-settings-form]' : '[data-settings-action]')) return false;
    if (event.type === 'submit') event.preventDefault();
    if (busy) return true;
    if (!current(context)) throw new Error(t('应用上下文已切换，请重新打开业务配置。'));
    busy = true;
    $('#app-settings-root')?.setAttribute('aria-busy', 'true');
    const requested = context;
    const control = event.target.closest('button') || event.target.querySelector?.('button[type="submit"]');
    if (control) control.disabled = true;
    try { return await operation(event, requested); }
    catch (error) { if (current(requested)) throw error; return true; }
    finally { busy = false; if (control?.isConnected) control.disabled = false; $('#app-settings-root')?.removeAttribute('aria-busy'); }
  };
  function showResult(scriptId, html) {
    const output = $(`[data-script-output="${CSS.escape(scriptId)}"]`);
    if (output) output.innerHTML = html;
  }
  async function showCollectionRun(scriptId, runId, requested = context) {
    if (!current(requested)) return;
    const result = await api(appURL(`/collection-scripts/${encodeURIComponent(scriptId)}/runs/${encodeURIComponent(runId)}`, requested));
    if (!current(requested)) return;
    const details = $(`[data-collection-script="${CSS.escape(scriptId)}"]`);
    if (details) details.open = true;
    showResult(scriptId, results.run(result));
    details?.scrollIntoView({ block: 'nearest', behavior: 'instant' });
  }
  return { open, showCollectionRun, submit: () => false, click: (event) => once(event, click) };
}
