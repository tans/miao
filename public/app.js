import { createPlatformAdmin } from '/modules/platform-admin.js';
import { createAppRuntime } from '/modules/app-runtime.js';
import { createAgentAssistant } from '/modules/agent-assistant.js';
import { createWorkspaceData } from '/modules/workspace-data.js';
import { createWorkspaceSession } from '/modules/workspace-session.js';
import { createNotifications } from '/modules/notifications.js';
import { createAppTasks } from '/modules/app-tasks.js';
import { createAppSettings } from '/modules/app-settings.js';
import { createWorkspaceAIUsage } from '/modules/workspace-ai-usage.js';
import { icon, hydrateIcons } from '/modules/icons.js';
import { openSSE } from '/modules/sse.js';
import { t, fmtDate, fmtDateTime, fmtNumber, applyTranslations, setLanguage, setAccountLanguage, setPersistHandler, getLanguage } from '/modules/i18n.js';

const TOKEN_KEY = 'miao_token';
const LAST_LOGIN_KEY = 'miao_last_login';
hydrateIcons();
applyTranslations();
const state = {
  token: localStorage.getItem(TOKEN_KEY), user: null, tenant: null,
  workspaces: [], apps: [], archivedApps: [], app: null, tables: [], table: null, records: [],
  appRuntime: null, runtimeQuery: { page: 1, search: '' }, recordFormContext: null,
  recordQuery: { page: 1, perPage: 25, search: '', sort: '-created', filterField: '', filterValue: '' }, recordResult: null,
  editingRecordId: null, editingApp: false, workspaceView: 'home', appPanel: 'runtime',
  authMode: 'login', agentBusy: false, isPlatformAdmin: false,
  agentConversationMessages: [], agentConversationRevision: 0,
  agentConversationLoadedKey: null, agentRun: null,
  agentTurnNumber: 0, agentPersistenceConflict: false, navOpen: true,
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
setPersistHandler((lang) => state.token ? api('/api/me', { method: 'PATCH', body: JSON.stringify({ language: lang }) }) : Promise.resolve());

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
  if (!response.ok) {
    const error = new Error(result.error || result.message || t('请求失败，请稍后重试'));
    error.status = response.status;
    throw error;
  }
  return result;
}

