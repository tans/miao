import { icon } from '/modules/icons.js';
import { t } from '/modules/i18n.js';

export function createWorkspaceData({ state, api, $, $$, esc, toast, renderWorkspace }) {
  const pendingDeletes = new Set();

  function renderTables() {
    $('#table-list').innerHTML = state.tables.map((table) => `<button class="table-nav-item ${state.table?.id === table.id ? 'active' : ''}" data-table="${esc(table.slug)}"><span>${icon('table', 12)}</span>${esc(table.name)}</button>`).join('') || `<p class="no-tables">${esc(t('还没有数据表。'))}</p>`;
  }

  function renderNoTables() {
    $('#records-root').innerHTML = `<div class="records-empty"><strong>${esc(t('还没有数据表'))}</strong><button class="btn btn-outline btn-sm" data-action="open-assistant">${esc(t('和小助手讨论工作流程'))}</button></div>`;
  }

  function renderRecordCell(row, field) {
    const value = row.data[field.name];
    if (field.type === 'file' && value) {
      return `<td title="${esc(value)}"><button class="btn btn-link btn-xs" type="button" data-download-file="${esc(field.name)}" data-record-id="${esc(row.id)}" data-file-name="${esc(value)}">${esc(value)}</button></td>`;
    }
    const display = field.type === 'bool' && value !== undefined ? (value ? t('是') : t('否')) : value ?? '—';
    return `<td title="${esc(display)}">${esc(Array.isArray(display) ? display.join(', ') : display)}</td>`;
  }

  async function renderRecords() {
    renderTables();
    if (!state.table) return renderNoTables();
    const focusedId = document.activeElement?.id;
    const selectionStart = document.activeElement?.selectionStart;
    const selectionEnd = document.activeElement?.selectionEnd;
    try {
      const params = new URLSearchParams(Object.fromEntries(Object.entries(state.recordQuery).filter(([, value]) => value !== '')));
      state.recordResult = await api(`/api/apps/${state.app.id}/collections/${encodeURIComponent(state.table.slug)}/records?${params}`);
      state.records = state.recordResult.items || [];
    } catch (error) {
      toast(error.message, true);
      return;
    }
    const fields = state.table.fields || [];
    const columns = fields.slice(0, 5);
    const sortOptions = ['-created', 'created', '-updated', 'updated', ...fields.map((field) => field.name), ...fields.map((field) => `-${field.name}`)];
    const canEdit = state.app.permission !== 'viewer';
    const canPublish = state.tenant?.role === 'owner' || state.app.permission === 'publisher';
    const recordActions = (row) => `${canEdit ? `<button class="btn btn-ghost btn-xs" title="${esc(t('编辑记录'))}" aria-label="${esc(t('编辑记录'))}" data-edit-record="${esc(row.id)}">${esc(t('编辑'))}</button><button class="btn btn-ghost btn-xs" title="${esc(t('删除记录'))}" aria-label="${esc(t('删除记录'))}" data-delete-record="${esc(row.id)}">${icon('x', 12)}</button>` : ''}${canPublish ? `<button class="btn btn-ghost btn-xs" title="${esc(t('生成跟进建议'))}" aria-label="${esc(t('生成跟进建议'))}" data-agent-followup="${esc(row.id)}">${esc(t('跟进建议'))}</button>` : ''}`;
    $('#records-root').innerHTML = `<div class="records-heading"><div><h2>${esc(state.table.name)}</h2><span>${t('{count} 条记录 · {fields} 个字段', { count: state.recordResult.totalItems, fields: fields.length })}</span></div><div>${canEdit ? `<button class="btn btn-ghost btn-sm" data-action="edit-table">${esc(t('重命名'))}</button><button class="btn btn-ghost btn-sm" data-action="edit-table-schema">${esc(t('字段设置'))}</button><button class="btn btn-ghost btn-sm" data-action="delete-table">${esc(t('删除表'))}</button><button class="btn btn-ghost btn-sm" data-action="add-record">${icon('plus', 12)}${esc(t('添加记录'))}</button>` : `<span class="badge badge-ghost">${esc(t('只读'))}</span>`}</div></div><div class="record-query"><input class="input input-bordered input-sm" id="record-search" type="search" placeholder="${esc(t('搜索文本字段'))}" value="${esc(state.recordQuery.search)}"><select class="select select-bordered select-sm" id="record-sort">${sortOptions.map((value) => `<option value="${esc(value)}" ${value === state.recordQuery.sort ? 'selected' : ''}>${esc(t('排序：{value}{direction}', { value: value.replace(/^-/, ''), direction: value.startsWith('-') ? ' ↓' : ' ↑' }))}</option>`).join('')}</select><select class="select select-bordered select-sm" id="record-filter-field"><option value="">${esc(t('筛选字段'))}</option>${fields.map((field) => `<option value="${esc(field.name)}" ${field.name === state.recordQuery.filterField ? 'selected' : ''}>${esc(field.label || field.name)}</option>`).join('')}</select><input class="input input-bordered input-sm" id="record-filter-value" placeholder="${esc(t('筛选值'))}" value="${esc(state.recordQuery.filterValue)}"><button class="btn btn-ghost btn-sm" data-action="clear-filter">${esc(t('清除'))}</button></div>${state.records.length ? `<div class="overflow-x-auto"><table class="table table-sm"><thead><tr>${columns.map((field) => `<th>${esc(field.label || field.name)}</th>`).join('')}<th></th></tr></thead><tbody>${state.records.map((row) => `<tr>${columns.map((field) => renderRecordCell(row, field)).join('')}<td class="record-actions">${recordActions(row)}</td></tr>`).join('')}</tbody></table></div>` : `<div class="records-empty"><strong>${esc(t('没有匹配的记录'))}</strong></div>`}<div class="record-pagination"><span>${t('第 {page} / {pages} 页', { page: state.recordResult.page, pages: Math.max(1, state.recordResult.totalPages) })}</span><button class="btn btn-ghost btn-sm" data-page="${Math.max(1, state.recordResult.page - 1)}" ${state.recordResult.page <= 1 ? 'disabled' : ''}>${esc(t('上一页'))}</button><button class="btn btn-ghost btn-sm" data-page="${Math.min(state.recordResult.totalPages || 1, state.recordResult.page + 1)}" ${state.recordResult.page >= state.recordResult.totalPages ? 'disabled' : ''}>${esc(t('下一页'))}</button><select class="select select-bordered select-sm" id="record-page-size"><option ${state.recordQuery.perPage === 25 ? 'selected' : ''}>25</option><option ${state.recordQuery.perPage === 50 ? 'selected' : ''}>50</option><option ${state.recordQuery.perPage === 100 ? 'selected' : ''}>100</option></select></div>`;
    if (['record-search', 'record-filter-value'].includes(focusedId)) {
      const control = $(`#${focusedId}`);
      control?.focus();
      if (selectionStart !== null && selectionEnd !== null) control?.setSelectionRange(selectionStart, selectionEnd);
    }
  }

  async function editTable() {
    if (!state.table) return;
    const name = window.prompt(t('修改数据表名称'), state.table.name);
    if (name === null) return;
    if (!name.trim()) return toast(t('数据表名称不能为空'), true);
    try {
      state.table = await api(`/api/apps/${state.app.id}/collections/${encodeURIComponent(state.table.slug)}`, { method: 'PATCH', body: JSON.stringify({ name }) });
      state.tables = state.tables.map((table) => table.slug === state.table.slug ? state.table : table);
      await renderRecords();
      toast(t('数据表名称已更新'));
    } catch (error) { toast(error.message, true); }
  }

  async function deleteTable() {
    if (!state.table || !window.confirm(t('永久删除「{name}」及其中全部记录？此操作无法撤销。', { name: state.table.name }))) return;
    try {
      await api(`/api/apps/${state.app.id}/collections/${encodeURIComponent(state.table.slug)}`, { method: 'DELETE', body: JSON.stringify({ confirm: true }) });
      state.tables = state.tables.filter((table) => table.slug !== state.table.slug);
      state.table = state.tables[0] || null;
      await renderWorkspace();
      toast(t('数据表及其记录已删除'));
    } catch (error) { toast(error.message, true); }
  }

  function fieldKey(label, index) {
    const slug = label.toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 20);
    return slug || `field_${index + 1}`;
  }

  async function createTable(event) {
    event.preventDefault();
    const editingSchema = Boolean(state.editingTableSchema);
    const form = new FormData(event.currentTarget);
    const fields = $$('.table-field-row').slice(0, 24).map((row, index) => {
      const label = row.querySelector('[name="field-label"]').value.trim();
      const field = { name: row.querySelector('[name="field-name"]').value || fieldKey(label, index), label, type: row.querySelector('[name="field-type"]').value, required: row.querySelector('[name="field-required"]').checked };
      if (field.type === 'select') field.options = row.querySelector('[name="field-options"]').value.split(/[,，\n]/).map((value) => value.trim()).filter(Boolean);
      if (field.type === 'relation') field.target = row.querySelector('[name="field-target"]').value;
      return field;
    }).filter((field) => field.label);
    if (!fields.length) return toast(t('请至少填写一个字段'), true);
    if ($$('.table-field-row').length !== fields.length) return toast(t('请填写所有字段名称'), true);
    try {
      let table;
      if (editingSchema) {
        const currentNames = new Set(fields.map((field) => field.name));
        const remove_fields = state.table.fields.filter((field) => !currentNames.has(field.name)).map((field) => field.name);
        if (remove_fields.length && !window.confirm(t('永久删除字段 {fields} 及其所有数据？此操作无法撤销。', { fields: remove_fields.join(t('、')) }))) return;
        table = await api(`/api/apps/${state.app.id}/collections/${encodeURIComponent(state.table.slug)}`, { method: 'PATCH', body: JSON.stringify({ name: form.get('name'), fields, remove_fields, confirm_data_loss: remove_fields.length > 0 }) });
        state.tables = state.tables.map((item) => item.slug === table.slug ? table : item);
      } else {
        table = await api(`/api/apps/${state.app.id}/collections`, { method: 'POST', body: JSON.stringify({ name: form.get('name'), fields }) });
        state.tables.push(table);
      }
      state.table = table;
      state.editingTableSchema = false;
      event.currentTarget.reset();
      $('#table-fields-editor').replaceChildren();
      $('#table-dialog-title').textContent = t('新建数据表');
      $('#table-dialog-submit').textContent = t('创建数据表');
      $('#table-dialog').close();
      await renderRecords();
      toast(editingSchema ? t('数据表字段已更新') : t('数据表已创建'));
    } catch (error) {
      toast(error.message, true);
    }
  }

  function tableFieldRow(field = null) {
    const relationOptions = state.tables.map((table) => `<option value="${esc(table.slug)}">${esc(table.name)}</option>`).join('');
    const types = [['text', t('文本')], ['number', t('数字')], ['bool', t('是/否')], ['date', t('日期')], ['email', t('邮箱')], ['url', t('网址')], ['select', t('选项')], ['relation', t('关联记录')], ['file', t('附件')]];
    const options = types.map(([value, label]) => `<option value="${value}" ${field?.type === value ? 'selected' : ''}>${esc(label)}</option>`).join('');
    return `<div class="table-field-row"><input type="hidden" name="field-name" value="${esc(field?.name || '')}" /><label>${esc(t('字段名称'))}<input class="input input-sm" name="field-label" required placeholder="${esc(t('例如：状态'))}" value="${esc(field?.label || '')}" /></label><label>${esc(t('类型'))}<select class="select select-bordered select-sm" name="field-type" ${field ? 'disabled' : ''}>${options}</select></label><label class="field-required"><input class="checkbox checkbox-sm" type="checkbox" name="field-required" ${field?.required ? 'checked' : ''} />${esc(t('必填'))}</label><label class="field-options ${field?.type === 'select' ? '' : 'hidden'}">${esc(t('选项（逗号分隔）'))}<input class="input input-sm" name="field-options" placeholder="${esc(t('待办,进行中,完成'))}" value="${esc(field?.options?.join(', ') || '')}" /></label><label class="field-target ${field?.type === 'relation' ? '' : 'hidden'}">${esc(t('关联数据表'))}<select class="select select-bordered select-sm" name="field-target"><option value="">${esc(t('选择数据表'))}</option>${relationOptions.replace(`value="${esc(field?.target || '')}"`, `value="${esc(field?.target || '')}" selected`)}</select></label><button class="btn btn-ghost btn-xs" type="button" data-action="remove-table-field" aria-label="${esc(t('移除字段'))}">${esc(t('移除'))}</button></div>`;
  }

  function openTableSchemaEditor() {
    if (!state.table) return;
    state.editingTableSchema = true;
    $('#table-dialog-title').textContent = t('修改数据表和字段');
    $('#table-dialog-submit').textContent = t('保存修改');
    $('#create-table-form [name="name"]').value = state.table.name;
    $('#table-fields-editor').innerHTML = state.table.fields.map((field) => tableFieldRow(field)).join('');
    $('#table-dialog').showModal();
  }

  async function openRecordEditor(record = null) {
    const recordFields = state.table?.fields;
    if (!state.app || !recordFields?.length) return;
    const context = {
      appId: state.app.id,
      tenantId: state.tenant?.id,
      expectedUpdated: record?.updated_at,
      collection: state.table?.slug,
    };
    if (!context.tenantId || !context.collection) return;
    state.recordFormContext = context;
    state.editingRecordId = record?.id || null;
    $('#record-dialog-title').textContent = record ? t('编辑记录') : t('添加记录');
    $('#record-save').textContent = record ? t('保存修改') : t('添加记录');
    $('#record-form-error').classList.add('hidden');
    $('#record-form-error').textContent = '';
    const fields = await Promise.all(recordFields.map(async (field) => {
      const label = esc(field.label || field.name);
      const value = record?.data?.[field.name];
      const required = field.required ? ' required' : '';
      const common = `data-record-field="${esc(field.name)}"`;
      if (field.type === 'bool') {
        const selected = (candidate) => value === candidate ? ' selected' : '';
        return `<label>${label}<select class="select select-bordered w-full" ${common}${required}><option value=""${selected(undefined)}>${esc(t('请选择'))}</option><option value="true"${selected(true)}>${esc(t('是'))}</option><option value="false"${selected(false)}>${esc(t('否'))}</option></select></label>`;
      }
      if (field.type === 'select') {
        const options = (field.options || []).map((option) => `<option value="${esc(option)}" ${value === option ? 'selected' : ''}>${esc(option)}</option>`).join('');
        const emptyLabel = field.required ? t('请选择') : record ? t('清空当前选项') : t('不设置');
        const emptySelected = value === undefined || value === null || value === '' ? ' selected' : '';
        return `<label>${label}<select class="select select-bordered w-full" ${common}${required}><option value=""${emptySelected}>${emptyLabel}</option>${options}</select></label>`;
      }
      if (field.type === 'relation') {
        const target = state.tables.find((table) => table.slug === field.target);
        let records = [];
        if (target) {
          const result = await api(`/api/apps/${state.app.id}/collections/${encodeURIComponent(target.slug)}/records?perPage=100`);
          records = result.items || [];
          if (value && !records.some((row) => row.id === value)) {
            const selected = await api(`/api/apps/${state.app.id}/collections/${encodeURIComponent(target.slug)}/records/${encodeURIComponent(value)}`);
            records.push(selected);
          }
        }
        const labelField = target?.fields?.[0]?.name;
        return `<label>${label}<select class="select select-bordered w-full" ${common}${required}><option value="">${esc(t('选择关联记录'))}</option>${records.map((row) => `<option value="${esc(row.id)}" ${value === row.id ? 'selected' : ''}>${esc(row.data[labelField] || row.id)}</option>`).join('')}</select></label>`;
      }
      if (field.type === 'file') {
        const currentFile = record?.data?.[field.name];
        const fileLink = currentFile ? `<small><button class="btn btn-link btn-xs" type="button" data-download-file="${esc(field.name)}" data-record-id="${esc(record.id)}" data-file-name="${esc(currentFile)}" data-file-collection="${esc(context.collection)}">${esc(t('下载：{name}', { name: currentFile }))}</button> <label class="inline-checkbox"><input type="checkbox" data-clear-file="${esc(field.name)}" />${esc(t('移除'))}</label></small>` : '';
        return `<label>${label}<input class="file-input file-input-bordered w-full" type="file" ${common} accept="image/png,image/jpeg,image/gif,image/webp,application/pdf,text/plain" ${!record && field.required ? 'required' : ''} />${fileLink}</label>`;
      }
      const type = field.type === 'number' ? 'number' : ['date', 'email', 'url'].includes(field.type) ? field.type : 'text';
      const shownValue = field.type === 'date' && value ? String(value).slice(0, 10) : value ?? '';
      const step = field.type === 'number' ? ' step="any"' : '';
      return `<label>${label}<input class="input input-bordered w-full" type="${type}" ${common}${step}${required} value="${esc(shownValue)}" /></label>`;
    }));
    if (state.recordFormContext !== context || state.app?.id !== context.appId || state.tenant?.id !== context.tenantId) return;
    $('#record-form-fields').innerHTML = fields.join('');
    $('#record-dialog').showModal();
    $('#record-form-fields input, #record-form-fields select')?.focus();
  }

  async function submitRecord(event) {
    event.preventDefault();
    const context = state.recordFormContext;
    if (!context) return;
    const errorNode = $('#record-form-error');
    const fail = (message) => {
      errorNode.textContent = message;
      errorNode.classList.remove('hidden');
    };
    if (state.app?.id !== context.appId || state.tenant?.id !== context.tenantId) {
      fail(t('工作区或应用已切换，请关闭表单后重新打开。'));
      return;
    }
    const fields = state.table?.fields;
    if (!fields?.length) return;
    const editing = Boolean(state.editingRecordId);
    const saveButton = $('#record-save');
    if (saveButton.disabled) return;
    saveButton.disabled = true;
    saveButton.textContent = t('保存中…');
    errorNode.classList.add('hidden');
    errorNode.textContent = '';
    const data = {};
    const files = {};
    try {
      for (const field of fields) {
        const control = $(`[data-record-field="${CSS.escape(field.name)}"]`);
        if (!control) throw new Error(t('找不到字段「{field}」', { field: field.label || field.name }));
        if (field.type === 'file') {
          const remove = $(`[data-clear-file="${CSS.escape(field.name)}"]`)?.checked;
          if (control.files?.[0]) {
            const file = control.files[0];
            const encoded = await new Promise((resolve, reject) => {
              const reader = new FileReader();
              reader.onload = () => resolve(reader.result);
              reader.onerror = () => reject(reader.error);
              reader.readAsDataURL(file);
            });
            files[field.name] = { name: file.name, type: file.type, base64: encoded };
          } else if (remove) data[field.name] = '';
          continue;
        }
        const value = control.value;
        const optionalZeroValue = ['number', 'bool'].includes(field.type);
        if (value === '' && !field.required && optionalZeroValue) continue;
        if (field.type === 'number') data[field.name] = Number(value);
        else if (field.type === 'bool') data[field.name] = value === 'true';
        else data[field.name] = value;
      }
      if (state.recordFormContext !== context || state.app?.id !== context.appId || state.tenant?.id !== context.tenantId) {
        throw new Error(t('工作区或应用已切换，请关闭表单后重新打开。'));
      }
      const collection = `/api/apps/${encodeURIComponent(context.appId)}/collections/${encodeURIComponent(context.collection)}/records`;
      if (state.editingRecordId) {
        await api(`${collection}/${encodeURIComponent(state.editingRecordId)}`, { method: 'PATCH', body: JSON.stringify({ data, files, expected_updated_at: context.expectedUpdated }) });
      } else {
        await api(collection, { method: 'POST', body: JSON.stringify({ data, files }) });
      }
      const stillCurrent = state.app?.id === context.appId && state.tenant?.id === context.tenantId;
      $('#record-dialog').close();
      state.editingRecordId = null;
      state.recordFormContext = null;
      if (stillCurrent) {
        state.recordQuery.page = editing ? state.recordQuery.page : 1;
        await renderRecords();
        toast(editing ? t('记录已更新') : t('记录已添加'));
      } else {
        toast(t('记录已{action}到原应用；工作区已切换，当前列表未刷新。', { action: editing ? t('更新') : t('添加') }));
      }
    } catch (error) {
      fail(error.message || t('保存失败，请检查填写内容后重试。'));
    } finally {
      saveButton.disabled = false;
      saveButton.textContent = editing ? t('保存修改') : t('添加记录');
    }
  }

  async function deleteRecord(recordId) {
    const appId = state.app?.id;
    const tenantId = state.tenant?.id;
    const collectionSlug = state.table?.slug;
    if (!appId || !tenantId || !collectionSlug) return;
    const deleteKey = `${appId}:${collectionSlug}:${recordId}`;
    if (pendingDeletes.has(deleteKey)) return;
    if (!window.confirm(t('永久删除这条记录？此操作无法撤销。'))) return;
    pendingDeletes.add(deleteKey);
    try {
      await api(`/api/apps/${encodeURIComponent(appId)}/collections/${encodeURIComponent(collectionSlug)}/records/${encodeURIComponent(recordId)}`, { method: 'DELETE' });
      if (state.app?.id === appId && state.tenant?.id === tenantId) {
        await renderRecords();
      }
      toast(t('记录已删除'));
    } catch (error) {
      toast(error.message, true);
    } finally {
      pendingDeletes.delete(deleteKey);
    }
  }

  async function createFollowUpTask(recordId) {
    if (!state.app?.id || !state.table?.slug) return;
    try {
      const task = await api(`/api/apps/${encodeURIComponent(state.app.id)}/tasks/from-agent`, {
        method: 'POST',
        body: JSON.stringify({
          name: t('生成跟进建议'),
          goal: t('分析当前记录并生成一份可执行的跟进建议。'),
          table: state.table.slug,
          record_id: recordId,
        }),
      });
      state.appPanel = 'tasks';
      await renderWorkspace();
      toast(t('已创建后台任务：{name}', { name: task.name || t('生成跟进建议') }));
    } catch (error) {
      toast(error.message, true);
    }
  }


  async function handleClick(event) {
    const action = event.target.closest('[data-action]')?.dataset.action;
    if (action === 'create-table') {
      state.editingTableSchema = false;
      $('#table-dialog-title').textContent = t('新建数据表');
      $('#table-dialog-submit').textContent = t('创建数据表');
      $('#create-table-form').reset();
      $('#table-fields-editor').replaceChildren();
      if (!$('#table-fields-editor').children.length) $('#table-fields-editor').innerHTML = tableFieldRow();
      $('#table-dialog').showModal();
      return true;
    }
    if (action === 'edit-table-schema') { openTableSchemaEditor(); return true; }
    if (action === 'add-table-field') {
      if ($$('.table-field-row').length >= 24) { toast(t('每张数据表最多 24 个字段'), true); return true; }
      $('#table-fields-editor').insertAdjacentHTML('beforeend', tableFieldRow());
      return true;
    }
    if (action === 'remove-table-field') { event.target.closest('.table-field-row')?.remove(); return true; }
    if (action === 'edit-table') { editTable(); return true; }
    if (action === 'delete-table') { deleteTable(); return true; }
    if (action === 'clear-filter') {
      state.recordQuery.filterField = '';
      state.recordQuery.filterValue = '';
      state.recordQuery.page = 1;
      renderRecords();
      return true;
    }
    if (action === 'close-table') { $('#table-dialog').close(); state.editingTableSchema = false; return true; }
    if (action === 'add-record') { openRecordEditor().catch((error) => toast(error.message, true)); return true; }
    if (action === 'close-record') { $('#record-dialog').close(); return true; }

    const tableButton = event.target.closest('[data-table]');
    if (tableButton) {
      state.table = state.tables.find((item) => item.slug === tableButton.dataset.table) || null;
      state.recordQuery = { ...state.recordQuery, page: 1, search: '', filterField: '', filterValue: '' };
      await renderRecords();
      return true;
    }
    const pageButton = event.target.closest('[data-page]');
    if (pageButton) { state.recordQuery.page = Number(pageButton.dataset.page); renderRecords(); return true; }
    const deleteButton = event.target.closest('[data-delete-record]');
    if (deleteButton) { deleteRecord(deleteButton.dataset.deleteRecord); return true; }
    const followUpButton = event.target.closest('[data-agent-followup]');
    if (followUpButton) { createFollowUpTask(followUpButton.dataset.agentFollowup); return true; }
    const editButton = event.target.closest('[data-edit-record]');
    if (editButton) { openRecordEditor(state.records.find((record) => record.id === editButton.dataset.editRecord)); return true; }
    return false;
  }

  function handleChange(event) {
    if (event.target.matches('[name="field-type"]')) {
      const row = event.target.closest('.table-field-row');
      row.querySelector('.field-options').classList.toggle('hidden', event.target.value !== 'select');
      row.querySelector('.field-target').classList.toggle('hidden', event.target.value !== 'relation');
      return true;
    }
    if (event.target.matches('#record-search')) state.recordQuery.search = event.target.value;
    if (event.target.matches('#record-sort')) state.recordQuery.sort = event.target.value;
    if (event.target.matches('#record-filter-field')) state.recordQuery.filterField = event.target.value;
    if (event.target.matches('#record-filter-value')) state.recordQuery.filterValue = event.target.value;
    if (event.target.matches('#record-page-size')) state.recordQuery.perPage = Number(event.target.value);
    if (event.target.matches('#record-search, #record-sort, #record-filter-field, #record-filter-value, #record-page-size')) {
      state.recordQuery.page = 1;
      renderRecords();
      return true;
    }
    return false;
  }

  function bind() {
    $('#create-table-form')?.addEventListener('submit', createTable);
    $('#record-form')?.addEventListener('submit', submitRecord);
    $('#record-dialog')?.addEventListener('close', () => {
      state.editingRecordId = null;
      state.recordFormContext = null;
      $('#record-form-error').classList.add('hidden');
    });
  }

  return { bind, handleClick, handleChange, renderTables, renderNoTables, renderRecords };
}
