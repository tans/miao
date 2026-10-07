import { createCollectionResults } from "/modules/collection-results.js";

export function createAppSettings({ state, api, $, esc, toast }) {
  const appURL = (path = '', requested = context) => `/api/apps/${encodeURIComponent(requested.appId)}${path}`;
  const results = createCollectionResults({ esc });
  const scopedToast = toast;
  const current = (requested) => requested && state.user?.id === requested.userId && state.app?.id === requested.appId && state.tenant?.id === requested.tenantId && state.appPanel === 'settings';
  let loadRevision = 0;
  let context = null;
  let busy = false;

  const pretty = (value) => JSON.stringify(value ?? {}, null, 2);
  const input = (label, name, value = '', placeholder = '') => `<label>${label}<input class="input input-sm" name="${name}" value="${esc(value)}" placeholder="${esc(placeholder)}"></label>`;

  async function load(renderAfter = true) {
    const root = $('#app-settings-root');
    if (!root) return false;
    root.innerHTML = '<p class="runtime-loading">正在读取发布与采集配置…</p>';
    const appId = state.app.id, tenantId = state.tenant.id, userId = state.user.id;
    const revision = ++loadRevision;
    const requested = { appId, tenantId, userId };
    const url = (path) => appURL(path, requested);
    const canManage = ["owner", "manager", "publisher"].includes(state.app.permission) || state.tenant.role === "owner";
    const [publication, scripts, tables, members, versions, actions, workflows] = await Promise.all([
      canManage ? api(url('/publication')) : {}, api(url('/collection-scripts')),
      api(url('/collections')), api(url('/members')), canManage ? api(url('/versions')) : {},
      canManage ? api(url('/actions')) : [], canManage ? api(url('/workflows')) : []
    ]);
    if (!current(requested) || revision !== loadRevision) return false;
    context = { appId, tenantId, userId, publication, scripts, tables, members: members.members || [], versions, actions, workflows };
    if (renderAfter) render();
    return true;
  }

  function publicPageDefaults(page, tables) {
    const reads = (page.data_sources || []).map((source) => {
      const table = tables.find((item) => item.slug === source.collection);
      const fields = table?.fields || [];
      const slugField = fields.find((field) => field.name === 'slug') || fields.find((field) => field.type === 'text');
      const statusField = fields.find((field) => field.name === 'status') || fields.find((field) => field.type === 'select');
      return { source: source.id, table: source.collection, fields: [], images: [], html_fields: [], status_field: statusField?.name || '', published_value: statusField?.type === 'select' ? (statusField.options || []).find((option) => /publish|live|已发布|发布/i.test(option)) || statusField.options?.at(-1) || '' : 'published', slug_field: slugField?.name || '', seo_title_field: fields.some((field) => field.name === 'title' && field.type === 'text') ? 'title' : '', seo_description_field: fields.some((field) => field.name === 'description' && field.type === 'text') ? 'description' : '' };
    });
    return { id: page.id, title: page.title, reads };
  }

  function render() {
    const { publication, scripts, tables, members, versions } = context;
    const canManage = ['owner', 'manager', 'publisher'].includes(state.app?.permission) || state.tenant?.role === 'owner';
    const canPublish = ['owner', 'publisher'].includes(state.app?.permission) || state.tenant?.role === 'owner';
    const publishedVersion = versions.published_version_id ? context.versionDefinition : null;
    const pages = publishedVersion?.pages || [];
    $('#app-settings-root').innerHTML = `<header class="settings-heading"><h2>发布、采集与业务配置</h2></header>
      ${canManage ? renderBusinessSettings() : ''}
      ${canPublish ? `<section class="settings-block"><h3>CMS 公开页面</h3><p>${publication.enabled ? `当前公开地址：<a href="${esc(publication.url)}" target="_blank" rel="noreferrer">${esc(publication.url)}</a>` : '当前未启用公开访问。'}</p>
      <form data-settings-form="publication" class="settings-form">${input('公开链接标识', 'slug', publication.slug, '例如 product-news')}<label class="settings-check"><input type="checkbox" name="enabled" ${publication.enabled ? 'checked' : ''}> 启用匿名公开页面</label>
      ${pages.length ? pages.map(renderPageSettings).join('') : '<p class="settings-muted">先发布一版界面，再配置公开页面授权。</p>'}
      <button class="btn btn-primary btn-sm" type="submit">保存公开授权</button></form></section>` : ''}
      <section class="settings-block"><div class="settings-section-heading"><div><h3>采集脚本</h3></div>${canPublish ? '<button class="btn btn-outline btn-sm" data-settings-action="new-script">新增采集脚本</button>' : ''}</div>
      <div class="settings-list">${scripts.map((item) => renderScript(item, canPublish)).join('') || '<p class="settings-muted">还没有采集脚本。</p>'}</div></section>`;
  }

  function renderScript(item, canPublish) {
    const canEdit = canPublish && (item.created_by === state.user.id || state.app.permission === 'owner' || state.tenant.role === 'owner');
    const buttons = canEdit ? `<button class="btn btn-sm" type="submit">保存草稿</button><button class="btn btn-outline btn-sm" type="button" data-settings-action="preview-script" data-id="${esc(item.id)}" data-revision="${esc(item.revision)}">只读试运行</button>${item.status === 'enabled' ? `<button class="btn btn-outline btn-sm" type="button" data-settings-action="run-script" data-id="${esc(item.id)}" data-revision="${esc(item.revision)}">立即运行</button><button class="btn btn-ghost btn-sm" type="button" data-settings-action="pause-script" data-id="${esc(item.id)}" data-revision="${esc(item.revision)}">暂停</button>` : `<button class="btn btn-outline btn-sm" type="button" data-settings-action="enable-script" data-id="${esc(item.id)}" data-revision="${esc(item.revision)}">确认启用</button>`}` : '';
    return `<details class="settings-item" data-collection-script="${esc(item.id)}"><summary><strong>${esc(item.name)}</strong><span class="badge badge-ghost">${esc({ enabled: '已启用', paused: '已暂停', draft: '草稿' }[item.status] || item.status)} · v${esc(item.revision)}</span></summary>${results.summary(item, context.members)}${canEdit ? `<form data-settings-form="script" data-id="${esc(item.id)}" data-revision="${esc(item.revision)}" class="settings-form">${input('名称', 'name', item.name)}<label>采集脚本声明（JSON）<textarea class="textarea" name="definition" rows="14" required>${esc(pretty(item.definition))}</textarea></label>` : '<div class="settings-form">'}<div class="settings-actions">${buttons}<button class="btn btn-ghost btn-sm" type="button" data-settings-action="runs-script" data-id="${esc(item.id)}">运行记录</button></div>${canEdit ? '</form>' : '</div>'}<div data-script-output="${esc(item.id)}"></div></details>`;
  }

  function renderBusinessSettings() {
    return [['action', '业务动作', context.actions], ['workflow', '状态流程', context.workflows]].map(([kind, label, rows]) => `<section class="settings-block"><div class="settings-section-heading"><div><h3>${label}</h3></div><button class="btn btn-outline btn-sm" data-settings-action="new-${kind}">新增${label}</button></div>
      <div class="settings-list">${rows.map((item) => `<details class="settings-item"><summary><strong>${esc(item.name)}</strong><span>${esc({ enabled: '已启用', paused: '已暂停', draft: '草稿' }[item.status] || item.status)} · v${esc(item.revision)}</span></summary><form data-settings-form="${kind}" data-id="${esc(item.id)}" data-revision="${esc(item.revision)}" class="settings-form">${input('名称', 'name', item.name)}${input('说明', 'description', item.description)}<label>${label}声明（JSON）<textarea class="textarea textarea-sm" name="definition" rows="12" required>${esc(pretty(item.definition))}</textarea></label>${item.pause_reason ? `<p class="settings-muted">${esc(item.pause_reason)}</p>` : ''}<div class="settings-actions"><button class="btn btn-sm" type="submit">保存为新修订草稿</button><button class="btn btn-outline btn-sm" type="button" data-settings-action="${item.status === 'enabled' ? 'pause' : 'enable'}-${kind}" data-id="${esc(item.id)}" data-revision="${esc(item.revision)}">${item.status === 'enabled' ? '暂停' : '确认启用此修订'}</button></div></form></details>`).join('') || `<p class="settings-muted">还没有${label}。</p>`}</div></section>`).join('');
  }

  function renderPageSettings(page) {
    const existing = context.publication.pages?.find((item) => item.id === page.id);
    const policy = existing || publicPageDefaults(page, context.tables);
    const available = (policy.reads || []).length > 0 && (policy.reads || []).every((read) => read.status_field && read.slug_field && read.published_value);
    const reads = (policy.reads || []).map((read) => {
      const table = context.tables.find((candidate) => candidate.slug === read.table);
      const imageFields = (table?.fields || []).filter((field) => field.type === 'file').map((field) => field.name);
      const imageField = imageFields.length ? `<label>明确公开图片字段（逗号分隔）<input class="input input-sm" name="images:${page.id}:${read.source}" value="${esc((read.images || []).join(', '))}" placeholder="${esc(imageFields.join(', '))}"></label>` : '';
      return `<div class="settings-read"><label>公开字段（逗号分隔）<input class="input input-sm" name="fields:${page.id}:${read.source}" value="${esc((read.fields || []).join(', '))}" placeholder="例如 title, slug, body"></label><label>受控 HTML 正文字段（逗号分隔，最多 4 个）<input class="input input-sm" name="html:${page.id}:${read.source}" value="${esc((read.html_fields || []).join(', '))}" placeholder="例如 body"></label>${imageField}${input('状态字段', `status:${page.id}:${read.source}`, read.status_field)}${input('已发布值', `published:${page.id}:${read.source}`, read.published_value)}${input('Slug 字段', `slugfield:${page.id}:${read.source}`, read.slug_field)}${input('SEO 标题字段', `seotitle:${page.id}:${read.source}`, read.seo_title_field || '')}${input('SEO 描述字段', `seodesc:${page.id}:${read.source}`, read.seo_description_field || '')}</div>`;
    }).join('');
    return `<fieldset class="settings-subform"><legend>${esc(page.title)}</legend><label class="settings-check"><input type="checkbox" name="page:${page.id}" ${existing ? 'checked' : ''}> 包含此公开页面${available ? '' : '（缺少兼容的状态/slug 字段，不适合公开）'}</label>${input('页面标题', `title:${page.id}`, policy.title || page.title)}${reads || '<p>此页面尚无可授权的数据源。</p>'}</fieldset>`;
  }

  async function loadVersionDefinition() {
    const requested = context;
    if (!requested.versions.published_version_id) return;
    const result = await api(`/api/apps/${encodeURIComponent(requested.appId)}/versions/${encodeURIComponent(requested.versions.published_version_id)}`);
    requested.versionDefinition = result.definition;
  }

  async function open() {
    if (!await load(false)) return;
    const requested = context;
    await loadVersionDefinition();
    if (context === requested && state.app?.id === requested.appId && state.tenant?.id === requested.tenantId) render();
  }

  async function submit(event, requested) {
    const context = requested;
    const appURL = (path) => `/api/apps/${encodeURIComponent(requested.appId)}${path}`;
    const refresh = async () => { if (current(requested)) await open(); };
    const toast = (...args) => { if (current(requested)) scopedToast(...args); };
    const form = event.target.closest('[data-settings-form]');
    if (!form) return false;
    event.preventDefault();
    const data = new FormData(form);
    const kind = form.dataset.settingsForm;
    if (kind === 'action' || kind === 'workflow') {
      const definition = JSON.parse(String(data.get('definition') || '{}'));
      await api(appURL(`/${kind === 'action' ? 'actions' : 'workflows'}/${encodeURIComponent(form.dataset.id)}`), { method: 'PATCH', body: JSON.stringify({ name: data.get('name'), description: data.get('description'), definition, expected_revision: Number(form.dataset.revision) }) });
      toast('业务配置草稿已保存'); await refresh(); return true;
    }
    if (kind === 'publication') {
      const pages = (context.versionDefinition?.pages || []).filter((page) => data.get(`page:${page.id}`) === 'on').map((page) => {
        const prior = context.publication.pages?.find((item) => item.id === page.id) || publicPageDefaults(page, context.tables);
        const reads = (prior.reads || []).map((read) => {
          const output = { ...read, fields: String(data.get(`fields:${page.id}:${read.source}`) || '').split(',').map((part) => part.trim()).filter(Boolean), status_field: data.get(`status:${page.id}:${read.source}`) || '', published_value: data.get(`published:${page.id}:${read.source}`) || '', slug_field: data.get(`slugfield:${page.id}:${read.source}`) || '' };
          for (const [key, prefix] of [['seo_title_field', 'seotitle'], ['seo_description_field', 'seodesc']]) { const value = String(data.get(`${prefix}:${page.id}:${read.source}`) || '').trim(); if (value) output[key] = value; else delete output[key]; }
          const images = String(data.get(`images:${page.id}:${read.source}`) || '').split(',').map((part) => part.trim()).filter(Boolean); if (images.length) output.images = images; else output.images = [];
          output.html_fields = String(data.get(`html:${page.id}:${read.source}`) || '').split(',').map((part) => part.trim()).filter(Boolean);
          return output;
        });
        if (!reads.length || reads.some((read) => !read.fields.length || !read.status_field || !read.published_value || !read.slug_field)) throw new Error(`页面「${page.title}」必须为每个数据源配置公开字段、状态字段、已发布值和 slug 字段。`);
        return { id: page.id, title: data.get(`title:${page.id}`) || page.title, reads };
      });
      const enabling = data.get('enabled') === 'on';
      if (enabling && !pages.length) { toast('公开访问至少需要选择一个完整配置的页面。', true); return true; }
      if (enabling && !window.confirm('确认启用匿名公开访问？访客无需登录即可读取以上选择的字段。')) return true;
      await api(appURL('/publication'), { method: 'PUT', body: JSON.stringify({ enabled: enabling, slug: data.get('slug'), pages, confirm: true }) });
      toast('公开配置已保存'); await refresh(); return true;
    }
    if (kind === 'script') {
      const definition = JSON.parse(String(data.get('definition') || '{}'));
      await api(appURL(`/collection-scripts/${encodeURIComponent(form.dataset.id)}`), { method: 'PATCH', body: JSON.stringify({ name: data.get('name'), definition, expected_revision: Number(form.dataset.revision) }) });
      toast('采集脚本草稿已保存'); await refresh(); return true;
    }
    return false;
  }

  async function click(event, requested) {
    const context = requested;
    const appURL = (path) => `/api/apps/${encodeURIComponent(requested.appId)}${path}`;
    const refresh = async () => { if (current(requested)) await open(); };
    const toast = (...args) => { if (current(requested)) scopedToast(...args); };
    const button = event.target.closest('[data-settings-action]');
    if (!button) return false;
    const id = button.dataset.id, revision = Number(button.dataset.revision);
    switch (button.dataset.settingsAction) {
      case 'new-action': {
        const name = window.prompt('业务动作名称'); if (!name?.trim()) break;
        const slug = window.prompt(`目标数据表标识：${context.tables.map((table) => table.slug).join('、')}`, context.tables[0]?.slug || '');
        if (slug === null) break;
        const table = context.tables.find((item) => item.slug === slug.trim());
        if (!table) throw new Error('请选择当前应用的数据表。');
        const fields = table.fields.filter((field) => ['text', 'number', 'bool', 'date', 'email', 'url', 'select'].includes(field.type));
        const fieldName = window.prompt(`要更新的字段：${fields.map((field) => `${field.label || field.name} (${field.name})`).join('、')}`, fields[0]?.name || '');
        if (fieldName === null) break;
        const field = fields.find((item) => item.name === fieldName.trim());
        if (!field) throw new Error('请选择当前表的普通字段。关联创建可在草稿声明中继续配置。');
        const rawValue = window.prompt(`新的「${field.label || field.name}」值${field.type === 'select' ? `：${field.options.join('、')}` : ''}`);
        if (rawValue === null) break;
        let value = rawValue;
        if (field.type === 'number') { if (!rawValue.trim() || !Number.isFinite(Number(rawValue))) throw new Error('请填写有效数字。'); value = Number(rawValue); }
        if (field.type === 'bool') { if (!['true', 'false'].includes(rawValue)) throw new Error('布尔字段请填写 true 或 false。'); value = rawValue === 'true'; }
        const definition = { inputs: [{ name: 'record_id', type: 'text', required: true }, { name: 'record_updated_at', type: 'text', required: true }], conditions: [], steps: [{ id: 'update_record', operation: 'update', table: table.slug, record_id: '$record_id', expected_updated_at: '$record_updated_at', data: { [field.name]: value } }] };
        await api(appURL('/actions'), { method: 'POST', body: JSON.stringify({ name: name.trim(), definition }) }); await refresh(); break;
      }
      case 'new-workflow': {
        const available = context.tables.filter((table) => table.fields.some((field) => field.type === 'select' && field.options?.length >= 2));
        if (!available.length) throw new Error('请先建立至少有两个选项的状态字段。');
        const name = window.prompt('状态流程名称'); if (!name?.trim()) break;
        const slug = window.prompt(`目标数据表标识：${available.map((table) => table.slug).join('、')}`, available[0].slug);
        if (slug === null) break;
        const table = available.find((item) => item.slug === slug.trim());
        if (!table) throw new Error('请选择有状态选项的当前应用数据表。');
        const fields = table.fields.filter((field) => field.type === 'select' && field.options?.length >= 2);
        const fieldName = window.prompt(`状态字段：${fields.map((field) => `${field.label || field.name} (${field.name})`).join('、')}`, fields[0].name);
        if (fieldName === null) break;
        const field = fields.find((item) => item.name === fieldName.trim());
        if (!field) throw new Error('请选择当前表的状态字段。');
        const definition = { table: table.slug, state_field: field.name, states: field.options.map((option) => ({ id: option, label: option })), transitions: field.options.slice(1).map((option, index) => ({ id: `transition_${index + 1}`, label: `设为${option}`, from: field.options[index], to: option })) };
        await api(appURL('/workflows'), { method: 'POST', body: JSON.stringify({ name: name.trim(), definition }) }); await refresh(); break;
      }
      case 'enable-action':
      case 'pause-action':
      case 'enable-workflow':
      case 'pause-workflow': {
        const kind = button.dataset.settingsAction.endsWith('workflow') ? 'workflows' : 'actions', enabled = button.dataset.settingsAction.startsWith('enable');
        if (enabled && !window.confirm('确认启用当前展示的修订和修改范围？绑定后有写权限的成员可以执行。')) break;
        await api(appURL(`/${kind}/${encodeURIComponent(id)}/enable`), { method: 'POST', body: JSON.stringify({ expected_revision: revision, enabled, confirm: true }) }); await refresh(); break;
      }
      case 'new-script': {
        if (!context.tables.length) { toast('请先创建目标数据表。', true); break; }
        const name = window.prompt('采集脚本名称'); if (!name?.trim()) break;
        const sourceUrl = window.prompt('来源 URL（完整 HTTP(S) 地址）'); if (!sourceUrl?.trim()) break;
        const table = context.tables.find((candidate) => candidate.fields.some((field) => field.required && ['text', 'number', 'bool', 'date', 'email', 'url', 'select'].includes(field.type)));
        if (!table) { toast('目标表需要至少一个可由来源映射满足的必填普通字段。', true); break; }
        const fields = table.fields.filter((field) => ['text', 'number', 'bool', 'date', 'email', 'url', 'select'].includes(field.type));
        const sourceNames = [];
        const extracted = {};
        const mappingDefaults = Object.fromEntries(fields.filter((field) => extracted[field.name]).map((field) => [field.name, { from: field.name, type: field.type }]));

        let mapping;
        try {
          const rawMapping = window.prompt(`将来源字段映射到「${table.name}」的目标字段。可用来源字段：${sourceNames.join(', ')}。值格式：{"目标字段":{"from":"来源字段","type":"text"}}`, pretty(mappingDefaults));
          if (rawMapping === null) break;
          mapping = JSON.parse(rawMapping);
        } catch { toast('映射 JSON 无效，请重新创建脚本。', true); break; }
        if (!mapping || Array.isArray(mapping) || typeof mapping !== 'object' || !Object.keys(mapping).length || fields.some((field) => field.required && !mapping[field.name])) { toast('映射必须覆盖目标表所有必填字段。', true); break; }
        const sourceNameSet = new Set(sourceNames);
        if (Object.values(mapping).some((value) => !sourceNameSet.has(typeof value === 'string' ? value : value?.from))) { toast('字段映射引用了连接器没有提取的来源字段。', true); break; }
        const mappedTarget = Object.keys(mapping);
        const dedupTarget = mappedTarget.find((field) => ['slug', 'source_id', 'url', 'source_url', 'id'].includes(field)) || mappedTarget[0];
        const dedupField = typeof mapping[dedupTarget] === 'string' ? mapping[dedupTarget] : mapping[dedupTarget].from;
        const definition = { source: { url: sourceUrl.trim(), pagination: { max_pages: 1 } }, target: { table: table.slug, fields: mapping }, filters: [], dedup: { fields: [dedupField], on_change: 'update' }, recipients: context.members.slice(0, 1).map((member) => member.id), baseline: 'silent', schedule: { type: 'manual', timezone: 'Asia/Shanghai' } };
        if (!definition.recipients.length) { toast('工作区没有可接收通知的成员。', true); break; }
        await api(appURL('/collection-scripts'), { method: 'POST', body: JSON.stringify({ name: name.trim(), definition }) });
        await refresh(); break;
      }
      case 'preview-script': {
        const result = await api(appURL(`/collection-scripts/${encodeURIComponent(id)}/preview`), { method: 'POST', body: JSON.stringify({ expected_revision: revision }) });
        if (current(requested)) showResult(id, results.run(result));
        break;
      }
      case 'run-script': {
        if (!button.dataset.requestId && !window.confirm('立即运行会读取外部来源，并按此脚本配置写入业务记录和创建通知。继续？')) break;
        const requestId = button.dataset.requestId || globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`;
        button.dataset.requestId = requestId;
        const result = await api(appURL(`/collection-scripts/${encodeURIComponent(id)}/run`), { method: 'POST', body: JSON.stringify({ expected_revision: revision, confirm: true, request_id: requestId }) });
        if (result.status !== 'running') delete button.dataset.requestId;
        if (current(requested)) showResult(id, results.run(result));
        break;
      }
      case 'enable-script':
        if (!window.confirm('确认启用此版本？之后将按配置运行采集、写入目标表并向指定成员创建通知。')) break;
        await api(appURL(`/collection-scripts/${encodeURIComponent(id)}/enable`), { method: 'POST', body: JSON.stringify({ expected_revision: revision, confirm: true }) }); await refresh(); break;
      case 'pause-script':
        await api(appURL(`/collection-scripts/${encodeURIComponent(id)}/pause`), { method: 'POST', body: JSON.stringify({ expected_revision: revision }) }); await refresh(); break;
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
    if (!current(context)) throw new Error('应用上下文已切换，请重新打开业务配置。');
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
  return { open, showCollectionRun, submit: (event) => once(event, submit), click: (event) => once(event, click) };
}
