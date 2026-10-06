import { mount as mountJsonRenderer } from './ui-renderer.bundle.js';
import { createUIEditor, formatUIChanges } from './ui-editor.js';

export function createAppRuntime({ state, api, $, esc, onUIRequest }) {
  let unmountRenderer = null;
  const reviewed = new Map();
  const actionRequests = new Map();
  const editor = createUIEditor({ state, api, $, esc, onSaved: () => loadAppRuntime(), onModelRequest: onUIRequest });
  const supportsDataManagement = () => Array.isArray(state.app?.capabilities) && state.app.capabilities.includes('data_management');

  function createPreviewCard(versionId) {
    const card = document.createElement('section');
    card.className = 'card card-border agent-ui-preview';
    card.setAttribute('aria-label', '业务界面只读预览');
    card.dataset.previewCard = 'true';
    card.dataset.previewVersionId = versionId;
    card.dataset.previewAppId = state.app?.id || '';
    card.dataset.previewTenantId = state.tenant?.id || '';
    $('#chat-messages').append(card);
    return card;
  }

  function mountRuntime(root, runtime, preview = false) {
    const appId = state.app.id, tenantId = state.tenant.id;
    const inContext = () => state.app?.id === appId && state.tenant?.id === tenantId;
    const assertWritable = () => {
      if (preview) throw new Error('草稿预览为只读');
      if (!inContext()) throw new Error('应用上下文已切换，请重新读取。');
    };
    const refresh = async () => { if (inContext()) await loadAppRuntime(); };
    const page = (runtime.definition?.pages || []).find((item) => item.id === runtime.ui_page) || runtime.definition?.pages?.[0];
    if (!page?.spec) throw new Error('页面 Spec 不可用');
    const host = document.createElement('div'); host.className = 'json-render-runtime'; root.replaceChildren(host);
    const encodeFiles = async (files) => {
      const encoded = {};
      for (const [name, file] of Object.entries(files || {})) encoded[name] = { name: file.name, type: file.type, base64: await new Promise((resolve, reject) => { const reader = new FileReader(); reader.onload = () => resolve(reader.result); reader.onerror = () => reject(reader.error); reader.readAsDataURL(file); }) };
      return encoded;
    };
    return mountJsonRenderer(host, page.spec, {
      sources: runtime.sources || {}, members: runtime.members || [], readOnly: preview || state.app?.permission === 'viewer',
      onOpenRecord: runtime.pages?.some((candidate) => candidate.detail_source) ? (source, row) => { const detail = runtime.pages?.find((candidate) => candidate.collection === source.collection && candidate.detail_source); if (!detail) return; state.runtimeQuery = { ...state.runtimeQuery, page: 1, search: '', record_id: row.id, ui_page: detail.id }; loadAppRuntime(); } : undefined,
      search: state.runtimeQuery.search || '',
      onSearch: (_source, search) => { state.runtimeQuery = { ...state.runtimeQuery, page: 1, search, record_id: runtime.record_id || '' }; loadAppRuntime(); },
      onPage: (_source, page) => { state.runtimeQuery = { ...state.runtimeQuery, page, record_id: runtime.record_id || '' }; loadAppRuntime(); },
      onAction: async (source, action, row, input = {}) => {
        if (preview) throw new Error('草稿预览为只读');
        if (state.app?.id !== appId || state.tenant?.id !== tenantId) throw new Error('应用上下文已切换，请重新读取。');
        const requestKey = JSON.stringify([tenantId, appId, runtime.version.id, runtime.ui_page, runtime.record_id, source.id, action.id, row.id, row.updated_at, input]);
        if (!actionRequests.has(requestKey)) {
          if (actionRequests.size >= 128) throw new Error('有过多待核实动作，请刷新并核对记录后继续。');
          actionRequests.set(requestKey, crypto.randomUUID());
        }
        await api(`/api/apps/${encodeURIComponent(appId)}/runtime/actions/${encodeURIComponent(action.id)}`, { method: 'POST', body: JSON.stringify({ ui_page: runtime.ui_page, expected_version_id: runtime.version.id, record_id: row.id, context_record_id: runtime.record_id || '', expected_updated_at: row.updated_at, input, idempotency_key: actionRequests.get(requestKey), confirm: true }) });
        if (state.app?.id === appId && state.tenant?.id === tenantId) await loadAppRuntime();
        actionRequests.delete(requestKey);
      },
      onCreate: async (source, data, files) => { assertWritable(); const encoded = await encodeFiles(files); assertWritable(); await api(`/api/apps/${encodeURIComponent(appId)}/collections/${encodeURIComponent(source.collection)}/records`, { method: 'POST', body: JSON.stringify({ data, files: encoded }) }); await refresh(); },
      onUpdate: async (source, row, data, files) => { assertWritable(); const encoded = await encodeFiles(files); assertWritable(); await api(`/api/apps/${encodeURIComponent(appId)}/collections/${encodeURIComponent(source.collection)}/records/${encodeURIComponent(row.id)}`, { method: 'PATCH', body: JSON.stringify({ data, files: encoded, expected_updated_at: row.updated_at }) }); await refresh(); },
      onDelete: async (source, row) => { assertWritable(); if (!window.confirm('确认删除这条记录？此操作无法撤销。')) return; await api(`/api/apps/${encodeURIComponent(appId)}/collections/${encodeURIComponent(source.collection)}/records/${encodeURIComponent(row.id)}`, { method: 'DELETE', body: JSON.stringify({ confirm: true, expected_updated_at: row.updated_at }) }); await refresh(); },
    });
  }

  async function loadPreview(card) {
    const { previewVersionId: versionId, previewAppId: appId, previewTenantId: tenantId } = card.dataset;
    card.setAttribute('aria-busy', 'true');
    try {
      if (!appId || state.app?.id !== appId || state.tenant?.id !== tenantId) throw new Error('当前工作上下文已切换，请重新选择原应用。');
      const preview = await api(`/api/apps/${encodeURIComponent(appId)}/versions/${encodeURIComponent(versionId)}/preview`);
      const heading = document.createElement('div'); heading.className = 'card-body'; heading.innerHTML = `<h3 class="card-title">${esc(preview.title || state.app.name)}</h3>`;
      const host = document.createElement('div'); host.className = 'json-render-preview'; card.replaceChildren(heading, host);
      const page = (preview.definition.pages || []).find((item) => item.id === preview.ui_page) || preview.definition.pages?.[0];
      const diff = document.createElement('details'); diff.className = 'preview-change-list'; diff.open = true;
      const changes = preview.changes || [];
      diff.innerHTML = `<summary>与当前正式版的差异 · ${changes.length} 项</summary>${changes.length ? `${formatUIChanges(changes, esc)}` : '<p>没有检测到界面定义差异。</p>'}`;
      card.insertBefore(diff, host);
      mountJsonRenderer(host, page?.spec, { sources: preview.sources || {}, members: preview.members || [], readOnly: true });
      card.removeAttribute('aria-busy');
      const firstSource = Object.values(preview.sources || {})[0] || {};
      return { version: Number(preview.version?.version) || 0, title: preview.title, collection: firstSource.collection || '', displayed_records: (firstSource.items || []).length, total_records: firstSource.total_items || 0 };
    } catch (error) {
      card.innerHTML = `<div class="card-body"><h3 class="card-title">预览暂时无法生成</h3><p role="alert">${esc(error.message || '读取版本失败')}</p><button class="btn btn-ghost btn-xs" type="button" data-preview-retry>重试预览</button></div>`;
      card.removeAttribute('aria-busy'); throw error;
    }
  }

  function renderAppRuntime() {
    const root = $('#app-runtime-root'); const runtime = state.appRuntime; unmountRenderer?.(); unmountRenderer = null;
    if (!runtime || runtime.status === 'not_published') { root.innerHTML = `<div class="app-runtime-empty"><h2>还没有已发布的业务界面</h2><button class="btn btn-primary btn-sm" data-action="open-assistant">和小助手设计界面</button>${supportsDataManagement() ? '<button class="btn btn-ghost btn-sm" data-action="view-app-data">查看数据表</button>' : ''}</div><section class="runtime-draft-section"><h3>界面草稿</h3><div id="draft-version-list" class="runtime-drafts"><span class="runtime-loading">正在读取草稿…</span></div></section>`; loadDraftVersions().catch((error) => { const drafts = $('#draft-version-list'); if (drafts) drafts.innerHTML = `<p class="app-runtime-notice" role="alert">${esc(error.message)}</p>`; }); return; }
    if (runtime.status !== 'published' || !runtime.definition) { root.innerHTML = '<div role="alert" class="alert alert-warning app-runtime-notice">已发布界面当前不可用，请修复后重新发布。</div><section class="runtime-draft-section"><h3>界面草稿</h3><div id="draft-version-list" class="runtime-drafts"></div></section>'; loadDraftVersions().catch(() => {}); return; }
    const nav = runtime.pages?.length > 1 ? `<nav class="runtime-page-nav" aria-label="应用页面">${runtime.pages.filter((page) => !page.detail_source).map((page) => `<button class="btn btn-sm ${runtime.ui_page === page.id ? 'btn-active' : 'btn-ghost'}" data-runtime-ui-page="${esc(page.id)}">${esc(page.title)}</button>`).join('')}</nav>` : '';
    const currentPage = runtime.pages?.find((page) => page.id === runtime.ui_page);
    const listPage = runtime.pages?.find((page) => !page.detail_source && page.collection === currentPage?.collection);
    const back = currentPage?.detail_source && listPage ? `<button class="btn btn-ghost btn-sm" data-runtime-ui-page="${esc(listPage.id)}">返回列表</button>` : '';
    root.innerHTML = `${nav}<div class="runtime-toolbar"><h2>${esc(runtime.title)}</h2><div class="runtime-toolbar-actions">${back}<button class="btn btn-ghost btn-sm" data-action="open-assistant">继续设计</button>${canManageDrafts() ? '<button class="btn btn-outline btn-sm" data-edit-ui-version="">编辑界面</button>' : ''}</div></div><div id="json-render-runtime-host"></div><section class="runtime-draft-section"><h3>界面草稿</h3><div id="draft-version-list" class="runtime-drafts"><span class="runtime-loading">正在读取草稿…</span></div></section>`;
    unmountRenderer = mountRuntime($('#json-render-runtime-host'), runtime);
    loadDraftVersions().catch((error) => { const drafts = $('#draft-version-list'); if (drafts) drafts.innerHTML = `<p class="app-runtime-notice" role="alert">${esc(error.message)}</p>`; });
  }

  async function loadDraftVersions() {
    if (!state.app) return;
    const appId = state.app.id, tenantId = state.tenant?.id;
    reviewed.clear();
    const result = await api(`/api/apps/${encodeURIComponent(appId)}/versions`);
    if (state.app?.id !== appId || state.tenant?.id !== tenantId) return;
    const drafts = (result.items || []).filter((item) => item.status === 'draft' && canManageDrafts());
    const root = $('#draft-version-list');
    if (!root) return;
    root.innerHTML = drafts.length ? drafts.map((version) => `<article class="runtime-draft"><div><strong>草稿 v${esc(version.version)}</strong><span>${esc(version.summary || '待审阅的界面定义')}</span></div><div class="runtime-toolbar-actions"><button class="btn btn-ghost btn-sm" data-edit-ui-version="${esc(version.id)}">编辑草稿</button><button class="btn btn-ghost btn-sm" data-preview-version="${esc(version.id)}">只读预览</button>${canPublishDrafts() ? `<button class="btn btn-primary btn-sm" data-publish-version="${esc(version.id)}" disabled>预览后发布</button>` : ''}</div></article><div data-version-preview="${esc(version.id)}"></div>`).join('') : '<p class="runtime-draft-empty">还没有界面草稿。</p>';
    if (canManageDrafts()) { const history = (result.items || []).filter((item) => item.status === 'superseded'); if (history.length) root.insertAdjacentHTML('beforeend', `<details class="preview-change-list"><summary>已发布的历史界面</summary>${history.map((item) => `<article class="runtime-draft"><span>v${esc(item.version)} · ${esc(item.summary)}</span><button class="btn btn-ghost btn-sm" data-restore-ui-version="${esc(item.id)}">创建恢复草稿</button></article>`).join('')}</details>`); }
  }

  function canManageDrafts() { return ['owner', 'manager', 'publisher'].includes(state.app?.permission) || state.tenant?.role === 'owner'; }
  function canPublishDrafts() { return ['owner', 'publisher'].includes(state.app?.permission) || state.tenant?.role === 'owner'; }

  async function previewVersion(versionId, pageId = '', recordId = '') {
    const appId = state.app?.id, tenantId = state.tenant?.id;
    const host = document.querySelector(`[data-version-preview="${CSS.escape(versionId)}"]`);
    if (!host) return;
    host.innerHTML = '<div class="runtime-loading">正在生成只读预览…</div>';
    const query = new URLSearchParams();
    if (pageId) query.set('ui_page', pageId);
    if (recordId) query.set('record_id', recordId);
    try {
      const preview = await api(`/api/apps/${encodeURIComponent(appId)}/versions/${encodeURIComponent(versionId)}/preview?${query}`);
      if (!host.isConnected || state.app?.id !== appId || state.tenant?.id !== tenantId) return;
      const page = preview.definition.pages.find((item) => item.id === preview.ui_page);
      host.innerHTML = `<div class="agent-ui-preview"><details class="preview-change-list" open><summary>与正式版的差异 · ${preview.changes.length} 项</summary>${formatUIChanges(preview.changes, esc)}</details><nav class="runtime-page-nav" aria-label="草稿页面">${preview.pages.map((item) => `<button class="btn btn-ghost btn-sm" data-draft-preview-page="${esc(item.id)}" data-version-id="${esc(versionId)}">${esc(item.title)}</button>`).join('')}</nav><div class="json-render-preview-host"></div></div>`;
      mountJsonRenderer(host.querySelector('.json-render-preview-host'), page.spec, {
        sources: preview.sources || {}, members: preview.members || [], readOnly: true,
        onOpenRecord: (source, row) => { const detail = preview.pages.find((item) => item.collection === source.collection && item.detail_source); if (detail) previewVersion(versionId, detail.id, row.id); },
      });
      reviewed.set(`${tenantId}:${appId}:${versionId}`, preview.current_version_id || null);
      const publishButton = document.querySelector(`[data-publish-version="${CSS.escape(versionId)}"]`);
      if (publishButton) { publishButton.disabled = false; publishButton.textContent = '确认发布'; }
    } catch (error) { if (host.isConnected) host.innerHTML = `<p class="alert alert-error" role="alert">${esc(error.message)}</p>`; }
  }

  async function publishVersion(versionId) {
    const key = `${state.tenant?.id}:${state.app?.id}:${versionId}`;
    if (!reviewed.has(key)) throw new Error('请先预览并审阅此版本的差异，再确认发布。');
    await api(`/api/apps/${encodeURIComponent(state.app.id)}/versions/${encodeURIComponent(versionId)}/publish`, { method: 'POST', body: JSON.stringify({ expected_published_version_id: reviewed.get(key) }) });
    reviewed.delete(key);
    state.app = { ...state.app, has_published_version: true };
    await loadAppRuntime();
  }

  async function loadAppRuntime() {
    if (!state.app) return;
    const appId = state.app.id; const params = new URLSearchParams({ page: String(state.runtimeQuery.page || 1), perPage: '25' });
    if (state.runtimeQuery.ui_page) params.set('ui_page', state.runtimeQuery.ui_page);
    if (state.runtimeQuery.search) params.set('search', state.runtimeQuery.search);
    if (state.runtimeQuery.record_id) params.set('record_id', state.runtimeQuery.record_id);
    $('#app-runtime-root').innerHTML = '<div class="runtime-loading">正在载入已发布界面…</div>';
    try {
      const [runtime, memberResult] = await Promise.all([api(`/api/apps/${encodeURIComponent(appId)}/runtime?${params}`), api(`/api/apps/${encodeURIComponent(appId)}/members`).catch(() => ({ members: [] }))]);
      runtime.members = (memberResult.members || []).map((member) => ({ id: member.id, label: member.label || member.name || member.email || member.id }));
      if (state.app?.id !== appId || state.workspaceView !== 'app' || state.appPanel !== 'runtime') return;
      state.appRuntime = runtime; state.runtimeQuery.ui_page = runtime.ui_page; renderAppRuntime();
    } catch (error) { if (state.app?.id === appId && state.workspaceView === 'app' && state.appPanel === 'runtime') $('#app-runtime-root').innerHTML = `<div role="alert" class="alert alert-error app-runtime-notice">${esc(error.message)}</div><button class="btn btn-ghost btn-sm" data-action="retry-app-runtime">重试</button>`; }
  }

  async function handleClick(event) {
    if (await editor.click(event)) return true;
    const restore = event.target.closest('[data-restore-ui-version]');
    if (restore) { if (!window.confirm('从历史界面创建新草稿？预览并发布后才切换正式界面，业务记录保持原样。')) return true; const appId = state.app.id; const versions = await api(`/api/apps/${encodeURIComponent(appId)}/versions`); await api(`/api/apps/${encodeURIComponent(appId)}/versions/${encodeURIComponent(restore.dataset.restoreUiVersion)}/restore`, { method:'POST', body:JSON.stringify({ expected_latest_version_id:versions.items?.[0]?.id || null,expected_published_version_id:versions.published_version_id || null }) }); await loadAppRuntime(); return true; }
    const draftPage = event.target.closest('[data-draft-preview-page]');
    if (draftPage) { await previewVersion(draftPage.dataset.versionId, draftPage.dataset.draftPreviewPage); return true; }
    const page = event.target.closest('[data-runtime-ui-page]'); if (page) { state.runtimeQuery = { ...state.runtimeQuery, page: 1, search: '', record_id: '', ui_page: page.dataset.runtimeUiPage }; await loadAppRuntime(); return true; } const preview = event.target.closest('[data-preview-version]'); if (preview) { await previewVersion(preview.dataset.previewVersion); return true; } const publish = event.target.closest('[data-publish-version]'); if (publish) { if (!window.confirm('发布此界面版本？正式页面将切换到这个版本，业务记录不会回滚。')) return true; await publishVersion(publish.dataset.publishVersion); return true; } return false; }
  return { handleClick, handleSubmit: editor.submit, handleChange: editor.change, createPreviewCard, loadPreview, load: loadAppRuntime };
}