async function stream(url, { signal, onEvent, headers: extraHeaders = {} } = {}) {
  const requestToken = state.token;
  const headers = { ...extraHeaders };
  if (state.token) headers.Authorization = `Bearer ${state.token}`;
  if (state.tenant?.id && !headers['X-Miao-Tenant-Id']) headers['X-Miao-Tenant-Id'] = state.tenant.id;
  return openSSE(url, {
    headers,
    signal,
    onEvent,
    onToken: (nextToken) => {
      if (nextToken && state.token === requestToken) {
        state.token = nextToken;
        localStorage.setItem(TOKEN_KEY, nextToken);
      }
    },
  });
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


function rememberLastLogin(email, password) {
  try { localStorage.setItem(LAST_LOGIN_KEY, JSON.stringify({ email, password })); } catch {}
}

function fillLastLogin() {
  let saved = null;
  try { saved = JSON.parse(localStorage.getItem(LAST_LOGIN_KEY) || 'null'); } catch {}
  if (!saved) return;
  const emailInput = $('#auth-form [name="email"]');
  const passwordInput = $('#auth-form [name="password"]');
  if (!emailInput.value) emailInput.value = saved.email || '';
  if (!passwordInput.value) passwordInput.value = saved.password || '';
}

function authMode(mode) {
  state.authMode = mode;
  $('#auth').classList.remove('session-checking');
  $('#auth').setAttribute('aria-busy', 'false');
  const registering = mode === 'register';
  $('#auth-title').textContent = registering ? t('创建工作区') : t('登录');
  if (state.pendingInvite) $('#auth-copy').textContent = t('你收到了工作区邀请。使用受邀邮箱登录或注册，即可直接加入。');
  $('#auth-submit').textContent = registering ? t('创建账号') : t('登录');
  $('#name-field').classList.toggle('hidden', !registering);
  $('#name-field input').required = registering;
  $('#auth-form [name="password"]').autocomplete = registering ? 'new-password' : 'current-password';
  $('#auth-switch-copy').textContent = registering ? t('已有账号？') : t('还没有账号？');
  $('#switch-auth').textContent = registering ? t('登录') : t('创建账号');
  if (registering) $('#auth-form [name="password"]').value = '';
  else fillLastLogin();
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
  $('#chat-messages').innerHTML = `<div class="assistant-intro"><img src="/mascots/cat-peek-square.png" alt="" /><div><h3>${esc(t('你想做什么？'))}</h3><div class="conversation-prompts"><button class="btn btn-outline btn-sm" data-prompt="${esc(t('帮我梳理每周团队周报的收集和汇总流程'))}">${esc(t('梳理工作流程'))}</button><button class="btn btn-outline btn-sm" data-prompt="${esc(t('我想做一个客户跟进流程，先帮我想清楚怎么开始'))}">${esc(t('从一个想法开始'))}</button></div></div></div>`;
  $('#agent-status').textContent = t('准备开始');
  $('#agent-status').className = 'badge badge-ghost';
}

const workspaceSession = createWorkspaceSession({ state, $, clearAgent, resetAgentConversation });

async function logout({ skipConversationPrompt = false } = {}) {
  const previousUserId = state.user?.id;
  const previousTenantId = state.tenant?.id;
  $('#invite-link').value = '';
  $('#invite-link-row').classList.add('hidden');
  workspaceAIUsage.reset();
  notifications.reset();
  appTasks.reset();
  const logoutRequest = state.token ? api('/api/auth/logout', { method: 'POST' }).catch(() => {}) : Promise.resolve();
  let storageError;
  try { await agentAssistant.clearSavedConversations({ skipConfirm: skipConversationPrompt }); } catch (error) { storageError = error; }
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
  workspaceSession.clear(previousUserId, previousTenantId);
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem('miao_workspace');
  history.replaceState({}, '', '/');
  authMode('login');
  if (storageError) toast(t('已退出登录，但服务器会话未能清理：{message}', { message: storageError.message || t('存储不可用') }), true);
}

async function bootstrap() {
  if (location.pathname === '/verify-email') {
    try {
      await api('/api/auth/verify-email', { method: 'POST', body: JSON.stringify({ token: new URLSearchParams(location.search).get('token') }) });
      state.pendingInvite = new URLSearchParams(location.search).get('invite');
      history.replaceState({}, '', state.pendingInvite ? `/?invite=${encodeURIComponent(state.pendingInvite)}` : '/');
      toast(t('邮箱验证完成，可以登录了'));
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
        toast(t('已加入工作区'));
      } catch (error) {
        toast(error.message, true);
      }
    }
    const me = await loadCurrentUser();
    state.user = me.user;
    setAccountLanguage(me.user?.language);
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
      await agentAssistant.enterConversation();
      show('workspace');
      await renderWorkspace();
    }
  } catch (error) {
    await logout({ skipConversationPrompt: true });
    if (error?.status === 401) toast(t('登录状态已失效，请重新登录'), true);
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
      toast(t('验证邮件已发送到 {email}', { email: result.email }));
      $('#auth-form').reset();
      return;
    }
    state.token = result.token;
    localStorage.setItem(TOKEN_KEY, result.token);
    rememberLastLogin(payload.email, payload.password);
    state.user = result.user;
    setAccountLanguage(result.user?.language);
    state.tenant = result.tenant;
    if (state.pendingInvite) {
      try {
        const accepted = await api('/api/invites/accept', { method: 'POST', body: JSON.stringify({ token: state.pendingInvite }) });
        state.tenant = accepted.tenant;
        localStorage.setItem('miao_workspace', accepted.tenant.id);
        state.pendingInvite = null;
        history.replaceState({}, '', location.pathname);
        toast(t('已加入工作区'));
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
      await agentAssistant.enterConversation();
      show('workspace');
      await renderWorkspace();
    }
  } catch (error) {
    toast(error.message, true);
  }
}

