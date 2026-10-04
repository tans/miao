import { mount as mountJsonRenderer } from './ui-renderer.bundle.js';

export function createAppRuntime({ state, api, $, esc }) {
  let unmountRenderer = null;
  const supportsDataManagement = () => Array.isArray(state.app?.capabilities) && state.app.capabilities.includes('data_management');

  function randomNonce() {
    const bytes = crypto.getRandomValues(new Uint8Array(24));
    return [...bytes].map((value) => value.toString(16).padStart(2, '0')).join('');
  }

  function escapeScript(value) {
    return String(value).replace(/<\/script/gi, '<\\/script').replace(/<!--/g, '<\\!--');
  }

  function appDocument(runtime, preview = false, routePath = '/') {
    const files = runtime.source || {};
    const manifest = runtime.manifest || {};
    const route = (manifest.routes || []).find((item) => item.path === routePath);
    const entry = files[route?.file || manifest.entry] || files['index.html'] || '';
    const nonce = randomNonce();
    const session = { nonce, appId: state.app.id, versionId: runtime.version.id, preview, frame: null, manifest, actionIDs: new Set(manifest.actions || []), capabilities: new Set(runtime.capabilities || []) };
    appFrameSessions.set(nonce, session);
    const style = files['styles.css'] ? `<style>${files['styles.css']}</style>` : '';
    const script = files['app.js'] || '';
    const bridge = `(function(){const nonce=${JSON.stringify(nonce)},appId=${JSON.stringify(session.appId)},versionId=${JSON.stringify(session.versionId)};let next=0;const pending=new Map();function call(action,args){return new Promise((resolve,reject)=>{const id=++next;pending.set(id,{resolve,reject});parent.postMessage({protocol:'miao-app-v1',nonce,appId,versionId,id,action,args},'*')})}window.miao={version:'1',query:(table,options={})=>call('records.read',{table,...options}),get:(table,id)=>call('records.get',{table,id}),create:(table,data)=>call('records.create',{table,data}),update:(table,id,data,expected_updated_at)=>call('records.update',{table,id,data,expected_updated_at}),remove:(table,id)=>call('records.delete',{table,id}),execute:(action_id,idempotency_key,input={})=>call('actions.execute',{action_id,idempotency_key,input}),navigate:path=>call('navigation.go',{path}),user:()=>call('user.read',{})};addEventListener('message',event=>{const m=event.data;if(!m||m.protocol!=='miao-app-v1-result'||m.nonce!==nonce)return;const p=pending.get(m.id);if(!p)return;pending.delete(m.id);m.ok?p.resolve(m.result):p.reject(new Error(m.error||'MIAO action failed'))});parent.postMessage({protocol:'miao-app-v1-ready',nonce,appId,versionId},'*')})();`;
    const content = entry
      .replace(/<head([^>]*)>/i, `<head$1>${style}`)
      .replace(/<script\b[^>]*src=["'][^"']*app\.js["'][^>]*><\/script>/i, '')
      .replace(/<\/body>/i, `<script>${escapeScript(bridge + script)}</script></body>`);
    const csp = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; connect-src 'none'; object-src 'none'; form-action 'none'; base-uri 'none'; frame-src 'none'; navigate-to 'none'";
    return { nonce, session, srcdoc: `<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="${csp}"><meta name="viewport" content="width=device-width,initial-scale=1">${style}</head><body>${content}</body></html>` };
  }

  async function handleAppMessage(event) {
    const message = event.data;
    if (!message || typeof message !== 'object' || typeof message.nonce !== 'string') return;
    const session = appFrameSessions.get(message.nonce);
    if (!session || event.source !== session.frame?.contentWindow || message.appId !== session.appId || message.versionId !== session.versionId) return;
    if (message.protocol === 'miao-app-v1-ready') return;
    if (message.protocol !== 'miao-app-v1') return;
    if (!Number.isSafeInteger(message.id)) return;
    const reply = (ok, result, error = '') => session.frame.contentWindow.postMessage({ protocol: 'miao-app-v1-result', nonce: session.nonce, id: message.id, ok, result, error }, '*');
    if (session.preview && message.action !== 'records.read' && message.action !== 'records.get' && message.action !== 'user.read') return reply(false, null, '草稿预览为只读');
    const args = message.args && typeof message.args === 'object' && !Array.isArray(message.args) ? message.args : {};
    const table = typeof args.table === 'string' ? args.table : '';
    try {
      let result;
      switch (message.action) {
        case 'records.read': {
          if (!session.capabilities.has('records.read')) throw new Error('未声明 records.read 能力');
          if (!/^[a-z][a-z0-9_]{0,39}$/.test(table)) throw new Error('数据表标识无效');
          const params = new URLSearchParams();
          for (const key of ['page', 'perPage', 'search', 'sort']) if (args[key] !== undefined) params.set(key, String(args[key]));
          result = await api(`/api/apps/${encodeURIComponent(session.appId)}/collections/${encodeURIComponent(table)}/records?${params}`);
          break;
        }
        case 'records.get':
          if (!session.capabilities.has('records.read')) throw new Error('未声明 records.read 能力');
          result = await api(`/api/apps/${encodeURIComponent(session.appId)}/collections/${encodeURIComponent(table)}/records/${encodeURIComponent(String(args.id || ''))}`);
          break;
        case 'records.create':
          if (!session.capabilities.has('records.create')) throw new Error('未声明 records.create 能力');
          if (session.preview) throw new Error('草稿预览为只读');
          result = await api(`/api/apps/${encodeURIComponent(session.appId)}/collections/${encodeURIComponent(table)}/records`, { method: 'POST', body: JSON.stringify({ data: args.data }) });
          break;
        case 'records.update':
          if (!session.capabilities.has('records.update')) throw new Error('未声明 records.update 能力');
          if (session.preview) throw new Error('草稿预览为只读');
          result = await api(`/api/apps/${encodeURIComponent(session.appId)}/collections/${encodeURIComponent(table)}/records/${encodeURIComponent(String(args.id || ''))}`, { method: 'PATCH', body: JSON.stringify({ data: args.data, expected_updated_at: args.expected_updated_at }) });
          break;
        case 'records.delete':
          if (!session.capabilities.has('records.delete')) throw new Error('未声明 records.delete 能力');
          if (session.preview) throw new Error('草稿预览为只读');
          result = await api(`/api/apps/${encodeURIComponent(session.appId)}/collections/${encodeURIComponent(table)}/records/${encodeURIComponent(String(args.id || ''))}`, { method: 'DELETE' });
          break;
        case 'actions.execute':
          if (!session.capabilities.has('actions.execute')) throw new Error('未声明 actions.execute 能力');
          if (typeof args.action_id !== 'string' || typeof args.idempotency_key !== 'string' || !session.actionIDs.has(args.action_id)) throw new Error('动作未被当前版本授权');
          result = await api(`/api/apps/${encodeURIComponent(session.appId)}/actions/${encodeURIComponent(args.action_id)}/execute`, { method: 'POST', body: JSON.stringify({ idempotency_key: args.idempotency_key, input: args.input || {} }) });
          break;
        case 'navigation.go': {
          if (!session.capabilities.has('navigation')) throw new Error('未声明 navigation 能力');
          const routes = session.manifest?.routes || [];
          const route = routes.find((item) => item.path === args.path);
          if (!route) throw new Error('页面路由未授权');
          const version = await api(`/api/apps/${encodeURIComponent(session.appId)}/versions/${encodeURIComponent(session.versionId)}`);
          const document = appDocument({ ...version, version: { id: version.id } }, session.preview, String(args.path));
          session.frame.srcdoc = document.srcdoc;
          break;
        }
        case 'user.read':
          if (!session.capabilities.has('user.read')) throw new Error('未声明 user.read 能力');
          result = await api('/api/me').then(({ user, tenant }) => ({ user: { id: user.id, name: user.name }, workspace: { id: tenant.id, name: tenant.name, role: tenant.role }, app_role: state.app?.permission }));
          break;
        default: throw new Error('能力未声明或操作不受支持');
      }
      reply(true, result);
    } catch (error) {
      reply(false, null, error.message || 'MIAO 请求失败');
    }
  }

  window.addEventListener('message', handleAppMessage);

  function renderSourceFrame(root, runtime, preview = false) {
    const { session, srcdoc } = appDocument(runtime, preview);
    root.innerHTML = `<div class="source-app-heading"><span class="badge ${preview ? 'badge-warning' : 'badge-success'}">${preview ? '草稿预览' : '已发布'} · v${esc(runtime.version.version)}</span></div><iframe class="source-app-frame" title="${esc(runtime.manifest?.title || state.app.name)}" sandbox="allow-scripts" referrerpolicy="no-referrer" data-app-version="${esc(runtime.version.id)}"></iframe>`;
    session.frame = root.querySelector('iframe');
    session.frame.srcdoc = srcdoc;
    session.frame.addEventListener('load', () => {
      for (const [nonce, value] of appFrameSessions) if (value.frame === session.frame && nonce !== session.nonce) appFrameSessions.delete(nonce);
    }, { once: true });
  }
  function runtimeValue(value) {
    if (value === null || value === undefined || value === '') return '—';
    if (typeof value === 'boolean') return value ? '是' : '否';
    if (Array.isArray(value)) return value.join('、');
    if (typeof value === 'object') return JSON.stringify(value);
    return String(value);
  }

  function createUiPreviewCard(versionId) {
    const card = document.createElement('section');
    card.className = 'card card-border fx-ui-preview';
    card.setAttribute('aria-label', '业务界面只读预览');
    card.setAttribute('aria-live', 'polite');
    card.dataset.previewCard = 'true';
    card.dataset.previewVersionId = versionId;
    card.dataset.previewAppId = state.app?.id || '';
    card.dataset.previewTenantId = state.tenant?.id || '';
    $('#chat-messages').append(card);
    $('#chat-messages').scrollTop = $('#chat-messages').scrollHeight;
    return card;
  }

  async function loadUiPreview(card) {
    const { previewVersionId: versionId, previewAppId: appId, previewTenantId: tenantId } = card.dataset;
    card.setAttribute('aria-busy', 'true');
    card.innerHTML = '<div class="card-body"><h3 class="card-title">正在读取界面预览</h3><div class="skeleton fx-preview-skeleton" aria-hidden="true"></div><p class="fx-preview-note">仅读取你有权访问的真实记录，不会修改业务数据。</p></div>';
    try {
      if (!appId || state.app?.id !== appId || state.tenant?.id !== tenantId) {
        throw new Error('当前工作区或应用已切换。请重新选择原应用后再试。');
      }
      const preview = await api(`/api/apps/${encodeURIComponent(appId)}/versions/${encodeURIComponent(versionId)}/preview`);
      if (state.app?.id !== appId || state.tenant?.id !== tenantId) throw new Error('工作上下文已切换，本次预览未显示。请重新选择原应用后再试。');
      if (preview.source) {
        const frameHost = document.createElement('div');
        frameHost.className = 'source-preview-frame-host';
        const runtimeData = { ...preview, version: preview.version, manifest: preview.manifest };
        renderSourceFrame(frameHost, runtimeData, true);
        card.replaceChildren();
        const heading = document.createElement('div'); heading.className = 'card-body';
        heading.innerHTML = `<div class="fx-preview-heading"><div><h3 class="card-title">${esc(state.app.name)}</h3><p class="fx-preview-meta">HTML 页面源码预览</p></div><span class="badge badge-warning badge-sm">草稿预览 · v${Number(preview.version?.version) || ''}</span></div><p class="fx-preview-note">页面运行在隔离 iframe 中；预览只读，不会修改业务数据。</p>${preview.changes?.length ? `<details><summary>源码变更（${preview.changes.length} 项）</summary><pre>${esc(JSON.stringify(preview.changes, null, 2))}</pre></details>` : ''}`;
        card.append(heading, frameHost);
        card.removeAttribute('aria-busy');
        return { version: Number(preview.version?.version) || 0, title: state.app.name, collection: '', displayed_records: 0, total_records: 0 };
      }
      const fields = preview.fields || [];
      const rows = preview.items || [];
      const versionNumber = Number(preview.version?.version) || '';
      const previewLabel = preview.version?.status === 'published'
        ? `已发布版本预览 · v${versionNumber}`
        : preview.version?.status === 'superseded'
          ? `历史版本预览 · v${versionNumber}`
          : `草稿预览 · v${versionNumber}`;
      const table = (preview.page_previews || [preview]).map((page) => {
        const fields = page.fields || [], rows = page.items || [];
        const sample = rows.length ? `<div class="overflow-x-auto fx-preview-table-wrap"><table class="table table-sm"><thead><tr>${fields.map((field) => `<th>${esc(field.label)}</th>`).join('')}</tr></thead><tbody>${rows.map((row) => `<tr>${fields.map((field) => `<td>${esc(runtimeValue(page.relation_labels?.[field.name]?.[row.data?.[field.name]] ?? row.data?.[field.name]))}</td>`).join('')}</tr>`).join('')}</tbody></table></div>` : '<p class="fx-preview-empty">暂无记录；预览不会生成示例数据。</p>';
        return `<section><h4>${esc(page.title)} · ${esc(page.collection)}</h4>${sample}<p class="fx-preview-count">展示 ${rows.length} / ${page.total_items} 条记录</p>${(page.actions || []).map((action) => `<p>业务按钮：${esc(action.label)} → ${esc(JSON.stringify(action.set))}</p>`).join('')}</section>`;
      }).join('');
      const changes = preview.changes?.length ? `<details><summary>与正式界面的差异（${preview.changes.length} 项）</summary><pre>${esc(JSON.stringify(preview.changes, null, 2))}</pre></details>` : '';
      card.innerHTML = `<div class="card-body"><div class="fx-preview-heading"><div><h3 class="card-title">${esc(preview.title)}</h3><p class="fx-preview-meta">${esc(state.app.name)} · ${esc(preview.collection)}</p></div><span class="badge badge-warning badge-sm">${esc(previewLabel)}</span></div><p class="fx-preview-note">这是只读预览，展示当前有权访问的真实记录；不会修改业务数据。</p>${changes}${table}</div>`;
      card.removeAttribute('aria-busy');
      $('#chat-messages').scrollTop = $('#chat-messages').scrollHeight;
      return { version: versionNumber, title: preview.title, collection: preview.collection, displayed_records: rows.length, total_records: preview.total_items };
    } catch (error) {
      card.innerHTML = `<div class="card-body"><h3 class="card-title">预览暂时无法生成</h3><p class="fx-preview-error" role="alert">${esc(error.message || '读取版本失败，请检查权限和数据结构后重试。')}</p><button class="btn btn-ghost btn-xs" type="button" data-preview-retry>重试预览</button></div>`;
      card.removeAttribute('aria-busy');
      $('#chat-messages').scrollTop = $('#chat-messages').scrollHeight;
      throw error;
    }
  }

  function renderAppRuntime() {
    const root = $('#app-runtime-root');
    const runtime = state.appRuntime;
    if (!runtime || runtime.status === 'not_published') { root.innerHTML = '<div class="app-runtime-empty"><h2>还没有已发布的业务界面</h2><button class="btn btn-primary btn-sm" data-action="open-assistant">和小助手设计界面</button></div>'; return; }
    if (runtime.status !== 'published' || !runtime.definition) { root.innerHTML = '<div role="alert" class="alert alert-warning app-runtime-notice">已发布界面当前不可用，请修复后重新发布。</div>'; return; }
    unmountRenderer?.();
    root.innerHTML = '';
    const nav = runtime.pages?.length > 1 ? `<nav class="runtime-page-nav" aria-label="应用页面">${runtime.pages.map((page) => `<button class="btn btn-sm ${runtime.ui_page === page.id ? 'btn-active' : 'btn-ghost'}" data-runtime-ui-page="${esc(page.id)}">${esc(page.title)}</button>`).join('')}</nav>` : '';
    root.insertAdjacentHTML('beforeend', nav);
    const host = document.createElement('div'); host.className = 'json-render-runtime'; root.append(host);
    const page = (runtime.definition.pages || []).find((item) => item.id === runtime.ui_page) || runtime.definition.pages?.[0];
    if (!page) { host.textContent = '页面配置不可用'; return; }
    unmountRenderer = mountJsonRenderer(host, page.spec, { sources: runtime.sources || {}, readOnly: state.app?.permission === 'viewer', onAction: async (source, action, row) => {
      try { await api(`/api/apps/${encodeURIComponent(state.app.id)}/runtime/actions/${encodeURIComponent(action.id)}`, { method: 'POST', body: JSON.stringify({ ui_page: runtime.ui_page, expected_version_id: runtime.version.id, record_id: row.id, expected_updated_at: row.updated_at, confirm: true }) }); await loadAppRuntime(); } catch (error) { throw error; }
    }});
  }

  async function loadAppRuntime() {
    if (!state.app) return;
    const appId = state.app.id;
    const params = new URLSearchParams({ page: String(state.runtimeQuery.page), perPage: '25' });
    if (state.runtimeQuery.ui_page) params.set('ui_page', state.runtimeQuery.ui_page);
    if (state.runtimeQuery.search) params.set('search', state.runtimeQuery.search);
    $('#app-runtime-root').innerHTML = '<div class="runtime-loading"><span class="loading loading-spinner loading-sm" aria-hidden="true"></span> 正在载入已发布界面…</div>';
    try {
      const runtime = await api(`/api/apps/${encodeURIComponent(appId)}/runtime?${params}`);
      if (state.app?.id !== appId || state.workspaceView !== 'app' || state.appPanel !== 'runtime') return;
      state.appRuntime = runtime;
      state.runtimeQuery.ui_page = runtime.ui_page;
      state.runtimeSelectedRecord = null;
      renderAppRuntime();
    } catch (error) {
      if (state.app?.id !== appId || state.workspaceView !== 'app' || state.appPanel !== 'runtime') return;
      $('#app-runtime-root').innerHTML = `<div role="alert" class="alert alert-error app-runtime-notice"><span>${esc(error.message)}</span></div><button class="btn btn-ghost btn-sm" data-action="retry-app-runtime">重试</button>`;
    }
  }

  async function handleClick(event) {
    const page = event.target.closest('[data-runtime-ui-page]');
    if (page) { state.runtimeQuery = { page: 1, search: '', ui_page: page.dataset.runtimeUiPage }; await loadAppRuntime(); return true; }
    const button = event.target.closest('[data-runtime-action]');
    if (button) {
      const row = state.appRuntime.items.find((row) => row.id === button.dataset.recordId);
      const action = state.appRuntime.actions.find((item) => item.id === button.dataset.runtimeAction);
      if (!row || !action || !confirm(`${action.label}：${JSON.stringify(action.set)}。确认修改这条记录？`)) return true;
      button.disabled = true;
      try { await api(`/api/apps/${encodeURIComponent(state.app.id)}/runtime/actions/${encodeURIComponent(action.id)}`, { method: 'POST', body: JSON.stringify({ ui_page: state.appRuntime.ui_page, expected_version_id: state.appRuntime.version.id, record_id: row.id, expected_updated_at: row.updated_at, confirm: true }) }); await loadAppRuntime(); }
      finally { button.disabled = false; }
      return true;
    }
    const detail = event.target.closest('[data-runtime-detail]');
    if (detail) {
      const appId = state.app.id, tenantId = state.tenant.id, collection = state.appRuntime.collection;
      const record = await api(`/api/apps/${encodeURIComponent(appId)}/collections/${encodeURIComponent(collection)}/records/${encodeURIComponent(detail.dataset.runtimeDetail)}`);
      if (state.app?.id !== appId || state.tenant?.id !== tenantId) return true;
      state.runtimeSelectedRecord = record.id;
      document.querySelector('#runtime-detail-dialog')?.remove();
      const dialog = document.createElement('dialog'); dialog.id = 'runtime-detail-dialog'; dialog.className = 'modal';
      const table = state.tables.find((table) => table.slug === collection);
      dialog.innerHTML = `<div class="modal-box"><h3 class="font-bold">记录详情</h3><dl>${(table?.fields || []).map((field) => `<dt>${esc(field.label)}</dt><dd>${field.type === 'file' && record.data[field.name] ? `<button class="btn btn-link btn-xs" data-download-file="${esc(field.name)}" data-record-id="${esc(record.id)}" data-file-name="${esc(record.data[field.name])}" data-file-collection="${esc(collection)}">${esc(record.data[field.name])}</button>` : esc(runtimeValue(state.appRuntime.relation_labels?.[field.name]?.[record.data[field.name]] ?? record.data[field.name]))}</dd>`).join('')}</dl><div class="modal-action"><button class="btn btn-sm" data-action="open-assistant">让小助手处理</button><form method="dialog"><button class="btn btn-sm">关闭</button></form></div></div>`;
      document.body.append(dialog); dialog.showModal(); return true;
    }
    return false;
  }

  return {
    handleClick,
    createPreviewCard: createUiPreviewCard,
    loadPreview: loadUiPreview,
    load: loadAppRuntime
  };
}
