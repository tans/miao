import { createFxAgent, supportsJspi } from '/vendor/fx/browser.js';

const TOKEN_KEY = 'miao_token';
const state = {
  token: localStorage.getItem(TOKEN_KEY), user: null, tenant: null,
  workspaces: [], apps: [], archivedApps: [], app: null, tables: [], table: null, records: [],
  recordQuery: { page: 1, perPage: 25, search: '', sort: '-created', filterField: '', filterValue: '' }, recordResult: null,
  editingRecordId: null, editingApp: false, creatingApp: false,
  authMode: 'register', fxAgent: null, fxBusy: false,
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

function show(screen) {
  for (const id of ['landing', 'auth', 'workspace']) $(`#${id}`).classList.toggle('hidden', id !== screen);
}

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
  show('auth');
}

function clearAgent() {
  if (state.fxAgent) state.fxAgent.close().catch(() => {});
  state.fxAgent = null;
  state.fxBusy = false;
}

function logout() {
  if (state.token) api('/api/auth/logout', { method: 'POST' }).catch(() => {});
  clearAgent();
  state.token = null;
  state.apps = [];
  state.workspaces = [];
  state.app = null;
  state.tenant = null;
  state.tables = [];
  state.table = null;
  localStorage.removeItem(TOKEN_KEY);
  show('landing');
}