function renderApps() {
  $('#workspace-switcher-name').textContent = state.tenant?.name || t('选择工作区');
  $('#workspace-switcher').toggleAttribute('inert', Boolean(state.agentBusy));
  $('#workspace-switcher-menu').innerHTML = state.workspaces.map((workspace) => `<button class="btn btn-ghost workspace-switcher-item${workspace.id === state.tenant?.id ? ' active' : ''}" data-workspace-select="${esc(workspace.id)}"><span class="workspace-switcher-mark">${workspace.id === state.tenant?.id ? icon('check', 12) : ''}</span><span class="workspace-switcher-label">${esc(workspace.name)}</span></button>`).join('') + `<div class="workspace-switcher-separator" role="separator"></div><button class="btn btn-ghost workspace-switcher-item" data-action="workspace-create"><span class="workspace-switcher-mark">${icon('plus', 12)}</span><span class="workspace-switcher-label">${esc(t('创建工作区…'))}</span></button>`;
  $('#user-name').textContent = state.user?.name || t('用户');
  $('#user-email').textContent = state.user?.email || '';
  $('#user-avatar').textContent = (state.user?.name || 'M').slice(0, 1);
  $('#ai-config-status').textContent = state.aiConfigured ? t('已连接企业 AI 服务') : t('企业尚未配置 AI 服务，请联系管理员。');
  $('#ai-config-status').classList.toggle('error', !state.aiConfigured);
  $('#platform-admin-entry').classList.toggle('hidden', !state.isPlatformAdmin);
  $('#workspace-audit-entry').classList.toggle('hidden', state.tenant?.role !== 'owner');
  $('#workspace-management-entry').classList.toggle('active', state.workspaceView === 'management');
  if (state.workspaceView === 'management') $('#workspace-management-entry').setAttribute('aria-current', 'page');
  else $('#workspace-management-entry').removeAttribute('aria-current');
  $('#app-list').innerHTML = state.apps.map((item) => `<button class="app-nav-item ${state.workspaceView === 'app' && state.app?.id === item.id ? 'active' : ''}" data-open-app="${esc(item.id)}"><span class="app-nav-mark">${esc(item.name.slice(0, 1))}</span>${esc(item.name)}</button>`).join('') || `<p class="empty-app-nav">${esc(t('还没有应用'))}</p>`;
  const homeLink = $('.workspace-home-link');
  const templateLink = $('.template-nav-link');
  homeLink.classList.toggle('active', state.workspaceView === 'home');
  templateLink.classList.toggle('active', state.workspaceView === 'templates');
  if (state.workspaceView === 'home') homeLink.setAttribute('aria-current', 'page');
  else homeLink.removeAttribute('aria-current');
  if (state.workspaceView === 'templates') templateLink.setAttribute('aria-current', 'page');
  else templateLink.removeAttribute('aria-current');
  const assistantSelector = $('#assistant-app-selector');
  if (assistantSelector) {
    assistantSelector.innerHTML = `<option value="">${esc(t('整个工作区'))}</option>${state.apps.map((item) => `<option value="${esc(item.id)}">${esc(item.name)}</option>`).join('')}`;
    assistantSelector.value = state.app?.id || '';
  }
}


