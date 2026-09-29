export function createAppRuntime({ state, api, $, esc }) {
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
      const fields = preview.fields || [];
      const rows = preview.items || [];
      const versionNumber = Number(preview.version?.version) || '';
      const previewLabel = preview.version?.status === 'published'
        ? `已发布版本预览 · v${versionNumber}`
        : preview.version?.status === 'superseded'
          ? `历史版本预览 · v${versionNumber}`
          : `草稿预览 · v${versionNumber}`;
      const table = rows.length ? `<div class="overflow-x-auto fx-preview-table-wrap"><table class="table table-sm"><thead><tr>${fields.map((field) => `<th>${esc(field.label)}</th>`).join('')}</tr></thead><tbody>${rows.map((row) => `<tr>${fields.map((field) => `<td>${esc(runtimeValue(row.data?.[field.name]))}</td>`).join('')}</tr>`).join('')}</tbody></table></div>` : '<p class="fx-preview-empty">当前数据表暂无记录；预览不会生成示例数据。</p>';
      const countLabel = preview.total_items > rows.length ? `显示最新 ${rows.length} 条，共 ${preview.total_items} 条` : `共 ${preview.total_items} 条`;
      card.innerHTML = `<div class="card-body"><div class="fx-preview-heading"><div><h3 class="card-title">${esc(preview.title)}</h3><p class="fx-preview-meta">${esc(state.app.name)} · ${esc(preview.collection)}</p></div><span class="badge badge-warning badge-sm">${esc(previewLabel)}</span></div><p class="fx-preview-note">这是只读预览，展示当前有权访问的真实记录；不会修改业务数据。</p>${table}<p class="fx-preview-count">${esc(countLabel)}</p></div>`;
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
    if (!runtime || runtime.status === 'not_published') {
      root.innerHTML = '<div class="app-runtime-empty"><span class="app-runtime-mark" aria-hidden="true">▤</span><span class="eyebrow">应用界面</span><h2>还没有已发布的业务界面</h2><p>和 fx 梳理需要展示的信息，审阅真实数据预览后再发布。已发布界面可在满足字段要求时直接新增记录。</p><div class="app-runtime-actions"><button class="btn btn-primary btn-sm" data-action="open-assistant">和 fx 设计界面</button><button class="btn btn-ghost btn-sm" data-action="view-app-data">查看数据表</button></div></div>';
      return;
    }
    if (runtime.status !== 'published') {
      root.innerHTML = '<div role="alert" class="alert alert-warning app-runtime-notice"><span>已发布界面当前不可用，可能引用了已删除或不兼容的数据字段。已有记录未受影响，请联系应用管理员修复后再发布新版本。</span></div><div class="app-runtime-actions"><button class="btn btn-primary btn-sm" data-action="open-assistant">和 fx 修复界面</button><button class="btn btn-ghost btn-sm" data-action="view-app-data">查看数据表</button></div>';
      return;
    }
    const columns = runtime.fields || [];
    const rows = runtime.items || [];
    const canEdit = state.app?.permission !== 'viewer';
    const canCreate = canEdit && runtime.create_form_available;
    const emptyCopy = canCreate
      ? '在应用中添加第一条真实记录。'
      : canEdit
        ? '当前界面无法生成完整的新增表单；请检查字段类型和必填字段设置，或从数据表添加记录。'
        : '这个界面还没有真实记录，请联系应用编辑者添加。';
    root.innerHTML = `<div class="runtime-content"><div class="runtime-toolbar"><div><span class="eyebrow">已发布 · v${esc(runtime.version.version)}</span><h2>${esc(runtime.title)}</h2><p>${runtime.total_items} 条记录</p></div><div class="runtime-toolbar-actions">${canCreate ? '<button class="btn btn-sm" data-action="add-runtime-record">＋ 新增记录</button>' : ''}<button class="btn btn-ghost btn-sm" data-action="view-app-data">查看数据表</button></div></div>${runtime.search_supported ? `<form class="runtime-search-form"><input class="input input-bordered input-sm" name="search" type="search" value="${esc(state.runtimeQuery.search)}" placeholder="搜索当前界面字段" aria-label="搜索记录"><button class="btn btn-sm" type="submit">搜索</button>${state.runtimeQuery.search ? '<button class="btn btn-ghost btn-sm" type="button" data-action="clear-runtime-search">清除</button>' : ''}</form>` : ''}${rows.length ? `<div class="overflow-x-auto runtime-table-wrap"><table class="table table-sm"><thead><tr>${columns.map((field) => `<th>${esc(field.label)}</th>`).join('')}</tr></thead><tbody>${rows.map((row) => `<tr>${columns.map((field) => `<td>${esc(runtimeValue(row.data[field.name]))}</td>`).join('')}</tr>`).join('')}</tbody></table></div>` : `<div class="runtime-list-empty"><strong>还没有记录</strong><span>${esc(emptyCopy)}</span>${canCreate ? '' : '<button class="btn btn-outline btn-sm" data-action="view-app-data">打开数据检查页</button>'}</div>`}<div class="runtime-pagination"><span>第 ${runtime.page} / ${Math.max(1, runtime.total_pages)} 页</span><div class="join"><button class="btn btn-sm join-item" data-runtime-page="${runtime.page - 1}" ${runtime.page <= 1 ? 'disabled' : ''}>上一页</button><button class="btn btn-sm join-item" data-runtime-page="${runtime.page + 1}" ${runtime.page >= runtime.total_pages ? 'disabled' : ''}>下一页</button></div></div></div>`;
  }

  async function loadAppRuntime() {
    if (!state.app) return;
    const appId = state.app.id;
    const params = new URLSearchParams({ page: String(state.runtimeQuery.page), perPage: '25' });
    if (state.runtimeQuery.search) params.set('search', state.runtimeQuery.search);
    $('#app-runtime-root').innerHTML = '<div class="runtime-loading"><span class="loading loading-spinner loading-sm" aria-hidden="true"></span> 正在载入已发布界面…</div>';
    try {
      const runtime = await api(`/api/apps/${encodeURIComponent(appId)}/runtime?${params}`);
      if (state.app?.id !== appId || state.workspaceView !== 'app' || state.appPanel !== 'runtime') return;
      state.appRuntime = runtime;
      renderAppRuntime();
    } catch (error) {
      if (state.app?.id !== appId || state.workspaceView !== 'app' || state.appPanel !== 'runtime') return;
      $('#app-runtime-root').innerHTML = `<div role="alert" class="alert alert-error app-runtime-notice"><span>${esc(error.message)}</span></div><button class="btn btn-ghost btn-sm" data-action="retry-app-runtime">重试</button>`;
    }
  }

  return {
    createPreviewCard: createUiPreviewCard,
    loadPreview: loadUiPreview,
    load: loadAppRuntime
  };
}
