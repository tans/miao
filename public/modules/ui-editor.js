import { mount } from './ui-renderer.bundle.js';

const types = { Page: '页面', Section: '分组', Text: '说明文字', Metric: '统计', RecordTable: '记录表格', RecordCards: '记录卡片', RecordDetail: '记录详情', RecordForm: '新增表单' };
const propsFor = { Page: ['title'], Section: ['title'], Text: ['text'], Metric: ['label', 'value'], RecordTable: ['title', 'source'], RecordCards: ['title', 'source'], RecordDetail: ['title', 'source'], RecordForm: ['title', 'source'] };
const copy = (value) => structuredClone(value);
const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);

export function formatUIChanges(changes, esc) {
  const labels = { title: '应用标题', page_order: '页面顺序', add_page: '新增页面', remove_page: '删除页面', page_title: '页面标题', add_source: '新增数据绑定', remove_source: '删除数据绑定', source_collection: '绑定数据表', source_fields: '展示字段与顺序', source_form_fields: '表单字段与顺序', source_actions: '记录动作', source_query: '筛选与排序', root: '根组件', add_component: '新增组件', remove_component: '删除组件', component_type: '替换组件', component_props: '组件内容与绑定', component_order: '组件顺序与归属' };
  return changes.length ? `<ol class="ui-change-list">${changes.map((change) => `<li><strong>${esc(labels[change.type] || change.type)}</strong><span>${esc([change.page, change.resource].filter(Boolean).join(' / '))}</span><div><span>修改前</span><pre>${esc(JSON.stringify(change.before ?? null, null, 2))}</pre><span>修改后</span><pre>${esc(JSON.stringify(change.after ?? null, null, 2))}</pre></div></li>`).join('')}</ol>` : '<p>没有界面定义差异。</p>';
}