async function renderWorkspace() {
  void notifications.refresh();
  workspaceSession.persist();
  renderApps();
  const management = state.workspaceView === 'management';
  const dashboard = state.workspaceView === 'home';
  const templates = state.workspaceView === 'templates';
  const editingForm = state.workspaceView === 'edit';
  const appView = state.workspaceView === 'app' && Boolean(state.app);
  const dataInspection = appView && state.appPanel === 'data';
  const taskView = appView && state.appPanel === 'tasks';
  const settingsView = appView && state.appPanel === 'settings';
  if (!appView || dataInspection || taskView || settingsView) appRuntimeModule.stopStream();
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
  const dockAllowed = dashboard || appView;
  $('#workspace').classList.toggle('assistant-open', dockAllowed);
  $('#workspace').classList.toggle('nav-closed', !state.navOpen);
  $('#nav-toggle').setAttribute('aria-expanded', String(state.navOpen));
  $('#nav-toggle').setAttribute('aria-label', state.navOpen ? t('收起导航') : t('展开导航'));
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
    const card = (item) => {
      const stats = [[t('访问'), item.view_count], [t('记录'), item.record_count], [t('数据表'), item.table_count]].map(([label, value]) => `<span><b>${value == null ? '—' : fmtNumber(value)}</b>${label}</span>`).join('');
      return `<button class="dashboard-app-card" data-open-app="${esc(item.id)}"><span class="dashboard-app-icon">${esc(item.name.slice(0, 1))}</span><span class="dashboard-app-copy"><strong>${esc(item.name)}</strong><small>${esc(item.description || t('暂无用途说明'))}</small><span class="dashboard-app-stats">${stats}</span></span><span class="dashboard-app-footer"><small>${item.has_published_version ? t('已发布') : t('草稿')} · ${item.updated_at ? fmtDate(item.updated_at) : t('时间未知')}</small><span aria-hidden="true">${esc(t('打开应用'))} ${icon('arrow-right', 12)}</span></span></button>`;
    };
    $('#dashboard-apps').innerHTML = state.apps.length ? state.apps.map(card).join('') : `<div class="dashboard-empty"><strong>${esc(t('还没有应用'))}</strong></div>`;
    $('#archived-app-section').classList.toggle('hidden', !state.archivedApps.length);
    $('#archived-apps').innerHTML = state.archivedApps.map((item) => `<div class="dashboard-app-card"><span class="dashboard-app-icon">${esc(item.name.slice(0, 1))}</span><span class="dashboard-app-copy"><strong>${esc(item.name)}</strong><small>${esc(item.description || t('暂无用途说明'))}</small></span><button class="btn btn-ghost btn-sm" data-restore-app="${esc(item.id)}">${esc(t('恢复'))}</button></div>`).join('');
    return;
  }
  if (templates) {
    await renderAppTemplates();
    return;
  }
  if (editingForm) {
    $('#app-form-title').textContent = t('修改应用信息');
    const form = $('#edit-app-form');
    form.querySelector('[name="name"]').value = state.editingApp?.name || '';
    form.querySelector('[name="description"]').value = state.editingApp?.description || '';
    form.querySelector('button[type="submit"]').textContent = t('保存修改');
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
  root.innerHTML = `<p class="runtime-loading">${esc(t('正在读取应用模板…'))}</p>`;
  try {
    const result = await api('/api/build/templates');
    const card = (item) => {
      const tables = Array.isArray(item.definition?.tables) ? item.definition.tables.length : null;
      return `<article class="app-template-card"><div class="app-template-card-head"><span class="app-template-mark">${esc(item.name.slice(0, 1))}</span><h3>${esc(item.name)}</h3></div><p>${esc(item.description || '')}</p><div class="app-template-card-footer"><small>${tables == null ? '' : esc(t('{count} 张数据表', { count: tables }))}</small><button class="btn btn-primary btn-sm" data-create-template="${esc(item.id)}">${esc(t('一键创建'))}</button></div></article>`;
    };
    root.innerHTML = (result.items || []).map(card).join('') || `<p class="dashboard-empty">${esc(t('暂时没有可用模板。'))}</p>`;
    root.dataset.loaded = 'true';
  } catch (error) {
    root.innerHTML = `<p class="alert alert-error" role="alert">${esc(error.message || t('应用模板暂时无法读取'))}</p>`;
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
    toast(t('应用信息已更新'));
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
  if (!state.app || !window.confirm(t('归档此应用？归档后会从工作区列表隐藏，数据仍可恢复。'))) return;
  try {
    await api(`/api/apps/${state.app.id}`, { method: 'PATCH', body: JSON.stringify({ archived: true }) });
    await refreshApps();
    toast(t('应用已归档'));
  } catch (error) { toast(error.message, true); }
}

async function restoreApp(appId) {
  try {
    await api(`/api/apps/${encodeURIComponent(appId)}`, { method: 'PATCH', body: JSON.stringify({ archived: false }) });
    await refreshApps();
    toast(t('应用已恢复'));
  } catch (error) { toast(error.message, true); }
}

async function deleteApp() {
  if (!state.app || !window.confirm(t('删除「{name}」并保留数据表及记录？应用配置和访问入口会移除。', { name: state.app.name }))) return;
  try {
    await api(`/api/apps/${state.app.id}`, { method: 'DELETE', body: JSON.stringify({ confirm: true }) });
    await refreshApps();
    toast(t('应用已删除，数据表及记录已保留'));
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
    toast(result.message || t('如果邮箱已登记，重置邮件将发送到邮箱'));
  } catch (error) { toast(error.message, true); }
}

async function submitPasswordReset(event) {
  event.preventDefault();
  const password = new FormData(event.target).get('password');
  try {
    await api('/api/auth/password-reset/confirm', { method: 'POST', body: JSON.stringify({ token: state.resetToken, password }) });
    try {
      const saved = JSON.parse(localStorage.getItem(LAST_LOGIN_KEY) || 'null');
      if (saved?.email) rememberLastLogin(saved.email, password);
    } catch {}
    $('#password-reset-dialog').close();
    state.resetToken = null;
    history.replaceState({}, '', '/');
    toast(t('密码已更新，请使用新密码登录'));
    authMode('login');
  } catch (error) { toast(error.message, true); }
}

async function submitAccountDeletion(event) {
  event.preventDefault();
  const data = Object.fromEntries(new FormData(event.target));
  try {
    await api('/api/me', { method: 'DELETE', body: JSON.stringify(data) });
    $('#account-delete-dialog').close();
    localStorage.removeItem(LAST_LOGIN_KEY);
    await logout();
    toast(t('账号和相关数据已删除'));
  } catch (error) { toast(error.message, true); }
}

async function submitAccountDeactivation(event) {
  event.preventDefault();
  const password = new FormData(event.target).get('password');
  try {
    await api('/api/me/deactivate', { method: 'POST', body: JSON.stringify({ password, confirm: true }) });
    $('#account-deactivate-dialog').close();
    await logout();
    toast(t('账号已停用'));
  } catch (error) { toast(error.message, true); }
}

async function openAppAccess() {
  if (!state.app) return;
  try {
    const access = await api(`/api/apps/${state.app.id}/access`);
    $('#app-access-form [name="restricted"]').checked = access.restricted;
    $('#app-access-members').innerHTML = access.members.map((member) => `<label class="member-row"><span>${esc(member.name)} · ${esc(member.email)}<small>${member.workspace_role === 'admin' ? esc(t('管理员')) : esc(t('成员'))}</small></span><select class="select select-bordered select-sm" data-access-user="${esc(member.id)}"><option value="">${esc(t('无权限'))}</option><option value="viewer" ${member.app_role === 'viewer' ? 'selected' : ''}>${esc(t('只读'))}</option><option value="editor" ${member.app_role === 'editor' ? 'selected' : ''}>${esc(t('可编辑记录'))}</option><option value="manager" ${member.app_role === 'manager' ? 'selected' : ''}>${esc(t('管理应用'))}</option><option value="publisher" ${member.app_role === 'publisher' ? 'selected' : ''}>${esc(t('管理并发布'))}</option></select><span><input type="checkbox" data-batch-user="${esc(member.id)}" ${member.can_batch ? 'checked' : ''}> ${esc(t('批量修改'))}</span></label>`).join('') || `<p class="empty-members">${esc(t('当前工作区没有其他成员。'))}</p>`;
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
    toast(t('应用访问权限已更新'));
  } catch (error) { toast(error.message, true); }
}

async function downloadWorkspaceExport() {
  try {
    const response = await fetch('/api/workspace/export', { headers: { Authorization: `Bearer ${state.token}`, 'X-Miao-Tenant-Id': state.tenant.id } });
    if (!response.ok) throw new Error((await response.json().catch(() => ({}))).error || t('数据导出失败'));
    const blob = await response.blob();
    const href = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = href;
    link.download = `miao-workspace-${state.tenant.id}.json`;
    link.click();
    setTimeout(() => URL.revokeObjectURL(href), 1000);
    toast(t('工作区数据已导出'));
  } catch (error) { toast(error.message, true); }
}

function managementNotice(message = '') {
  $('#workspace-management-notice').textContent = message;
  $('#workspace-management-notice').classList.toggle('hidden', !message);
}

async function openWorkspaceManagement(page = 'members') {
  if (state.agentBusy) { toast(t('小助手正在处理，请稍后再进入工作区管理。'), true); return; }
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
    const role = { owner: t('所有者'), admin: t('管理员'), member: t('成员') }[state.tenant?.role] || t('成员');
    $('#workspace-settings-summary').innerHTML = `<div><dt>${esc(t('工作区名称'))}</dt><dd>${esc(state.tenant?.name)}</dd></div><div><dt>${esc(t('当前角色'))}</dt><dd>${esc(role)}</dd></div><div><dt>${esc(t('可访问应用'))}</dt><dd>${state.apps.length}</dd></div>`;
  }
}

