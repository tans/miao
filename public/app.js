import { createFxAgent, supportsJspi } from '/vendor/fx/browser.js';

const TOKEN_KEY = 'miao_token';
const state = {
  token: localStorage.getItem(TOKEN_KEY), user: null, tenant: null,
  workspaces: [], apps: [], archivedApps: [], app: null, tables: [], table: null, records: [],
  recordQuery: { page: 1, perPage: 25, search: '', sort: '-created', filterField: '', filterValue: '' }, recordResult: null,
  editingRecordId: null, editingApp: false, workspaceView: 'home', appPanel: 'runtime',
  authMode: 'register', fxAgent: null, fxBusy: false, isPlatformAdmin: false,
  workspaceAuditPage: 1,
  admin: {
    page: 'overview',
    users: { page: 1, q: '', status: '' },
    workspaces: { page: 1, q: '' },
    apps: { page: 1, q: '', archived: '' },
    usage: { page: 1, from: '', to: '' },
    audit: { page: 1, targetType: '', targetId: '' }
  },
  aiConfigured: false, pendingInvite: new URLSearchParams(location.search).get('invite')
};
const $ = (selector) => document.querySelector(selector);
const $$ = (selector) => [...document.querySelectorAll(selector)];
const esc = (value) => String(value ?? '').replace(/[&<>"']/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' }[char]));
const toast = (message, error = false) => {
  const node = $('#toast');
  node.innerHTML = `<div class="alert ${error ? 'alert-error' : 'alert-success'}"><span>${esc(message)}</span></div>`;
  setTimeout(() => { node.replaceChildren(); }, 3000);
};

async function api(url, options = {}) {
  const headers = { ...(options.body instanceof FormData ? {} : options.body ? { 'Content-Type': 'application/json' } : {}), ...(options.headers || {}) };
  if (state.token) headers.Authorization = `Bearer ${state.token}`;
  if (state.tenant?.id) headers['X-Miao-Tenant-Id'] = state.tenant.id;
  const response = await fetch(url, { ...options, headers });
  const nextToken = response.headers.get('X-PocketBase-Token');
  if (nextToken) {
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
    return api('/api/me');
  }
}

function show(screen) {
  for (const id of ['landing', 'auth', 'workspace', 'platform-admin']) $(`#${id}`).classList.toggle('hidden', id !== screen);
}

const adminPageTitles = { overview: '平台总览', users: '用户账号', workspaces: '工作区', apps: '应用目录', usage: 'AI 用量', audit: '平台审计', ai: 'AI 服务' };
const adminRoutePage = () => {
  if (!location.pathname.startsWith('/admin')) return '';
  const page = location.pathname.split('/').filter(Boolean)[1] || 'overview';
  return Object.hasOwn(adminPageTitles, page) ? page : 'overview';
};

function authMode(mode) {
  state.authMode = mode;
  const registering = mode === 'register';
  $('#auth-title').textContent = registering ? '创建工作区' : '欢迎回来';
  $('#auth-copy').textContent = registering ? '先创建一个工作区，再开始搭建内部工具。' : '登录后继续管理内部工具。';
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
  if (state.fxAgent) state.fxAgent.close().catch(() => {});
  state.fxAgent = null;
  state.fxBusy = false;
}

function resetAgentConversation() {
  $('#chat-messages').innerHTML = '<div class="assistant-intro"><img src="/mascots/cat-peek-square.png" alt="" /><div><h3>你想把什么工作做好？</h3><p>先说目标、现在的做法和最麻烦的地方。我会帮你梳理流程、提出方案，并在需要时创建工具、处理信息和推进任务。重要变更会先征求你的确认。</p><div class="conversation-prompts"><button class="btn btn-outline btn-sm" data-prompt="帮我梳理每周团队周报的收集和汇总流程">梳理一个工作流程</button><button class="btn btn-outline btn-sm" data-prompt="我想做一个客户跟进流程，先帮我想清楚怎么开始">从一个想法开始</button></div></div></div>';
  $('#agent-status').textContent = '准备开始';
  $('#agent-status').className = 'badge badge-ghost';
}

function logout() {
  if (state.token) api('/api/auth/logout', { method: 'POST' }).catch(() => {});
  clearAgent();
  resetAgentConversation();
  state.token = null;
  state.apps = [];
  state.workspaces = [];
  state.app = null;
  state.appPanel = 'runtime';
  state.tenant = null;
  state.tables = [];
  state.table = null;
  state.records = [];
  state.recordResult = null;
  state.isPlatformAdmin = false;
  localStorage.removeItem(TOKEN_KEY);
  history.replaceState({}, '', '/');
  show('landing');
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
    return show('landing');
  }
  if (location.pathname === '/reset-password') {
    state.resetToken = new URLSearchParams(location.search).get('token');
    $('#password-reset-dialog').showModal();
    return;
  }
  if (!state.token) return state.pendingInvite ? authMode('register') : show('landing');
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
    const requestedAdminPage = adminRoutePage();
    if (requestedAdminPage && state.isPlatformAdmin) await openPlatformAdmin(requestedAdminPage, false);
    else {
      if (requestedAdminPage) history.replaceState({}, '', '/');
      show('workspace');
      await renderWorkspace();
    }
  } catch {
    logout();
  }
}

