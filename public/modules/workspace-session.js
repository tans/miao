export function createWorkspaceSession({ state, $, clearAgent, resetAgentConversation }) {
  const storageKey = (userId, tenantId) => `miao_workspace_session:${userId}:${tenantId}`;

  function clear(userId, tenantId) {
    if (userId && tenantId) localStorage.removeItem(storageKey(userId, tenantId));
  }

  function persist() {
    if (!state.user?.id || !state.tenant?.id || state.workspaceView === 'edit') return;
    const view = ['app', 'assistant', 'management'].includes(state.workspaceView) ? state.workspaceView : 'home';
    const appId = state.workspaceView !== 'management' && state.app && (state.apps || []).some((item) => item.id === state.app.id) ? state.app.id : null;
    localStorage.setItem(storageKey(state.user.id, state.tenant.id), JSON.stringify({ version: 1, view, app_id: appId, app_panel: state.appPanel, management_page: state.workspaceManagementPage, nav_open: state.navOpen !== false, assistant_open: state.assistantOpen !== false }));
  }

  function restore() {
    clearAgent();
    resetAgentConversation();
    if ($('#record-dialog').open) $('#record-dialog').close();
    state.app = null;
    state.appRuntime = null;
    state.runtimeQuery = { page: 1, search: '' };
    state.appPanel = 'runtime';
    state.recordFormContext = null;
    state.editingRecordId = null;
    state.editingApp = false;
    state.tables = [];
    state.table = null;
    state.records = [];
    state.recordResult = null;
    state.recordQuery = { page: 1, perPage: 25, search: '', sort: '-created', filterField: '', filterValue: '' };
    state.workspaceView = 'home';
    state.assistantOpen = true;
    if (!state.user?.id || !state.tenant?.id) return;

    const key = storageKey(state.user.id, state.tenant.id);
    const serialized = localStorage.getItem(key);
    if (!serialized) return;
    let session;
    try { session = JSON.parse(serialized); }
    catch { clear(state.user.id, state.tenant.id); return; }
    if (session?.version !== 1 || !['home', 'app', 'assistant', 'management'].includes(session.view)) {
      clear(state.user.id, state.tenant.id);
      return;
    }
    const app = session.app_id ? (state.apps || []).find((item) => item.id === session.app_id) : null;
    if (session.app_id && !app) {
      clear(state.user.id, state.tenant.id);
      return;
    }
    if (session.view === 'app' && !app) {
      clear(state.user.id, state.tenant.id);
      return;
    }
    state.app = ['home', 'management'].includes(session.view) ? null : app;
    state.workspaceManagementPage = ['members', 'audit', 'settings'].includes(session.management_page) ? session.management_page : 'members';
    state.workspaceView = session.view === 'assistant' ? 'home' : session.view;
    if (session.nav_open !== undefined) state.navOpen = session.nav_open === true;
    if (session.assistant_open !== undefined) state.assistantOpen = session.assistant_open === true;
    if (session.view === 'app' && session.app_panel === 'tasks') state.appPanel = 'tasks';
  }

  return { clear, persist, restore };
}