export function createUIEditor({ state, api, $, esc, onSaved }) {
  let session = null;
  let unmount = null;
  const active = () => session && state.app?.id === session.appId && state.tenant?.id === session.tenantId;
  const url = (path) => `/api/apps/${encodeURIComponent(session.appId)}${path}`;
  const page = () => session.definition.pages.find((item) => item.id === session.pageId);
  const button = (label, action, attrs = '') => `<button class="btn btn-ghost btn-sm" type="button" data-ui-edit="${action}" ${attrs}>${label}</button>`;
  const field = (label, name, value, attrs = '') => `<label>${label}<input class="input input-sm" name="${esc(name)}" value="${esc(value ?? '')}" ${attrs}></label>`;
  const select = (label, name, value, options) => `<label>${label}<select class="select select-sm" name="${esc(name)}">${options.map(([key, text]) => `<option value="${esc(key)}" ${key === value ? 'selected' : ''}>${esc(text)}</option>`).join('')}</select></label>`;

  function showError(error) {
    const root = $('#ui-editor-feedback');
    if (root) { root.textContent = error.message || String(error); root.hidden = false; root.focus(); }
  }

  function capture() {
    const form = $('#ui-editor-form');
    if (!form || !active()) return;
    const values = new FormData(form);
    const current = page();
    session.definition.title = values.get('app_title').trim();
    current.title = values.get('page_title').trim();
    for (const [id, element] of Object.entries(current.spec.elements)) {
      for (const key of propsFor[element.type]) {
        const value = values.get(`prop:${id}:${key}`);
        if (value !== null) element.props[key] = value;
      }
    }
    for (const source of current.data_sources) {
      const parseFields = (key) => String(values.get(`${key}:${source.id}`) || '').split(',').map((item) => item.trim()).filter(Boolean);
      source.fields = parseFields('fields');
      if (values.get(`custom_form:${source.id}`)) source.form_fields = parseFields('form_fields');
      else delete source.form_fields;
      const filters = JSON.parse(String(values.get(`filters:${source.id}`) || '[]'));
      if (!Array.isArray(filters)) throw new Error('筛选条件需要填写数组。编辑内容已保留，请修正后重试。');
      source.query = { filters, sort: values.get(`sort:${source.id}`) || '-created' };
    }
  }

  function descendants(id) {
    const elements = page().spec.elements, result = new Set();
    const walk = (key) => { if (result.has(key)) return; result.add(key); for (const child of elements[key]?.children || []) walk(child); };
    walk(id);
    return result;
  }

  function parentOf(id) {
    return Object.entries(page().spec.elements).find(([, element]) => (element.children || []).includes(id));
  }

  function localValidate() {
    const definition = session.definition;
    if (!definition.title || definition.title.length > 120) throw new Error('应用标题需要 1–120 个字符。');
    if (!definition.pages.length || definition.pages.length > 12) throw new Error('应用需要 1–12 个页面。');
    for (const current of definition.pages) {
      if (!current.title || current.title.length > 120) throw new Error('每个页面都需要 1–120 个字符的标题。');
      const sources = new Set(current.data_sources.map((source) => source.id));
      const used = new Set();
      const elements = current.spec.elements;
      const visited = new Set();
      const walk = (id) => {
        if (visited.has(id) || !elements[id]) throw new Error('组件包含循环、共享或失效引用。');
        visited.add(id);
        const element = elements[id];
        if (element.type.startsWith('Record')) { if (!sources.has(element.props.source)) throw new Error('请为记录组件选择有效数据绑定。'); used.add(element.props.source); }
        if ((element.children || []).length && !['Page', 'Section'].includes(element.type)) throw new Error('只有页面和分组可以容纳其他组件。');
        for (const child of element.children || []) walk(child);
      };
      walk(current.spec.root);
      if (visited.size !== Object.keys(elements).length || visited.size > 80) throw new Error('页面有未归属组件或超过 80 个组件。');
      if (used.size !== sources.size) throw new Error('每组数据绑定至少需要一个记录组件使用；请先调整组件。');
      for (const source of current.data_sources) {
        const table = session.tables.find((item) => item.slug === source.collection);
        const fields = table?.fields || [];
        for (const names of [source.fields, source.form_fields].filter(Boolean)) {
          if (names.length > 24 || new Set(names).size !== names.length || names.some((name) => !fields.some((field) => field.name === name))) throw new Error(`「${current.title}」有重复或无效字段，请参考可用字段。`);
        }
        if (!source.fields.length) throw new Error('每个数据绑定至少需要一个展示字段。');
      }
    }
  }

  function render() {
    if (!active()) return;
    unmount?.(); unmount = null;
    const current = page(), elements = current.spec.elements;
    const containers = Object.entries(elements).filter(([, item]) => ['Page', 'Section'].includes(item.type)).map(([id, item]) => [id, `${types[item.type]} · ${typeof item.props.title === 'string' ? item.props.title : id} (${id})`]);
    const renderElement = (id, depth = 0) => {
      const element = elements[id], isRoot = id === current.spec.root, parent = parentOf(id);
      const compatible = Object.entries(types).filter(([type]) => !element.children?.length || ['Page', 'Section'].includes(type));
      return `<article class="ui-editor-element" data-depth="${Math.min(depth, 3)}"><div class="ui-editor-element-heading"><strong>${esc(types[element.type])} · ${esc(id)}</strong><div>${isRoot ? '' : `${button('上移', 'up', `data-id="${esc(id)}"`)}${button('下移', 'down', `data-id="${esc(id)}"`)}${button('删除', 'remove', `data-id="${esc(id)}"`)}`}</div></div>
        <div class="ui-editor-props">${select('组件类型', `type:${id}`, element.type, compatible)}
        ${propsFor[element.type].map((key) => key === 'source' ? select('绑定数据', `prop:${id}:source`, element.props.source, current.data_sources.map((source) => [source.id, `${source.collection} (${source.id})`])) : typeof element.props[key] === 'object' ? `<label>${esc(key)}<input class="input input-sm" disabled value="${esc(JSON.stringify(element.props[key]))}"><small>已有动态绑定会保留。</small></label>` : field({ title: '标题', text: '说明', label: '统计标签', value: '统计值' }[key], `prop:${id}:${key}`, element.props[key])).join('')}
        ${isRoot ? '' : select('归属分组', `parent:${id}`, parent?.[0] || '', containers.filter(([key]) => !descendants(id).has(key)))}</div></article>${(element.children || []).map((child) => renderElement(child, depth + 1)).join('')}`;
    };
    $('#ui-editor-root').innerHTML = `<header class="ui-editor-heading"><div><h3>编辑界面 · 基于 v${esc(session.version.version)}</h3><p>调整页面与组件，保存为新草稿后再预览发布。</p></div>${button('收起编辑', 'close')}</header><p id="ui-editor-feedback" class="alert alert-error" role="alert" tabindex="-1" hidden></p>
      <form id="ui-editor-form"><div class="ui-editor-layout"><aside class="ui-editor-pages"><h4>页面</h4>${session.definition.pages.map((item) => `<button class="btn btn-sm ${item.id === current.id ? 'btn-active' : 'btn-ghost'}" type="button" data-ui-edit="page" data-id="${esc(item.id)}">${esc(item.title)}</button>`).join('')}${button('新增页面', 'add-page')}</aside>
      <div class="ui-editor-content"><div class="ui-editor-props">${field('应用标题', 'app_title', session.definition.title, 'required maxlength="120"')}${field('当前页面标题', 'page_title', current.title, 'required maxlength="120"')}</div>
      <div class="ui-editor-page-actions">${button('页面前移', 'page-up')}${button('页面后移', 'page-down')}${button('删除页面', 'remove-page')}</div>
      <section><h4>数据与字段</h4>${current.data_sources.map((source) => {
        const table = session.tables.find((item) => item.slug === source.collection), fields = table?.fields || [];
        return `<div class="ui-editor-source"><strong>${esc(table?.name || source.collection)} · ${esc(source.id)}</strong><p>可用字段：${esc(fields.map((item) => `${item.label || item.name} (${item.name})`).join('、'))}</p>${field('展示字段（按顺序，逗号分隔）', `fields:${source.id}`, source.fields.join(', '))}
          <label class="ui-editor-check"><input class="checkbox checkbox-sm" type="checkbox" name="custom_form:${esc(source.id)}" ${source.form_fields ? 'checked' : ''}> 自定义新增表单字段</label>${field('表单字段（按顺序，逗号分隔）', `form_fields:${source.id}`, (source.form_fields || fields.map((item) => item.name)).join(', '))}
          <div class="ui-editor-props">${select('排序', `sort:${source.id}`, source.query?.sort || '-created', [['-created', '创建时间降序'], ['created', '创建时间升序'], ['-updated', '修改时间降序'], ['updated', '修改时间升序'], ...fields.filter((item) => !['file', 'member', 'relation'].includes(item.type)).flatMap((item) => [[item.name, `${item.label || item.name} 升序`], [`-${item.name}`, `${item.label || item.name} 降序`]])])}
          <label>筛选条件（JSON 数组）<textarea class="textarea textarea-sm" name="filters:${esc(source.id)}" rows="3">${esc(JSON.stringify(source.query?.filters || [], null, 2))}</textarea><small>最多 8 项，示例：${esc('[{"field":"status","op":"eq","value":"已发布"}]')}。支持 eq、neq，文本支持 contains。</small></label></div></div>`;
      }).join('')}</section><section><h4>组件与顺序</h4>${renderElement(current.spec.root)}<div class="ui-editor-props">${select('添加组件', 'new_type', 'Text', Object.entries(types).filter(([key]) => key !== 'Page'))}${select('添加到', 'new_parent', current.spec.root, containers)}${button('添加组件', 'add')}</div></section>
      <div class="ui-editor-actions"><button class="btn btn-primary btn-sm" type="submit">保存新草稿</button>${button('本地预览', 'preview')}<span>尚未发布，业务记录保持原样。</span></div><div id="ui-editor-local-preview"></div></div></div></form>`;
  }

  async function open(versionId) {
    if (session && active()) { try { capture(); } catch (error) { showError(error); return; } if (!same(session.original, session.definition) && !window.confirm('放弃尚未保存的界面编辑并重新载入？')) return; }
    const appId = state.app.id, tenantId = state.tenant.id;
    const root = $('#app-runtime-root');
    let host = $('#ui-editor-root');
    if (!host) { host = document.createElement('section'); host.id = 'ui-editor-root'; root.append(host); }
    host.innerHTML = '<p class="runtime-loading">正在载入界面定义…</p>';
    try {
      const [versions, tables] = await Promise.all([api(`/api/apps/${encodeURIComponent(appId)}/versions`), api(`/api/apps/${encodeURIComponent(appId)}/collections`)]);
      const id = versionId || versions.items?.find((item) => item.status === 'draft')?.id || versions.published_version_id;
      if (!id) throw new Error('请先用小助手生成一份界面草稿。');
      const version = await api(`/api/apps/${encodeURIComponent(appId)}/versions/${encodeURIComponent(id)}`);
      if (state.app?.id !== appId || state.tenant?.id !== tenantId || !host.isConnected) return;
      session = { appId, tenantId, tables, version, latestId: versions.items?.[0]?.id || null, publishedId: versions.published_version_id || null, original: copy(version.definition), definition: copy(version.definition), pageId: version.definition.pages[0].id };
      render(); host.scrollIntoView({ block: 'start', behavior: 'smooth' });
    } catch (error) { if (host.isConnected) host.innerHTML = `<p class="alert alert-error" role="alert">${esc(error.message)}</p>${button('重试载入', 'reload')}`; }
  }

  function change(event) {
    if (!active() || !event.target.closest('#ui-editor-form')) return false;
    const target = event.target;
    if (!target.name?.startsWith('type:') && !target.name?.startsWith('parent:')) return false;
    try {
      capture();
      const [kind, id] = target.name.split(':'), elements = page().spec.elements;
      if (kind === 'type') {
        const element = elements[id], typ = target.value;
        if (element.children?.length && !['Page', 'Section'].includes(typ)) throw new Error('先移动分组中的组件，再替换组件类型。');
        const props = {};
        for (const key of propsFor[typ]) props[key] = element.props[key] ?? (key === 'source' ? page().data_sources[0]?.id : key === 'title' ? types[typ] : '');
        element.type = typ; element.props = props;
      } else {
        if (descendants(id).has(target.value)) throw new Error('组件不能移到自身或自己的子组件中。');
        const parent = parentOf(id);
        if (!parent || !['Page', 'Section'].includes(elements[target.value]?.type)) throw new Error('请选择有效的归属分组。');
        parent[1].children = parent[1].children.filter((key) => key !== id);
        elements[target.value].children.push(id);
      }
      render();
    } catch (error) { showError(error); }
    return true;
  }

  async function click(event) {
    const edit = event.target.closest('[data-edit-ui-version]');
    if (edit) { await open(edit.dataset.editUiVersion); return true; }
    const control = event.target.closest('[data-ui-edit]');
    if (!control) return false;
    if (control.dataset.uiEdit === 'reload') { await open(); return true; }
    if (!active()) return true;
    try {
      capture();
      const current = page(), elements = current.spec.elements, action = control.dataset.uiEdit, id = control.dataset.id;
      if (action === 'close') { if (!same(session.original, session.definition) && !window.confirm('放弃尚未保存的界面编辑？')) return true; unmount?.(); session = null; $('#ui-editor-root')?.remove(); return true; }
      if (action === 'page') session.pageId = id;
      if (action === 'page-up' || action === 'page-down') {
        const pages = session.definition.pages, index = pages.indexOf(current), next = index + (action === 'page-up' ? -1 : 1);
        if (next >= 0 && next < pages.length) [pages[index], pages[next]] = [pages[next], pages[index]];
      }
      if (action === 'remove-page') { if (session.definition.pages.length === 1) throw new Error('至少保留一个页面。'); if (!window.confirm(`删除「${current.title}」页面？保存和发布后才影响正式界面。`)) return true; session.definition.pages = session.definition.pages.filter((item) => item !== current); session.pageId = session.definition.pages[0].id; }
      if (action === 'add-page') {
        if (session.definition.pages.length >= 12) throw new Error('最多 12 个页面。');
        const available = session.tables.filter((table) => table.fields?.length);
        if (!available.length) throw new Error('请先创建至少一个数据表。');
        const slug = window.prompt(`新页面绑定的数据表标识：${available.map((table) => table.slug).join('、')}`, available[0].slug);
        if (slug === null) return true;
        const table = available.find((item) => item.slug === slug.trim());
        if (!table) throw new Error('请选择当前应用的数据表标识。');
        const key = `page_${crypto.randomUUID().replaceAll('-', '').slice(0, 16)}`;
        session.definition.pages.push({ id: key, title: table.name, data_sources: [{ id: 'records', collection: table.slug, fields: table.fields.slice(0, 24).map((item) => item.name), actions: [] }], spec: { root: 'page', elements: { page: { type: 'Page', props: { title: table.name }, children: ['records'] }, records: { type: 'RecordTable', props: { title: table.name, source: 'records' }, children: [] } } } });
        session.pageId = key;
      }
      if (action === 'add') {
        if (Object.keys(elements).length >= 80) throw new Error('当前页面最多 80 个组件。');
        const values = new FormData($('#ui-editor-form')), typ = values.get('new_type'), parent = values.get('new_parent');
        if (!['Page', 'Section'].includes(elements[parent]?.type)) throw new Error('请先选择有效的分组。');
        const key = `element_${crypto.randomUUID().replaceAll('-', '').slice(0, 16)}`, props = {};
        for (const name of propsFor[typ]) props[name] = name === 'source' ? current.data_sources[0].id : name === 'value' ? '0' : types[typ];
        elements[key] = { type: typ, props, children: [] }; elements[parent].children.push(key);
      }
      if (action === 'remove') { const parent = parentOf(id); if (!parent) throw new Error('根组件必须保留。'); if (!window.confirm('删除此组件及其中的组件？保存和发布后才影响正式界面。')) return true; parent[1].children = parent[1].children.filter((key) => key !== id); for (const key of descendants(id)) delete elements[key]; }
      if (action === 'up' || action === 'down') { const parent = parentOf(id); if (parent) { const siblings = parent[1].children, index = siblings.indexOf(id), next = index + (action === 'up' ? -1 : 1); if (next >= 0 && next < siblings.length) [siblings[index], siblings[next]] = [siblings[next], siblings[index]]; } }
      if (action === 'preview') {
        localValidate();
        const requested = session;
        const preview = await api(url('/versions/preview'), { method: 'POST', body: JSON.stringify({ definition: copy(session.definition), ui_page: current.id }) });
        if (!active() || session !== requested) return true;
        const host = $('#ui-editor-local-preview');
        unmount?.(); host.innerHTML = `<details class="preview-change-list" open><summary>未保存的界面差异 · ${preview.changes.length} 项</summary>${formatUIChanges(preview.changes, esc)}</details><div class="json-render-preview-host"></div>`; unmount = mount(host.querySelector('.json-render-preview-host'), current.spec, { sources: preview.sources || {}, members: preview.members || [], readOnly: true });
        return true;
      }
      render();
    } catch (error) { showError(error); }
    return true;
  }

  async function submit(event) {
    if (event.target.id !== 'ui-editor-form') return false;
    event.preventDefault();
    if (!active()) return true;
    const form = event.target, submitButton = form.querySelector('button[type="submit"]');
    if (submitButton.disabled) return true;
    try {
      capture(); localValidate();
      submitButton.disabled = true; submitButton.textContent = '正在保存草稿…';
      const requested = session;
      const payload = { definition: copy(session.definition), based_on_version_id: session.version.id, expected_latest_version_id: session.latestId, expected_published_version_id: session.publishedId, summary: '编辑页面、组件及数据绑定' };
      await api(url('/versions'), { method: 'POST', body: JSON.stringify(payload) });
      if (!active() || session !== requested) return true;
      unmount?.(); session = null; $('#ui-editor-root')?.remove();
      await onSaved();
    } catch (error) { showError(error); } finally { submitButton.disabled = false; submitButton.textContent = '保存新草稿'; }
    return true;
  }

  return { open, click, change, submit };
}