async function loadAuditLog(page = 1) {
  const tenantId = state.tenant?.id;
  $('#audit-log-list').textContent = t('正在读取操作日志…');
  try {
    const result = await api(`/api/workspace/audit?page=${Math.max(1, page)}`);
    if (tenantId !== state.tenant?.id || state.workspaceView !== 'management') return;
    state.workspaceAuditPage = result.page;
    $('#audit-log-list').innerHTML = result.items.map((item) => `<div class="audit-log-row"><span><strong>${esc(item.action)} ${esc(item.route)}</strong><small>${esc(item.actor_email)} · ${esc(fmtDateTime(item.created_at))} · ${item.status}</small></span><code>${esc(item.target_id || '—')}</code></div>`).join('') || `<p class="empty-members">${esc(t('还没有操作记录。'))}</p>`;
    $('#audit-log-page-label').textContent = t('第 {page} / {pages} 页 · 共 {total} 条', { page: result.page, pages: Math.max(1, result.totalPages), total: result.totalItems });
    $('[data-audit-delta="-1"]').disabled = result.page <= 1;
    $('[data-audit-delta="1"]').disabled = result.page >= result.totalPages;
  } catch (error) {
    if (tenantId !== state.tenant?.id) return;
    $('#audit-log-list').textContent = t('操作日志暂时不可用，请刷新重试。');
    managementNotice(error.message);
  }
}


