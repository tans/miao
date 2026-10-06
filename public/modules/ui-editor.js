import { mount } from './ui-renderer.bundle.js';

const types = { Page: '页面', Section: '分组', Text: '说明文字', Metric: '统计', RecordTable: '记录表格', RecordCards: '记录卡片', RecordDetail: '记录详情', RecordForm: '新增表单' };
const propsFor = { Page: ['title'], Section: ['title'], Text: ['text'], Metric: ['label', 'value'], RecordTable: ['title', 'source'], RecordCards: ['title', 'source'], RecordDetail: ['title', 'source'], RecordForm: ['title', 'source'] };
const copy = (value) => structuredClone(value);
const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);

export function formatUIChanges(changes, esc) {
  const labels = { title: '应用标题', page_order: '页面顺序', add_page: '新增页面', remove_page: '删除页面', page_title: '页面标题', add_source: '新增数据绑定', remove_source: '删除数据绑定', source_collection: '绑定数据表', source_fields: '展示字段与顺序', source_form_fields: '表单字段与顺序', source_actions: '记录动作', source_query: '筛选与排序', source_context: '当前详情关联范围', root: '根组件', add_component: '新增组件', remove_component: '删除组件', component_type: '替换组件', component_props: '组件内容与绑定', component_order: '组件顺序与归属' };
  return changes.length ? `<ol class="ui-change-list">${changes.map((change) => `<li><strong>${esc(labels[change.type] || change.type)}</strong><span>${esc([change.page, change.resource].filter(Boolean).join(' / '))}</span><div><span>修改前</span><pre>${esc(JSON.stringify(change.before ?? null, null, 2))}</pre><span>修改后</span><pre>${esc(JSON.stringify(change.after ?? null, null, 2))}</pre></div></li>`).join('')}</ol>` : '<p>没有界面定义差异。</p>';
}

