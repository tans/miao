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
    const [publication, scripts, tables, members, actions, workflows] = await Promise.all([
      canManage ? api(url('/publication')) : {}, api(url('/collection-scripts')),
      api(url('/collections')), api(url('/members')),
      canManage ? api(url('/actions')) : [], canManage ? api(url('/workflows')) : []
    ]);
    if (!current(requested) || revision !== loadRevision) return false;
    context = { appId, tenantId, userId, publication, scripts, tables, members: members.members || [], actions, workflows };
    if (renderAfter) render();
    return true;
  }

  async function open() {
    if (!await load(false)) return;
    render();
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
      case 'retry-script-notifications': {
        const scriptId = button.closest('[data-collection-script]')?.dataset.collectionScript;
        if (!scriptId || !window.confirm('只重试通知投递，不会重新抓取或重复写入业务记录。继续？')) break;
        await api(appURL(`/collection-scripts/${encodeURIComponent(scriptId)}/runs/${encodeURIComponent(button.dataset.run)}/retry-notifications`), { method: 'POST', body: JSON.stringify({ confirm: true }) });
        await showCollectionRun(scriptId, button.dataset.run, requested); break;
      }
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
