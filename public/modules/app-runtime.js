import { mount as mountJsonRenderer } from './ui-renderer.bundle.js';

export function createAppRuntime({ state, api, $, esc }) {
  let unmountRenderer = null;
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
      onAction: async (source, action, row) => { if (preview) throw new Error('草稿预览为只读'); await api(`/api/apps/${encodeURIComponent(state.app.id)}/runtime/actions/${encodeURIComponent(action.id)}`, { method: 'POST', body: JSON.stringify({ ui_page: runtime.ui_page, expected_version_id: runtime.version.id, record_id: row.id, expected_updated_at: row.updated_at, confirm: true }) }); await loadAppRuntime(); },
      onCreate: async (source, data, files) => { if (preview) throw new Error('草稿预览为只读'); await api(`/api/apps/${encodeURIComponent(state.app.id)}/collections/${encodeURIComponent(source.collection)}/records`, { method: 'POST', body: JSON.stringify({ data, files: await encodeFiles(files) }) }); await loadAppRuntime(); },
      onUpdate: async (source, row, data) => { if (preview) throw new Error('草稿预览为只读'); await api(`/api/apps/${encodeURIComponent(state.app.id)}/collections/${encodeURIComponent(source.collection)}/records/${encodeURIComponent(row.id)}`, { method: 'PATCH', body: JSON.stringify({ data, expected_updated_at: row.updated_at }) }); await loadAppRuntime(); },
      onDelete: async (source, row) => { if (preview) throw new Error('草稿预览为只读'); if (!window.confirm('确认删除这条记录？此操作无法撤销。')) return; await api(`/api/apps/${encodeURIComponent(state.app.id)}/collections/${encodeURIComponent(source.collection)}/records/${encodeURIComponent(row.id)}`, { method: 'DELETE', body: JSON.stringify({ confirm: true, expected_updated_at: row.updated_at }) }); await loadAppRuntime(); },
    });
  }

  async function loadPreview(card) {
    const { previewVersionId: versionId, previewAppId: appId, previewTenantId: tenantId } = card.dataset;
    card.setAttribute('aria-busy', 'true');
    try {
      if (!appId || state.app?.id !== appId || state.tenant?.id !== tenantId) throw new Error('当前工作上下文已切换，请重新选择原应用。');
      const preview = await api(`/api/apps/${encodeURIComponent(appId)}/versions/${encodeURIComponent(versionId)}/preview`);
      const heading = document.createElement('div'); heading.className = 'card-body'; heading.innerHTML = `<h3 class="card-title">${esc(preview.title || state.app.name)}</h3><p class="agent-preview-note">只读预览使用与正式界面相同的受控 Spec 渲染器。</p>`;
      const host = document.createElement('div'); host.className = 'json-render-preview'; card.replaceChildren(heading, host);
      mountJsonRenderer(host, (preview.definition.pages || []).find((page) => page.id === preview.ui_page)?.spec || preview.definition.pages?.[0]?.spec, { sources: preview.sources || {}, members: preview.members || [], readOnly: true });
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
    if (!runtime || runtime.status === 'not_published') { root.innerHTML = `<div class="app-runtime-empty"><h2>还没有已发布的业务界面</h2><p>先让小助手生成草稿，再从预览确认后发布。</p><button class="btn btn-primary btn-sm" data-action="open-assistant">和小助手设计界面</button>${supportsDataManagement() ? '<button class="btn btn-ghost btn-sm" data-action="view-app-data">查看数据表</button>' : ''}<div id="draft-version-list" class="runtime-drafts"><span class="runtime-loading">正在读取草稿…</span></div></div>`; loadDraftVersions().catch((error) => { const drafts = $('#draft-version-list'); if (drafts) drafts.innerHTML = `<p class="app-runtime-notice" role="alert">${esc(error.message)}</p>`; }); return; }
    if (runtime.status !== 'published' || !runtime.definition) { root.innerHTML = '<div role="alert" class="alert alert-warning app-runtime-notice">已发布界面当前不可用，请修复后重新发布。</div>'; return; }
    const nav = runtime.pages?.length > 1 ? `<nav class="runtime-page-nav" aria-label="应用页面">${runtime.pages.map((page) => `<button class="btn btn-sm ${runtime.ui_page === page.id ? 'btn-active' : 'btn-ghost'}" data-runtime-ui-page="${esc(page.id)}">${esc(page.title)}</button>`).join('')}</nav>` : '';
    root.innerHTML = `${nav}<div class="runtime-toolbar"><h2>${esc(runtime.title)}</h2></div><div id="json-render-runtime-host"></div>`;
    unmountRenderer = mountRuntime($('#json-render-runtime-host'), runtime);
  }

  async function loadDraftVersions() {
    if (!state.app) return;
    const result = await api(`/api/apps/${encodeURIComponent(state.app.id)}/versions`);
    const drafts = (result.items || []).filter((item) => item.status === 'draft');
    const root = $('#draft-version-list');
    if (!root) return;
    root.innerHTML = drafts.length ? drafts.map((version) => `<article class="runtime-draft"><div><strong>草稿 v${esc(version.version)}</strong><span>${esc(version.summary || '待审阅的界面定义')}</span></div><div class="runtime-toolbar-actions"><button class="btn btn-ghost btn-sm" data-preview-version="${esc(version.id)}">只读预览</button><button class="btn btn-primary btn-sm" data-publish-version="${esc(version.id)}">确认发布</button></div></article><div data-version-preview="${esc(version.id)}"></div>`).join('') : '<p class="runtime-draft-empty">还没有界面草稿。通过小助手选择模板后，草稿会出现在这里。</p>';
  }

  async function previewVersion(versionId) {
    const host = document.querySelector(`[data-version-preview="${CSS.escape(versionId)}"]`);
    if (!host) return;
    host.innerHTML = '<div class="runtime-loading">正在生成只读预览…</div>';
    const preview = await api(`/api/apps/${encodeURIComponent(state.app.id)}/versions/${encodeURIComponent(versionId)}/preview`);
    const page = (preview.definition?.pages || []).find((item) => item.id === preview.ui_page) || preview.definition?.pages?.[0];
    host.innerHTML = '<div class="agent-ui-preview"></div>';
    mountJsonRenderer(host.firstElementChild, page?.spec, { sources: preview.sources || {}, members: preview.members || [], readOnly: true });
  }

  async function publishVersion(versionId) {
    const versions = await api(`/api/apps/${encodeURIComponent(state.app.id)}/versions`);
    const response = await api(`/api/apps/${encodeURIComponent(state.app.id)}/versions/${encodeURIComponent(versionId)}/publish`, { method: 'POST', body: JSON.stringify({ expected_published_version_id: versions.published_version_id || null }) });
    state.appRuntime = response;
    state.app = { ...state.app, has_published_version: true };
    renderAppRuntime();
  }

  async function loadAppRuntime() {
    if (!state.app) return;
    const appId = state.app.id; const params = new URLSearchParams({ page: String(state.runtimeQuery.page || 1), perPage: '25' });
    if (state.runtimeQuery.ui_page) params.set('ui_page', state.runtimeQuery.ui_page);
    if (state.runtimeQuery.search) params.set('search', state.runtimeQuery.search);
    $('#app-runtime-root').innerHTML = '<div class="runtime-loading">正在载入已发布界面…</div>';
    try {
      const [runtime, memberResult] = await Promise.all([api(`/api/apps/${encodeURIComponent(appId)}/runtime?${params}`), api(`/api/apps/${encodeURIComponent(appId)}/members`).catch(() => ({ members: [] }))]);
      runtime.members = (memberResult.members || []).map((member) => ({ id: member.id, label: member.name || member.email || member.id }));
      if (state.app?.id !== appId || state.workspaceView !== 'app' || state.appPanel !== 'runtime') return;
      state.appRuntime = runtime; state.runtimeQuery.ui_page = runtime.ui_page; renderAppRuntime();
    } catch (error) { if (state.app?.id === appId && state.workspaceView === 'app' && state.appPanel === 'runtime') $('#app-runtime-root').innerHTML = `<div role="alert" class="alert alert-error app-runtime-notice">${esc(error.message)}</div><button class="btn btn-ghost btn-sm" data-action="retry-app-runtime">重试</button>`; }
  }

  async function handleClick(event) { const page = event.target.closest('[data-runtime-ui-page]'); if (page) { state.runtimeQuery = { ...state.runtimeQuery, page: 1, search: '', ui_page: page.dataset.runtimeUiPage }; await loadAppRuntime(); return true; } const preview = event.target.closest('[data-preview-version]'); if (preview) { await previewVersion(preview.dataset.previewVersion); return true; } const publish = event.target.closest('[data-publish-version]'); if (publish) { if (!window.confirm('发布此界面版本？正式页面将切换到这个版本，业务记录不会回滚。')) return true; await publishVersion(publish.dataset.publishVersion); return true; } return false; }
  return { handleClick, createPreviewCard, loadPreview, load: loadAppRuntime };
}
