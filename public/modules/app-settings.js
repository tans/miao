export function createAppSettings({ state, api, $, esc, toast }) {
  const appURL = (path = '') => `/api/apps/${encodeURIComponent(state.app.id)}${path}`;
  let context = null;

  const pretty = (value) => JSON.stringify(value ?? {}, null, 2);
  const input = (label, name, value = '', placeholder = '') => `<label>${label}<input class="input input-sm" name="${name}" value="${esc(value)}" placeholder="${esc(placeholder)}"></label>`;

  async function load(renderAfter = true) {
    const root = $('#app-settings-root');
    root.innerHTML = '<p class="runtime-loading">正在读取发布与采集配置…</p>';
    const [publication, connectors, scripts, tables, members, versions] = await Promise.all([
      api(appURL('/publication')), api(appURL('/connectors')), api(appURL('/collection-scripts')),
      api(appURL('/collections')), api(appURL('/members')), api(appURL('/versions'))
    ]);
    context = { publication, connectors, scripts, tables, members: members.members || [], versions };
    if (renderAfter) render();
  }

  function publicPageDefaults(page, tables) {
    const reads = (page.data_sources || []).map((source) => {
      const table = tables.find((item) => item.slug === source.collection);
      const fields = table?.fields || [];
      const slugField = fields.find((field) => field.name === 'slug') || fields.find((field) => field.type === 'text');
      const statusField = fields.find((field) => field.name === 'status') || fields.find((field) => field.type === 'select');
      return { source: source.id, table: source.collection, fields: [], images: [], status_field: statusField?.name || '', published_value: statusField?.type === 'select' ? (statusField.options || []).find((option) => /publish|live|已发布|发布/i.test(option)) || statusField.options?.at(-1) || '' : 'published', slug_field: slugField?.name || '', seo_title_field: fields.some((field) => field.name === 'title' && field.type === 'text') ? 'title' : '', seo_description_field: fields.some((field) => field.name === 'description' && field.type === 'text') ? 'description' : '' };
    });
    return { id: page.id, title: page.title, reads };
  }

  function render() {
    const { publication, connectors, scripts, tables, members, versions } = context;
    const canManage = ['owner', 'manager', 'publisher'].includes(state.app?.permission) || state.tenant?.role === 'owner';
    const canPublish = ['owner', 'publisher'].includes(state.app?.permission) || state.tenant?.role === 'owner';
    const publishedVersion = versions.published_version_id ? context.versionDefinition : null;
    const pages = publishedVersion?.pages || [];
    $('#app-settings-root').innerHTML = `<header class="settings-heading"><h2>发布与采集配置</h2><p>公开授权与外部数据源单独确认；关闭浏览器后，已启用采集按配置运行。</p></header>
      ${canPublish ? `<section class="settings-block"><h3>CMS 公开页面</h3><p>${publication.enabled ? `当前公开地址：<a href="${esc(publication.url)}" target="_blank" rel="noreferrer">${esc(publication.url)}</a>` : '当前未启用公开访问。只有明确授权的字段会对匿名访客开放。'}</p>
      <form data-settings-form="publication" class="settings-form">${input('公开链接标识', 'slug', publication.slug, '例如 product-news')}<label class="settings-check"><input type="checkbox" name="enabled" ${publication.enabled ? 'checked' : ''}> 启用匿名公开页面</label>
      ${pages.length ? pages.map(renderPageSettings).join('') : '<p class="settings-muted">请先发布一版包含内容数据源的界面，再配置公开页面授权。</p>'}
      <button class="btn btn-primary btn-sm" type="submit">保存公开授权</button></form></section>` : ''}
      ${canManage ? `<section class="settings-block"><div class="settings-section-heading"><div><h3>受限 HTTPS 连接器</h3><p>每个连接器只允许 HTTPS 与声明的路径前缀。</p></div><button class="btn btn-outline btn-sm" data-settings-action="new-connector">新增连接器</button></div>
      <div class="settings-list">${connectors.map((item) => `<details class="settings-item"><summary><strong>${esc(item.name)}</strong><span class="badge badge-ghost">${esc(item.status)}</span></summary><form data-settings-form="connector" data-id="${esc(item.id)}" class="settings-form">${input('名称', 'name', item.name)}<label>连接器声明（JSON）<textarea class="textarea" name="definition" rows="8" required>${esc(pretty(item.definition))}</textarea></label><div class="settings-actions"><button class="btn btn-sm" type="submit">保存草稿</button>${item.status !== 'enabled' ? `<button class="btn btn-outline btn-sm" type="button" data-settings-action="enable-connector" data-id="${esc(item.id)}" data-revision="${esc(item.revision)}">确认启用</button>` : `<button class="btn btn-outline btn-sm" type="button" data-settings-action="test-connector" data-id="${esc(item.id)}">测试读取</button><button class="btn btn-ghost btn-sm" type="button" data-settings-action="pause-connector" data-id="${esc(item.id)}" data-revision="${esc(item.revision)}">暂停</button>`}</div><div data-connector-output="${esc(item.id)}"></div></form></details>`).join('') || '<p class="settings-muted">还没有连接器。</p>'}</div></section>` : ''}
      ${canPublish ? `<section class="settings-block"><div class="settings-section-heading"><div><h3>采集脚本</h3><p>创建脚本后可只读试运行；试运行不写业务表，确认启用后才会按计划入库和通知。</p></div><button class="btn btn-outline btn-sm" data-settings-action="new-script">新增采集脚本</button></div>
      <div class="settings-list">${scripts.map((item) => `<details class="settings-item"><summary><strong>${esc(item.name)}</strong><span class="badge badge-ghost">${esc(item.status)} · v${esc(item.revision)}</span></summary><form data-settings-form="script" data-id="${esc(item.id)}" data-revision="${esc(item.revision)}" class="settings-form">${input('名称', 'name', item.name)}<label>采集脚本声明（JSON）<textarea class="textarea" name="definition" rows="14" required>${esc(pretty(item.definition))}</textarea></label><div class="settings-actions"><button class="btn btn-sm" type="submit">保存草稿</button><button class="btn btn-outline btn-sm" type="button" data-settings-action="preview-script" data-id="${esc(item.id)}" data-revision="${esc(item.revision)}">只读试运行</button>${item.status === 'enabled' ? `<button class="btn btn-outline btn-sm" type="button" data-settings-action="run-script" data-id="${esc(item.id)}" data-revision="${esc(item.revision)}">立即运行</button><button class="btn btn-ghost btn-sm" type="button" data-settings-action="pause-script" data-id="${esc(item.id)}">暂停</button>` : `<button class="btn btn-primary btn-sm" type="button" data-settings-action="enable-script" data-id="${esc(item.id)}" data-revision="${esc(item.revision)}">确认启用</button>`}<button class="btn btn-ghost btn-sm" type="button" data-settings-action="runs-script" data-id="${esc(item.id)}">运行记录</button></div><div data-script-output="${esc(item.id)}"></div></form></details>`).join('') || '<p class="settings-muted">还没有采集脚本。先准备目标数据表和连接器。</p>'}</div></section>` : ''}`;
  }

  function renderPageSettings(page) {
    const existing = context.publication.pages?.find((item) => item.id === page.id);
    const policy = existing || publicPageDefaults(page, context.tables);
    const available = (policy.reads || []).length > 0 && (policy.reads || []).every((read) => read.status_field && read.slug_field && read.published_value);
    const reads = (policy.reads || []).map((read) => {
      const table = context.tables.find((candidate) => candidate.slug === read.table);
      const imageFields = (table?.fields || []).filter((field) => field.type === 'file').map((field) => field.name);
      const imageField = imageFields.length ? `<label>明确公开图片字段（逗号分隔）<input class="input input-sm" name="images:${page.id}:${read.source}" value="${esc((read.images || []).join(', '))}" placeholder="${esc(imageFields.join(', '))}"></label>` : '';
      return `<div class="settings-read"><label>公开字段（逗号分隔）<input class="input input-sm" name="fields:${page.id}:${read.source}" value="${esc((read.fields || []).join(', '))}" placeholder="例如 title, slug, body"></label>${imageField}${input('状态字段', `status:${page.id}:${read.source}`, read.status_field)}${input('已发布值', `published:${page.id}:${read.source}`, read.published_value)}${input('Slug 字段', `slugfield:${page.id}:${read.source}`, read.slug_field)}${input('SEO 标题字段', `seotitle:${page.id}:${read.source}`, read.seo_title_field || '')}${input('SEO 描述字段', `seodesc:${page.id}:${read.source}`, read.seo_description_field || '')}</div>`;
    }).join('');
    return `<fieldset class="settings-subform"><legend>${esc(page.title)}</legend><label class="settings-check"><input type="checkbox" name="page:${page.id}" ${existing ? 'checked' : ''}> 包含此公开页面${available ? '' : '（缺少兼容的状态/slug 字段，不适合公开）'}</label>${input('页面标题', `title:${page.id}`, policy.title || page.title)}${reads || '<p>此页面尚无可授权的数据源。</p>'}</fieldset>`;
  }

  async function loadVersionDefinition() {
    if (!context.versions.published_version_id) return;
    const result = await api(appURL(`/versions/${encodeURIComponent(context.versions.published_version_id)}`));
    context.versionDefinition = result.definition;
  }

  async function open() { await load(false); await loadVersionDefinition(); render(); }

  async function submit(event) {
    const form = event.target.closest('[data-settings-form]');
    if (!form) return false;
    event.preventDefault();
    const data = new FormData(form);
    const kind = form.dataset.settingsForm;
    if (kind === 'publication') {
      const pages = (context.versionDefinition?.pages || []).filter((page) => data.get(`page:${page.id}`) === 'on').map((page) => {
        const prior = context.publication.pages?.find((item) => item.id === page.id) || publicPageDefaults(page, context.tables);
        const reads = (prior.reads || []).map((read) => {
          const output = { ...read, fields: String(data.get(`fields:${page.id}:${read.source}`) || '').split(',').map((part) => part.trim()).filter(Boolean), status_field: data.get(`status:${page.id}:${read.source}`) || '', published_value: data.get(`published:${page.id}:${read.source}`) || '', slug_field: data.get(`slugfield:${page.id}:${read.source}`) || '' };
          for (const [key, prefix] of [['seo_title_field', 'seotitle'], ['seo_description_field', 'seodesc']]) { const value = String(data.get(`${prefix}:${page.id}:${read.source}`) || '').trim(); if (value) output[key] = value; else delete output[key]; }
          const images = String(data.get(`images:${page.id}:${read.source}`) || '').split(',').map((part) => part.trim()).filter(Boolean); if (images.length) output.images = images; else output.images = [];
          return output;
        });
        if (!reads.length || reads.some((read) => !read.fields.length || !read.status_field || !read.published_value || !read.slug_field)) throw new Error(`页面「${page.title}」必须为每个数据源配置公开字段、状态字段、已发布值和 slug 字段。`);
        return { id: page.id, title: data.get(`title:${page.id}`) || page.title, reads };
      });
      const enabling = data.get('enabled') === 'on';
      if (enabling && !pages.length) { toast('公开访问至少需要选择一个完整配置的页面。', true); return true; }
      if (enabling && !window.confirm('确认启用匿名公开访问？访客无需登录即可读取以上选择的字段。')) return true;
      await api(appURL('/publication'), { method: 'PUT', body: JSON.stringify({ enabled: enabling, slug: data.get('slug'), pages, confirm: true }) });
      toast('公开配置已保存'); await open(); return true;
    }
    if (kind === 'connector') {
      const definition = JSON.parse(String(data.get('definition') || '{}'));
        const connector = context.connectors.find((item) => item.id === form.dataset.id);
        await api(appURL(`/connectors/${encodeURIComponent(form.dataset.id)}`), { method: 'PATCH', body: JSON.stringify({ name: data.get('name'), definition, expected_revision: Number(connector?.revision) || 0 }) });
      toast('连接器草稿已保存'); await open(); return true;
    }
    if (kind === 'script') {
      const definition = JSON.parse(String(data.get('definition') || '{}'));
      await api(appURL(`/collection-scripts/${encodeURIComponent(form.dataset.id)}`), { method: 'PATCH', body: JSON.stringify({ name: data.get('name'), definition, expected_revision: Number(form.dataset.revision) }) });
      toast('采集脚本草稿已保存'); await open(); return true;
    }
    return false;
  }

  async function click(event) {
    const button = event.target.closest('[data-settings-action]');
    if (!button) return false;
    const id = button.dataset.id, revision = Number(button.dataset.revision);
    switch (button.dataset.settingsAction) {
      case 'new-connector': {
        const name = window.prompt('连接器名称'); if (!name?.trim()) break;
        const base = window.prompt('HTTPS 主机地址（仅协议和主机，例如 https://example.org）'); if (!base?.trim()) break;
        await api(appURL('/connectors'), { method: 'POST', body: JSON.stringify({ name: name.trim(), definition: { type: 'https_fetch', base_url: base.trim(), allowed_paths: ['/'], max_bytes: 2097152 } }) }); await open(); break;
      }
      case 'enable-connector':
      case 'pause-connector':
        if (button.dataset.settingsAction === 'enable-connector' && !window.confirm('确认启用此连接器及其声明的外部读取权限？')) break;
        await api(appURL(`/connectors/${encodeURIComponent(id)}/enable`), { method: 'POST', body: JSON.stringify({ confirm: true, expected_revision: revision, enabled: button.dataset.settingsAction === 'enable-connector' }) }); await open(); break;
      case 'test-connector': {
        const connector = context.connectors.find((item) => item.id === id);
        const path = window.prompt('测试路径（必须匹配连接器允许的路径前缀）', '/');
        if (!path) break;
        const output = $(`[data-connector-output="${CSS.escape(id)}"]`);
        output.textContent = '正在发起受限 HTTPS 读取…';
        const result = await api(appURL(`/connectors/${encodeURIComponent(id)}/fetch`), { method: 'POST', body: JSON.stringify({ path, idempotency_key: `ui-${globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`}` }) });
        output.innerHTML = `<pre class="settings-result">${esc(pretty({ revision: connector?.revision, ...result }))}</pre>`;
        break;
      }
      case 'new-script': {
        if (!context.connectors.some((item) => item.status === 'enabled') || !context.tables.length) { toast('先启用连接器并创建目标数据表。', true); break; }
        const name = window.prompt('采集脚本名称'); if (!name?.trim()) break;
        const connector = context.connectors.find((item) => item.status === 'enabled');
        const table = context.tables.find((candidate) => candidate.fields.some((field) => field.required && ['text', 'number', 'bool', 'date', 'email', 'url', 'select'].includes(field.type)));
        if (!table) { toast('目标表需要至少一个可由来源映射满足的必填普通字段。', true); break; }
        const fields = table.fields.filter((field) => ['text', 'number', 'bool', 'date', 'email', 'url', 'select'].includes(field.type));
        const extracted = connector.definition.extract?.fields || {};
        const sourceNames = Object.keys(extracted);
        const mappingDefaults = Object.fromEntries(fields.filter((field) => extracted[field.name]).map((field) => [field.name, { from: field.name, type: field.type }]));
        if (!sourceNames.length) { toast('请先在连接器 JSON 中配置结构化 extract 字段，再创建采集脚本。', true); break; }
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
        const definition = { source: { connector_id: connector.id, path: '/', pagination: { max_pages: 1 } }, target: { table: table.slug, fields: mapping }, filters: [], dedup: { fields: [dedupField], on_change: 'update' }, recipients: context.members.slice(0, 1).map((member) => member.id), baseline: 'silent', schedule: { type: 'manual', timezone: 'Asia/Shanghai' } };
        if (!definition.recipients.length) { toast('工作区没有可接收通知的成员。', true); break; }
        try {
          await api(appURL('/collection-scripts'), { method: 'POST', body: JSON.stringify({ name: name.trim(), definition }) });
        } catch (error) {
          if (!connector.definition.extract) toast('连接器已启用，但请先在连接器声明 JSON 中配置 extract 格式和字段映射，再创建脚本。', true);
          else throw error;
          break;
        }
        await open(); break;
      }
      case 'preview-script':
        if (!window.confirm('只读试运行会读取外部来源并展示样本，不会写入业务数据。继续？')) break;
        { const result = await api(appURL(`/collection-scripts/${encodeURIComponent(id)}/preview`), { method: 'POST', body: JSON.stringify({ expected_revision: revision }) }); const output = $(`[data-script-output="${CSS.escape(id)}"]`); if (output) output.innerHTML = `<pre class="settings-result">${esc(pretty(result))}</pre>`; }
        break;
      case 'run-script':
        if (!window.confirm('立即运行会读取外部来源，并按此脚本配置写入业务记录和创建通知。继续？')) break;
        { const result = await api(appURL(`/collection-scripts/${encodeURIComponent(id)}/run`), { method: 'POST', body: JSON.stringify({ expected_revision: revision, confirm: true }) }); const output = $(`[data-script-output="${CSS.escape(id)}"]`); if (output) output.innerHTML = `<pre class="settings-result">${esc(pretty(result))}</pre>`; }
        break;
      case 'enable-script':
        if (!window.confirm('确认启用此版本？之后将按配置运行采集、写入目标表并向指定成员创建通知。')) break;
        await api(appURL(`/collection-scripts/${encodeURIComponent(id)}/enable`), { method: 'POST', body: JSON.stringify({ expected_revision: revision, confirm: true }) }); await open(); break;
      case 'pause-script':
        await api(appURL(`/collection-scripts/${encodeURIComponent(id)}/pause`), { method: 'POST', body: JSON.stringify({}) }); await open(); break;
      case 'runs-script':
        { const result = await api(appURL(`/collection-scripts/${encodeURIComponent(id)}/runs`)); const output = $(`[data-script-output="${CSS.escape(id)}"]`); if (output) output.innerHTML = `<pre class="settings-result">${esc(pretty(result.items || result))}</pre>`; }
        break;
    }
    return true;
  }

  return { open, submit, click };
}
