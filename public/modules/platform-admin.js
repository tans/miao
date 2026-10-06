export function createPlatformAdmin({ state, api, $, $$, esc, toast, show, renderApps, renderWorkspace }) {
  const adminState = {
    users: { page: 1, q: '', status: '' },
    workspaces: { page: 1, q: '' },
    apps: { page: 1, q: '', archived: '' },
    usage: { page: 1, from: '', to: '' },
    audit: { page: 1, targetType: '', targetId: '' }
  };

  const adminPageTitles = { overview: '平台总览', users: '用户账号', workspaces: '工作区', apps: '应用目录', usage: 'AI 用量', audit: '平台审计', ai: 'AI 服务' };
  const adminRoutePage = () => {
    if (!location.pathname.startsWith('/admin')) return '';
    const page = location.pathname.split('/').filter(Boolean)[1] || 'overview';
    return Object.hasOwn(adminPageTitles, page) ? page : 'overview';
  };

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
    state.appRuntime = null;
    state.runtimeQuery = { page: 1, search: '' };
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
    const filter = adminState.users;
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
    const filter = adminState.workspaces;
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
    const filter = adminState.apps;
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
    const filter = adminState.usage;
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
    const filter = adminState.audit;
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
    const pageState = adminState[name];
    pageState.page = Math.max(1, pageState.page + delta);
    await adminLoaders[name]();
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

  async function handleClick(event) {
    const pageButton = event.target.closest('[data-admin-page]');
    if (pageButton) { await openPlatformAdmin(pageButton.dataset.adminPage); return true; }
    const previous = event.target.closest('[data-admin-prev]');
    if (previous) { await changeAdminPage(previous.dataset.adminPrev, -1); return true; }
    const next = event.target.closest('[data-admin-next]');
    if (next) { await changeAdminPage(next.dataset.adminNext, 1); return true; }
    const statusButton = event.target.closest('[data-admin-user-status]');
    if (statusButton) { openAdminUserStatus(statusButton); return true; }
    const retry = event.target.closest('[data-admin-retry]');
    if (retry) { await adminLoaders[retry.dataset.adminRetry]?.(); return true; }

    const action = event.target.closest('[data-action]')?.dataset.action;
    if (action === 'return-workspace') { await returnToWorkspace(); return true; }
    if (action === 'admin-ai') { await openAIAdmin(); return true; }
    if (action === 'close-admin-user-status') { $('#admin-user-status-dialog').close(); return true; }
    if (action === 'use-env-ai-key') {
      api('/api/admin/ai', { method: 'DELETE' }).then(async (result) => {
        state.aiConfigured = result.source === 'environment';
        await loadAdminAI();
        renderApps();
        toast('已改用服务器环境配置');
      }).catch((error) => toast(error.message, true));
      return true;
    }
    return false;
  }

  function bind() {
    $('#admin-users-filter')?.addEventListener('submit', (event) => {
      event.preventDefault();
      const values = new FormData(event.currentTarget);
      adminState.users = { page: 1, q: String(values.get('q') || '').trim(), status: String(values.get('status') || '') };
      loadAdminUsers();
    });
    $('#admin-workspaces-filter')?.addEventListener('submit', (event) => {
      event.preventDefault();
      adminState.workspaces = { page: 1, q: String(new FormData(event.currentTarget).get('q') || '').trim() };
      loadAdminWorkspaces();
    });
    $('#admin-apps-filter')?.addEventListener('submit', (event) => {
      event.preventDefault();
      const values = new FormData(event.currentTarget);
      adminState.apps = { page: 1, q: String(values.get('q') || '').trim(), archived: String(values.get('archived') || '') };
      loadAdminApps();
    });
    $('#admin-usage-filter')?.addEventListener('submit', (event) => {
      event.preventDefault();
      const values = new FormData(event.currentTarget);
      adminState.usage = { page: 1, from: String(values.get('from') || ''), to: String(values.get('to') || '') };
      loadAdminUsage();
    });
    $('#admin-audit-filter')?.addEventListener('submit', (event) => {
      event.preventDefault();
      const values = new FormData(event.currentTarget);
      adminState.audit = { page: 1, targetType: String(values.get('targetType') || ''), targetId: String(values.get('targetId') || '').trim() };
      loadAdminAudit();
    });
    $('#admin-ai-form')?.addEventListener('submit', saveAIKey);
    $('#admin-user-status-form')?.addEventListener('submit', submitAdminUserStatus);
  }

  return {
    bind,
    handleClick,
    routePage: adminRoutePage,
    open: openPlatformAdmin
  };
}