async function loadMembers() {
  const tenantId = state.tenant?.id;
  $('#member-list').textContent = t('正在读取成员…');
  $('#pending-invites').replaceChildren();
  $('#invite-form').classList.add('hidden');
  try {
    const [result, invites] = await Promise.all([
      api('/api/workspace/members'),
      ['owner', 'admin'].includes(state.tenant?.role) ? api('/api/workspace/invites') : Promise.resolve([])
    ]);
    if (tenantId !== state.tenant?.id || state.workspaceView !== 'management') return;
    $('#member-list').innerHTML = result.members.map((member) => `<div class="member-row"><span>${esc(member.name)} · ${esc(member.email)}<small>${member.role === 'owner' ? esc(t('所有者')) : member.role === 'admin' ? esc(t('管理员')) : esc(t('成员'))}</small></span>${result.can_edit_roles && member.role !== 'owner' ? `<span class="member-actions"><select class="select select-bordered select-xs" data-member-role="${esc(member.membership_id)}"><option value="member" ${member.role === 'member' ? 'selected' : ''}>${esc(t('成员'))}</option><option value="admin" ${member.role === 'admin' ? 'selected' : ''}>${esc(t('管理员'))}</option></select><button class="btn btn-ghost btn-xs" data-remove-member="${esc(member.membership_id)}">${esc(t('从工作区移除'))}</button></span>` : ''}</div>`).join('') || `<p class="empty-members">${esc(t('还没有成员。'))}</p>`;
    $('#invite-form').classList.toggle('hidden', !result.can_manage);
    if (!result.can_manage) {
      $('#invite-link').value = '';
      $('#invite-link-row').classList.add('hidden');
    }
    $('#pending-invites').innerHTML = invites.map((invite) => `<div class="pending-invite-row"><span>${esc(invite.email)}<small>${esc(t('邀请待接受'))} · ${esc(fmtDateTime(invite.expires_at))}</small></span><button class="btn btn-ghost btn-xs" data-revoke-invite="${esc(invite.id)}">${esc(t('撤销'))}</button></div>`).join('');
  } catch (error) {
    if (tenantId !== state.tenant?.id) return;
    $('#member-list').textContent = t('成员列表暂时不可用，请刷新重试。');
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
    let copied = false;
    try {
      await navigator.clipboard.writeText($('#invite-link').value);
      copied = true;
    } catch {}
    if (invite.emailed) toast(copied ? t('邀请已发送至 {email}，链接已复制', { email: invite.email }) : t('邀请已发送至 {email}', { email: invite.email }));
    else toast(copied ? t('邮件服务未配置，链接已复制，可直接发给同事') : t('邮件服务未配置，请复制链接发给同事'));
    await openMembers();
  } catch (error) {
    toast(error.message, true);
  }
}

async function removeMember(membershipId) {
  if (!window.confirm(t('移除此成员后，对方将无法继续访问当前工作区。'))) return;
  try {
    await api(`/api/workspace/members/${encodeURIComponent(membershipId)}`, { method: 'DELETE' });
    await openMembers();
    toast(t('成员已移除'));
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
    toast(t('邀请已撤销'));
  } catch (error) {
    toast(error.message, true);
  }
}


