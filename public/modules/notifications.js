import { t, fmtDateTime } from '/modules/i18n.js';

export function createNotifications({ state, api, $, esc, toast, renderWorkspace, appTasks, appSettings }) {
  let items = [];
  let page = 1;
  let generation = 0;
  const filters = { read: '', kind: '', app_id: '' };
  const context = () => state.user?.id && state.tenant?.id ? `${state.user.id}:${state.tenant.id}` : '';
  let groups = [];
  function queryString() {
    const params = new URLSearchParams({ page: String(page) });
    for (const [key, value] of Object.entries(filters)) if (value) params.set(key, value);
    return params.toString();
  }
  async function load() {
    const scope = context();
    const revision = ++generation;
    const result = await api(`/api/notifications?${queryString()}`);
    if (!scope || scope !== context() || revision !== generation) return;
    items = result.items;
    groups = result.groups || [];
    $('#notification-list').innerHTML = items.map((item) => `<article class="task-card"><p>${esc(item.message)}</p><p class="task-muted">${esc(item.app_name || '')} · ${esc(t(`通知类型_${item.kind}`))} · ${esc(fmtDateTime(item.created_at))} · ${item.read ? esc(t('已读')) : esc(t('未读'))}</p><div class="task-buttons"><button class="btn btn-outline btn-sm" data-notification-open="${esc(item.id)}">${item.run_id ? esc(t('查看运行')) : esc(t('打开应用'))}</button>${item.read ? '' : `<button class="btn btn-ghost btn-sm" data-notification-read="${esc(item.id)}">${esc(t('标为已读'))}</button>`}</div></article>`).join('') || `<p class="task-muted">${esc(t('暂无可访问的通知。'))}</p>`;
    const appOptions = [`<option value="">${esc(t('全部应用'))}</option>`, ...groups.map((group) => `<option value="${esc(group.app_id)}" ${filters.app_id === group.app_id ? 'selected' : ''}>${esc(group.app_name || group.app_id)}${group.unread ? ` (${group.unread})` : ''}</option>`)].join('');
    $('#notification-toolbar').innerHTML = `<div class="notification-toolbar"><select class="select" data-notification-filter="app_id">${appOptions}</select><select class="select" data-notification-filter="kind"><option value="" ${filters.kind ? '' : 'selected'}>${esc(t('全部类型'))}</option><option value="task" ${filters.kind === 'task' ? 'selected' : ''}>${esc(t('通知类型_task'))}</option><option value="collection" ${filters.kind === 'collection' ? 'selected' : ''}>${esc(t('通知类型_collection'))}</option><option value="automation" ${filters.kind === 'automation' ? 'selected' : ''}>${esc(t('通知类型_automation'))}</option></select><select class="select" data-notification-filter="read"><option value="" ${filters.read ? '' : 'selected'}>${esc(t('全部状态'))}</option><option value="unread" ${filters.read === 'unread' ? 'selected' : ''}>${esc(t('未读'))}</option><option value="read" ${filters.read === 'read' ? 'selected' : ''}>${esc(t('已读'))}</option></select><button class="btn btn-ghost btn-sm" data-notification-read-all>${esc(t('全部已读'))}</button></div>`;
    $('#notification-pager').innerHTML = `<button class="btn btn-ghost btn-sm" data-notification-page="${page - 1}" ${page <= 1 ? 'disabled' : ''}>${esc(t('上一页'))}</button><span>${page} / ${Math.max(1, result.totalPages)}</span><button class="btn btn-ghost btn-sm" data-notification-page="${page + 1}" ${page >= result.totalPages ? 'disabled' : ''}>${esc(t('下一页'))}</button>`;
  }
  async function refresh() {
    const scope = context();
    if (!scope || document.hidden) return;
    try {
      const result = await api('/api/notifications?page=1');
      if (scope === context()) $('#notification-indicator').classList.toggle('active', result.items.some((item) => !item.read));
    } catch {}
  }
  setInterval(() => { if (!$('#workspace').classList.contains('hidden')) void refresh(); }, 30000);
  async function handleClick(event) {
    const action = event.target.closest('[data-action]')?.dataset.action;
    const read = event.target.closest('[data-notification-read]');
    const open = event.target.closest('[data-notification-open]');
    const next = event.target.closest('[data-notification-page]');
    const filter = event.target.closest('[data-notification-filter]');
    const readAll = event.target.closest('[data-notification-read-all]');
    if (!['open-notifications', 'close-notifications'].includes(action) && !read && !open && !next && !filter && !readAll) return false;
    try {
      if (action === 'close-notifications') { $('#notification-dialog').close(); return true; }
      if (action === 'open-notifications') { page = 1; await load(); if (context() && !$('#notification-dialog').open) $('#notification-dialog').showModal(); return true; }
      if (next) { page = Math.max(1, Number(next.dataset.notificationPage)); await load(); return true; }
      if (filter) { filters[filter.dataset.notificationFilter] = filter.value; page = 1; await load(); return true; }
      if (readAll) { await api('/api/notifications/read-all', { method: 'POST' }); page = 1; await load(); void refresh(); return true; }
      const item = items.find((row) => row.id === (open?.dataset.notificationOpen || read?.dataset.notificationRead));
      if (!item) return true;
      const scope = context();
      await api(`/api/notifications/${item.id}/read`, { method: 'POST' });
      if (scope !== context()) return true;
      if (read) { await load(); return true; }
      const application = await api(`/api/apps/${item.app_id}`);
      if (scope !== context()) return true;
      $('#notification-dialog').close();
      appTasks.reset();
      state.app = application;
      state.workspaceView = 'app';
      state.appPanel = item.run_id ? (item.kind === 'collection' ? 'settings' : 'tasks') : 'runtime';
      state.table = null;
      await renderWorkspace();
      if (item.run_id && scope === context()) {
        if (item.kind === 'collection' && item.script_id) {
          await appSettings.open();
          await appSettings.showCollectionRun(item.script_id, item.run_id);
        }
        else await appTasks.showRun(item.run_id);
      }
    } catch (error) { toast(error.message, true); }
    return true;
  }
  return { handleClick, refresh, reset() { generation++; filters.read = ''; filters.kind = ''; filters.app_id = ''; $('#notification-indicator').classList.remove('active'); items = []; groups = []; page = 1; $('#notification-dialog').close(); $('#notification-list').replaceChildren(); } };
}