async function submitAuth(event) {
  event.preventDefault();
  const form = new FormData(event.currentTarget);
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
    const requestedAdminPage = adminRoutePage();
    if (requestedAdminPage && state.isPlatformAdmin) await openPlatformAdmin(requestedAdminPage, false);
    else {
      if (requestedAdminPage) history.replaceState({}, '', '/');
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
  $('#workspace-switcher').disabled = state.fxBusy;
  $('#user-name').textContent = state.user?.name || '用户';
  $('#user-email').textContent = state.user?.email || '';
  $('#user-avatar').textContent = (state.user?.name || 'M').slice(0, 1);
  $('#ai-config-status').textContent = state.aiConfigured ? 'fx 助手已连接企业 AI 服务。' : '企业尚未配置 AI 服务，请联系管理员。';
  $('#ai-config-status').classList.toggle('error', !state.aiConfigured);
  $('#platform-admin-entry').classList.toggle('hidden', !state.isPlatformAdmin);
  $('#audit-log-trigger').classList.toggle('hidden', state.tenant?.role !== 'owner');
  $('#app-list').innerHTML = state.apps.map((item) => `<button class="app-nav-item ${state.workspaceView === 'app' && state.app?.id === item.id ? 'active' : ''}" data-open-app="${esc(item.id)}"><span class="app-nav-mark">${esc(item.name.slice(0, 1))}</span>${esc(item.name)}</button>`).join('') || '<p class="empty-app-nav">还没有应用</p>';
  const homeLink = $('.workspace-home-link');
  const assistantLink = $('.assistant-nav-link');
  homeLink.classList.toggle('active', state.workspaceView === 'home');
  assistantLink.classList.toggle('active', state.workspaceView === 'assistant');
  if (state.workspaceView === 'home') homeLink.setAttribute('aria-current', 'page');
  else homeLink.removeAttribute('aria-current');
  if (state.workspaceView === 'assistant') assistantLink.setAttribute('aria-current', 'page');
  else assistantLink.removeAttribute('aria-current');
}

async function renderWorkspace() {
  renderApps();
  const dashboard = state.workspaceView === 'home';
  const assistant = state.workspaceView === 'assistant';
  const editingForm = state.workspaceView === 'edit';
  const appView = state.workspaceView === 'app' && Boolean(state.app);
  const dataInspection = appView && state.appPanel === 'data';
  if (!appView) $('#app-actions-menu').open = false;
  $('#dashboard').classList.toggle('hidden', !dashboard);
  $('#app-creation').classList.toggle('hidden', !editingForm);
  $('#app-runtime').classList.toggle('hidden', !appView || dataInspection);
  $('#app-content').classList.toggle('hidden', !dataInspection);
  $('#fx-assistant-view').classList.toggle('hidden', !assistant);
  const canManageApp = Boolean(appView && state.tenant?.role === 'owner');
  $('#app-actions-menu').classList.toggle('hidden', !appView);
  $('#app-return-entry').classList.toggle('hidden', !dataInspection);
  $('#app-data-entry').classList.toggle('hidden', dataInspection);
  $('#app-create-table-entry').classList.toggle('hidden', !dataInspection || state.app.permission === 'viewer');
  $('#app-access-entry').classList.toggle('hidden', !canManageApp);
  $('#app-edit-entry').classList.toggle('hidden', !canManageApp);
  $('#app-archive-entry').classList.toggle('hidden', !canManageApp);
  $('#app-delete-entry').classList.toggle('hidden', !canManageApp);
  $('#app-title').textContent = dashboard ? '应用工作台' : assistant ? 'fx 助手' : editingForm ? '修改应用' : state.app?.name || '工作台';
  $('#breadcrumb-app').textContent = dashboard ? '概览' : assistant ? 'fx 助手' : state.app?.name || '修改应用';
  $('#app-description').textContent = dashboard ? '选择应用继续工作，或告诉 fx 你想完成什么。' : assistant ? '梳理工作流程、查询信息，并在你确认后推进具体操作。' : editingForm ? '' : state.app?.description || '';
  if (dashboard) {
    $('#dashboard-workspace-name').textContent = state.tenant?.name || '';
    $('#dashboard-apps').innerHTML = state.apps.length ? state.apps.map((item) => `<button class="dashboard-app-card" data-open-app="${esc(item.id)}"><span class="dashboard-app-icon">${esc(item.name.slice(0, 1))}</span><span class="dashboard-app-copy"><strong>${esc(item.name)}</strong><small>${esc(item.description || '暂无用途说明')}</small><small>更新于 ${item.updated_at ? new Date(item.updated_at).toLocaleDateString() : '时间未知'}</small></span><span aria-hidden="true">→</span></button>`).join('') : '<div class="dashboard-empty"><strong>还没有应用</strong><span>从上方描述你想做的应用，fx 会先了解需求，再和你一起设计。</span></div>';
    $('#archived-app-section').classList.toggle('hidden', !state.archivedApps.length);
    $('#archived-apps').innerHTML = state.archivedApps.map((item) => `<div class="dashboard-app-card"><span class="dashboard-app-icon">${esc(item.name.slice(0, 1))}</span><span class="dashboard-app-copy"><strong>${esc(item.name)}</strong><small>${esc(item.description || '暂无用途说明')}</small></span><button class="btn btn-ghost btn-sm" data-restore-app="${esc(item.id)}">恢复</button></div>`).join('');
    return;
  }
  if (editingForm) {
    $('#app-form-title').textContent = '修改应用信息';
    $('#app-form-copy').textContent = '更新名称和用途说明，保存后立即生效。';
    const form = $('#edit-app-form');
    form.querySelector('[name="name"]').value = state.editingApp?.name || '';
    form.querySelector('[name="description"]').value = state.editingApp?.description || '';
    form.querySelector('button[type="submit"]').textContent = '保存修改';
    return;
  }
  if (!appView) return;
  state.editingApp = false;
  if (!dataInspection) return;
  state.tables = await api(`/api/apps/${state.app.id}/collections`);
  if (!state.tables.some((item) => item.slug === state.table?.slug)) state.table = state.tables[0] || null;
  renderTables();
  if (state.table) await renderRecords();
  else renderNoTables();
}

const adminLoaders = {
  overview: loadAdminOverview,
  users: loadAdminUsers,
  workspaces: loadAdminWorkspaces,
  apps: loadAdminApps,
  usage: loadAdminUsage,
  audit: loadAdminAudit,
  ai: loadAdminAI,
};

async function openPlatformAdmin(page = 'overview', updateHistory = true) {
  if (!state.isPlatformAdmin) {
    if (location.pathname.startsWith('/admin')) history.replaceState({}, '', '/');
    show('workspace');
    toast('此账号无权访问平台后台', true);
    return;
  }
  const selectedPage = Object.hasOwn(adminPageTitles, page) ? page : 'overview';
  if (updateHistory) history.pushState({}, '', `/admin/${selectedPage}`);
  state.admin.page = selectedPage;
  $('#admin-current-user').textContent = `${state.user?.name || ''} · ${state.user?.email || ''}`;
  $('#admin-page-title').textContent = adminPageTitles[selectedPage];
  $('#admin-page-notice').classList.add('hidden');
  for (const section of $$('[data-admin-section]')) section.classList.toggle('hidden', section.dataset.adminSection !== selectedPage);
  for (const button of $$('[data-admin-page]')) {
    if (button.dataset.adminPage === selectedPage) button.setAttribute('aria-current', 'page');
    else button.removeAttribute('aria-current');
  }
  show('platform-admin');
  await adminLoaders[selectedPage]();
}

async function returnToWorkspace() {
  history.pushState({}, '', '/');
  state.app = null;
  state.appPanel = 'runtime';
  state.table = null;
  state.workspaceView = 'home';
  show('workspace');
  await renderWorkspace();
}

function setAdminNotice(message = '') {
  const notice = $('#admin-page-notice');
  notice.textContent = message;
  notice.classList.toggle('hidden', !message);
}

function updateAdminPager(name, result) {
  const label = $(`#admin-${name}-page-label`);
  if (label) label.textContent = `第 ${result.page} / ${Math.max(1, result.totalPages)} 页 · 共 ${result.totalItems} 条`;
  const previous = $(`[data-admin-prev="${name}"]`);
  const next = $(`[data-admin-next="${name}"]`);
  if (previous) previous.disabled = result.page <= 1;
  if (next) next.disabled = result.page >= result.totalPages;
}

const dateTimeLabel = (value) => value ? new Date(value).toLocaleString() : '—';
const numberLabel = (value) => Number(value || 0).toLocaleString();

async function loadAdminOverview() {
  setAdminNotice();
  $('#admin-overview-usage-body').innerHTML = '<tr><td colspan="6">正在读取用量…</td></tr>';
  try {
    const now = new Date();
    const todayStart = new Date(now);
    todayStart.setUTCHours(0, 0, 0, 0);
    const weekStart = new Date(todayStart.getTime() - 6 * 24 * 60 * 60 * 1000);
    const [overview, runtime, usage] = await Promise.all([
      api('/api/admin/overview'),
      api('/api/admin/runtime'),
      api(`/api/admin/usage?from=${encodeURIComponent(weekStart.toISOString())}&to=${encodeURIComponent(now.toISOString())}&page=1&perPage=8`),
    ]);
    $('#admin-overview-users').textContent = numberLabel(overview.users.total);
    $('#admin-overview-users-desc').textContent = `${numberLabel(overview.users.active)} 可用 · ${numberLabel(overview.users.disabled)} 已停用`;
    $('#admin-overview-workspaces').textContent = numberLabel(overview.workspaces);
    $('#admin-overview-apps').textContent = numberLabel(overview.apps);
    $('#admin-overview-ai').textContent = numberLabel(overview.ai_today.requests);
    $('#admin-overview-ai-desc').textContent = `${numberLabel(overview.ai_today.errors)} 次失败 · ${numberLabel(overview.ai_today.input_tokens + overview.ai_today.output_tokens)} tokens`;
    const registrationLabels = { open: '开放注册', invite: '仅邀请', closed: '关闭注册', invalid: '配置无效' };
    const runtimeRows = [
      ['注册方式', `${registrationLabels[runtime.registration.mode] || '未配置'}${runtime.registration.email_verification_required ? ' · 需验证邮箱' : ''}${runtime.registration.allowed_email_domains.length ? ` · 限制 ${runtime.registration.allowed_email_domains.length} 个邮箱域` : ''}`],
      ['邮件服务', runtime.mail.configured ? '已配置' : '未配置'],
      ['AI 服务', runtime.ai.configured ? `${runtime.ai.provider === 'capi' ? 'CAPI' : 'Vercel Gateway'} · ${esc(runtime.ai.model)} · ${runtime.ai.source === 'admin' ? '管理后台密钥' : '环境变量'}` : '未配置'],
    ];
    $('#admin-runtime-summary').innerHTML = runtimeRows.map(([title, value]) => `<div><dt>${title}</dt><dd>${value}</dd></div>`).join('');
    renderAdminUsageRows($('#admin-overview-usage-body'), usage.items, 6);
  } catch (error) {
    setAdminNotice(error.message || '平台总览读取失败');
    $('#admin-overview-usage-body').innerHTML = '<tr><td colspan="6">用量暂时不可用</td></tr>';
  }
}

async function loadAdminUsers() {
  const filter = state.admin.users;
  const bodyNode = $('#admin-users-body');
  bodyNode.innerHTML = '<tr><td colspan="5">正在读取用户…</td></tr>';
  setAdminNotice();
  try {
    const params = new URLSearchParams({ page: String(filter.page), perPage: '25' });
    if (filter.q) params.set('q', filter.q);
    if (filter.status) params.set('status', filter.status);
    const result = await api(`/api/admin/users?${params}`);
    bodyNode.innerHTML = result.items.length ? result.items.map((user) => {
      const status = user.disabled ? '<span class="badge badge-error badge-soft">已停用</span>' : '<span class="badge badge-success badge-soft">可用</span>';
      const verified = user.verified ? '<span class="badge badge-success badge-soft">已验证</span>' : '<span class="badge badge-warning badge-soft">未验证</span>';
      const isCurrentUser = user.id === state.user?.id;
      const action = isCurrentUser ? '<span class="badge badge-ghost">当前账号</span>' : `<button class="btn btn-xs" data-admin-user-status data-user-id="${esc(user.id)}" data-user-email="${esc(user.email)}" data-user-disabled="${user.disabled}">${user.disabled ? '恢复账号' : '停用账号'}</button>`;
      return `<tr><td><strong>${esc(user.name || '未填写姓名')}</strong><small class="admin-cell-secondary">${esc(user.email)}</small></td><td>${verified}</td><td>${status}</td><td>${dateTimeLabel(user.created_at)}</td><td>${action}</td></tr>`;
    }).join('') : '<tr><td colspan="5">没有符合条件的账号。</td></tr>';
    updateAdminPager('users', result);
  } catch (error) {
    bodyNode.innerHTML = '<tr><td colspan="5">用户列表暂时不可用。</td></tr>';
    setAdminNotice(error.message || '用户列表读取失败');
  }
}

async function loadAdminWorkspaces() {
  const filter = state.admin.workspaces;
  const bodyNode = $('#admin-workspaces-body');
  bodyNode.innerHTML = '<tr><td colspan="5">正在读取工作区…</td></tr>';
  setAdminNotice();
  try {
    const params = new URLSearchParams({ page: String(filter.page), perPage: '25' });
    if (filter.q) params.set('q', filter.q);
    const result = await api(`/api/admin/workspaces?${params}`);
    bodyNode.innerHTML = result.items.length ? result.items.map((workspace) => `<tr><td><strong>${esc(workspace.name)}</strong><small class="admin-cell-secondary">${esc(workspace.slug)}</small></td><td>${workspace.owner ? `${esc(workspace.owner.name)}<small class="admin-cell-secondary">${esc(workspace.owner.email)}</small>` : '<span class="badge badge-warning">所有者不存在</span>'}</td><td>${numberLabel(workspace.member_count)}</td><td>${numberLabel(workspace.app_count)}</td><td>${dateTimeLabel(workspace.created_at)}</td></tr>`).join('') : '<tr><td colspan="5">没有符合条件的工作区。</td></tr>';
    updateAdminPager('workspaces', result);
  } catch (error) {
    bodyNode.innerHTML = '<tr><td colspan="5">工作区列表暂时不可用。</td></tr>';
    setAdminNotice(error.message || '工作区列表读取失败');
  }
}

async function loadAdminApps() {
  const filter = state.admin.apps;
  const bodyNode = $('#admin-apps-body');
  bodyNode.innerHTML = '<tr><td colspan="5">正在读取应用…</td></tr>';
  setAdminNotice();
  try {
    const params = new URLSearchParams({ page: String(filter.page), perPage: '25' });
    if (filter.q) params.set('q', filter.q);
    if (filter.archived) params.set('archived', filter.archived);
    const result = await api(`/api/admin/apps?${params}`);
    bodyNode.innerHTML = result.items.length ? result.items.map((item) => `<tr><td><strong>${esc(item.name)}</strong><small class="admin-cell-secondary">${item.restricted ? '受限访问' : '工作区默认访问'}</small></td><td>${esc(item.tenant?.name || '工作区已删除')}</td><td>${numberLabel(item.table_count)}</td><td>${item.archived ? '<span class="badge badge-ghost">已归档</span>' : '<span class="badge badge-success badge-soft">使用中</span>'}</td><td>${dateTimeLabel(item.created_at)}</td></tr>`).join('') : '<tr><td colspan="5">没有符合条件的应用。</td></tr>';
    updateAdminPager('apps', result);
  } catch (error) {
    bodyNode.innerHTML = '<tr><td colspan="5">应用目录暂时不可用。</td></tr>';
    setAdminNotice(error.message || '应用目录读取失败');
  }
}

function renderAdminUsageRows(bodyNode, items, columnCount = 7) {
  bodyNode.innerHTML = items.length ? items.map((row) => `<tr><td>${esc(row.tenant?.name || '工作区已删除')}</td><td>${numberLabel(row.requests)}</td><td>${numberLabel(row.successes)}</td><td>${numberLabel(row.errors)}</td>${columnCount === 7 ? `<td>${numberLabel(row.pending)}</td>` : ''}<td>${numberLabel(row.input_tokens)}</td><td>${numberLabel(row.output_tokens)}</td></tr>`).join('') : `<tr><td colspan="${columnCount}">所选时间范围内没有用量。</td></tr>`;
}

async function loadAdminUsage() {
  const filter = state.admin.usage;
  const bodyNode = $('#admin-usage-body');
  if (!filter.from || !filter.to) {
    const today = new Date().toISOString().slice(0, 10);
    const start = new Date(Date.now() - 6 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10);
    filter.from ||= start;
    filter.to ||= today;
    $('#admin-usage-filter [name="from"]').value = filter.from;
    $('#admin-usage-filter [name="to"]').value = filter.to;
  }
  bodyNode.innerHTML = '<tr><td colspan="7">正在汇总用量…</td></tr>';
  setAdminNotice();
  try {
    const from = new Date(`${filter.from}T00:00:00.000Z`).toISOString();
    const to = new Date(`${filter.to}T23:59:59.999Z`).toISOString();
    const params = new URLSearchParams({ from, to, page: String(filter.page), perPage: '25' });
    const result = await api(`/api/admin/usage?${params}`);
    $('#admin-usage-totals').innerHTML = `<span>请求 <strong>${numberLabel(result.totals.requests)}</strong></span><span>成功 <strong>${numberLabel(result.totals.successes)}</strong></span><span>失败 <strong>${numberLabel(result.totals.errors)}</strong></span><span>输入 <strong>${numberLabel(result.totals.input_tokens)}</strong> tokens</span><span>输出 <strong>${numberLabel(result.totals.output_tokens)}</strong> tokens</span>`;
    renderAdminUsageRows(bodyNode, result.items);
    updateAdminPager('usage', result);
  } catch (error) {
    bodyNode.innerHTML = '<tr><td colspan="7">平台用量暂时不可用。</td></tr>';
    setAdminNotice(error.message || '平台用量读取失败');
  }
}

async function loadAdminAudit() {
  const filter = state.admin.audit;
  const bodyNode = $('#admin-audit-body');
  bodyNode.innerHTML = '<tr><td colspan="6">正在读取审计记录…</td></tr>';
  setAdminNotice();
  try {
    const params = new URLSearchParams({ page: String(filter.page), perPage: '25' });
    if (filter.targetType) params.set('targetType', filter.targetType);
    if (filter.targetId) params.set('targetId', filter.targetId);
    const result = await api(`/api/admin/audit?${params}`);
    bodyNode.innerHTML = result.items.length ? result.items.map((row) => `<tr><td>${dateTimeLabel(row.created_at)}</td><td>${esc(row.actor_email)}</td><td>${esc(row.action)}</td><td><span class="badge badge-outline">${esc(row.target_type)}</span><small class="admin-cell-secondary">${esc(row.target_id || '—')}</small></td><td>${esc(row.reason || '—')}</td><td><span class="badge ${row.status >= 400 ? 'badge-error badge-soft' : row.status >= 200 ? 'badge-success badge-soft' : 'badge-warning badge-soft'}">${numberLabel(row.status)}</span></td></tr>`).join('') : '<tr><td colspan="6">还没有平台审计记录。</td></tr>';
    updateAdminPager('audit', result);
  } catch (error) {
    bodyNode.innerHTML = '<tr><td colspan="6">平台审计暂时不可用。</td></tr>';
    setAdminNotice(error.message || '平台审计读取失败');
  }
}

async function loadAdminAI() {
  setAdminNotice();
  $('#admin-ai-status').textContent = '正在读取 AI 配置…';
  try {
    const [config, runtime] = await Promise.all([api('/api/admin/ai'), api('/api/admin/runtime')]);
    const keyState = config.configured ? `已配置 ${config.key_hint}` : '尚未配置密钥';
    const source = config.source === 'admin' ? '管理后台密钥' : config.source === 'environment' ? '服务器环境变量' : '无';
    const encryption = config.encryption_ready ? '加密设置可用' : '需先配置至少 32 个字符的 MIAO_SETTINGS_ENCRYPTION_KEY';
    const provider = config.provider === 'capi' ? 'CAPI' : 'Vercel AI Gateway';
    $('#admin-ai-status').textContent = `${keyState} · ${provider} · ${config.model} · 来源：${source} · ${encryption}`;
    $('#admin-ai-status').className = `alert ${config.configured ? 'alert-success' : 'alert-warning'} admin-ai-status`;
    state.aiConfigured = Boolean(runtime.ai.configured);
    renderApps();
  } catch (error) {
    $('#admin-ai-status').textContent = error.message || 'AI 配置读取失败';
    $('#admin-ai-status').className = 'alert alert-error admin-ai-status';
  }
}

function openAdminUserStatus(button) {
  const userId = button.dataset.userId;
  const email = button.dataset.userEmail;
  const currentlyDisabled = button.dataset.userDisabled === 'true';
  const disabled = !currentlyDisabled;
  $('#admin-user-status-form').reset();
  $('#admin-user-status-form [name="id"]').value = userId;
  $('#admin-user-status-form [name="disabled"]').value = String(disabled);
  $('#admin-user-status-title').textContent = disabled ? '停用账号' : '恢复账号';
  $('#admin-user-status-copy').textContent = disabled
    ? `停用 ${email} 后，该账号将无法进入任何工作区。操作原因会写入平台审计。`
    : `恢复 ${email} 后，该账号可以重新登录并进入有权限的工作区。操作原因会写入平台审计。`;
  $('#admin-user-status-submit').textContent = disabled ? '确认停用' : '确认恢复';
  $('#admin-user-status-submit').className = `btn ${disabled ? 'btn-error' : 'btn-primary'}`;
  $('#admin-user-status-dialog').showModal();
}

async function submitAdminUserStatus(event) {
  event.preventDefault();
  const values = Object.fromEntries(new FormData(event.currentTarget));
  try {
    await api(`/api/admin/users/${encodeURIComponent(values.id)}/status`, {
      method: 'PATCH', body: JSON.stringify({ disabled: values.disabled === 'true', reason: values.reason }),
    });
    $('#admin-user-status-dialog').close();
    toast(values.disabled === 'true' ? '账号已停用' : '账号已恢复');
    await loadAdminUsers();
    await loadAdminOverview();
  } catch (error) { toast(error.message, true); }
}

async function changeAdminPage(name, delta) {
  const pageState = state.admin[name];
  pageState.page = Math.max(1, pageState.page + delta);
  await adminLoaders[name]();
}

function renderTables() {
  $('#table-list').innerHTML = state.tables.map((table) => `<button class="table-nav-item ${state.table?.id === table.id ? 'active' : ''}" data-table="${esc(table.slug)}"><span>▤</span>${esc(table.name)}</button>`).join('') || '<p class="no-tables">还没有数据表。可以先和 fx 梳理需要管理的信息。</p>';
}

function renderNoTables() {
  $('#records-root').innerHTML = '<div class="records-empty"><strong>还没有数据表</strong><span>先和 fx 梳理应用需要保存的信息，再建立对应的数据结构。</span><button class="btn btn-outline btn-sm" data-action="open-assistant">和 fx 讨论工作流程</button></div>';
}

function renderRecordCell(row, field) {
  const value = row.data[field.name];
  if (field.type === 'file' && value) {
    return `<td title="${esc(value)}"><button class="btn btn-link btn-xs" type="button" data-download-file="${esc(field.name)}" data-record-id="${esc(row.id)}" data-file-name="${esc(value)}">${esc(value)}</button></td>`;
  }
  const display = field.type === 'bool' && value !== undefined ? (value ? '是' : '否') : value ?? '—';
  return `<td title="${esc(display)}">${esc(Array.isArray(display) ? display.join(', ') : display)}</td>`;
}

async function renderRecords() {
  renderTables();
  if (!state.table) return renderNoTables();
  const focusedId = document.activeElement?.id;
  const selectionStart = document.activeElement?.selectionStart;
  const selectionEnd = document.activeElement?.selectionEnd;
  try {
    const params = new URLSearchParams(Object.fromEntries(Object.entries(state.recordQuery).filter(([, value]) => value !== '')));
    state.recordResult = await api(`/api/apps/${state.app.id}/collections/${encodeURIComponent(state.table.slug)}/records?${params}`);
    state.records = state.recordResult.items || [];
  } catch (error) {
    toast(error.message, true);
    return;
  }
  const fields = state.table.fields || [];
  const columns = fields.slice(0, 5);
  const sortOptions = ['-created', 'created', '-updated', 'updated', ...fields.map((field) => field.name), ...fields.map((field) => `-${field.name}`)];
  const canEdit = state.app.permission !== 'viewer';
  $('#records-root').innerHTML = `<div class="records-heading"><div><h2>${esc(state.table.name)}</h2><span>${state.recordResult.totalItems} 条记录 · ${fields.length} 个字段</span></div><div>${canEdit ? `<button class="btn btn-ghost btn-sm" data-action="edit-table">改名</button><button class="btn btn-ghost btn-sm" data-action="edit-table-schema">字段设置</button><button class="btn btn-ghost btn-sm" data-action="delete-table">删除表</button><button class="btn btn-ghost btn-sm" data-action="add-record">＋ 添加记录</button>` : '<span class="badge badge-ghost">只读</span>'}</div></div><div class="record-query"><input class="input input-bordered input-sm" id="record-search" type="search" placeholder="搜索文本字段" value="${esc(state.recordQuery.search)}"><select class="select select-bordered select-sm" id="record-sort">${sortOptions.map((value) => `<option value="${esc(value)}" ${value === state.recordQuery.sort ? 'selected' : ''}>排序：${esc(value.replace(/^-/, ''))}${value.startsWith('-') ? ' ↓' : ' ↑'}</option>`).join('')}</select><select class="select select-bordered select-sm" id="record-filter-field"><option value="">筛选字段</option>${fields.map((field) => `<option value="${esc(field.name)}" ${field.name === state.recordQuery.filterField ? 'selected' : ''}>${esc(field.label || field.name)}</option>`).join('')}</select><input class="input input-bordered input-sm" id="record-filter-value" placeholder="筛选值" value="${esc(state.recordQuery.filterValue)}"><button class="btn btn-ghost btn-sm" data-action="clear-filter">清除</button></div>${state.records.length ? `<div class="overflow-x-auto"><table class="table table-sm"><thead><tr>${columns.map((field) => `<th>${esc(field.label || field.name)}</th>`).join('')}<th></th></tr></thead><tbody>${state.records.map((row) => `<tr>${columns.map((field) => renderRecordCell(row, field)).join('')}<td class="record-actions">${canEdit ? `<button class="btn btn-ghost btn-xs" title="编辑记录" aria-label="编辑记录" data-edit-record="${esc(row.id)}">编辑</button><button class="btn btn-ghost btn-xs" title="删除记录" aria-label="删除记录" data-delete-record="${esc(row.id)}">×</button>` : ''}</td></tr>`).join('')}</tbody></table></div>` : '<div class="records-empty"><strong>没有匹配的记录</strong><span>调整搜索条件或添加一条记录。</span></div>'}<div class="record-pagination"><span>第 ${state.recordResult.page} / ${Math.max(1, state.recordResult.totalPages)} 页</span><button class="btn btn-ghost btn-sm" data-page="${Math.max(1, state.recordResult.page - 1)}" ${state.recordResult.page <= 1 ? 'disabled' : ''}>上一页</button><button class="btn btn-ghost btn-sm" data-page="${Math.min(state.recordResult.totalPages || 1, state.recordResult.page + 1)}" ${state.recordResult.page >= state.recordResult.totalPages ? 'disabled' : ''}>下一页</button><select class="select select-bordered select-sm" id="record-page-size"><option ${state.recordQuery.perPage === 25 ? 'selected' : ''}>25</option><option ${state.recordQuery.perPage === 50 ? 'selected' : ''}>50</option><option ${state.recordQuery.perPage === 100 ? 'selected' : ''}>100</option></select></div>`;
  if (['record-search', 'record-filter-value'].includes(focusedId)) {
    const control = $(`#${focusedId}`);
    control?.focus();
    if (selectionStart !== null && selectionEnd !== null) control?.setSelectionRange(selectionStart, selectionEnd);
  }
}

async function saveAppDetails(event) {
  event.preventDefault();
  if (!state.editingApp) return;
  const form = new FormData(event.currentTarget);
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

async function editTable() {
  if (!state.table) return;
  const name = window.prompt('修改数据表名称', state.table.name);
  if (name === null) return;
  if (!name.trim()) return toast('数据表名称不能为空', true);
  try {
    state.table = await api(`/api/apps/${state.app.id}/collections/${encodeURIComponent(state.table.slug)}`, { method: 'PATCH', body: JSON.stringify({ name }) });
    state.tables = state.tables.map((table) => table.slug === state.table.slug ? state.table : table);
    await renderRecords();
    toast('数据表名称已更新');
  } catch (error) { toast(error.message, true); }
}

async function deleteTable() {
  if (!state.table || !window.confirm(`永久删除「${state.table.name}」及其中全部记录？此操作无法撤销。`)) return;
  try {
    await api(`/api/apps/${state.app.id}/collections/${encodeURIComponent(state.table.slug)}`, { method: 'DELETE', body: JSON.stringify({ confirm: true }) });
    state.tables = state.tables.filter((table) => table.slug !== state.table.slug);
    state.table = state.tables[0] || null;
    await renderWorkspace();
    toast('数据表及其记录已删除');
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
  const email = new FormData(event.currentTarget).get('email');
  try {
    const result = await api('/api/auth/password-reset/request', { method: 'POST', body: JSON.stringify({ email }) });
    toast(result.message || '如果邮箱已登记，重置邮件将发送到邮箱');
  } catch (error) { toast(error.message, true); }
}

async function submitPasswordReset(event) {
  event.preventDefault();
  const password = new FormData(event.currentTarget).get('password');
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
  const data = Object.fromEntries(new FormData(event.currentTarget));
  try {
    await api('/api/me', { method: 'DELETE', body: JSON.stringify(data) });
    $('#account-delete-dialog').close();
    logout();
    toast('账号和相关数据已删除');
  } catch (error) { toast(error.message, true); }
}

async function submitAccountDeactivation(event) {
  event.preventDefault();
  const password = new FormData(event.currentTarget).get('password');
  try {
    await api('/api/me/deactivate', { method: 'POST', body: JSON.stringify({ password, confirm: true }) });
    $('#account-deactivate-dialog').close();
    logout();
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
  const daily_limit = Number(new FormData(event.currentTarget).get('daily_limit'));
  try {
    await api('/api/workspace/ai-budget', { method: 'PATCH', body: JSON.stringify({ daily_limit }) });
    await openAIUsage();
    toast('AI 每日预算已保存');
  } catch (error) { toast(error.message, true); }
}

async function openAIAdmin() {
  await openPlatformAdmin('ai');
}

async function saveAIKey(event) {
  event.preventDefault();
  const api_key = new FormData(event.currentTarget).get('api_key');
  try {
    await api('/api/admin/ai', { method: 'PUT', body: JSON.stringify({ api_key }) });
    event.currentTarget.reset();
    state.aiConfigured = true;
    await loadAdminAI();
    toast('AI 服务密钥已轮换');
  } catch (error) { toast(error.message, true); }
}

async function openAppAccess() {
  if (!state.app) return;
  try {
    const access = await api(`/api/apps/${state.app.id}/access`);
    $('#app-access-form [name="restricted"]').checked = access.restricted;
    $('#app-access-members').innerHTML = access.members.map((member) => `<label class="member-row"><span>${esc(member.name)} · ${esc(member.email)}<small>${member.workspace_role === 'admin' ? '管理员' : '成员'}</small></span><select class="select select-bordered select-sm" data-access-user="${esc(member.id)}"><option value="">无权限</option><option value="viewer" ${member.app_role === 'viewer' ? 'selected' : ''}>只读</option><option value="editor" ${member.app_role === 'editor' ? 'selected' : ''}>可编辑</option></select></label>`).join('') || '<p class="empty-members">当前工作区没有其他成员。</p>';
    $('#app-access-dialog').showModal();
  } catch (error) { toast(error.message, true); }
}

async function saveAppAccess(event) {
  event.preventDefault();
  if (!state.app) return;
  const permissions = $$('[data-access-user]').map((select) => ({ user_id: select.dataset.accessUser, role: select.value })).filter((permission) => permission.role);
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

async function openAuditLog(page = 1) {
  try {
    const result = await api(`/api/workspace/audit?page=${Math.max(1, page)}`);
    state.workspaceAuditPage = result.page;
    $('#audit-log-list').innerHTML = result.items.map((item) => `<div class="audit-log-row"><span><strong>${esc(item.action)} ${esc(item.route)}</strong><small>${esc(item.actor_email)} · ${new Date(item.created_at).toLocaleString()} · ${item.status}</small></span><code>${esc(item.target_id || '—')}</code></div>`).join('') || '<p class="empty-members">还没有操作记录。</p>';
    $('#audit-log-page-label').textContent = `第 ${result.page} / ${Math.max(1, result.totalPages)} 页 · 共 ${result.totalItems} 条`;
    $('[data-audit-delta="-1"]').disabled = result.page <= 1;
    $('[data-audit-delta="1"]').disabled = result.page >= result.totalPages;
    if (!$('#audit-dialog').open) $('#audit-dialog').showModal();
  } catch (error) { toast(error.message, true); }
}

function fieldKey(label, index) {
  const slug = label.toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 20);
  return slug || `field_${index + 1}`;
}

async function createTable(event) {
  event.preventDefault();
  const editingSchema = Boolean(state.editingTableSchema);
  const form = new FormData(event.currentTarget);
  const fields = $$('.table-field-row').slice(0, 24).map((row, index) => {
    const label = row.querySelector('[name="field-label"]').value.trim();
    const field = { name: row.querySelector('[name="field-name"]').value || fieldKey(label, index), label, type: row.querySelector('[name="field-type"]').value, required: row.querySelector('[name="field-required"]').checked };
    if (field.type === 'select') field.options = row.querySelector('[name="field-options"]').value.split(/[,，\n]/).map((value) => value.trim()).filter(Boolean);
    if (field.type === 'relation') field.target = row.querySelector('[name="field-target"]').value;
    return field;
  }).filter((field) => field.label);
  if (!fields.length) return toast('请至少填写一个字段', true);
  if ($$('.table-field-row').length !== fields.length) return toast('请填写所有字段名称', true);
  try {
    let table;
    if (editingSchema) {
      const currentNames = new Set(fields.map((field) => field.name));
      const remove_fields = state.table.fields.filter((field) => !currentNames.has(field.name)).map((field) => field.name);
      if (remove_fields.length && !window.confirm(`永久删除字段 ${remove_fields.join('、')} 及其所有数据？此操作无法撤销。`)) return;
      table = await api(`/api/apps/${state.app.id}/collections/${encodeURIComponent(state.table.slug)}`, { method: 'PATCH', body: JSON.stringify({ name: form.get('name'), fields, remove_fields, confirm_data_loss: remove_fields.length > 0 }) });
      state.tables = state.tables.map((item) => item.slug === table.slug ? table : item);
    } else {
      table = await api(`/api/apps/${state.app.id}/collections`, { method: 'POST', body: JSON.stringify({ name: form.get('name'), fields }) });
      state.tables.push(table);
    }
    state.table = table;
    state.editingTableSchema = false;
    event.currentTarget.reset();
    $('#table-fields-editor').replaceChildren();
    $('#table-dialog-title').textContent = '新建数据表';
    $('#table-dialog-submit').textContent = '创建数据表';
    $('#table-dialog').close();
    await renderRecords();
    toast(editingSchema ? '数据表字段已更新' : '数据表已创建');
  } catch (error) {
    toast(error.message, true);
  }
}

function tableFieldRow(field = null) {
  const relationOptions = state.tables.map((table) => `<option value="${esc(table.slug)}">${esc(table.name)}</option>`).join('');
  const types = [['text', '文本'], ['number', '数字'], ['bool', '是/否'], ['date', '日期'], ['email', '邮箱'], ['url', '网址'], ['select', '选项'], ['relation', '关联记录'], ['file', '附件']];
  const options = types.map(([value, label]) => `<option value="${value}" ${field?.type === value ? 'selected' : ''}>${label}</option>`).join('');
  return `<div class="table-field-row"><input type="hidden" name="field-name" value="${esc(field?.name || '')}" /><label>字段名称<input class="input input-sm" name="field-label" required placeholder="例如：状态" value="${esc(field?.label || '')}" /></label><label>类型<select class="select select-bordered select-sm" name="field-type" ${field ? 'disabled' : ''}>${options}</select></label><label class="field-required"><input class="checkbox checkbox-sm" type="checkbox" name="field-required" ${field?.required ? 'checked' : ''} />必填</label><label class="field-options ${field?.type === 'select' ? '' : 'hidden'}">选项（逗号分隔）<input class="input input-sm" name="field-options" placeholder="待办,进行中,完成" value="${esc(field?.options?.join(', ') || '')}" /></label><label class="field-target ${field?.type === 'relation' ? '' : 'hidden'}">关联数据表<select class="select select-bordered select-sm" name="field-target"><option value="">选择数据表</option>${relationOptions.replace(`value="${esc(field?.target || '')}"`, `value="${esc(field?.target || '')}" selected`)}</select></label><button class="btn btn-ghost btn-xs" type="button" data-action="remove-table-field" aria-label="移除字段">移除</button></div>`;
}

function openTableSchemaEditor() {
  if (!state.table) return;
  state.editingTableSchema = true;
  $('#table-dialog-title').textContent = '修改数据表和字段';
  $('#table-dialog-submit').textContent = '保存修改';
  $('#create-table-form [name="name"]').value = state.table.name;
  $('#table-fields-editor').innerHTML = state.table.fields.map((field) => tableFieldRow(field)).join('');
  $('#table-dialog').showModal();
}

async function openRecordEditor(record = null) {
  if (!state.table) return;
  state.editingRecordId = record?.id || null;
  $('#record-dialog-title').textContent = record ? '编辑记录' : '添加记录';
  $('#record-save').textContent = record ? '保存修改' : '添加记录';
  const fields = await Promise.all(state.table.fields.map(async (field) => {
    const label = esc(field.label || field.name);
    const value = record?.data?.[field.name];
    const required = field.required ? ' required' : '';
    const common = `data-record-field="${esc(field.name)}"`;
    if (field.type === 'bool') {
      const selected = (candidate) => value === candidate ? ' selected' : '';
      return `<label>${label}<select class="select select-bordered w-full" ${common}${required}><option value=""${selected(undefined)}>请选择</option><option value="true"${selected(true)}>是</option><option value="false"${selected(false)}>否</option></select></label>`;
    }
    if (field.type === 'select') {
      const options = (field.options || []).map((option) => `<option value="${esc(option)}" ${value === option ? 'selected' : ''}>${esc(option)}</option>`).join('');
      return `<label>${label}<select class="select select-bordered w-full" ${common}${required}><option value="">请选择</option>${options}</select></label>`;
    }
    if (field.type === 'relation') {
      const target = state.tables.find((table) => table.slug === field.target);
      let records = [];
      if (target) {
        const result = await api(`/api/apps/${state.app.id}/collections/${encodeURIComponent(target.slug)}/records?perPage=100`);
        records = result.items || [];
      }
      const labelField = target?.fields?.[0]?.name;
      return `<label>${label}<select class="select select-bordered w-full" ${common}${required}><option value="">选择关联记录</option>${records.map((row) => `<option value="${esc(row.id)}" ${value === row.id ? 'selected' : ''}>${esc(row.data[labelField] || row.id)}</option>`).join('')}</select></label>`;
    }
    if (field.type === 'file') {
      const currentFile = record?.data?.[field.name];
      const fileLink = currentFile ? `<small><button class="btn btn-link btn-xs" type="button" data-download-file="${esc(field.name)}" data-record-id="${esc(record.id)}" data-file-name="${esc(currentFile)}">下载：${esc(currentFile)}</button> <label class="inline-checkbox"><input type="checkbox" data-clear-file="${esc(field.name)}" />移除</label></small>` : '';
      return `<label>${label}<input class="file-input file-input-bordered w-full" type="file" ${common} accept="image/png,image/jpeg,image/gif,image/webp,application/pdf,text/plain" ${!record && field.required ? 'required' : ''} />${fileLink}<small>支持图片、PDF、文本，最大 5 MB</small></label>`;
    }
    const type = field.type === 'number' ? 'number' : ['date', 'email', 'url'].includes(field.type) ? field.type : 'text';
    const shownValue = field.type === 'date' && value ? String(value).slice(0, 10) : value ?? '';
    const step = field.type === 'number' ? ' step="any"' : '';
    return `<label>${label}<input class="input input-bordered w-full" type="${type}" ${common}${step}${required} value="${esc(shownValue)}" /></label>`;
  }));
  $('#record-form-fields').innerHTML = fields.join('');
  $('#record-dialog').showModal();
  $('#record-form-fields input, #record-form-fields select')?.focus();
}

async function submitRecord(event) {
  event.preventDefault();
  if (!state.table) return;
  const editing = Boolean(state.editingRecordId);
  const data = {};
  const files = {};
  for (const field of state.table.fields) {
    const control = $(`[data-record-field="${CSS.escape(field.name)}"]`);
    if (field.type === 'file') {
      const remove = $(`[data-clear-file="${CSS.escape(field.name)}"]`)?.checked;
      if (control.files?.[0]) {
        const file = control.files[0];
        const encoded = await new Promise((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = () => resolve(reader.result);
          reader.onerror = () => reject(reader.error);
          reader.readAsDataURL(file);
        });
        files[field.name] = { name: file.name, type: file.type, base64: encoded };
      } else if (remove) data[field.name] = '';
      continue;
    }
    const value = control.value;
    if (field.type === 'number') {
      if (value === '' && !field.required) continue;
      data[field.name] = Number(value);
    } else if (field.type === 'bool') {
      if (value === '' && !field.required) continue;
      data[field.name] = value === 'true';
    } else {
      data[field.name] = value;
    }
  }
  try {
    const collection = `/api/apps/${state.app.id}/collections/${encodeURIComponent(state.table.slug)}/records`;
    if (state.editingRecordId) {
      await api(`${collection}/${encodeURIComponent(state.editingRecordId)}`, { method: 'PATCH', body: JSON.stringify({ data, files }) });
    } else {
      await api(collection, { method: 'POST', body: JSON.stringify({ data, files }) });
      state.recordQuery.page = 1;
    }
    $('#record-dialog').close();
    state.editingRecordId = null;
    await renderRecords();
    toast(editing ? '记录已更新' : '记录已添加');
  } catch (error) {
    toast(error.message, true);
  }
}

async function deleteRecord(recordId) {
  if (!state.table || !window.confirm('删除这条记录？')) return;
  try {
    await api(`/api/apps/${state.app.id}/collections/${encodeURIComponent(state.table.slug)}/records/${recordId}`, { method: 'DELETE' });
    await renderRecords();
    toast('记录已删除');
  } catch (error) {
    toast(error.message, true);
  }
}

function appendChat(message, role) {
  const wrap = document.createElement('div');
  wrap.className = `chat ${role === 'user' ? 'chat-end' : 'chat-start'}`;
  const bubble = document.createElement('div');
  bubble.className = `chat-bubble ${role === 'user' ? 'chat-bubble-primary' : 'chat-bubble-neutral'}`;
  bubble.textContent = message;
  wrap.append(bubble);
  $('#chat-messages').append(wrap);
  $('#chat-messages').scrollTop = $('#chat-messages').scrollHeight;
  return bubble;
}

const toolResult = (value) => JSON.stringify(value);
function agentTools() {
  const request = (path, options) => {
    if (!state.app && !path.startsWith('/apps')) throw new Error('请先通过 create_app 创建或通过 activate_app 选择一个工具。');
    return api(state.app ? `/api/apps/${state.app.id}${path}` : `/api${path}`, options);
  };
  const tableSchema = { type: 'object', required: ['name', 'fields'], properties: { name: { type: 'string' }, fields: { type: 'array', items: { type: 'object', required: ['name', 'label'], properties: { name: { type: 'string' }, label: { type: 'string' }, type: { type: 'string', enum: ['text', 'number', 'bool', 'date', 'email', 'url', 'select', 'relation'] }, required: { type: 'boolean' }, options: { type: 'array', items: { type: 'string' } }, target: { type: 'string' } } } } } };
  return [
    { name: 'list_apps', description: '查看当前工作区可用的工具，帮助用户继续已有工作。', inputSchema: { type: 'object', properties: {} }, async execute() { return toolResult(state.apps); } },
    { name: 'create_app', description: '根据用户确认的工作目标创建新工具。创建后它自动成为当前工具。', inputSchema: { type: 'object', required: ['name', 'description'], properties: { name: { type: 'string' }, description: { type: 'string' } } }, async execute(input) {
      const app = await api('/api/apps', { method: 'POST', body: JSON.stringify(input) });
      state.apps = [app, ...state.apps.filter((item) => item.id !== app.id)]; state.app = app; state.appPanel = 'runtime'; state.table = null;
      await renderWorkspace(); return toolResult({ created: app, next: '工具已创建。可以继续梳理工作流程，并在用户认可后创建所需数据结构。' });
    } },
    { name: 'activate_app', description: '切换当前对话正在处理的工具。先用 list_apps 找到目标工具。', inputSchema: { type: 'object', required: ['app_id'], properties: { app_id: { type: 'string' } } }, async execute({ app_id }) {
      const found = state.apps.find((item) => item.id === app_id); if (!found) throw new Error('当前工作区找不到这个工具');
      state.app = found; state.table = null; await renderWorkspace(); return toolResult({ active_app: found });
    } },
    { name: 'list_tables', description: '了解当前工具的数据结构，为后续工作做准备。', inputSchema: { type: 'object', properties: {} }, async execute() { return toolResult(await request('/collections')); } },
    { name: 'create_table', description: '按已讨论的工作流程建立数据结构。字段名使用英文 snake_case，label 使用清晰的中文名称；关系字段 target 必须是当前工具中已存在的数据表 slug。', inputSchema: tableSchema, async execute(input) { return toolResult(await request('/collections', { method: 'POST', body: JSON.stringify(input) })); } },
    { name: 'list_records', description: '按用户问题读取当前工具的数据记录。可用搜索和分页缩小结果。', inputSchema: { type: 'object', required: ['table'], properties: { table: { type: 'string', description: '数据表 slug' }, search: { type: 'string' }, page: { type: 'number' }, perPage: { type: 'number' } } }, async execute(input) {
      const params = new URLSearchParams(); for (const key of ['search', 'page', 'perPage']) if (input[key] !== undefined) params.set(key, String(input[key]));
      return toolResult(await request(`/collections/${encodeURIComponent(input.table)}/records?${params}`));
    } },
    { name: 'add_record', description: '根据用户提供的信息新增业务记录；缺失的事实必须先询问，不可猜测。', inputSchema: { type: 'object', required: ['table', 'data'], properties: { table: { type: 'string' }, data: { type: 'object', additionalProperties: true } } }, async execute(input) { return toolResult(await request(`/collections/${encodeURIComponent(input.table)}/records`, { method: 'POST', body: JSON.stringify({ data: input.data }) })); } },
    { name: 'update_record', description: '根据用户明确的指示修改业务记录。先定位并复述目标记录和改动，再调用工具。', inputSchema: { type: 'object', required: ['table', 'record_id', 'data'], properties: { table: { type: 'string' }, record_id: { type: 'string' }, data: { type: 'object', additionalProperties: true } } }, async execute(input) { return toolResult(await request(`/collections/${encodeURIComponent(input.table)}/records/${encodeURIComponent(input.record_id)}`, { method: 'PATCH', body: JSON.stringify({ data: input.data }) })); } },
    { name: 'delete_record', description: '永久删除业务记录。只可在用户明确确认删除具体记录后调用。', inputSchema: { type: 'object', required: ['table', 'record_id'], properties: { table: { type: 'string' }, record_id: { type: 'string' } } }, async execute(input) { return toolResult(await request(`/collections/${encodeURIComponent(input.table)}/records/${encodeURIComponent(input.record_id)}`, { method: 'DELETE' })); } }
  ];
}

async function getAgent() {
  if (!state.aiConfigured) throw new Error('企业尚未配置 AI Gateway，请联系管理员。');
  if (!supportsJspi()) throw new Error('当前浏览器不支持 agent 所需的 WebAssembly JSPI。请使用较新的 Chrome、Edge 或 Safari 后重试。');
  if (!state.fxAgent) {
    state.fxAgent = await createFxAgent({
      apiKey: 'miao-server-managed',
      wasm: '/vendor/fx/fx-core.wasm',
      instructions: `你是 MIAO 的工作协作 agent，帮助用户把真实工作从目标推进到完成。不要把自己描述成低代码/建表助手，也不要默认每个问题都要做应用或数据表。先理解目标、现状、约束和成功标准；复杂任务先提出清晰的步骤或方案，信息不足时只问最关键的问题。你可以梳理和改进流程、创建并切换工作工具、检查结构、查询和整理数据、录入或更新记录。只在确有需要且用户认可方案后才创建工具或结构。更新前确认目标记录与具体变更；删除属于破坏性操作，必须先说清对象与后果并取得明确确认。绝不编造业务事实、执行结果或外部能力。先用 list_apps 理解可继续的工作，有明确对象后再用 activate_app。当前工具会随这些工具调用动态切换。仅访问当前用户有权限的工作区与工具。每次工具执行后说明实际结果与未完成项。`,
      tools: agentTools(),
      fetch(url, init) {
        const headers = new Headers(init.headers);
        headers.delete('authorization');
        headers.set('Authorization', `Bearer ${state.token}`);
        headers.set('X-Miao-Tenant-Id', state.tenant.id);
        if (state.app?.id) headers.set('X-Miao-App-Id', state.app.id);
        else headers.delete('X-Miao-App-Id');
        headers.set('X-Fx-Path', new URL(url).pathname);
        return fetch('/api/fx/gateway', { ...init, headers }).then((response) => {
          const nextToken = response.headers.get('X-PocketBase-Token');
          if (nextToken) {
            state.token = nextToken;
            localStorage.setItem(TOKEN_KEY, nextToken);
          }
          return response;
        });
      }
    });
  }
  $('#agent-status').textContent = '在线';
  $('#agent-status').className = 'badge badge-success';
  return state.fxAgent;
}

async function openMembers() {
  const [result, invites] = await Promise.all([
    api('/api/workspace/members'),
    ['owner', 'admin'].includes(state.tenant?.role) ? api('/api/workspace/invites') : Promise.resolve([])
  ]);
  $('#member-list').innerHTML = result.members.map((member) => `<div class="member-row"><span>${esc(member.name)} · ${esc(member.email)}<small>${member.role === 'owner' ? '所有者' : member.role === 'admin' ? '管理员' : '成员'}</small></span>${result.can_edit_roles && member.role !== 'owner' ? `<span class="member-actions"><select class="select select-bordered select-xs" data-member-role="${esc(member.membership_id)}"><option value="member" ${member.role === 'member' ? 'selected' : ''}>成员</option><option value="admin" ${member.role === 'admin' ? 'selected' : ''}>管理员</option></select><button class="btn btn-ghost btn-xs" data-remove-member="${esc(member.membership_id)}">从工作区移除</button></span>` : ''}</div>`).join('') || '<p class="empty-members">还没有成员。</p>';
  $('#invite-form').classList.toggle('hidden', !result.can_manage);
  $('#pending-invites').innerHTML = invites.map((invite) => `<div class="pending-invite-row"><span>${esc(invite.email)}<small>邀请待接受 · ${new Date(invite.expires_at).toLocaleString()}</small></span><button class="btn btn-ghost btn-xs" data-revoke-invite="${esc(invite.id)}">撤销</button></div>`).join('');
  if (!$('#member-dialog').open) $('#member-dialog').showModal();
}

async function createInvite(event) {
  event.preventDefault();
  const email = new FormData(event.currentTarget).get('email');
  try {
    const invite = await api('/api/workspace/invites', { method: 'POST', body: JSON.stringify({ email }) });
    $('#invite-link').value = new URL(invite.invite_url, location.origin).href;
    $('#invite-link-row').classList.remove('hidden');
    event.currentTarget.reset();
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
    await openMembers();
    toast('邀请已撤销');
  } catch (error) {
    toast(error.message, true);
  }
}

async function submitPrompt(event) {
  event.preventDefault();
  if (state.fxBusy) return;
  const composer = event.currentTarget;
  const prompt = new FormData(composer).get('prompt').toString().trim();
  if (!prompt) return;
  state.fxBusy = true;
  let response = null;
  for (const form of [$('#agent-form'), $('#home-agent-form')]) {
    form.querySelector('[name="prompt"]').disabled = true;
    form.querySelector('button[type="submit"]').disabled = true;
  }
  $('#workspace-switcher').disabled = true;
  try {
    if (composer.id === 'home-agent-form') {
      state.workspaceView = 'assistant';
      await renderWorkspace();
    }
    composer.reset();
    appendChat(prompt, 'user');
    response = appendChat('', 'assistant');
    $('#agent-status').textContent = '思考中';
    $('#agent-status').className = 'badge badge-info';
    const agent = await getAgent();
    const turn = agent.prompt(prompt);
    for await (const event of turn) {
      if (event.type === 'text_delta') response.textContent += event.delta;
      if (event.type === 'tool_start') {
        const note = document.createElement('small');
        note.className = 'tool-note';
        const labels = { list_apps: '正在查看已有工具', create_app: '正在创建工具', activate_app: '正在切换工作上下文', list_tables: '正在了解现有结构', create_table: '正在建立工作所需结构', list_records: '正在查找相关信息', add_record: '正在新增记录', update_record: '正在更新记录', delete_record: '正在删除记录' };
        note.textContent = labels[event.name] || '正在处理下一步';
        $('#chat-messages').append(note);
      }
      $('#chat-messages').scrollTop = $('#chat-messages').scrollHeight;
    }
    await turn.result;
    if (!response.textContent) response.textContent = '已完成。';
    $('#agent-status').textContent = '在线';
    $('#agent-status').className = 'badge badge-success';
    await renderWorkspace();
  } catch (error) {
    if (response) {
      response.textContent = error.message || 'Agent 暂时无法响应。';
      $('#agent-form [name="prompt"]').value = prompt;
    }
    else toast(error.message || 'Agent 暂时无法响应。', true);
    if ($('#agent-status')) {
      $('#agent-status').textContent = '连接失败';
      $('#agent-status').className = 'badge badge-error';
    }
  } finally {
    state.fxBusy = false;
    $('#workspace-switcher').disabled = false;
    for (const form of [$('#agent-form'), $('#home-agent-form')]) {
      form.querySelector('[name="prompt"]').disabled = false;
      form.querySelector('button[type="submit"]').disabled = false;
    }
  }
}

document.addEventListener('click', async (event) => {
  const suggestedPrompt = event.target.closest('[data-prompt]');
  if (suggestedPrompt) { $('#agent-form [name=prompt]').value = suggestedPrompt.dataset.prompt; $('#agent-form [name=prompt]').focus(); return; }
  const adminPageButton = event.target.closest('[data-admin-page]');
  if (adminPageButton) { await openPlatformAdmin(adminPageButton.dataset.adminPage); return; }
  const adminPagePrevious = event.target.closest('[data-admin-prev]');
  if (adminPagePrevious) { await changeAdminPage(adminPagePrevious.dataset.adminPrev, -1); return; }
  const adminPageNext = event.target.closest('[data-admin-next]');
  if (adminPageNext) { await changeAdminPage(adminPageNext.dataset.adminNext, 1); return; }
  const adminUserStatus = event.target.closest('[data-admin-user-status]');
  if (adminUserStatus) { openAdminUserStatus(adminUserStatus); return; }
  const adminRetry = event.target.closest('[data-admin-retry]');
  if (adminRetry) { await adminLoaders[adminRetry.dataset.adminRetry]?.(); return; }
  const action = event.target.closest('[data-action]')?.dataset.action;
  if (action === 'register') authMode('register');
  if (action === 'login') authMode('login');
  if (action === 'home') show('landing');
  if (action === 'logout') logout();
  if (action === 'open-platform-admin') await openPlatformAdmin('overview');
  if (action === 'return-workspace') await returnToWorkspace();
  if (action === 'show-dashboard') {
    state.app = null;
    state.appPanel = 'runtime';
    state.table = null;
    state.editingApp = false;
    state.workspaceView = 'home';
    await renderWorkspace();
  }
  if (action === 'open-assistant') {
    state.workspaceView = 'assistant';
    await renderWorkspace();
    $('#agent-form [name="prompt"]').focus();
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
    $('#app-actions-menu').open = false;
    state.appPanel = 'data';
    state.table = null;
    await renderWorkspace();
  }
  if (action === 'return-to-app' && state.app) {
    $('#app-actions-menu').open = false;
    state.appPanel = 'runtime';
    await renderWorkspace();
  }
  if (action === 'create-table') {
    state.editingTableSchema = false;
    $('#table-dialog-title').textContent = '新建数据表';
    $('#table-dialog-submit').textContent = '创建数据表';
    $('#create-table-form').reset();
    $('#table-fields-editor').replaceChildren();
    if (!$('#table-fields-editor').children.length) $('#table-fields-editor').innerHTML = tableFieldRow();
    $('#table-dialog').showModal();
  }
  if (action === 'edit-table-schema') openTableSchemaEditor();
  if (action === 'add-table-field') {
    if ($$('.table-field-row').length >= 24) return toast('每张数据表最多 24 个字段', true);
    $('#table-fields-editor').insertAdjacentHTML('beforeend', tableFieldRow());
  }
  if (action === 'remove-table-field') event.target.closest('.table-field-row')?.remove();
  if (action === 'archive-app') archiveApp();
  if (action === 'delete-app') deleteApp();
  if (action === 'edit-table') editTable();
  if (action === 'delete-table') deleteTable();
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
  if (action === 'admin-ai') openAIAdmin();
  if (action === 'close-admin-user-status') $('#admin-user-status-dialog').close();
  if (action === 'app-access') openAppAccess();
  if (action === 'close-app-access') $('#app-access-dialog').close();
  if (action === 'export-data') downloadWorkspaceExport();
  if (action === 'open-audit') openAuditLog();
  const auditPageButton = event.target.closest('[data-audit-delta]');
  if (auditPageButton) openAuditLog(state.workspaceAuditPage + Number(auditPageButton.dataset.auditDelta));
  if (action === 'close-audit') $('#audit-dialog').close();
  if (action === 'use-env-ai-key') {
    api('/api/admin/ai', { method: 'DELETE' }).then(async (result) => { state.aiConfigured = result.source === 'environment'; await loadAdminAI(); renderApps(); toast('已改用服务器环境配置'); }).catch((error) => toast(error.message, true));
  }
  if (action === 'clear-filter') {
    state.recordQuery.filterField = '';
    state.recordQuery.filterValue = '';
    state.recordQuery.page = 1;
    renderRecords();
  }
  const downloadFileButton = event.target.closest('[data-download-file]');
  if (downloadFileButton) {
    try {
      const path = `/api/apps/${encodeURIComponent(state.app.id)}/collections/${encodeURIComponent(state.table.slug)}/records/${encodeURIComponent(downloadFileButton.dataset.recordId)}/files/${encodeURIComponent(downloadFileButton.dataset.downloadFile)}`;
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
  if (action === 'close-table') { $('#table-dialog').close(); state.editingTableSchema = false; }
  if (action === 'add-record') openRecordEditor();
  if (action === 'close-record') { $('#record-dialog').close(); state.editingRecordId = null; }
  if (action === 'manage-members') openMembers().catch((error) => toast(error.message, true));
  if (action === 'close-members') $('#member-dialog').close();
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
    state.table = null;
    state.workspaceView = 'app';
    await renderWorkspace();
  }
  const restoreButton = event.target.closest('[data-restore-app]');
  if (restoreButton) await restoreApp(restoreButton.dataset.restoreApp);
  const tableButton = event.target.closest('[data-table]');
  if (tableButton) {
    state.table = state.tables.find((item) => item.slug === tableButton.dataset.table) || null;
    state.recordQuery = { ...state.recordQuery, page: 1, search: '', filterField: '', filterValue: '' };
    await renderRecords();
  }
  const pageButton = event.target.closest('[data-page]');
  if (pageButton) { state.recordQuery.page = Number(pageButton.dataset.page); renderRecords(); }
  const deleteButton = event.target.closest('[data-delete-record]');
  if (deleteButton) deleteRecord(deleteButton.dataset.deleteRecord);
  const editButton = event.target.closest('[data-edit-record]');
  if (editButton) openRecordEditor(state.records.find((record) => record.id === editButton.dataset.editRecord));
});

document.addEventListener('change', (event) => {
  if (event.target.matches('#workspace-switcher')) {
    switchWorkspace(event.target.value);
    return;
  }
  if (event.target.matches('[data-member-role]')) {
    api(`/api/workspace/members/${encodeURIComponent(event.target.dataset.memberRole)}`, { method: 'PATCH', body: JSON.stringify({ role: event.target.value }) })
      .then(() => toast('成员角色已更新')).catch((error) => toast(error.message, true));
  }
  if (event.target.matches('[name="field-type"]')) {
    const row = event.target.closest('.table-field-row');
    row.querySelector('.field-options').classList.toggle('hidden', event.target.value !== 'select');
    row.querySelector('.field-target').classList.toggle('hidden', event.target.value !== 'relation');
  }
  if (event.target.matches('#record-search')) state.recordQuery.search = event.target.value;
  if (event.target.matches('#record-sort')) state.recordQuery.sort = event.target.value;
  if (event.target.matches('#record-filter-field')) state.recordQuery.filterField = event.target.value;
  if (event.target.matches('#record-filter-value')) state.recordQuery.filterValue = event.target.value;
  if (event.target.matches('#record-page-size')) state.recordQuery.perPage = Number(event.target.value);
  if (event.target.matches('#record-search, #record-sort, #record-filter-field, #record-filter-value, #record-page-size')) {
    state.recordQuery.page = 1;
    renderRecords();
  }
});

async function switchWorkspace(workspaceId) {
  if (state.fxBusy) {
    renderApps();
    toast('fx 助手正在处理，请稍后再切换工作区。', true);
    return;
  }
  const selected = state.workspaces.find((workspace) => workspace.id === workspaceId);
  if (!selected || selected.id === state.tenant?.id) return;
  const previousTenant = state.tenant;
  state.tenant = selected;
  let me;
  let archivedApps;
  try {
    me = await api('/api/me');
    archivedApps = await api('/api/apps?archived=true').catch(() => []);
  } catch (error) {
    state.tenant = previousTenant;
    toast(error.message, true);
    renderApps();
    return;
  }
  clearAgent();
  resetAgentConversation();
  state.workspaces = me.workspaces || [];
  state.tenant = me.tenant;
  localStorage.setItem('miao_workspace', state.tenant.id);
  state.apps = me.apps || [];
  state.archivedApps = archivedApps;
  state.aiConfigured = me.ai_configured;
  state.isPlatformAdmin = me.is_platform_admin;
  state.app = null;
  state.appPanel = 'runtime';
  state.tables = [];
  state.table = null;
  state.records = [];
  state.recordResult = null;
  state.recordQuery = { page: 1, perPage: 25, search: '', sort: '-created', filterField: '', filterValue: '' };
  state.workspaceView = 'home';
  await renderWorkspace();
}

$('#admin-users-filter').addEventListener('submit', (event) => {
  event.preventDefault();
  const values = new FormData(event.currentTarget);
  state.admin.users = { page: 1, q: String(values.get('q') || '').trim(), status: String(values.get('status') || '') };
  loadAdminUsers();
});
$('#admin-workspaces-filter').addEventListener('submit', (event) => {
  event.preventDefault();
  state.admin.workspaces = { page: 1, q: String(new FormData(event.currentTarget).get('q') || '').trim() };
  loadAdminWorkspaces();
});
$('#admin-apps-filter').addEventListener('submit', (event) => {
  event.preventDefault();
  const values = new FormData(event.currentTarget);
  state.admin.apps = { page: 1, q: String(values.get('q') || '').trim(), archived: String(values.get('archived') || '') };
  loadAdminApps();
});
$('#admin-usage-filter').addEventListener('submit', (event) => {
  event.preventDefault();
  const values = new FormData(event.currentTarget);
  state.admin.usage = { page: 1, from: String(values.get('from') || ''), to: String(values.get('to') || '') };
  loadAdminUsage();
});
$('#admin-audit-filter').addEventListener('submit', (event) => {
  event.preventDefault();
  const values = new FormData(event.currentTarget);
  state.admin.audit = { page: 1, targetType: String(values.get('targetType') || ''), targetId: String(values.get('targetId') || '').trim() };
  loadAdminAudit();
});

$('#auth-form').addEventListener('submit', submitAuth);
$('#edit-app-form').addEventListener('submit', saveAppDetails);
$('#switch-auth').addEventListener('click', () => authMode(state.authMode === 'register' ? 'login' : 'register'));
$('#password-reset-form').addEventListener('submit', submitPasswordReset);
$('#request-reset-form').addEventListener('submit', submitPasswordResetRequest);
$('#account-delete-form').addEventListener('submit', submitAccountDeletion);
$('#account-deactivate-form').addEventListener('submit', submitAccountDeactivation);
$('#ai-budget-form').addEventListener('submit', saveAIBudget);
$('#admin-ai-form').addEventListener('submit', saveAIKey);
$('#admin-user-status-form').addEventListener('submit', submitAdminUserStatus);
$('#app-access-form').addEventListener('submit', saveAppAccess);
$('#create-table-form').addEventListener('submit', createTable);
$('#record-form').addEventListener('submit', submitRecord);
$('#agent-form').addEventListener('submit', submitPrompt);
$('#home-agent-form').addEventListener('submit', submitPrompt);
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
  if (location.pathname.startsWith('/admin') && state.isPlatformAdmin) await openPlatformAdmin(adminRoutePage(), false);
  else {
    if (location.pathname.startsWith('/admin')) history.replaceState({}, '', '/');
    show('workspace');
    await renderWorkspace();
  }
});
bootstrap();