async function bootstrap() {
  if (!state.token) return state.pendingInvite ? authMode('register') : show('landing');
  try {
    if (state.pendingInvite) {
      try {
        const accepted = await api('/api/invites/accept', { method: 'POST', body: JSON.stringify({ token: state.pendingInvite }) });
        state.tenant = accepted.tenant;
        state.pendingInvite = null;
        history.replaceState({}, '', location.pathname);
        toast('已加入工作区');
      } catch (error) {
        toast(error.message, true);
      }
    }
    const me = await api('/api/me');
    state.user = me.user;
    state.tenant = me.tenant;
    state.workspaces = me.workspaces || [];
    state.aiConfigured = me.ai_configured;
    state.apps = me.apps || [];
    state.archivedApps = await api('/api/apps?archived=true').catch(() => []);
    state.app = null;
    state.table = null;
    show('workspace');
    await renderWorkspace();
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
  try {
    const result = await api(path, { method: 'POST', body: JSON.stringify(payload) });
    state.token = result.token;
    localStorage.setItem(TOKEN_KEY, result.token);
    state.user = result.user;
    state.tenant = result.tenant;
    if (state.pendingInvite) {
      try {
        const accepted = await api('/api/invites/accept', { method: 'POST', body: JSON.stringify({ token: state.pendingInvite }) });
        state.tenant = accepted.tenant;
        state.pendingInvite = null;
        history.replaceState({}, '', location.pathname);
        toast('已加入工作区');
      } catch (error) {
        toast(error.message, true);
      }
    }
    const me = await api('/api/me');
    state.workspaces = me.workspaces || [];
    state.aiConfigured = me.ai_configured;
    state.apps = me.apps || [];
    state.archivedApps = await api('/api/apps?archived=true').catch(() => []);
    state.app = null;
    state.table = null;
    show('workspace');
    await renderWorkspace();
  } catch (error) {
    toast(error.message, true);
  }
}

function renderApps() {
  $('#workspace-switcher').innerHTML = state.workspaces.map((workspace) => `<option value="${esc(workspace.id)}">${esc(workspace.name)}${workspace.role === 'owner' ? ' · 所有者' : ' · 成员'}</option>`).join('');
  $('#workspace-switcher').value = state.tenant?.id || '';
  $('#user-name').textContent = state.user?.name || '用户';
  $('#user-email').textContent = state.user?.email || '';
  $('#user-avatar').textContent = (state.user?.name || 'M').slice(0, 1);
  $('#ai-config-status').textContent = state.aiConfigured ? 'AI 助手使用企业统一配置。' : '企业尚未配置 AI Gateway，暂时无法使用 AI 助手。';
  $('#ai-config-status').classList.toggle('error', !state.aiConfigured);
  $('#app-list').innerHTML = state.apps.map((item) => `<button class="app-nav-item ${state.app?.id === item.id ? 'active' : ''}" data-open-app="${esc(item.id)}"><span class="app-nav-mark">${esc(item.name.slice(0, 1))}</span>${esc(item.name)}</button>`).join('');
}

async function renderWorkspace() {
  renderApps();
  const dashboard = !state.app && !state.editingApp && !state.creatingApp;
  $('#dashboard').classList.toggle('hidden', !dashboard);
  $('#app-creation').classList.toggle('hidden', Boolean(state.app));
  $('#app-content').classList.toggle('hidden', !state.app);
  $('#create-table-open').classList.toggle('hidden', !state.app);
  const canManageApp = Boolean(state.app && state.tenant?.role === 'owner');
  $('#edit-app-open').classList.toggle('hidden', !canManageApp);
  $('#archive-app').classList.toggle('hidden', !canManageApp);
  $('#delete-app').classList.toggle('hidden', !canManageApp);
  if (dashboard) {
    $('#app-title').textContent = '日常工作台';
    $('#breadcrumb-app').textContent = '工作台';
    $('#app-description').textContent = '从最近使用的工具继续，或创建新的工作工具。';
    $('#dashboard-workspace-name').textContent = state.tenant?.name || '';
    $('#dashboard-stats').innerHTML = `<div class="dashboard-stat"><strong>${state.apps.length}</strong><span>个应用</span></div><div class="dashboard-stat"><strong>${state.workspaces.length}</strong><span>个工作区</span></div><div class="dashboard-stat"><strong>${state.aiConfigured ? '就绪' : '待配置'}</strong><span>fx 助手</span></div>`;
    $('#dashboard-apps').innerHTML = state.apps.length ? state.apps.map((item) => `<button class="dashboard-app-card" data-open-app="${esc(item.id)}"><span class="dashboard-app-icon">${esc(item.name.slice(0, 1))}</span><span class="dashboard-app-copy"><strong>${esc(item.name)}</strong><small>${esc(item.description || '尚未填写用途说明')}</small><small>最近更新 ${item.updated_at ? new Date(item.updated_at).toLocaleDateString() : '—'}</small></span><span aria-hidden="true">→</span></button>`).join('') : '<div class="dashboard-empty"><strong>还没有应用</strong><span>创建应用后，就能在这里继续日常工作。</span><button class="btn btn-primary btn-sm" data-action="create-app">创建第一个应用</button></div>';
    $('#archived-app-section').classList.toggle('hidden', !state.archivedApps.length);
    $('#archived-apps').innerHTML = state.archivedApps.map((item) => `<div class="dashboard-app-card"><span class="dashboard-app-icon">${esc(item.name.slice(0, 1))}</span><span class="dashboard-app-copy"><strong>${esc(item.name)}</strong><small>${esc(item.description || '尚未填写用途说明')}</small></span><button class="btn btn-ghost btn-sm" data-restore-app="${esc(item.id)}">恢复</button></div>`).join('');
    return;
  }
  if (!state.app) {
    $('#app-title').textContent = state.editingApp ? '修改应用' : '创建应用';
    $('#app-description').textContent = '';
    $('#app-form-title').textContent = state.editingApp ? '修改应用信息' : '先创建要用的工具';
    $('#app-form-copy').textContent = state.editingApp ? '更新名称和用途说明，保存后立即生效。' : '起个名字，再用一句话说明它要解决什么工作。';
    const form = $('#create-app-form');
    form.querySelector('[name="name"]').value = state.editingApp?.name || '';
    form.querySelector('[name="description"]').value = state.editingApp?.description || '';
    form.querySelector('button[type="submit"]').textContent = state.editingApp ? '保存修改' : '创建工具';
    return;
  }
  state.editingApp = false;
  $('#dashboard').classList.add('hidden');
  $('#app-title').textContent = state.app.name;
  $('#breadcrumb-app').textContent = state.app.name;
  $('#app-description').textContent = state.app.description || '';
  state.tables = await api(`/api/apps/${state.app.id}/collections`);
  if (!state.tables.some((item) => item.slug === state.table?.slug)) state.table = state.tables[0] || null;
  renderTables();
  if (state.table) await renderRecords();
  else renderNoTables();
}

function renderTables() {
  $('#table-list').innerHTML = state.tables.map((table) => `<button class="table-nav-item ${state.table?.id === table.id ? 'active' : ''}" data-table="${esc(table.slug)}"><span>▤</span>${esc(table.name)}</button>`).join('') || '<p class="no-tables">还没有数据表。自己创建一张表，或让 AI 助手帮忙。</p>';
}

function renderNoTables() {
  $('#records-root').innerHTML = '<div class="records-empty"><strong>从一张数据表开始</strong><span>你可以自己创建，也可以让 AI 助手按你的描述创建。</span></div>';
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
  $('#records-root').innerHTML = `<div class="records-heading"><div><h2>${esc(state.table.name)}</h2><span>${state.recordResult.totalItems} 条记录 · ${fields.length} 个字段</span></div><div><button class="btn btn-ghost btn-sm" data-action="edit-table">改名</button><button class="btn btn-ghost btn-sm" data-action="delete-table">删除表</button><button class="btn btn-ghost btn-sm" data-action="add-record">＋ 添加记录</button></div></div><div class="record-query"><input class="input input-bordered input-sm" id="record-search" type="search" placeholder="搜索文本字段" value="${esc(state.recordQuery.search)}"><select class="select select-bordered select-sm" id="record-sort">${sortOptions.map((value) => `<option value="${esc(value)}" ${value === state.recordQuery.sort ? 'selected' : ''}>排序：${esc(value.replace(/^-/, ''))}${value.startsWith('-') ? ' ↓' : ' ↑'}</option>`).join('')}</select><select class="select select-bordered select-sm" id="record-filter-field"><option value="">筛选字段</option>${fields.map((field) => `<option value="${esc(field.name)}" ${field.name === state.recordQuery.filterField ? 'selected' : ''}>${esc(field.label || field.name)}</option>`).join('')}</select><input class="input input-bordered input-sm" id="record-filter-value" placeholder="筛选值" value="${esc(state.recordQuery.filterValue)}"><button class="btn btn-ghost btn-sm" data-action="clear-filter">清除</button></div>${state.records.length ? `<div class="overflow-x-auto"><table class="table table-sm"><thead><tr>${columns.map((field) => `<th>${esc(field.label || field.name)}</th>`).join('')}<th></th></tr></thead><tbody>${state.records.map((row) => `<tr>${columns.map((field) => { const value = row.data[field.name]; const display = field.type === 'bool' && value !== undefined ? (value ? '是' : '否') : value ?? '—'; return `<td title="${esc(display)}">${esc(display)}</td>`; }).join('')}<td class="record-actions"><button class="btn btn-ghost btn-xs" title="编辑记录" aria-label="编辑记录" data-edit-record="${esc(row.id)}">编辑</button><button class="btn btn-ghost btn-xs" title="删除记录" aria-label="删除记录" data-delete-record="${esc(row.id)}">×</button></td></tr>`).join('')}</tbody></table></div>` : '<div class="records-empty"><strong>没有匹配的记录</strong><span>调整搜索条件或添加一条记录。</span></div>'}<div class="record-pagination"><span>第 ${state.recordResult.page} / ${Math.max(1, state.recordResult.totalPages)} 页</span><button class="btn btn-ghost btn-sm" data-page="${Math.max(1, state.recordResult.page - 1)}" ${state.recordResult.page <= 1 ? 'disabled' : ''}>上一页</button><button class="btn btn-ghost btn-sm" data-page="${Math.min(state.recordResult.totalPages || 1, state.recordResult.page + 1)}" ${state.recordResult.page >= state.recordResult.totalPages ? 'disabled' : ''}>下一页</button><select class="select select-bordered select-sm" id="record-page-size"><option ${state.recordQuery.perPage === 25 ? 'selected' : ''}>25</option><option ${state.recordQuery.perPage === 50 ? 'selected' : ''}>50</option><option ${state.recordQuery.perPage === 100 ? 'selected' : ''}>100</option></select></div>`;
  if (['record-search', 'record-filter-value'].includes(focusedId)) {
    const control = $(`#${focusedId}`);
    control?.focus();
    if (selectionStart !== null && selectionEnd !== null) control?.setSelectionRange(selectionStart, selectionEnd);
  }
}

async function createApp(event) {
  event.preventDefault();
  const form = new FormData(event.currentTarget);
  try {
    const editing = Boolean(state.editingApp);
    const app = editing
      ? await api(`/api/apps/${state.editingApp.id}`, { method: 'PATCH', body: JSON.stringify({ name: form.get('name'), description: form.get('description') }) })
      : await api('/api/apps', { method: 'POST', body: JSON.stringify({ name: form.get('name'), description: form.get('description') }) });
    if (editing) state.apps = state.apps.map((item) => item.id === app.id ? app : item);
    else state.apps.unshift(app);
    state.app = editing ? app : app;
    state.editingApp = false;
    state.creatingApp = false;
    state.table = null;
    event.currentTarget.reset();
    await renderWorkspace();
    toast(editing ? '应用信息已更新' : '工具已创建');
  } catch (error) {
    toast(error.message, true);
  }
}

async function refreshApps() {
  const me = await api('/api/me');
  state.apps = me.apps || [];
  state.archivedApps = await api('/api/apps?archived=true').catch(() => []);
  state.app = null;
  state.table = null;
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

function fieldKey(label, index) {
  const slug = label.toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 20);
  return slug || `field_${index + 1}`;
}

async function createTable(event) {
  event.preventDefault();
  const form = new FormData(event.currentTarget);
  const fields = String(form.get('fields') || '').split('\n').map((value) => value.trim()).filter(Boolean).slice(0, 24).map((label, index) => ({ name: fieldKey(label, index), label, type: 'text' }));
  if (!fields.length) return toast('请至少填写一个字段', true);
  try {
    const table = await api(`/api/apps/${state.app.id}/collections`, { method: 'POST', body: JSON.stringify({ name: form.get('name'), fields }) });
    state.tables.push(table);
    state.table = table;
    event.currentTarget.reset();
    $('#table-dialog').close();
    await renderRecords();
    toast('数据表已创建');
  } catch (error) {
    toast(error.message, true);
  }
}

function openRecordEditor(record = null) {
  if (!state.table) return;
  state.editingRecordId = record?.id || null;
  $('#record-dialog-title').textContent = record ? '编辑记录' : '添加记录';
  $('#record-save').textContent = record ? '保存修改' : '添加记录';
  $('#record-form-fields').innerHTML = state.table.fields.map((field) => {
    const label = esc(field.label || field.name);
    const value = record?.data?.[field.name];
    const required = field.required ? ' required' : '';
    const common = `data-record-field="${esc(field.name)}"`;
    if (field.type === 'bool') {
      const selected = (candidate) => value === candidate ? ' selected' : '';
      return `<label>${label}<select class="select select-bordered w-full" ${common}${required}><option value=""${selected(undefined)}>请选择</option><option value="true"${selected(true)}>是</option><option value="false"${selected(false)}>否</option></select></label>`;
    }
    const type = field.type === 'number' ? 'number' : ['date', 'email', 'url'].includes(field.type) ? field.type : 'text';
    const shownValue = field.type === 'date' && value ? String(value).slice(0, 10) : value ?? '';
    const step = field.type === 'number' ? ' step="any"' : '';
    return `<label>${label}<input class="input input-bordered w-full" type="${type}" ${common}${step}${required} value="${esc(shownValue)}" /></label>`;
  }).join('');
  $('#record-dialog').showModal();
  $('#record-form-fields input, #record-form-fields select')?.focus();
}

async function submitRecord(event) {
  event.preventDefault();
  if (!state.table) return;
  const editing = Boolean(state.editingRecordId);
  const data = {};
  for (const field of state.table.fields) {
    const control = $(`[data-record-field="${CSS.escape(field.name)}"]`);
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
      await api(`${collection}/${encodeURIComponent(state.editingRecordId)}`, { method: 'PATCH', body: JSON.stringify({ data }) });
    } else {
      await api(collection, { method: 'POST', body: JSON.stringify({ data }) });
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
  const appId = state.app.id;
  const request = (path, options) => api(`/api/apps/${appId}${path}`, options);
  return [
    {
      name: 'list_tables', description: '查看当前工具里的数据表和字段。',
      inputSchema: { type: 'object', properties: {} },
      async execute() { return toolResult(await request('/collections')); }
    },
    {
      name: 'create_table', description: '为当前工具创建一张数据表。字段 name 使用英文下划线格式，label 用用户看得懂的名称。',
      inputSchema: { type: 'object', required: ['name', 'fields'], properties: { name: { type: 'string' }, fields: { type: 'array', items: { type: 'object', required: ['name', 'label'], properties: { name: { type: 'string' }, label: { type: 'string' }, type: { type: 'string', enum: ['text', 'number', 'bool', 'date', 'email', 'url'] }, required: { type: 'boolean' } } } } } },
      async execute(input) { const result = await request('/collections', { method: 'POST', body: JSON.stringify(input) }); await renderWorkspace(); return toolResult(result); }
    },
    {
      name: 'list_records', description: '读取指定数据表的记录。',
      inputSchema: { type: 'object', required: ['table'], properties: { table: { type: 'string', description: '数据表 slug' } } },
      async execute(input) { return toolResult(await request(`/collections/${encodeURIComponent(input.table)}/records`)); }
    },
    {
      name: 'add_record', description: '在指定数据表中新增一条记录。',
      inputSchema: { type: 'object', required: ['table', 'data'], properties: { table: { type: 'string' }, data: { type: 'object', additionalProperties: true } } },
      async execute(input) { const result = await request(`/collections/${encodeURIComponent(input.table)}/records`, { method: 'POST', body: JSON.stringify({ data: input.data }) }); await renderWorkspace(); return toolResult(result); }
    },
    {
      name: 'update_record', description: '修改指定数据表中的一条记录。',
      inputSchema: { type: 'object', required: ['table', 'record_id', 'data'], properties: { table: { type: 'string' }, record_id: { type: 'string' }, data: { type: 'object', additionalProperties: true } } },
      async execute(input) { const result = await request(`/collections/${encodeURIComponent(input.table)}/records/${encodeURIComponent(input.record_id)}`, { method: 'PATCH', body: JSON.stringify({ data: input.data }) }); await renderWorkspace(); return toolResult(result); }
    }
  ];
}

async function getAgent() {
  if (!state.aiConfigured) throw new Error('企业尚未配置 AI Gateway，请联系管理员。');
  if (!supportsJspi()) throw new Error('当前浏览器不支持 fx 所需的 WebAssembly JSPI，请使用新版 Chrome、Edge 或 Safari。');
  if (!state.fxAgent) {
    state.fxAgent = await createFxAgent({
      apiKey: 'miao-server-managed',
      wasm: '/vendor/fx/fx-core.wasm',
      instructions: `你是 MIAO 内部工具助手，正在协助团队使用「${state.app.name}」。${state.app.description || ''}\n使用工具前先查看数据表。需要新建数据表时，字段名用简洁的英文 snake_case，label 使用中文。新增或修改业务记录前，先确认用户给出的值，不要编造数据。只操作当前工具。`,
      tools: agentTools(),
      fetch(url, init) {
        const headers = new Headers(init.headers);
        headers.delete('authorization');
        headers.set('Authorization', `Bearer ${state.token}`);
        headers.set('X-Miao-Tenant-Id', state.tenant.id);
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
  $('#agent-status').textContent = '已连接';
  $('#agent-status').className = 'badge badge-success';
  return state.fxAgent;
}

async function openMembers() {
  const [result, invites] = await Promise.all([
    api('/api/workspace/members'),
    state.tenant?.role === 'owner' ? api('/api/workspace/invites') : Promise.resolve([])
  ]);
  $('#member-list').innerHTML = result.members.map((member) => `<div class="member-row"><span>${esc(member.name)} · ${esc(member.email)}<small>${member.role === 'owner' ? '所有者' : '成员'}</small></span>${result.can_manage && member.role !== 'owner' ? `<button class="btn btn-ghost btn-xs" data-remove-member="${esc(member.membership_id)}">移除</button>` : ''}</div>`).join('') || '<p class="empty-members">还没有成员。</p>';
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
  if (!state.app || state.fxBusy) return;
  const prompt = new FormData(event.currentTarget).get('prompt').toString().trim();
  if (!prompt) return;
  event.currentTarget.reset();
  appendChat(prompt, 'user');
  const response = appendChat('', 'assistant');
  state.fxBusy = true;
  $('#agent-status').textContent = '思考中';
  $('#agent-status').className = 'badge badge-info';
  try {
    const agent = await getAgent();
    const turn = agent.prompt(prompt);
    for await (const event of turn) {
      if (event.type === 'text_delta') response.textContent += event.delta;
      if (event.type === 'tool_start') {
        const note = document.createElement('small');
        note.className = 'tool-note';
        note.textContent = `正在执行：${event.name}`;
        $('#chat-messages').append(note);
      }
      $('#chat-messages').scrollTop = $('#chat-messages').scrollHeight;
    }
    await turn.result;
    if (!response.textContent) response.textContent = '已完成。';
    $('#agent-status').textContent = '已连接';
    $('#agent-status').className = 'badge badge-success';
    await renderWorkspace();
  } catch (error) {
    response.textContent = error.message || 'Agent 暂时无法响应。';
    $('#agent-status').textContent = '连接失败';
    $('#agent-status').className = 'badge badge-error';
  } finally {
    state.fxBusy = false;
  }
}

document.addEventListener('click', async (event) => {
  const action = event.target.closest('[data-action]')?.dataset.action;
  if (action === 'register') authMode('register');
  if (action === 'login') authMode('login');
  if (action === 'home') show('landing');
  if (action === 'logout') logout();
  if (action === 'create-app') {
    clearAgent();
    state.app = null;
    state.editingApp = false;
    state.creatingApp = true;
    await renderWorkspace();
  }
  if (action === 'edit-app' && state.app) {
    state.editingApp = state.app;
    state.creatingApp = false;
    state.app = null;
    await renderWorkspace();
  }
  if (action === 'cancel-app-form') {
    state.editingApp = false;
    state.creatingApp = false;
    await renderWorkspace();
  }
  if (action === 'create-table') $('#table-dialog').showModal();
  if (action === 'archive-app') archiveApp();
  if (action === 'delete-app') deleteApp();
  if (action === 'edit-table') editTable();
  if (action === 'delete-table') deleteTable();
  if (action === 'clear-filter') {
    state.recordQuery.filterField = '';
    state.recordQuery.filterValue = '';
    state.recordQuery.page = 1;
    renderRecords();
  }
  if (action === 'close-table') $('#table-dialog').close();
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
    clearAgent();
    state.editingApp = false;
    state.creatingApp = false;
    state.app = state.apps.find((item) => item.id === appButton.dataset.openApp) || null;
    state.table = null;
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

$('#workspace-switcher').addEventListener('change', async (event) => {
  const selected = state.workspaces.find((workspace) => workspace.id === event.target.value);
  if (!selected) return;
  clearAgent();
  state.tenant = selected;
  state.apps = [];
  state.app = null;
  state.tables = [];
  state.table = null;
  try {
    const me = await api('/api/me');
    state.apps = me.apps || [];
    state.archivedApps = await api('/api/apps?archived=true').catch(() => []);
    state.aiConfigured = me.ai_configured;
    state.app = null;
    state.editingApp = false;
    state.creatingApp = false;
    await renderWorkspace();
  } catch (error) {
    toast(error.message, true);
  }
});

$('#auth-form').addEventListener('submit', submitAuth);
$('#switch-auth').addEventListener('click', () => authMode(state.authMode === 'register' ? 'login' : 'register'));
$('#create-app-form').addEventListener('submit', createApp);
$('#create-table-form').addEventListener('submit', createTable);
$('#record-form').addEventListener('submit', submitRecord);
$('#agent-form').addEventListener('submit', submitPrompt);
$('#invite-form').addEventListener('submit', createInvite);
bootstrap();
