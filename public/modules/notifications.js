export function createNotifications({ state, api, $, esc, toast, renderWorkspace, appTasks }) {
  let items = [];
  let page = 1;
  let generation = 0;
  const context = () => state.user?.id && state.tenant?.id ? `${state.user.id}:${state.tenant.id}` : '';
  async function load() {
    const scope = context();
    const revision = ++generation;
    const result = await api(`/api/notifications?page=${page}`);
    if (!scope || scope !== context() || revision !== generation) return;
    items = result.items;
    $('#notification-list').innerHTML = items.map((item) => `<article class="task-card"><p>${esc(item.message)}</p><p class="task-muted">${esc(new Date(item.created_at).toLocaleString())} · ${item.read ? '已读' : '未读'}</p><div class="task-buttons"><button class="btn btn-outline btn-sm" data-notification-open="${esc(item.id)}">${item.run_id ? '查看运行' : '打开应用'}</button>${item.read ? '' : `<button class="btn btn-ghost btn-sm" data-notification-read="${esc(item.id)}">标为已读</button>`}</div></article>`).join('') || '<p class="task-muted">暂无可访问的通知。</p>';
    $('#notification-pager').innerHTML = `<button class="btn btn-ghost btn-sm" data-notification-page="${page - 1}" ${page <= 1 ? 'disabled' : ''}>上一页</button><span>${page} / ${Math.max(1, result.totalPages)}</span><button class="btn btn-ghost btn-sm" data-notification-page="${page + 1}" ${page >= result.totalPages ? 'disabled' : ''}>下一页</button>`;
  }
  async function refresh() {
    const scope = context();
    if (!scope || document.hidden) return;
    try {
      const result = await api('/api/notifications?page=1');
      if (scope === context()) $('#notification-indicator').textContent = result.items.some((item) => !item.read) ? ' · 有新通知' : '';
    } catch {}
  }
  setInterval(() => { if (!$('#workspace').classList.contains('hidden')) void refresh(); }, 30000);
  async function handleClick(event) {
    const action = event.target.closest('[data-action]')?.dataset.action;
    const read = event.target.closest('[data-notification-read]');
    const open = event.target.closest('[data-notification-open]');
    const next = event.target.closest('[data-notification-page]');
    if (!['open-notifications', 'close-notifications'].includes(action) && !read && !open && !next) return false;
    try {
      if (action === 'close-notifications') { $('#notification-dialog').close(); return true; }
      if (action === 'open-notifications') { page = 1; await load(); if (context() && !$('#notification-dialog').open) $('#notification-dialog').showModal(); return true; }
      if (next) { page = Math.max(1, Number(next.dataset.notificationPage)); await load(); return true; }
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
      state.appPanel = item.run_id ? 'tasks' : 'runtime';
      state.table = null;
      await renderWorkspace();
      if (item.run_id && scope === context()) await appTasks.showRun(item.run_id);
    } catch (error) { toast(error.message, true); }
    return true;
  }
  return { handleClick, refresh, reset() { generation++; $('#notification-indicator').textContent = ''; items = []; page = 1; $('#notification-dialog').close(); $('#notification-list').replaceChildren(); } };
}