document.addEventListener('click', async (event) => {
  for (const menu of $$('details.app-manage-menu[open], details.account-actions[open], #workspace-switcher[open]')) {
    if (!menu.contains(event.target) || event.target.closest('button')) menu.removeAttribute('open');
  }
  try { if (await appSettings.click(event)) return; } catch (error) { toast(error.message || t('应用配置操作失败'), true); }
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
  const languageButton = event.target.closest('[data-lang]');
  if (languageButton) {
    await setLanguage(languageButton.dataset.lang);
    location.reload();
    return;
  }
  const workspaceOption = event.target.closest('[data-workspace-select]');
  if (workspaceOption) {
    await switchWorkspace(workspaceOption.dataset.workspaceSelect);
    return;
  }
  if (action === 'workspace-create') { openWorkspaceCreate(); return; }
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
    if (!(state.workspaceView === 'home' || (state.workspaceView === 'app' && state.app))) {
      state.workspaceView = 'home';
      state.app = null;
      state.appPanel = 'runtime';
      state.table = null;
    }
    await agentAssistant.enterConversation();
    await renderWorkspace();
    $('#agent-form [name="prompt"]').focus();
  }
  if (action === 'toggle-nav') {
    state.navOpen = !state.navOpen;
    await renderWorkspace();
  }
  const templateButton = event.target.closest('[data-create-template]');
  if (templateButton) {
    await agentAssistant.createTemplate(templateButton.dataset.createTemplate).catch((error) => toast(error.message || t('应用模板创建失败'), true));
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
  if (action === 'close-workspace-create') $('#workspace-create-dialog').close();
  if (action === 'delete-account') {
    $('#account-delete-form [name="confirm"]').value = '';
    $('#account-delete-form [name="confirm"]').placeholder = state.user?.email || '';
    $('#account-delete-dialog').showModal();
  }
  if (action === 'deactivate-account') $('#account-deactivate-dialog').showModal();
  if (action === 'close-account-deactivate') $('#account-deactivate-dialog').close();
  if (action === 'close-account-delete') $('#account-delete-dialog').close();
  if (action === 'ai-usage') workspaceAIUsage.open();
  if (action === 'app-access') openAppAccess();
  if (action === 'close-app-access') $('#app-access-dialog').close();
  if (action === 'export-data') downloadWorkspaceExport();
  const auditPageButton = event.target.closest('[data-audit-delta]');
  if (auditPageButton) openAuditLog(state.workspaceAuditPage + Number(auditPageButton.dataset.auditDelta));
  const downloadFileButton = event.target.closest('[data-download-file]');
  if (downloadFileButton) {
    try {
      const path = `/api/apps/${encodeURIComponent(state.app.id)}/collections/${encodeURIComponent(downloadFileButton.dataset.fileCollection || state.recordFormContext?.collection || state.table?.slug || state.appRuntime?.collection)}/records/${encodeURIComponent(downloadFileButton.dataset.recordId)}/files/${encodeURIComponent(downloadFileButton.dataset.downloadFile)}`;
      const response = await fetch(path, { headers: { Authorization: `Bearer ${state.token}`, 'X-Miao-Tenant-Id': state.tenant.id } });
      if (!response.ok) throw new Error(t('附件下载失败'));
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
  if (action === 'copy-invite') {
    navigator.clipboard?.writeText($('#invite-link').value)
      .then(() => toast(t('链接已复制')))
      .catch(() => toast(t('复制失败，请手动复制链接'), true));
  }
  const removeMemberButton = event.target.closest('[data-remove-member]');
  if (removeMemberButton) removeMember(removeMemberButton.dataset.removeMember);
  const revokeInviteButton = event.target.closest('[data-revoke-invite]');
  if (revokeInviteButton) revokeInvite(revokeInviteButton.dataset.revokeInvite);
  const appButton = event.target.closest('[data-open-app]');
  if (appButton) {
    const appId = appButton.dataset.openApp;
    if (state.app?.id !== appId) api(`/api/apps/${encodeURIComponent(appId)}/visit`, { method: 'POST' }).then((result) => {
      const cached = state.apps.find((item) => item.id === appId);
      if (cached && result?.view_count != null) cached.view_count = result.view_count;
    }).catch(() => {});
    state.editingApp = false;
    state.app = state.apps.find((item) => item.id === appId) || null;
    state.appPanel = 'runtime';
    state.runtimeQuery = { page: 1, search: '' };
    state.table = null;
    state.workspaceView = 'app';
    await renderWorkspace();
  }
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
  try { if (await appSettings.submit(event)) return; } catch (error) { toast(error.message || t('应用配置保存失败'), true); }
});

document.addEventListener('change', (event) => {
  if (event.target.matches('[name="attachment"]')) { const label = event.target.parentElement.querySelector('[data-attachment-name]'); if (label) label.textContent = event.target.files?.[0]?.name || ''; return; }
  if (event.target.matches('#assistant-app-selector')) {
    agentAssistant.selectApp(event.target.value).catch((error) => toast(error.message, true));
    return;
  }
  if (event.target.matches('[data-member-role]')) {
    api(`/api/workspace/members/${encodeURIComponent(event.target.dataset.memberRole)}`, { method: 'PATCH', body: JSON.stringify({ role: event.target.value }) })
      .then(async () => { await loadMembers(); toast(t('成员角色已更新')); }).catch(async (error) => { await loadMembers(); toast(error.message, true); });
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
    toast(t('助手正在处理，请稍后再切换工作区。'), true);
    return;
  }
  const selected = state.workspaces.find((workspace) => workspace.id === workspaceId);
  if (!selected || selected.id === state.tenant?.id) return;
  $('#invite-link').value = '';
  $('#invite-link-row').classList.add('hidden');
  state.workspaceAuditPage = 1;
  workspaceAIUsage.reset();
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
  await agentAssistant.enterConversation();
  await renderWorkspace();
}

function openWorkspaceCreate() {
  if (state.agentBusy) {
    toast(t('小助手正在处理，请稍后再创建工作区。'), true);
    return;
  }
  $('#workspace-create-form').reset();
  $('#workspace-create-error').classList.add('hidden');
  $('#workspace-create-dialog').showModal();
}

async function submitWorkspaceCreate(event) {
  event.preventDefault();
  const name = String(new FormData(event.target).get('name') || '').trim();
  const submit = $('#workspace-create-submit');
  submit.disabled = true;
  try {
    const workspace = await api('/api/workspaces', { method: 'POST', body: JSON.stringify({ name }) });
    $('#workspace-create-dialog').close();
    state.workspaces.push(workspace);
    toast(t('工作区「{name}」已创建', { name: workspace.name }));
    await switchWorkspace(workspace.id);
  } catch (error) {
    const box = $('#workspace-create-error');
    box.textContent = error.message;
    box.classList.remove('hidden');
  } finally {
    submit.disabled = false;
  }
}

const appRuntimeModule = createAppRuntime({ state, api, stream, $, esc, fmtDateTime });
const workspaceData = createWorkspaceData({ state, api, $, $$, esc, toast, renderWorkspace });
workspaceData.bind();
const agentAssistant = createAgentAssistant({ state, api, stream, $, esc, toast, renderWorkspace });
const appTasks = createAppTasks({ state, api, $, esc, toast });
const appSettings = createAppSettings({ state, api, $, esc, toast });
const workspaceAIUsage = createWorkspaceAIUsage({ state, api, $, esc, toast, renderWorkspace });
workspaceAIUsage.bind();
const notifications = createNotifications({ state, api, $, esc, toast, renderWorkspace, appTasks, appSettings });
const platformAdmin = createPlatformAdmin({ state, api, $, $$, esc, toast, show, renderApps, renderWorkspace });
platformAdmin.bind();

$('#auth-form').addEventListener('submit', submitAuth);
$('#edit-app-form').addEventListener('submit', saveAppDetails);
$('#switch-auth').addEventListener('click', () => authMode(state.authMode === 'register' ? 'login' : 'register'));
$('#password-reset-form').addEventListener('submit', submitPasswordReset);
$('#workspace-create-form').addEventListener('submit', submitWorkspaceCreate);
$('#request-reset-form').addEventListener('submit', submitPasswordResetRequest);
$('#account-delete-form').addEventListener('submit', submitAccountDeletion);
$('#account-deactivate-form').addEventListener('submit', submitAccountDeactivation);
$('#app-access-form').addEventListener('submit', saveAppAccess);
$('#agent-form').addEventListener('submit', agentAssistant.submitPrompt);
$('#agent-form').querySelector('[name="prompt"]').addEventListener('keydown', (event) => {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault();
    $('#agent-form').requestSubmit();
  }
});
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
function markLanguageButtons() {
  for (const button of $$('[data-lang]')) button.classList.toggle('lang-active', button.dataset.lang === getLanguage());
}
markLanguageButtons();
bootstrap();
