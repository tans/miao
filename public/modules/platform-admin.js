import { renderUsageBreakdown, renderUsageRequestRows } from '/modules/ai-usage-view.js';
export function createPlatformAdmin({ state, api, $, $$, esc, toast, show, renderApps, renderWorkspace }) {
  const adminState = {
    users: { page: 1, q: '', status: '' },
    workspaces: { page: 1, q: '' },
    apps: { page: 1, q: '', archived: '' },
    usage: { page: 1, from: '', to: '', kind: '' },
    requests: { page: 1 },
    audit: { page: 1, targetType: '', targetId: '' }
  };

  const adminPageTitles = { overview: '平台总览', users: '用户账号', workspaces: '工作区', settings: '平台基本设置', apps: '应用目录', usage: 'AI 用量', audit: '平台审计', ai: 'AI 服务' };
  const adminRoutePage = () => {
    if (!location.pathname.startsWith('/admin')) return '';
    const page = location.pathname.split('/').filter(Boolean)[1] || 'overview';
    return Object.hasOwn(adminPageTitles, page) ? page : 'overview';
  };

  const adminLoaders = {
    overview: loadAdminOverview,
    users: loadAdminUsers,
    workspaces: loadAdminWorkspaces,
    settings: loadAdminSettings,
    apps: loadAdminApps,
    usage: loadAdminUsage,
    requests: loadAdminRequests,
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
        ['LLM 服务', runtime.ai.configured ? `${runtime.ai.provider === 'capi' ? 'CAPI' : 'Vercel Gateway'} · ${esc(runtime.ai.model)} · ${runtime.ai.source === 'admin' ? '管理后台密钥' : '环境变量'}` : runtime.ai.enabled ? '未配置' : '已停用'],
        ['JEV 服务', runtime.jev.configured ? `${runtime.jev.provider === 'typesafe' ? 'Typesafe 官方接口' : 'Vercel Gateway'} · ${esc(runtime.jev.model)}` : runtime.jev.enabled ? '未配置' : '已停用'],
      ];
      $('#admin-runtime-summary').innerHTML = runtimeRows.map(([title, value]) => `<div><dt>${title}</dt><dd>${value}</dd></div>`).join('');
      renderAdminUsageRows($('#admin-overview-usage-body'), usage.items, 6);
      $('#admin-overview-ai-desc').textContent = `LLM ${numberLabel(overview.ai_today_by_kind?.llm?.requests)} · JEV ${numberLabel(overview.ai_today_by_kind?.jev?.requests)} · 未分类 ${numberLabel(overview.ai_today_by_kind?.unclassified?.requests)}`;
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
    bodyNode.innerHTML = items.length ? items.map((row) => `<tr><td>${esc(row.tenant?.name || '工作区已删除')}</td><td>${numberLabel(row.requests)}</td><td>${numberLabel(row.successes)}</td><td>${numberLabel(row.errors)}</td>${columnCount === 7 ? `<td>${numberLabel(row.pending)}</td>` : ''}<td>${numberLabel(row.input_tokens)}${row.input_unknown ? ` + ${row.input_unknown} 次未知` : ''}</td><td>${numberLabel(row.output_tokens)}${row.output_unknown ? ` + ${row.output_unknown} 次未知` : ''}</td></tr>`).join('') : `<tr><td colspan="${columnCount}">所选时间范围内没有用量。</td></tr>`;
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
      const params = new URLSearchParams({ from, to, kind: filter.kind, page: String(filter.page), perPage: '25' });
      const result = await api(`/api/admin/usage?${params}`);
      $('#admin-usage-totals').innerHTML = `<span>请求 <strong>${numberLabel(result.totals.requests)}</strong></span><span>成功 <strong>${numberLabel(result.totals.successes)}</strong></span><span>失败 <strong>${numberLabel(result.totals.errors)}</strong></span><span>输入 <strong>${numberLabel(result.totals.input_tokens)}</strong> tokens${result.totals.input_unknown ? ` + ${result.totals.input_unknown} 次未知` : ''}</span><span>输出 <strong>${numberLabel(result.totals.output_tokens)}</strong> tokens${result.totals.output_unknown ? ` + ${result.totals.output_unknown} 次未知` : ''}</span>`;
      $('#admin-usage-by-kind').innerHTML = renderUsageBreakdown(result.by_kind, esc);
      renderAdminUsageRows(bodyNode, result.items);
      await loadAdminRequests();
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

  async function loadAdminRequests() {
    const filter = adminState.usage;
    const params = new URLSearchParams({ from: new Date(`${filter.from}T00:00:00.000Z`).toISOString(), to: new Date(`${filter.to}T23:59:59.999Z`).toISOString(), kind: filter.kind, page: String(adminState.requests.page), perPage: '25' });
    const body = $('#admin-requests-body');
    body.innerHTML = '<tr><td colspan="7">正在读取明细…</td></tr>';
    try {
      const result = await api(`/api/admin/usage/requests?${params}`);
      body.innerHTML = renderUsageRequestRows(result.items, esc, true);
      updateAdminPager('requests', result);
    } catch (error) { body.innerHTML = '<tr><td colspan="7">调用明细暂不可用，请重新查询。</td></tr>'; setAdminNotice(error.message); }
  }

  async function loadAdminAI() {
    setAdminNotice();
    $('#admin-ai-services').textContent = '正在读取 LLM / JEV 配置…';
    try {
      const config = await api('/api/admin/ai');
      const sourceLabels = { admin: '独立后台密钥', environment: '服务器环境密钥', llm: '复用 LLM Gateway 密钥', none: '未配置' };
      $('#admin-ai-services').innerHTML = ['llm', 'jev'].map((kind) => {
        const service = config[kind];
        const label = kind.toUpperCase();
        const check = service.last_check;
        const status = !service.enabled ? '已停用' : !service.configured ? '未配置' : check ? check.ok ? '最近检查成功' : '最近检查失败' : '已配置 · 尚需检查';
        return `<section class="ai-service-section"><div class="admin-section-heading"><div><h2>${label} · ${kind === 'llm' ? '生成模型' : '决策模型'}</h2><p>${service.enabled ? '已启用' : '已停用'} · ${esc(sourceLabels[service.source])} ${esc(service.key_hint)}</p></div><span class="badge ${service.enabled && check?.ok ? 'badge-success' : check && !check.ok ? 'badge-warning' : 'badge-ghost'}">${status}</span></div>
          <form data-ai-service-form="${kind}" class="ai-service-form">
            <label class="confirm-row"><input class="toggle toggle-sm" name="enabled" type="checkbox" ${service.enabled ? 'checked' : ''} />启用 ${label}</label>
            ${kind === 'llm' ? `<label>提供商<select class="select" name="provider"><option value="vercel" ${service.provider === 'vercel' ? 'selected' : ''}>Vercel Gateway</option><option value="capi" ${service.provider === 'capi' ? 'selected' : ''}>CAPI / OpenAI-compatible</option></select></label><label>API 基础地址<input class="input" name="base_url" value="${esc(service.base_url)}" required /></label>` : `<label>提供商<select class="select" name="provider"><option value="typesafe" ${service.provider === 'typesafe' ? 'selected' : ''}>Typesafe 官方接口</option><option value="vercel" ${service.provider === 'vercel' ? 'selected' : ''}>Vercel Gateway</option></select></label><p class="ai-service-note">Typesafe 官方接口使用 console.typesafe.ai 签发的密钥；Vercel Gateway 没有独立密钥时，仅可复用 Vercel 类型的 LLM 密钥。</p>`}
            <label>模型<input class="input" name="model" value="${esc(service.model)}" maxlength="160" required /></label>
            <label>密钥操作<select class="select" name="key_mode"><option value="keep">保留当前密钥</option><option value="replace">设置 / 轮换独立密钥</option><option value="environment">清除后台密钥，使用环境配置${kind === 'jev' ? '或 LLM 复用' : ''}</option></select></label>
            <label data-ai-key-field class="hidden">新 API 密钥<input class="input" type="password" name="api_key" minlength="16" maxlength="2000" autocomplete="new-password" placeholder="留空不会覆盖已有密钥" /></label>
            <small>${config.encryption_ready ? '服务端加密已就绪。修改提供商或接口地址时须明确选择密钥来源。' : '尚未配置服务端加密密钥，不能保存独立 API 密钥。'}</small>
            <div class="task-buttons"><button class="btn btn-primary btn-sm" type="submit">保存 ${label} 配置</button><button class="btn btn-sm" type="button" data-ai-check="${kind}">检查连接</button><button class="btn btn-ghost btn-sm" type="button" data-ai-reset="${kind}">恢复全部环境配置</button></div>
          </form>
          <div class="ai-check-result" data-ai-check-result="${kind}" role="status">${check ? `${check.ok ? '检查成功' : '检查失败'} · ${esc(new Date(check.checked_at).toLocaleString())} · ${numberLabel(check.latency_ms)} ms · ${esc(check.message)}` : '尚未检查连接'}</div>
        </section>`;
      }).join('');
      state.aiConfigured = config.llm.enabled && config.llm.configured;
      renderApps();
    } catch (error) { $('#admin-ai-services').textContent = 'AI 配置读取失败，请重新进入页面。'; setAdminNotice(error.message); }
  }

  async function saveAIService(event) {
    const form = event.target.closest('[data-ai-service-form]');
    if (!form) return;
    event.preventDefault();
    const submit = form.querySelector('[type="submit"]');
    submit.disabled = true;
    const values = Object.fromEntries(new FormData(form));
    values.enabled = form.elements.enabled.checked;
    try {
      await api(`/api/admin/ai/${form.dataset.aiServiceForm}`, { method: 'PUT', body: JSON.stringify(values) });
      await loadAdminAI();
      toast('服务配置已保存并生效');
    } catch (error) { toast(error.message, true); submit.disabled = false; }
  }

  async function loadAdminSettings() {
    setAdminNotice();
    const form = $('#admin-settings-form');
    const save = form.querySelector('[type="submit"]');
    save.disabled = true;
    $('#admin-settings-runtime').textContent = '正在读取平台设置…';
    try {
      const runtime = await api('/api/admin/runtime');
      form.elements.mode.value = runtime.registration.mode;
      form.elements.domains.value = runtime.registration.allowed_email_domains.join('\n');
      const rows = [
        ['邮箱验证', runtime.registration.email_verification_required ? '已开启' : '未开启'],
        ['邮件服务', runtime.mail.configured ? '已配置' : '未配置'],
        ['公开访问地址', runtime.mail.public_url_configured ? '已配置' : '未配置'],
        ['LLM 服务', runtime.ai.configured ? `已配置 · ${runtime.ai.model}` : runtime.ai.enabled ? '未配置' : '已停用'],
        ['JEV 服务', runtime.jev.configured ? `已配置 · ${runtime.jev.model}` : runtime.jev.enabled ? '未配置' : '已停用'],
        ['密钥加密', runtime.ai.encryption_key_ready ? '已就绪' : '未配置'],
        ['平台管理员', '通过 MIAO_ADMIN_EMAILS 在服务端维护'],
      ];
      $('#admin-settings-runtime').innerHTML = rows.map(([title, value]) => `<div><dt>${esc(title)}</dt><dd>${esc(value)}</dd></div>`).join('');
      save.disabled = false;
    } catch (error) {
      $('#admin-settings-runtime').textContent = '平台设置读取失败。';
      setAdminNotice(error.message);
    }
  }

  async function saveAdminSettings(event) {
    event.preventDefault();
    const form = event.currentTarget;
    const save = form.querySelector('[type="submit"]');
    const values = new FormData(form);
    save.disabled = true;
    setAdminNotice();
    try {
      await api('/api/admin/settings', { method: 'PUT', body: JSON.stringify({
        mode: values.get('mode'),
        allowed_email_domains: String(values.get('domains') || '').split(/[\s,，]+/).filter(Boolean),
      }) });
      toast('平台基本设置已保存并生效');
    } catch (error) { setAdminNotice(error.message); }
    finally { save.disabled = false; }
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
    if (action === 'open-platform-admin') { await openPlatformAdmin(); return true; }
    if (action === 'return-workspace') { await returnToWorkspace(); return true; }
    if (action === 'admin-ai') { await openAIAdmin(); return true; }
    if (action === 'close-admin-user-status') { $('#admin-user-status-dialog').close(); return true; }
    const check = event.target.closest('[data-ai-check]');
    if (check) {
      if (!window.confirm('连接检查将使用已保存配置发起真实模型请求，可能产生费用，计入当前工作区预算。继续？')) return true;
      check.disabled = true;
      const resultNode = $(`[data-ai-check-result="${check.dataset.aiCheck}"]`);
      resultNode.textContent = '正在发起真实模型请求…';
      try {
        const result = await api(`/api/admin/ai/${check.dataset.aiCheck}/check`, { method: 'POST' });
        await loadAdminAI();
      } catch (error) { resultNode.textContent = error.message; }
      finally { check.disabled = false; }
      return true;
    }
    const reset = event.target.closest('[data-ai-reset]');
    if (reset) {
      if (!window.confirm('清除后台保存的模型、提供商和密钥，恢复服务器环境配置？')) return true;
      reset.disabled = true;
      try { await api(`/api/admin/ai/${reset.dataset.aiReset}`, { method: 'DELETE' }); await loadAdminAI(); toast('已恢复环境配置'); }
      catch (error) { toast(error.message, true); reset.disabled = false; }
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
      adminState.usage = { page: 1, from: String(values.get('from') || ''), to: String(values.get('to') || ''), kind: String(values.get('kind') || '') };
      adminState.requests.page = 1;
      loadAdminUsage();
    });
    $('#admin-audit-filter')?.addEventListener('submit', (event) => {
      event.preventDefault();
      const values = new FormData(event.currentTarget);
      adminState.audit = { page: 1, targetType: String(values.get('targetType') || ''), targetId: String(values.get('targetId') || '').trim() };
      loadAdminAudit();
    });
    $('#admin-ai-services').addEventListener('submit', saveAIService);
    $('#admin-ai-services').addEventListener('change', (event) => {
      const form = event.target.closest('[data-ai-service-form]');
      if (!form) return;
      const replacing = form.elements.key_mode.value === 'replace';
      form.querySelector('[data-ai-key-field]').classList.toggle('hidden', !replacing);
      form.elements.api_key.required = replacing;
      if (form.dataset.aiServiceForm === 'jev' && event.target.name === 'provider') {
        const defaults = { typesafe: 'jev-latest', vercel: 'typesafe-ai/jev' };
        const current = form.elements.model.value.trim();
        if (!current || Object.values(defaults).includes(current)) {
          form.elements.model.value = defaults[event.target.value] || '';
        }
      }
    });
    $('#admin-settings-form')?.addEventListener('submit', saveAdminSettings);
    $('#admin-user-status-form')?.addEventListener('submit', submitAdminUserStatus);
  }

  return {
    bind,
    handleClick,
    routePage: adminRoutePage,
    open: openPlatformAdmin
  };
}