export function createUIEditor({ state, api, $, esc, onSaved, onModelRequest }) {
  let session = null;
  let unmount = null;
  const active = () => session && state.app?.id === session.appId && state.tenant?.id === session.tenantId;
  const url = (path) => `/api/apps/${encodeURIComponent(session.appId)}${path}`;
  const page = () => session.definition.pages.find((item) => item.id === session.pageId);
  const button = (label, action, attrs = '') => `<button class="btn btn-ghost btn-sm" type="button" data-ui-edit="${action}" ${attrs}>${label}</button>`;
  const field = (label, name, value, attrs = '') => `<label>${label}<input class="input input-sm" name="${esc(name)}" value="${esc(value ?? '')}" ${attrs}></label>`;
  const select = (label, name, value, options) => `<label>${label}<select class="select select-sm" name="${esc(name)}">${options.map(([key, text]) => `<option value="${esc(key)}" ${key === value ? 'selected' : ''}>${esc(text)}</option>`).join('')}</select></label>`;
  const workflowBindings = (collection) => session.workflows.filter((item) => item.status === 'enabled' && item.definition.table === collection).flatMap((item) => item.definition.transitions.map((transition) => ({ workflow: item, transition })));
  const primarySource = (current) => {
    const queue = [current.spec.root], seen = new Set();
    while (queue.length) {
      const id = queue.shift();
      if (seen.has(id)) continue;
      seen.add(id);
      const element = current.spec.elements[id];
      if (element?.type === 'RecordDetail') return current.data_sources.find((source) => source.id === element.props.source);
      queue.push(...(element?.children || []));
    }
    return null;
  };

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
      source.actions = JSON.parse(String(values.get(`actions:${source.id}`) || '[]'));
      if (!Array.isArray(source.actions) || source.actions.length > 24) throw new Error('记录动作需要数组，最多 24 项；当前编辑已保留。');
      const contextField = values.get(`context:${source.id}`), primary = primarySource(current);
      if (contextField) source.context = { source: primary?.id || source.context?.source || '', field: contextField };
      else delete source.context;
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
        if (source.context) {
          const primary = primarySource(current);
          const relation = fields.find((field) => field.name === source.context.field);
          if (!primary || primary === source || primary.context || source.context.source !== primary.id || relation?.type !== 'relation' || relation.target !== primary.collection) throw new Error('关联数据源必须通过真实 relation 字段绑定当前页的主记录详情。请调整上下文或清除绑定。');
        }
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
        ${propsFor[element.type].map((key) => key === 'source' ? select('绑定数据', `prop:${id}:source`, element.props.source, current.data_sources.map((source) => [source.id, `${source.collection} (${source.id})`])) : typeof element.props[key] === 'object' ? `<label>${esc(key)}<input class="input input-sm" disabled value="${esc(JSON.stringify(element.props[key]))}"></label>` : field({ title: '标题', text: '说明', label: '统计标签', value: '统计值' }[key], `prop:${id}:${key}`, element.props[key])).join('')}
        ${isRoot ? '' : select('归属分组', `parent:${id}`, parent?.[0] || '', containers.filter(([key]) => !descendants(id).has(key)))}</div></article>${(element.children || []).map((child) => renderElement(child, depth + 1)).join('')}`;
    };
    $('#ui-editor-root').innerHTML = `<header class="ui-editor-heading"><div><h3>编辑界面 · 基于 v${esc(session.version.version)}</h3></div>${button('收起编辑', 'close')}</header><p id="ui-editor-feedback" class="alert alert-error" role="alert" tabindex="-1" hidden></p>
      <form id="ui-editor-form"><div class="ui-editor-layout"><aside class="ui-editor-pages"><h4>页面</h4>${session.definition.pages.map((item) => `<button class="btn btn-sm ${item.id === current.id ? 'btn-active' : 'btn-ghost'}" type="button" data-ui-edit="page" data-id="${esc(item.id)}">${esc(item.title)}</button>`).join('')}${button('新增页面', 'add-page')}</aside>
      <div class="ui-editor-content"><div class="ui-editor-props">${field('应用标题', 'app_title', session.definition.title, 'required maxlength="120"')}${field('当前页面标题', 'page_title', current.title, 'required maxlength="120"')}</div>
      <div class="ui-editor-page-actions">${button('页面前移', 'page-up')}${button('页面后移', 'page-down')}${button('删除页面', 'remove-page')}</div>
      <section><h4>数据与字段</h4>${current.data_sources.map((source) => {
        const table = session.tables.find((item) => item.slug === source.collection), fields = table?.fields || [];
        const primary = primarySource(current), relations = fields.filter((field) => primary && primary !== source && field.type === 'relation' && field.target === primary.collection);
        const contexts = [['', '不按当前详情过滤'], ...relations.map((field) => [field.name, `${field.label || field.name} = 当前 ${primary.collection} 记录`])];
        if (source.context && !relations.some((field) => field.name === source.context.field)) contexts.push([source.context.field, '现有上下文已失效，请调整绑定']);
        return `<div class="ui-editor-source"><strong>${esc(table?.name || source.collection)} · ${esc(source.id)}</strong>${current.data_sources.length > 1 ? button('移除数据绑定', 'remove-source', `data-id="${esc(source.id)}"`) : ''}<p>可用字段：${esc(fields.map((item) => `${item.label || item.name} (${item.name})`).join('、'))}</p>${select('关联详情范围', `context:${source.id}`, source.context?.field || '', contexts)}${field('展示字段（按顺序，逗号分隔）', `fields:${source.id}`, source.fields.join(', '))}
          <label class="ui-editor-check"><input class="checkbox checkbox-sm" type="checkbox" name="custom_form:${esc(source.id)}" ${source.form_fields ? 'checked' : ''}> 自定义新增表单字段</label>${field('表单字段（按顺序，逗号分隔）', `form_fields:${source.id}`, (source.form_fields || fields.map((item) => item.name)).join(', '))}
          <div class="ui-editor-props">${select('排序', `sort:${source.id}`, source.query?.sort || '-created', [['-created', '创建时间降序'], ['created', '创建时间升序'], ['-updated', '修改时间降序'], ['updated', '修改时间升序'], ...fields.filter((item) => !['file', 'member', 'relation'].includes(item.type)).flatMap((item) => [[item.name, `${item.label || item.name} 升序`], [`-${item.name}`, `${item.label || item.name} 降序`]])])}
          <label>筛选条件（JSON 数组）<textarea class="textarea textarea-sm" name="filters:${esc(source.id)}" rows="3">${esc(JSON.stringify(source.query?.filters || [], null, 2))}</textarea></label></div>
          <details><summary>记录动作 · ${source.actions?.length || 0} 项</summary><label>动作定义（JSON 数组）<textarea class="textarea textarea-sm" name="actions:${esc(source.id)}" rows="5">${esc(JSON.stringify(source.actions || [], null, 2))}</textarea></label>
          <div class="ui-editor-props">${select('已启用业务动作', `action_binding:${source.id}`, '', [['', '选择业务动作'], ...session.actions.filter((item) => item.status === 'enabled').map((item) => [item.id, `${item.name} · v${item.revision}`])])}${button('绑定业务动作', 'bind-action', `data-id="${esc(source.id)}"`)}
          ${select('当前表状态转换', `workflow_binding:${source.id}`, '', [['', '选择状态转换'], ...workflowBindings(source.collection).map((item, index) => [String(index), `${item.workflow.name} · ${item.transition.label} (${item.transition.from} → ${item.transition.to})`])])}${button('绑定状态转换', 'bind-workflow', `data-id="${esc(source.id)}"`)}</div></details></div>`;
      }).join('')}<div class="ui-editor-props">${select('新的数据绑定', 'new_source_table', '', [['', '选择当前应用数据表'], ...session.tables.map((table) => [table.slug, table.name])])}${button('添加数据绑定及列表', 'add-source')}</div></section><section><h4>组件与顺序</h4>${renderElement(current.spec.root)}<div class="ui-editor-props">${select('添加组件', 'new_type', 'Text', Object.entries(types).filter(([key]) => key !== 'Page'))}${select('添加到', 'new_parent', current.spec.root, containers)}${button('添加组件', 'add')}</div></section>
      <div class="ui-editor-actions"><button class="btn btn-primary btn-sm" type="submit">保存新草稿</button>${button('本地预览', 'preview')}${onModelRequest ? button('描述界面修改', 'model-edit') : ''}</div><div id="ui-editor-local-preview"></div></div></div></form>`;
  }

  async function open(versionId) {
    if (session && active()) { try { capture(); } catch (error) { showError(error); return; } if (!same(session.original, session.definition) && !window.confirm('放弃尚未保存的界面编辑并重新载入？')) return; }
    const appId = state.app.id, tenantId = state.tenant.id;
    const root = $('#app-runtime-root');
    let host = $('#ui-editor-root');
    if (!host) { host = document.createElement('section'); host.id = 'ui-editor-root'; root.append(host); }
    host.innerHTML = '<p class="runtime-loading">正在载入界面定义…</p>';
    try {
      const [versions, tables, actions, workflows] = await Promise.all([api(`/api/apps/${encodeURIComponent(appId)}/versions`), api(`/api/apps/${encodeURIComponent(appId)}/collections`), api(`/api/apps/${encodeURIComponent(appId)}/actions`), api(`/api/apps/${encodeURIComponent(appId)}/workflows`)]);
      const id = versionId || versions.items?.find((item) => item.status === 'draft')?.id || versions.published_version_id;
      if (!id) throw new Error('请先用小助手生成一份界面草稿。');
      const version = await api(`/api/apps/${encodeURIComponent(appId)}/versions/${encodeURIComponent(id)}`);
      if (state.app?.id !== appId || state.tenant?.id !== tenantId || !host.isConnected) return;
      session = { appId, tenantId, tables, actions, workflows, version, latestId: versions.items?.[0]?.id || null, publishedId: versions.published_version_id || null, original: copy(version.definition), definition: copy(version.definition), pageId: version.definition.pages[0].id };
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
      if (action === 'add-source') {
        if (current.data_sources.length >= 12 || Object.keys(elements).length >= 80) throw new Error('当前页面的数据源或组件数量已达上限。');
        const table = session.tables.find((item) => item.slug === new FormData($('#ui-editor-form')).get('new_source_table'));
        if (!table?.fields.length) throw new Error('请选择有字段的当前应用数据表。');
        const root = elements[current.spec.root];
        if (!['Page', 'Section'].includes(root.type)) throw new Error('请先将根组件设置为能容纳列表的页面或分组。');
        const key = `source_${crypto.randomUUID().replaceAll('-', '').slice(0, 16)}`;
        current.data_sources.push({ id: key, collection: table.slug, fields: table.fields.slice(0, 24).map((field) => field.name), actions: [] });
        elements[key] = { type: 'RecordCards', props: { title: table.name, source: key }, children: [] }; root.children.push(key);
      }
      if (action === 'remove-source') {
        if (current.data_sources.length === 1) throw new Error('每个页面至少保留一个数据绑定。');
        if (current.data_sources.some((source) => source.context?.source === id)) throw new Error('先清除其他数据源对这个主记录的关联上下文，再移除绑定。');
        const matching = Object.entries(elements).filter(([, element]) => element.type.startsWith('Record') && element.props.source === id);
        if (matching.some(([key]) => key === current.spec.root)) throw new Error('根组件正在使用此绑定，请先调整根组件。');
        if (!window.confirm('移除数据绑定及使用它的记录组件？保存发布后才影响界面，业务记录会保留。')) return true;
        for (const [key] of matching) {
          const parent = parentOf(key);
          if (parent) parent[1].children = parent[1].children.filter((child) => child !== key);
          delete elements[key];
        }
        current.data_sources = current.data_sources.filter((source) => source.id !== id);
      }
      if (action === 'bind-action' || action === 'bind-workflow') {
        const source = current.data_sources.find((item) => item.id === id);
        if (!source || source.actions.length >= 24) throw new Error('数据绑定不存在或动作已达 24 项上限。');
        const values = new FormData($('#ui-editor-form'));
        const binding = { id: `action_${crypto.randomUUID().replaceAll('-', '').slice(0, 16)}` };
        if (action === 'bind-action') {
          const selected = session.actions.find((item) => item.id === values.get(`action_binding:${id}`) && item.status === 'enabled');
          if (!selected) throw new Error('请先在业务配置中启用动作，再选择绑定。');
          Object.assign(binding, { label: selected.name, action_id: selected.id, action_revision: selected.revision });
        } else {
          const index = values.get(`workflow_binding:${id}`);
          const selected = index !== '' ? workflowBindings(source.collection)[Number(index)] : null;
          if (!selected) throw new Error('请选择当前表已启用的具体状态转换。');
          Object.assign(binding, { label: selected.transition.label, workflow_id: selected.workflow.id, workflow_revision: selected.workflow.revision, transition_id: selected.transition.id });
        }
        source.actions.push(binding);
      }
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
      if (action === 'model-edit') {
        localValidate();
        const prompt = window.prompt('描述想修改的页面、内容或布局。模型只整理可审阅的草稿，不会自动发布。');
        if (!prompt?.trim()) return true;
        await onModelRequest({ app_id:session.appId,prompt:prompt.trim(),context:{mode:'ui_edit'},initial_definition:copy(session.definition),based_on_version_id:session.version.id,expected_latest_version_id:session.latestId,expected_published_version_id:session.publishedId });
        return true;
      }
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
