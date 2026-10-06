import { createPlatformAdmin } from '/modules/platform-admin.js';
import { createAppRuntime } from '/modules/app-runtime.js';
import { createAgentAssistant } from '/modules/agent-assistant.js';
import { createWorkspaceData } from '/modules/workspace-data.js';
import { createWorkspaceSession } from '/modules/workspace-session.js';
import { createNotifications } from '/modules/notifications.js';
import { createAppTasks } from '/modules/app-tasks.js';
import { createAppSettings } from '/modules/app-settings.js';

const TOKEN_KEY = 'miao_token';
const state = {
  token: localStorage.getItem(TOKEN_KEY), user: null, tenant: null,
  workspaces: [], apps: [], archivedApps: [], app: null, tables: [], table: null, records: [],
  appRuntime: null, runtimeQuery: { page: 1, search: '' }, recordFormContext: null,
  recordQuery: { page: 1, perPage: 25, search: '', sort: '-created', filterField: '', filterValue: '' }, recordResult: null,
  editingRecordId: null, editingApp: false, workspaceView: 'home', appPanel: 'runtime',
  authMode: 'login', agentBusy: false, isPlatformAdmin: false,
  agentConversationMessages: [], agentConversationRevision: 0,
  agentConversationLoadedKey: null, agentRun: null,
  agentTurnNumber: 0, agentPersistenceConflict: false,
  workspaceAuditPage: 1, workspaceManagementPage: 'members',
  aiConfigured: false, pendingInvite: new URLSearchParams(location.search).get('invite')
};
const $ = (selector) => document.querySelector(selector);
const $$ = (selector) => [...document.querySelectorAll(selector)];
const esc = (value) => String(value ?? '').replace(/[&<>"']/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' }[char]));
const appSupports = (app, capability) => Array.isArray(app?.capabilities) && app.capabilities.includes(capability);
const toast = (message, error = false) => {
  const node = $('#toast');
  node.innerHTML = `<div class="alert ${error ? 'alert-error' : 'alert-success'}"><span>${esc(message)}</span></div>`;
  setTimeout(() => { node.replaceChildren(); }, 3000);
};

async function api(url, options = {}) {
  const requestToken = state.token;
  const headers = { ...(options.body instanceof FormData ? {} : options.body ? { 'Content-Type': 'application/json' } : {}), ...(options.headers || {}) };
  if (state.token) headers.Authorization = `Bearer ${state.token}`;
  if (state.tenant?.id && !headers['X-Miao-Tenant-Id']) headers['X-Miao-Tenant-Id'] = state.tenant.id;
  const response = await fetch(url, { ...options, headers });
  const nextToken = response.headers.get('X-PocketBase-Token');
  if (nextToken && state.token === requestToken) {
    state.token = nextToken;
    localStorage.setItem(TOKEN_KEY, nextToken);
  }
  const result = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(result.error || result.message || '请求失败，请稍后重试');
  return result;
}

async function loadCurrentUser() {
  const preferredTenantId = localStorage.getItem('miao_workspace');
  try {
    return await api('/api/me', { headers: preferredTenantId ? { 'X-Miao-Tenant-Id': preferredTenantId } : {} });
  } catch (error) {
    if (!preferredTenantId) throw error;
    localStorage.removeItem('miao_workspace');
    const me = await api('/api/me');
    workspaceSession.clear(me.user.id, preferredTenantId);
    return me;
  }
}

function show(screen) {
  for (const id of ['auth', 'workspace', 'platform-admin']) $(`#${id}`).classList.toggle('hidden', id !== screen);
}


function authMode(mode) {
  state.authMode = mode;
  const registering = mode === 'register';
  $('#auth-title').textContent = registering ? '创建工作区' : '登录';
  if (state.pendingInvite) $('#auth-copy').textContent = '你收到了工作区邀请。请使用受邀邮箱登录或注册，完成后即可加入。';
  $('#auth-submit').textContent = registering ? '创建账号' : '登录';
  $('#name-field').classList.toggle('hidden', !registering);
  $('#name-field input').required = registering;
  $('#auth-form [name="password"]').autocomplete = registering ? 'new-password' : 'current-password';
  $('#auth-switch-copy').textContent = registering ? '已有账号？' : '还没有账号？';
  $('#switch-auth').textContent = registering ? '登录' : '创建账号';
  $('#auth-form').classList.remove('hidden');
  $('#request-reset-form').classList.add('hidden');
  $('#switch-auth').closest('.auth-switch').classList.remove('hidden');
  $('[data-action="forgot-password"]').classList.remove('hidden');
  show('auth');
}

function clearAgent() {
  state.agentBusy = false;
  state.agentConversationMessages = [];
  state.agentConversationRevision = 0;
  state.agentConversationLoadedKey = null;
  state.agentRun = null;
  state.agentTurnNumber = 0;
  state.agentPersistenceConflict = false;
}

function resetAgentConversation() {
  $('#chat-messages').innerHTML = '<div class="assistant-intro"><img src="/mascots/cat-peek-square.png" alt="" /><div><h3>你想做什么？</h3><div class="conversation-prompts"><button class="btn btn-outline btn-sm" data-prompt="帮我梳理每周团队周报的收集和汇总流程">梳理工作流程</button><button class="btn btn-outline btn-sm" data-prompt="我想做一个客户跟进流程，先帮我想清楚怎么开始">从一个想法开始</button></div></div></div>';
  $('#agent-status').textContent = '准备开始';
  $('#agent-status').className = 'badge badge-ghost';
}

const workspaceSession = createWorkspaceSession({ state, $, clearAgent, resetAgentConversation });

async function logout() {
  $('#invite-link').value = '';
  $('#invite-link-row').classList.add('hidden');
  notifications.reset();
  appTasks.reset();
  const logoutRequest = state.token ? api('/api/auth/logout', { method: 'POST' }).catch(() => {}) : Promise.resolve();
  let storageError;
  try { await agentAssistant.clearSavedConversations(); } catch (error) { storageError = error; }
  await logoutRequest;
  if ($('#record-dialog').open) $('#record-dialog').close();
  state.recordFormContext = null;
  clearAgent();
  resetAgentConversation();
  state.token = null;
  state.apps = [];
  state.workspaces = [];
  state.app = null;
  state.appRuntime = null;
  state.runtimeQuery = { page: 1, search: '' };
  state.appPanel = 'runtime';
  state.tenant = null;
  state.tables = [];
  state.table = null;
  state.records = [];
  state.recordResult = null;
  state.isPlatformAdmin = false;
  localStorage.removeItem(TOKEN_KEY);
  history.replaceState({}, '', '/');
  authMode('login');
  if (storageError) toast(`已退出登录，但服务器会话未能清理：${storageError.message || '存储不可用'}`, true);
}

async function bootstrap() {
  if (location.pathname === '/verify-email') {
    try {
      await api('/api/auth/verify-email', { method: 'POST', body: JSON.stringify({ token: new URLSearchParams(location.search).get('token') }) });
      state.pendingInvite = new URLSearchParams(location.search).get('invite');
      history.replaceState({}, '', state.pendingInvite ? `/?invite=${encodeURIComponent(state.pendingInvite)}` : '/');
      toast('邮箱验证完成，可以登录了');
      if (state.pendingInvite) return authMode('login');
    } catch (error) { toast(error.message, true); }
    return authMode('login');
  }
  if (location.pathname === '/reset-password') {
    state.resetToken = new URLSearchParams(location.search).get('token');
    authMode('login');
    $('#password-reset-dialog').showModal();
    return;
  }
  if (!state.token) return authMode('login');
  try {
    if (state.pendingInvite) {
      try {
        const accepted = await api('/api/invites/accept', { method: 'POST', body: JSON.stringify({ token: state.pendingInvite }) });
        state.tenant = accepted.tenant;
        localStorage.setItem('miao_workspace', accepted.tenant.id);
        state.pendingInvite = null;
        history.replaceState({}, '', location.pathname);
        toast('已加入工作区');
      } catch (error) {
        toast(error.message, true);
      }
    }
    const me = await loadCurrentUser();
    state.user = me.user;
    state.tenant = me.tenant;
    if (state.tenant?.id) localStorage.setItem('miao_workspace', state.tenant.id);
    state.workspaces = me.workspaces || [];
    state.aiConfigured = me.ai_configured;
    state.isPlatformAdmin = me.is_platform_admin;
    state.apps = me.apps || [];
    state.archivedApps = await api('/api/apps?archived=true').catch(() => []);
    state.app = null;
    state.appPanel = 'runtime';
    state.table = null;
    const requestedAdminPage = platformAdmin.routePage();
    if (requestedAdminPage && state.isPlatformAdmin) await platformAdmin.open(requestedAdminPage, false);
    else {
      if (requestedAdminPage) history.replaceState({}, '', '/');
      notifications.reset();
  workspaceSession.restore();
      if (state.workspaceView === 'assistant') await agentAssistant.enterConversation();
      show('workspace');
      await renderWorkspace();
    }
  } catch {
    logout();
  }
}

async function submitAuth(event) {
  event.preventDefault();
  const form = new FormData(event.target);
  const registering = state.authMode === 'register';
  const path = registering ? '/api/auth/register' : '/api/auth/login';
  const payload = Object.fromEntries(form);
  if (registering && state.pendingInvite) payload.invite_token = state.pendingInvite;
  try {
    const result = await api(path, { method: 'POST', body: JSON.stringify(payload) });
    if (result.requires_verification) {
      toast(`验证邮件已发送到 ${result.email}`);
      $('#auth-form').reset();
      return;
    }
    state.token = result.token;
    localStorage.setItem(TOKEN_KEY, result.token);
    state.user = result.user;
    state.tenant = result.tenant;
    if (state.pendingInvite) {
      try {
        const accepted = await api('/api/invites/accept', { method: 'POST', body: JSON.stringify({ token: state.pendingInvite }) });
        state.tenant = accepted.tenant;
        localStorage.setItem('miao_workspace', accepted.tenant.id);
        state.pendingInvite = null;
        history.replaceState({}, '', location.pathname);
        toast('已加入工作区');
      } catch (error) {
        toast(error.message, true);
      }
    }
    const me = await loadCurrentUser();
    state.workspaces = me.workspaces || [];
    state.tenant = me.tenant;
    if (state.tenant?.id) localStorage.setItem('miao_workspace', state.tenant.id);
    state.aiConfigured = me.ai_configured;
    state.isPlatformAdmin = me.is_platform_admin;
    state.apps = me.apps || [];
    state.archivedApps = await api('/api/apps?archived=true').catch(() => []);
    state.app = null;
    state.appPanel = 'runtime';
    state.table = null;
    const requestedAdminPage = platformAdmin.routePage();
    if (requestedAdminPage && state.isPlatformAdmin) await platformAdmin.open(requestedAdminPage, false);
    else {
      if (requestedAdminPage) history.replaceState({}, '', '/');
      notifications.reset();
  workspaceSession.restore();
      if (state.workspaceView === 'assistant') await agentAssistant.enterConversation();
      show('workspace');
      await renderWorkspace();
    }
  } catch (error) {
    toast(error.message, true);
  }
}

function renderApps() {
  $('#workspace-switcher').innerHTML = state.workspaces.map((workspace) => `<option value="${esc(workspace.id)}">${esc(workspace.name)}${workspace.role === 'owner' ? ' · 所有者' : workspace.role === 'admin' ? ' · 管理员' : ' · 成员'}</option>`).join('');
  $('#workspace-switcher').value = state.tenant?.id || '';
  $('#workspace-switcher').disabled = state.agentBusy;
  $('#user-name').textContent = state.user?.name || '用户';
  $('#user-email').textContent = state.user?.email || '';
  $('#user-avatar').textContent = (state.user?.name || 'M').slice(0, 1);
  $('#ai-config-status').textContent = state.aiConfigured ? '小助手已连接企业 AI 服务。' : '企业尚未配置 AI 服务，请联系管理员。';
  $('#ai-config-status').classList.toggle('error', !state.aiConfigured);
  $('#platform-admin-entry').classList.toggle('hidden', !state.isPlatformAdmin);
  $('#workspace-audit-entry').classList.toggle('hidden', state.tenant?.role !== 'owner');
  $('#workspace-management-entry').classList.toggle('btn-active', state.workspaceView === 'management');
  if (state.workspaceView === 'management') $('#workspace-management-entry').setAttribute('aria-current', 'page');
  else $('#workspace-management-entry').removeAttribute('aria-current');
  $('#app-list').innerHTML = state.apps.map((item) => `<button class="app-nav-item ${state.workspaceView === 'app' && state.app?.id === item.id ? 'active' : ''}" data-open-app="${esc(item.id)}"><span class="app-nav-mark">${esc(item.name.slice(0, 1))}</span>${esc(item.name)}</button>`).join('') || '<p class="empty-app-nav">还没有应用</p>';
  const homeLink = $('.workspace-home-link');
  const assistantLink = $('.assistant-nav-link');
  const templateLink = $('.template-nav-link');
  homeLink.classList.toggle('active', state.workspaceView === 'home');
  assistantLink.classList.toggle('active', state.workspaceView === 'assistant');
  templateLink.classList.toggle('active', state.workspaceView === 'templates');
  if (state.workspaceView === 'home') homeLink.setAttribute('aria-current', 'page');
  else homeLink.removeAttribute('aria-current');
  if (state.workspaceView === 'assistant') assistantLink.setAttribute('aria-current', 'page');
  else assistantLink.removeAttribute('aria-current');
  if (state.workspaceView === 'templates') templateLink.setAttribute('aria-current', 'page');
  else templateLink.removeAttribute('aria-current');
  const assistantSelector = $('#assistant-app-selector');
  if (assistantSelector) {
    assistantSelector.innerHTML = `<option value="">整个工作区</option>${state.apps.map((item) => `<option value="${esc(item.id)}">${esc(item.name)}</option>`).join('')}`;
    assistantSelector.value = state.app?.id || '';
  }
}


async function renderWorkspace() {
  document.querySelector('#runtime-detail-dialog')?.remove();
  void notifications.refresh();
  workspaceSession.persist();
  renderApps();
  const management = state.workspaceView === 'management';
  const dashboard = state.workspaceView === 'home';
  const assistant = state.workspaceView === 'assistant';
  const templates = state.workspaceView === 'templates';
  const editingForm = state.workspaceView === 'edit';
  const appView = state.workspaceView === 'app' && Boolean(state.app);
  const dataInspection = appView && state.appPanel === 'data';
  const taskView = appView && state.appPanel === 'tasks';
  const settingsView = appView && state.appPanel === 'settings';
  const dataManagement = appSupports(state.app, 'data_management');
  if (!taskView) appTasks.reset();
  $('#workspace-management').classList.toggle('hidden', !management);
  $('#dashboard').classList.toggle('hidden', !dashboard);
  $('#app-templates').classList.toggle('hidden', !templates);
  $('#app-creation').classList.toggle('hidden', !editingForm);
  $('#app-runtime').classList.toggle('hidden', !appView || dataInspection || taskView || settingsView);
  $('#app-tasks').classList.toggle('hidden', !taskView);
  $('#app-content').classList.toggle('hidden', !dataInspection);
  $('#app-settings').classList.toggle('hidden', !settingsView);
  $('#assistant-view').classList.toggle('hidden', !assistant);
  const canManageApp = Boolean(appView && state.tenant?.role === 'owner');
  $('.workspace-header').classList.toggle('hidden', !appView);
  $('#app-primary-actions').classList.toggle('hidden', !appView);
  $('#app-manage-menu').classList.toggle('hidden', !canManageApp);
  $('#app-return-entry').classList.toggle('hidden', !dataInspection && !taskView && !settingsView);
  $('#app-tasks-entry').classList.toggle('hidden', taskView);
  $('#app-data-entry').classList.toggle('hidden', !dataManagement || dataInspection);
  $('#app-settings-entry').classList.toggle('hidden', settingsView || !['owner', 'manager', 'publisher'].includes(state.app?.permission));
  $('#app-create-table-entry').classList.toggle('hidden', !dataManagement || !dataInspection || !['owner', 'manager', 'publisher'].includes(state.app?.permission));
  $('#app-access-entry').classList.toggle('hidden', !canManageApp);
  $('#app-edit-entry').classList.toggle('hidden', !canManageApp);
  $('#app-archive-entry').classList.toggle('hidden', !canManageApp);
  $('#app-delete-entry').classList.toggle('hidden', !canManageApp);
  if (management) {
    await renderWorkspaceManagement();
    return;
  }
  if (dashboard) {
    $('#dashboard-workspace-name').textContent = state.tenant?.name || '';
    $('#dashboard-apps').innerHTML = state.apps.length ? state.apps.map((item) => `<button class="dashboard-app-card" data-open-app="${esc(item.id)}"><span class="dashboard-app-icon">${esc(item.name.slice(0, 1))}</span><span class="dashboard-app-copy"><strong>${esc(item.name)}</strong><small>${esc(item.description || '暂无用途说明')}</small></span><span class="dashboard-app-footer"><small>${item.has_published_version ? '已发布' : '草稿'} · ${item.updated_at ? new Date(item.updated_at).toLocaleDateString() : '时间未知'}</small><span aria-hidden="true">打开应用 →</span></span></button>`).join('') : '<div class="dashboard-empty"><strong>还没有应用</strong></div>';
    $('#archived-app-section').classList.toggle('hidden', !state.archivedApps.length);
    $('#archived-apps').innerHTML = state.archivedApps.map((item) => `<div class="dashboard-app-card"><span class="dashboard-app-icon">${esc(item.name.slice(0, 1))}</span><span class="dashboard-app-copy"><strong>${esc(item.name)}</strong><small>${esc(item.description || '暂无用途说明')}</small></span><button class="btn btn-ghost btn-sm" data-restore-app="${esc(item.id)}">恢复</button></div>`).join('');
    return;
  }
  if (templates) {
    await renderAppTemplates();
    return;
  }
  if (editingForm) {
    $('#app-form-title').textContent = '修改应用信息';
    const form = $('#edit-app-form');
    form.querySelector('[name="name"]').value = state.editingApp?.name || '';
    form.querySelector('[name="description"]').value = state.editingApp?.description || '';
    form.querySelector('button[type="submit"]').textContent = '保存修改';
    return;
  }
  if (!appView) return;
  state.editingApp = false;
  if (taskView) { await appTasks.load(); return; }
  if (settingsView) { await appSettings.open(); return; }
  if (!dataInspection) { await appRuntimeModule.load(); return; }
  state.tables = await api(`/api/apps/${state.app.id}/collections`);
  if (!state.tables.some((item) => item.slug === state.table?.slug)) state.table = state.tables[0] || null;
  workspaceData.renderTables();
  if (state.table) await workspaceData.renderRecords();
  else workspaceData.renderNoTables();
}

async function renderAppTemplates() {
  const root = $('#app-template-list');
  if (!root || root.dataset.loaded === 'true') return;
  root.innerHTML = '<p class="runtime-loading">正在读取应用案例…</p>';
  try {
    const result = await api('/api/build/templates');
    root.innerHTML = (result.items || []).map((item) => `<article class="app-template-card"><div class="app-template-card-copy"><span class="app-template-mark">${esc(item.name.slice(0, 1))}</span><div><h3>${esc(item.name)}</h3><p>${esc(item.description || '')}</p></div></div><button class="btn btn-primary btn-sm" data-create-template="${esc(item.id)}">一键创建</button></article>`).join('') || '<p class="dashboard-empty">暂时没有可用案例。</p>';
    root.dataset.loaded = 'true';
  } catch (error) {
    root.innerHTML = `<p class="alert alert-error" role="alert">${esc(error.message || '应用案例暂时无法读取')}</p>`;
  }
}



async function saveAppDetails(event) {
  event.preventDefault();
  if (!state.editingApp) return;
  const form = new FormData(event.target);
  try {
    const app = await api(`/api/apps/${state.editingApp.id}`, { method: 'PATCH', body: JSON.stringify({ name: form.get('name'), description: form.get('description') }) });
    state.apps = state.apps.map((item) => item.id === app.id ? app : item);
    state.app = app;
    state.workspaceView = 'app';
    state.appPanel = 'runtime';
    state.editingApp = false;
    state.table = null;
    event.currentTarget.reset();
    await renderWorkspace();
    toast('应用信息已更新');
  } catch (error) {
    toast(error.message, true);
  }
}

async function refreshApps() {
  const me = await api('/api/me');
  state.apps = me.apps || [];
  state.archivedApps = await api('/api/apps?archived=true').catch(() => []);
  state.app = null;
  state.appPanel = 'runtime';
  state.table = null;
  state.workspaceView = 'home';
  await renderWorkspace();
}

async function archiveApp() {
  if (!state.app || !window.confirm('归档此应用？归档后会从工作区列表隐藏，数据仍可恢复。')) return;
  try {
    await api(`/api/apps/${state.app.id}`, { method: 'PATCH', body: JSON.stringify({ archived: true }) });
    await refreshApps();
    toast('应用已归档');
  } catch (error) { toast(error.message, true); }
}

async function restoreApp(appId) {
  try {
    await api(`/api/apps/${encodeURIComponent(appId)}`, { method: 'PATCH', body: JSON.stringify({ archived: false }) });
    await refreshApps();
    toast('应用已恢复');
  } catch (error) { toast(error.message, true); }
}

async function deleteApp() {
  if (!state.app || !window.confirm(`永久删除「${state.app.name}」及其全部数据？此操作无法撤销。`)) return;
  try {
    await api(`/api/apps/${state.app.id}`, { method: 'DELETE', body: JSON.stringify({ confirm: true }) });
    await refreshApps();
    toast('应用及其数据已删除');
  } catch (error) { toast(error.message, true); }
}


async function requestPasswordReset() {
  $('#auth-form').classList.add('hidden');
  $('#auth-switch-copy').closest('.auth-switch').classList.add('hidden');
  $('[data-action="forgot-password"]').classList.add('hidden');
  $('#account-message').classList.add('hidden');
  $('#request-reset-form').classList.remove('hidden');
  $('#request-reset-form [name="email"]').value = $('#auth-form [name="email"]').value;
  $('#request-reset-form [name="email"]').focus();
}

async function submitPasswordResetRequest(event) {
  event.preventDefault();
  const email = new FormData(event.target).get('email');
  try {
    const result = await api('/api/auth/password-reset/request', { method: 'POST', body: JSON.stringify({ email }) });
    toast(result.message || '如果邮箱已登记，重置邮件将发送到邮箱');
  } catch (error) { toast(error.message, true); }
}

async function submitPasswordReset(event) {
  event.preventDefault();
  const password = new FormData(event.target).get('password');
  try {
    await api('/api/auth/password-reset/confirm', { method: 'POST', body: JSON.stringify({ token: state.resetToken, password }) });
    $('#password-reset-dialog').close();
    state.resetToken = null;
    history.replaceState({}, '', '/');
    toast('密码已更新，请使用新密码登录');
    authMode('login');
  } catch (error) { toast(error.message, true); }
}

async function submitAccountDeletion(event) {
  event.preventDefault();
  const data = Object.fromEntries(new FormData(event.target));
  try {
    await api('/api/me', { method: 'DELETE', body: JSON.stringify(data) });
    $('#account-delete-dialog').close();
    await logout();
    toast('账号和相关数据已删除');
  } catch (error) { toast(error.message, true); }
}

async function submitAccountDeactivation(event) {
  event.preventDefault();
  const password = new FormData(event.target).get('password');
  try {
    await api('/api/me/deactivate', { method: 'POST', body: JSON.stringify({ password, confirm: true }) });
    $('#account-deactivate-dialog').close();
    await logout();
    toast('账号已停用');
  } catch (error) { toast(error.message, true); }
}

async function openAIUsage() {
  try {
    const usage = await api('/api/workspace/ai-usage');
    $('#ai-usage-summary').textContent = `${usage.day}：${usage.requests} 次请求 · 输入 ${usage.input_tokens.toLocaleString()} tokens · 输出 ${usage.output_tokens.toLocaleString()} tokens`;
    $('#ai-budget-form [name="daily_limit"]').value = usage.daily_limit;
    $('#ai-budget-form [name="daily_limit"]').disabled = !usage.can_manage;
    $('#ai-budget-save').classList.toggle('hidden', !usage.can_manage);
    $('#ai-usage-dialog').showModal();
  } catch (error) { toast(error.message, true); }
}

async function saveAIBudget(event) {
  event.preventDefault();
  const daily_limit = Number(new FormData(event.target).get('daily_limit'));
  try {
    await api('/api/workspace/ai-budget', { method: 'PATCH', body: JSON.stringify({ daily_limit }) });
    await openAIUsage();
    toast('AI 每日预算已保存');
  } catch (error) { toast(error.message, true); }
}


async function openAppAccess() {
  if (!state.app) return;
  try {
    const access = await api(`/api/apps/${state.app.id}/access`);
    $('#app-access-form [name="restricted"]').checked = access.restricted;
    $('#app-access-members').innerHTML = access.members.map((member) => `<label class="member-row"><span>${esc(member.name)} · ${esc(member.email)}<small>${member.workspace_role === 'admin' ? '管理员' : '成员'}</small></span><select class="select select-bordered select-sm" data-access-user="${esc(member.id)}"><option value="">无权限</option><option value="viewer" ${member.app_role === 'viewer' ? 'selected' : ''}>只读</option><option value="editor" ${member.app_role === 'editor' ? 'selected' : ''}>可编辑记录</option><option value="manager" ${member.app_role === 'manager' ? 'selected' : ''}>管理应用</option><option value="publisher" ${member.app_role === 'publisher' ? 'selected' : ''}>管理并发布</option></select><span><input type="checkbox" data-batch-user="${esc(member.id)}" ${member.can_batch ? 'checked' : ''}> 批量修改</span></label>`).join('') || '<p class="empty-members">当前工作区没有其他成员。</p>';
    $('#app-access-dialog').showModal();
  } catch (error) { toast(error.message, true); }
}

async function saveAppAccess(event) {
  event.preventDefault();
  if (!state.app) return;
  const permissions = $$('[data-access-user]').map((select) => ({ user_id: select.dataset.accessUser, role: select.value, can_batch: Boolean($(`[data-batch-user="${select.dataset.accessUser}"]`)?.checked) })).filter((permission) => permission.role);
  const restricted = $('#app-access-form [name="restricted"]').checked;
  try {
    await api(`/api/apps/${state.app.id}/access`, { method: 'PUT', body: JSON.stringify({ restricted, permissions }) });
    const app = await api(`/api/apps/${state.app.id}`);
    state.app = app;
    state.apps = state.apps.map((item) => item.id === app.id ? app : item);
    $('#app-access-dialog').close();
    await renderWorkspace();
    toast('应用访问权限已更新');
  } catch (error) { toast(error.message, true); }
}

async function downloadWorkspaceExport() {
  try {
    const response = await fetch('/api/workspace/export', { headers: { Authorization: `Bearer ${state.token}`, 'X-Miao-Tenant-Id': state.tenant.id } });
    if (!response.ok) throw new Error((await response.json().catch(() => ({}))).error || '数据导出失败');
    const blob = await response.blob();
    const href = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = href;
    link.download = `miao-workspace-${state.tenant.id}.json`;
    link.click();
    setTimeout(() => URL.revokeObjectURL(href), 1000);
    toast('工作区数据已导出');
  } catch (error) { toast(error.message, true); }
}

function managementNotice(message = '') {
  $('#workspace-management-notice').textContent = message;
  $('#workspace-management-notice').classList.toggle('hidden', !message);
}

async function openWorkspaceManagement(page = 'members') {
  if (state.agentBusy) { toast('助手正在处理，请稍后再进入空间管理。', true); return; }
  state.app = null;
  state.table = null;
  state.workspaceView = 'management';
  state.workspaceManagementPage = page;
  await renderWorkspace();
}

const openMembers = () => openWorkspaceManagement('members');
async function openAuditLog(page = 1) {
  state.workspaceAuditPage = page;
  await openWorkspaceManagement('audit');
}

async function renderWorkspaceManagement() {
  let page = state.workspaceManagementPage || 'members';
  if (page === 'audit' && state.tenant?.role !== 'owner') page = 'members';
  state.workspaceManagementPage = page;
  managementNotice();
  for (const button of $$('[data-workspace-management]')) {
    const selected = button.dataset.workspaceManagement === page;
    button.classList.toggle('btn-active', selected);
    if (selected) button.setAttribute('aria-current', 'page');
    else button.removeAttribute('aria-current');
  }
  for (const section of $$('[data-workspace-management-section]')) section.classList.toggle('hidden', section.dataset.workspaceManagementSection !== page);
  if (page === 'members') await loadMembers();
  if (page === 'audit') await loadAuditLog(state.workspaceAuditPage);
  if (page === 'settings') {
    const role = { owner: '所有者', admin: '管理员', member: '成员' }[state.tenant?.role] || '成员';
    $('#workspace-settings-summary').innerHTML = `<div><dt>空间名称</dt><dd>${esc(state.tenant?.name)}</dd></div><div><dt>当前角色</dt><dd>${role}</dd></div><div><dt>可访问应用</dt><dd>${state.apps.length}</dd></div>`;
  }
}

async function loadAuditLog(page = 1) {
  const tenantId = state.tenant?.id;
  $('#audit-log-list').textContent = '正在读取操作日志…';
  try {
    const result = await api(`/api/workspace/audit?page=${Math.max(1, page)}`);
    if (tenantId !== state.tenant?.id || state.workspaceView !== 'management') return;
    state.workspaceAuditPage = result.page;
    $('#audit-log-list').innerHTML = result.items.map((item) => `<div class="audit-log-row"><span><strong>${esc(item.action)} ${esc(item.route)}</strong><small>${esc(item.actor_email)} · ${new Date(item.created_at).toLocaleString()} · ${item.status}</small></span><code>${esc(item.target_id || '—')}</code></div>`).join('') || '<p class="empty-members">还没有操作记录。</p>';
    $('#audit-log-page-label').textContent = `第 ${result.page} / ${Math.max(1, result.totalPages)} 页 · 共 ${result.totalItems} 条`;
    $('[data-audit-delta="-1"]').disabled = result.page <= 1;
    $('[data-audit-delta="1"]').disabled = result.page >= result.totalPages;
  } catch (error) {
    if (tenantId !== state.tenant?.id) return;
    $('#audit-log-list').textContent = '操作日志暂时不可用，请刷新重试。';
    managementNotice(error.message);
  }
}


async function loadMembers() {
  const tenantId = state.tenant?.id;
  $('#member-list').textContent = '正在读取成员…';
  $('#pending-invites').replaceChildren();
  $('#invite-form').classList.add('hidden');
  try {
    const [result, invites] = await Promise.all([
      api('/api/workspace/members'),
      ['owner', 'admin'].includes(state.tenant?.role) ? api('/api/workspace/invites') : Promise.resolve([])
    ]);
    if (tenantId !== state.tenant?.id || state.workspaceView !== 'management') return;
    $('#member-list').innerHTML = result.members.map((member) => `<div class="member-row"><span>${esc(member.name)} · ${esc(member.email)}<small>${member.role === 'owner' ? '所有者' : member.role === 'admin' ? '管理员' : '成员'}</small></span>${result.can_edit_roles && member.role !== 'owner' ? `<span class="member-actions"><select class="select select-bordered select-xs" data-member-role="${esc(member.membership_id)}"><option value="member" ${member.role === 'member' ? 'selected' : ''}>成员</option><option value="admin" ${member.role === 'admin' ? 'selected' : ''}>管理员</option></select><button class="btn btn-ghost btn-xs" data-remove-member="${esc(member.membership_id)}">从工作区移除</button></span>` : ''}</div>`).join('') || '<p class="empty-members">还没有成员。</p>';
    $('#invite-form').classList.toggle('hidden', !result.can_manage);
    if (!result.can_manage) {
      $('#invite-link').value = '';
      $('#invite-link-row').classList.add('hidden');
    }
    $('#pending-invites').innerHTML = invites.map((invite) => `<div class="pending-invite-row"><span>${esc(invite.email)}<small>邀请待接受 · ${new Date(invite.expires_at).toLocaleString()}</small></span><button class="btn btn-ghost btn-xs" data-revoke-invite="${esc(invite.id)}">撤销</button></div>`).join('');
  } catch (error) {
    if (tenantId !== state.tenant?.id) return;
    $('#member-list').textContent = '成员列表暂时不可用，请刷新重试。';
    managementNotice(error.message);
  }
}

async function createInvite(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const tenantId = state.tenant?.id;
  const email = new FormData(form).get('email');
  try {
    const invite = await api('/api/workspace/invites', { method: 'POST', body: JSON.stringify({ email }) });
    if (tenantId !== state.tenant?.id) return;
    $('#invite-link').value = new URL(invite.invite_url, location.origin).href;
    $('#invite-link-row').classList.remove('hidden');
    form.reset();
    try {
      await navigator.clipboard.writeText($('#invite-link').value);
      toast('邀请链接已生成并复制');
    } catch {
      toast('邀请链接已生成，请复制后发给同事');
    }
    await openMembers();
  } catch (error) {
    toast(error.message, true);
  }
}

async function removeMember(membershipId) {
  if (!window.confirm('移除此成员后，对方将无法继续访问当前工作区。')) return;
  try {
    await api(`/api/workspace/members/${encodeURIComponent(membershipId)}`, { method: 'DELETE' });
    await openMembers();
    toast('成员已移除');
  } catch (error) {
    toast(error.message, true);
  }
}

async function revokeInvite(inviteId) {
  try {
    await api(`/api/workspace/invites/${encodeURIComponent(inviteId)}`, { method: 'DELETE' });
    $('#invite-link').value = '';
    $('#invite-link-row').classList.add('hidden');
    await openMembers();
    toast('邀请已撤销');
  } catch (error) {
    toast(error.message, true);
  }
}


document.addEventListener('click', async (event) => {
  for (const menu of $$('details.app-manage-menu[open], details.account-actions[open]')) {
    if (!menu.contains(event.target) || event.target.closest('button')) menu.removeAttribute('open');
  }
  try { if (await appSettings.click(event)) return; } catch (error) { toast(error.message || '应用配置操作失败', true); }
  const suggestedPrompt = event.target.closest('[data-prompt]');
  if (suggestedPrompt) { $('#agent-form [name=prompt]').value = suggestedPrompt.dataset.prompt; $('#agent-form [name=prompt]').focus(); return; }
  const previewRetry = event.target.closest('[data-preview-retry]');
  if (previewRetry) {
    const card = previewRetry.closest('[data-preview-card]');
    if (card) await appRuntimeModule.loadPreview(card).catch(() => {});
    return;
  }
  if (await platformAdmin.handleClick(event)) return;
  if (await appTasks.handleClick(event)) return;
  if (await notifications.handleClick(event)) return;
  if (await workspaceData.handleClick(event)) return;
  const action = event.target.closest('[data-action]')?.dataset.action;
  if (action === 'register') authMode('register');
  if (action === 'login') authMode('login');
  if (action === 'logout') await logout();
  if (action === 'show-dashboard') {
    state.app = null;
    state.appPanel = 'runtime';
    state.table = null;
    state.editingApp = false;
    state.workspaceView = 'home';
    await renderWorkspace();
  }
  if (action === 'show-templates') {
    state.app = null;
    state.appPanel = 'runtime';
    state.table = null;
    state.workspaceView = 'templates';
    await renderWorkspace();
  }
  if (action === 'open-assistant') {
    state.workspaceView = 'assistant';
    await agentAssistant.enterConversation();
    await renderWorkspace();
    $('#agent-form [name="prompt"]').focus();
  }
  if (action === 'choose-build-template') {
    await agentAssistant.showTemplateChoices().catch((error) => toast(error.message, true));
  }
  const templateButton = event.target.closest('[data-create-template]');
  if (templateButton) {
    await agentAssistant.createTemplate(templateButton.dataset.createTemplate).catch((error) => toast(error.message || '应用案例创建失败', true));
  }
  if (action === 'clear-agent-conversation') {
    if (!state.agentBusy && window.confirm('清除当前工作区保存在服务端的私人会话？此操作不能撤销。')) {
      await agentAssistant.clearSavedConversation().catch((error) => toast(error.message || '无法清除会话', true));
    }
  }
  if (action === 'retry-app-runtime') await appRuntimeModule.load();
  if (action === 'clear-runtime-search') {
    state.runtimeQuery = { ...state.runtimeQuery, page: 1, search: '' };
    await appRuntimeModule.load();
  }
  if (action === 'edit-app' && state.app) {
    state.editingApp = state.app;
    state.app = null;
    state.appPanel = 'runtime';
    state.workspaceView = 'edit';
    await renderWorkspace();
  }
  if (action === 'cancel-app-form') {
    state.editingApp = false;
    state.workspaceView = 'home';
    await renderWorkspace();
  }
  if (action === 'view-app-data' && state.app) {
    state.appPanel = 'data';
    state.table = null;
    await renderWorkspace();
  }
  if (action === 'view-app-tasks' && state.app) {
    state.appPanel = 'tasks';
    await renderWorkspace();
  }
  if (action === 'view-app-settings' && state.app) {
    state.appPanel = 'settings';
    await renderWorkspace();
  }
  if (action === 'return-to-app' && state.app) {
    state.appPanel = 'runtime';
    await renderWorkspace();
  }
  if (action === 'archive-app') archiveApp();
  if (action === 'delete-app') deleteApp();
  if (action === 'forgot-password') requestPasswordReset();
  if (action === 'cancel-password-request') authMode('login');
  if (action === 'close-password-reset') $('#password-reset-dialog').close();
  if (action === 'delete-account') {
    $('#account-delete-form [name="confirm"]').value = '';
    $('#account-delete-form [name="confirm"]').placeholder = state.user?.email || '';
    $('#account-delete-dialog').showModal();
  }
  if (action === 'deactivate-account') $('#account-deactivate-dialog').showModal();
  if (action === 'close-account-deactivate') $('#account-deactivate-dialog').close();
  if (action === 'close-account-delete') $('#account-delete-dialog').close();
  if (action === 'ai-usage') openAIUsage();
  if (action === 'close-ai-usage') $('#ai-usage-dialog').close();
  if (action === 'app-access') openAppAccess();
  if (action === 'close-app-access') $('#app-access-dialog').close();
  if (action === 'export-data') downloadWorkspaceExport();
  if (action === 'open-audit') openAuditLog();
  const auditPageButton = event.target.closest('[data-audit-delta]');
  if (auditPageButton) openAuditLog(state.workspaceAuditPage + Number(auditPageButton.dataset.auditDelta));
  const downloadFileButton = event.target.closest('[data-download-file]');
  if (downloadFileButton) {
    try {
      const path = `/api/apps/${encodeURIComponent(state.app.id)}/collections/${encodeURIComponent(downloadFileButton.dataset.fileCollection || state.recordFormContext?.collection || state.table?.slug || state.appRuntime?.collection)}/records/${encodeURIComponent(downloadFileButton.dataset.recordId)}/files/${encodeURIComponent(downloadFileButton.dataset.downloadFile)}`;
      const response = await fetch(path, { headers: { Authorization: `Bearer ${state.token}`, 'X-Miao-Tenant-Id': state.tenant.id } });
      if (!response.ok) throw new Error('附件下载失败');
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = downloadFileButton.dataset.fileName || 'attachment';
      link.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (error) { toast(error.message, true); }
  }
  if (action === 'open-workspace-management') await openWorkspaceManagement();
  const managementButton = event.target.closest('[data-workspace-management]');
  if (managementButton) await openWorkspaceManagement(managementButton.dataset.workspaceManagement);
  if (action === 'manage-members') openMembers().catch((error) => toast(error.message, true));
  if (action === 'copy-invite') {
    navigator.clipboard?.writeText($('#invite-link').value)
      .then(() => toast('链接已复制'))
      .catch(() => toast('复制失败，请手动复制链接', true));
  }
  const removeMemberButton = event.target.closest('[data-remove-member]');
  if (removeMemberButton) removeMember(removeMemberButton.dataset.removeMember);
  const revokeInviteButton = event.target.closest('[data-revoke-invite]');
  if (revokeInviteButton) revokeInvite(revokeInviteButton.dataset.revokeInvite);
  const appButton = event.target.closest('[data-open-app]');
  if (appButton) {
    state.editingApp = false;
    state.app = state.apps.find((item) => item.id === appButton.dataset.openApp) || null;
    state.appPanel = 'runtime';
    state.runtimeQuery = { page: 1, search: '' };
    state.table = null;
    state.workspaceView = 'app';
    await renderWorkspace();
  }
  if (action === 'open-assistant') document.querySelector('#runtime-detail-dialog')?.close();
  try { if (await appRuntimeModule.handleClick(event)) return; } catch (error) { toast(error.message, true); }
  const runtimePageButton = event.target.closest('[data-runtime-page]');
  if (runtimePageButton && !runtimePageButton.disabled) {
    state.runtimeQuery.page = Number(runtimePageButton.dataset.runtimePage);
    await appRuntimeModule.load();
  }
  const restoreButton = event.target.closest('[data-restore-app]');
  if (restoreButton) await restoreApp(restoreButton.dataset.restoreApp);
});

document.addEventListener('submit', async (event) => {
  if (event.target.id === 'ui-editor-form') { await appRuntimeModule.handleSubmit(event); return; }
  try { if (await appSettings.submit(event)) return; } catch (error) { toast(error.message || '应用配置保存失败', true); }
});

document.addEventListener('change', (event) => {
  if (appRuntimeModule.handleChange(event)) return;
  if (event.target.matches('[name="attachment"]')) { const label = event.target.parentElement.querySelector('[data-attachment-name]'); if (label) label.textContent = event.target.files?.[0]?.name || ''; return; }
  if (event.target.matches('#workspace-switcher')) {
    switchWorkspace(event.target.value);
    return;
  }
  if (event.target.matches('#assistant-app-selector')) {
    agentAssistant.selectApp(event.target.value).catch((error) => toast(error.message, true));
    return;
  }
  if (event.target.matches('[data-member-role]')) {
    api(`/api/workspace/members/${encodeURIComponent(event.target.dataset.memberRole)}`, { method: 'PATCH', body: JSON.stringify({ role: event.target.value }) })
      .then(async () => { await loadMembers(); toast('成员角色已更新'); }).catch(async (error) => { await loadMembers(); toast(error.message, true); });
  }
  if (workspaceData.handleChange(event)) return;
});

document.addEventListener('submit', (event) => {
  if (!event.target.matches('.runtime-search-form')) return;
  event.preventDefault();
  state.runtimeQuery = { ...state.runtimeQuery, page: 1, search: String(new FormData(event.target).get('search') || '').trim() };
  appRuntimeModule.load();
});

async function switchWorkspace(workspaceId) {
  if (state.agentBusy) {
    renderApps();
    toast('助手正在处理，请稍后再切换工作区。', true);
    return;
  }
  const selected = state.workspaces.find((workspace) => workspace.id === workspaceId);
  if (!selected || selected.id === state.tenant?.id) return;
  $('#invite-link').value = '';
  $('#invite-link-row').classList.add('hidden');
  state.workspaceAuditPage = 1;
  const previousTenant = state.tenant;
  clearAgent();
  resetAgentConversation();
  if ($('#record-dialog').open) $('#record-dialog').close();
  state.recordFormContext = null;
  state.app = null;
  state.appRuntime = null;
  state.appPanel = 'runtime';
  state.editingRecordId = null;
  state.table = null;
  state.tables = [];
  state.records = [];
  state.recordResult = null;
  state.recordQuery = { page: 1, perPage: 25, search: '', sort: '-created', filterField: '', filterValue: '' };
  state.editingApp = false;
  state.workspaceView = 'home';
  state.tenant = selected;
  let me;
  let archivedApps;
  try {
    me = await api('/api/me');
    archivedApps = await api('/api/apps?archived=true').catch(() => []);
  } catch (error) {
    state.tenant = previousTenant;
    toast(error.message, true);
    notifications.reset();
  workspaceSession.restore();
    await renderWorkspace();
    return;
  }
  state.workspaces = me.workspaces || [];
  state.tenant = me.tenant;
  localStorage.setItem('miao_workspace', state.tenant.id);
  state.apps = me.apps || [];
  state.archivedApps = archivedApps;
  state.aiConfigured = me.ai_configured;
  state.isPlatformAdmin = me.is_platform_admin;
  if ($('#record-dialog').open) $('#record-dialog').close();
  state.recordFormContext = null;
  state.app = null;
  state.appRuntime = null;
  state.runtimeQuery = { page: 1, search: '' };
  state.appPanel = 'runtime';
  state.tables = [];
  state.table = null;
  state.records = [];
  state.recordResult = null;
  state.recordQuery = { page: 1, perPage: 25, search: '', sort: '-created', filterField: '', filterValue: '' };
  notifications.reset();
  workspaceSession.restore();
  if (state.workspaceView === 'assistant') await agentAssistant.enterConversation();
  await renderWorkspace();
}

const appRuntimeModule = createAppRuntime({ state, api, $, esc, onUIRequest: (request) => agentAssistant.startUIEdit(request) });
const workspaceData = createWorkspaceData({ state, api, $, $$, esc, toast, renderWorkspace, loadRuntime: () => appRuntimeModule.load() });
workspaceData.bind();
const agentAssistant = createAgentAssistant({ state, api, $, esc, toast, renderWorkspace });
const appTasks = createAppTasks({ state, api, $, esc, toast });
const appSettings = createAppSettings({ state, api, $, esc, toast });
const notifications = createNotifications({ state, api, $, esc, toast, renderWorkspace, appTasks, appSettings });
const platformAdmin = createPlatformAdmin({ state, api, $, $$, esc, toast, show, renderApps, renderWorkspace });
platformAdmin.bind();

$('#auth-form').addEventListener('submit', submitAuth);
$('#edit-app-form').addEventListener('submit', saveAppDetails);
$('#switch-auth').addEventListener('click', () => authMode(state.authMode === 'register' ? 'login' : 'register'));
$('#password-reset-form').addEventListener('submit', submitPasswordReset);
$('#request-reset-form').addEventListener('submit', submitPasswordResetRequest);
$('#account-delete-form').addEventListener('submit', submitAccountDeletion);
$('#account-deactivate-form').addEventListener('submit', submitAccountDeactivation);
$('#ai-budget-form').addEventListener('submit', saveAIBudget);
$('#app-access-form').addEventListener('submit', saveAppAccess);
$('#agent-form').addEventListener('submit', agentAssistant.submitPrompt);
$('#home-agent-form').addEventListener('submit', agentAssistant.submitPrompt);
for (const form of [$('#agent-form'), $('#home-agent-form')]) {
  form.querySelector('[name="prompt"]').addEventListener('keydown', (event) => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      form.requestSubmit();
    }
  });
}
$('#invite-form').addEventListener('submit', createInvite);
window.addEventListener('popstate', async () => {
  if (!state.token) return;
  if (location.pathname.startsWith('/admin') && state.isPlatformAdmin) await platformAdmin.open(platformAdmin.routePage(), false);
  else {
    if (location.pathname.startsWith('/admin')) history.replaceState({}, '', '/');
    show('workspace');
    await renderWorkspace();
  }
});
bootstrap();
