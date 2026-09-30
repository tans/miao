export function createAppTasks({ state, api, $, esc, toast }) {
  let generation = 0;
  let polling;
  let tasks = [];
  let currentRun = null;
  let taskPage = 1;
  let runPage = 1;
  let taskSignature = '';
  let memberNames = new Map();
  let tableLabels = new Map();
  let labelsLoadedAt = 0;
  const memberLabel = (id) => memberNames.get(id) || '已离开工作区的成员';
  const tableLabel = (slug) => tableLabels.get(slug)?.name || slug;
  const fieldLabel = (slug, name) => tableLabels.get(slug)?.fields?.find((field) => field.name === name)?.label || name;
  const status = (value) => ({ draft: '待启用', enabled: '已启用', paused: '已暂停', archived: '已归档', queued: '排队中', running: '执行中', waiting: '等待处理', completed: '已完成', partial: '部分完成', failed: '失败', cancelled: '已取消' })[value] || value;
  const date = (value) => value ? new Date(value).toLocaleString() : '—';
  const active = () => state.workspaceView === 'app' && state.appPanel === 'tasks' && state.app;
  const manage = (record) => state.tenant?.role === 'owner' || (state.app?.permission === 'publisher' && record.created_by === state.user.id);
  const base = () => `/api/apps/${encodeURIComponent(state.app.id)}`;
  function stop() { generation++; clearTimeout(polling); }
  function triggerText(trigger) {
    if (trigger.type === 'manual') return '手动运行';
    if (trigger.type === 'once') return `一次性 · ${date(trigger.at)}`;
    if (trigger.type === 'daily') return `每天 ${trigger.time} · ${trigger.timezone}`;
    if (trigger.type === 'weekly') return `每周 ${trigger.weekdays.map((day) => ['日', '一', '二', '三', '四', '五', '六'][day]).join('、')} ${trigger.time} · ${trigger.timezone}`;
    return trigger.type === 'record_created' ? `${tableLabel(trigger.table)} 新增记录` : `${tableLabel(trigger.table)} · ${fieldLabel(trigger.table, trigger.field)}：${trigger.from} → ${trigger.to}`;
  }
  function scopeText(task) {
    const { scope, limits } = task.definition;
    return `执行方式：${task.definition.execution === 'agent' ? '后台 Agent' : '固定第一页数据快照'}\n${scope.tables.map((table) => `${tableLabel(table.table)}：读取 ${table.read_fields.map((field) => fieldLabel(table.table, field)).join('、')}；自动写入 ${table.write_fields.map((field) => fieldLabel(table.table, field)).join('、') || '无'}`).join('\n')}\n通知接收人：${scope.recipient_ids.map(memberLabel).join('、') || '仅负责人'}\n单次最多写入 ${limits.max_writes} 条；最多 ${limits.max_requests} 次模型请求；超时 ${limits.timeout_seconds} 秒；待处理期限 ${limits.confirmation_timeout_hours || 72} 小时`;
  }
  async function showRun(runId) {
    const prefix = base();
    const context = `${state.tenant.id}:${state.app.id}`;
    const run = await api(`${prefix}/runs/${encodeURIComponent(runId)}`);
    if (!active() || context !== `${state.tenant?.id}:${state.app?.id}`) return;
    currentRun = run;
    const pending = run.status === 'waiting' && run.pending;
    $('#task-run-detail').innerHTML = `<div class="task-detail-heading"><h3>${esc(run.task_name)}${run.mode === 'preview' ? ' · 只读试运行' : ''} · ${esc(status(run.status))}</h3><button class="btn btn-ghost btn-sm" data-task-action="close-run">收起</button></div>
      <p class="task-muted">版本 ${esc(run.revision)} · 开始 ${esc(date(run.started_at))} · 执行段 ${esc(run.attempts)} 次 · 重试 ${esc(run.retry_count || 0)} 次 · 模型请求 ${esc(run.model_requests)}</p>
      ${run.error ? `<p class="task-error">${esc(run.error)}</p>` : ''}
      ${pending ? `<div class="task-pending"><strong>${esc(pending.reason)}</strong><p class="task-muted">处理期限 ${esc(date(pending.expires_at))}</p>${pending.evidence ? `<div class="task-diff"><div><small>修改前</small><pre>${esc(JSON.stringify(pending.evidence.before, null, 2))}</pre></div><div><small>具体变更</small><pre>${esc(JSON.stringify(pending.input, null, 2))}</pre></div></div>` : ''}
        ${manage(run) ? `${pending.kind === 'information' ? '<label>补充信息<textarea id="task-answer" class="textarea" rows="3" maxlength="6000"></textarea></label>' : ''}<div class="task-buttons"><button class="btn btn-primary btn-sm" data-task-action="resolve">${pending.kind === 'information' ? '提交并继续' : pending.kind === 'uncertain' ? '核实当前记录并继续' : '批准本次具体变更'}</button><button class="btn btn-ghost btn-sm" data-task-action="reject">拒绝并结束</button></div>` : '<p>等待任务负责人或工作区所有者处理。</p>'}</div>` : ''}
      ${run.output ? `<pre class="task-output">${esc(run.output)}</pre>` : '<p class="task-muted">尚无文本结果。</p>'}
      <details><summary>动作记录 · ${run.actions.length}</summary>${run.actions.map((action) => `<div class="task-action-log"><strong>${esc(action.tool)} · ${esc(action.status)}</strong><pre>${esc(JSON.stringify({ input: action.input, result: action.result, evidence: action.evidence }, null, 2))}</pre></div>`).join('') || '<p>暂无写入动作。</p>'}</details>
      <details><summary>执行历史 · ${(run.attempt_history || []).length}</summary>${(run.attempt_history || []).map((attempt) => `<div class="task-action-log"><strong>第 ${esc(attempt.sequence)} 段 · ${esc(status(attempt.status))}</strong><p class="task-muted">${esc(date(attempt.started_at))} → ${esc(date(attempt.finished_at))} · ${esc(attempt.model_requests)} 次模型请求</p><pre>${esc(attempt.error || attempt.output || '尚无结果')}</pre></div>`).join('')}</details>
      ${manage(run) ? `<div class="task-buttons">${['queued', 'running', 'waiting'].includes(run.status) ? '<button class="btn btn-ghost btn-sm" data-task-action="cancel">取消运行</button>' : ''}${['failed', 'partial'].includes(run.status) && Number(run.retry_count || 0) < 2 ? '<button class="btn btn-outline btn-sm" data-task-action="retry">重试未完成部分</button>' : ''}</div>` : ''}`;
  }
  async function load() {
    stop();
    if (!active()) return;
    const revision = generation;
    const prefix = base();
    const reloadLabels = Date.now() - labelsLoadedAt > 60000;
    const result = await Promise.all([api(`${prefix}/tasks?page=${taskPage}`), api(`${prefix}/runs?page=${runPage}`), reloadLabels ? api('/api/workspace/members') : null, reloadLabels ? api(`${prefix}/collections`) : null]).catch((error) => {
      if (revision === generation && active()) $('#task-list').innerHTML = `<p class="task-error">${esc(error.message)}</p>`;
      return null;
    });
    if (!result || revision !== generation || !active()) return;
    if (reloadLabels) { memberNames = new Map(result[2].members.map((member) => [member.id, member.name || member.email])); tableLabels = new Map(result[3].map((table) => [table.slug, table])); labelsLoadedAt = Date.now(); }
    tasks = result[0].items;
    const signature = JSON.stringify([tasks, [...memberNames], [...tableLabels]]);
    if (signature !== taskSignature) {
    $('#task-list').innerHTML = tasks.map((task) => `<article class="task-card"><div class="task-detail-heading"><strong>${esc(task.name)}</strong><span class="badge badge-ghost">${esc(status(task.status))}</span></div><p>${esc(task.definition.goal)}</p><p class="task-muted">${esc(triggerText(task.definition.trigger))} · 负责人 ${esc(memberLabel(task.created_by))} · 版本 ${task.revision}${task.next_run_at ? ` · 下次 ${esc(date(task.next_run_at))}` : ''}</p>
      <details><summary>查看执行范围与授权</summary><pre>${esc(scopeText(task))}</pre></details>${task.pause_reason ? `<p class="task-muted">${esc(task.pause_reason)}</p>` : ''}
      ${manage(task) ? `<div class="task-buttons">${task.status === 'archived' ? `<button class="btn btn-outline btn-sm" data-task-action="restore" data-task-id="${esc(task.id)}">恢复为草稿</button>` : `<button class="btn btn-ghost btn-sm" data-task-action="preview" data-task-id="${esc(task.id)}">只读试运行</button>${task.status === 'enabled' ? `<button class="btn btn-outline btn-sm" data-task-action="run" data-task-id="${esc(task.id)}">立即运行</button><button class="btn btn-ghost btn-sm" data-task-action="pause" data-task-id="${esc(task.id)}">暂停</button>` : `<button class="btn btn-primary btn-sm" data-task-action="enable" data-task-id="${esc(task.id)}">审阅并启用</button>`}<button class="btn btn-ghost btn-sm" data-task-action="archive" data-task-id="${esc(task.id)}">归档</button>`}</div>` : ''}</article>`).join('') || '<div class="dashboard-empty"><strong>还没有后台任务</strong><span>告诉 fx 运行目标、时间和允许执行的动作，先审阅，再启用。</span></div>';
    taskSignature = signature;
    }
    const pager = (kind, result, page) => `<button class="btn btn-ghost btn-sm" data-task-action="${kind}-page" data-page="${page - 1}" ${page <= 1 ? 'disabled' : ''}>上一页</button><span>${page} / ${Math.max(1, result.totalPages)} · ${result.totalItems} 项</span><button class="btn btn-ghost btn-sm" data-task-action="${kind}-page" data-page="${page + 1}" ${page >= result.totalPages ? 'disabled' : ''}>下一页</button>`;
    $('#task-list-pager').innerHTML = pager('tasks', result[0], taskPage);
    $('#task-runs-pager').innerHTML = pager('runs', result[1], runPage);
    $('#task-runs').innerHTML = result[1].items.map((run) => `<button class="task-run-row" data-task-run="${esc(run.id)}"><span><strong>${esc(run.task_name)}${run.mode === 'preview' ? ' · 试运行' : ''}</strong><small>${esc(date(run.created))}${run.error ? ` · ${esc(run.error)}` : ''}</small></span><span class="badge badge-ghost">${esc(status(run.status))}</span></button>`).join('') || '<p class="task-muted">暂无运行记录。</p>';
    if (currentRun && !$('#task-run-detail').contains(document.activeElement)) await showRun(currentRun.id).catch((error) => toast(error.message, true));
    if (revision === generation && active()) polling = setTimeout(() => { void load(); }, 5000);
  }
  async function handleClick(event) {
    const open = event.target.closest('[data-task-run]');
    const button = event.target.closest('[data-task-action]');
    if (!open && !button) return false;
    try {
      if (open) { await showRun(open.dataset.taskRun); return true; }
      button.disabled = true;
      const action = button.dataset.taskAction;
      if (action === 'tasks-page') { taskPage = Number(button.dataset.page); await load(); return true; }
      if (action === 'runs-page') { runPage = Number(button.dataset.page); await load(); return true; }
      if (action === 'refresh') { await load(); return true; }
      if (action === 'close-run') { currentRun = null; $('#task-run-detail').replaceChildren(); return true; }
      const task = tasks.find((item) => item.id === button.dataset.taskId);
      if (task) {
        if (action === 'archive' && !window.confirm(`归档 ${task.name} 并取消未结束运行？已完成的写入不会撤销。`)) return true;
        if (action === 'enable' && !window.confirm(`启用 ${task.name}？\n${task.definition.goal}\n${triggerText(task.definition.trigger)}\n${scopeText(task)}\n浏览器关闭后仍运行；授权范围内自动执行，退出登录不会停用。`)) return true;
        const body = { confirm: true, expected_revision: task.revision, request_id: crypto.randomUUID() };
        const result = await api(`${base()}/tasks/${task.id}/${action}`, { method: 'POST', body: JSON.stringify(body) });
        if (['run', 'preview'].includes(action)) { currentRun = result; runPage = 1; }
      } else if (currentRun) {
        const body = { confirm: true, expected_updated_at: currentRun.updated };
        let endpoint = action;
        if (['resolve', 'reject'].includes(action)) {
          endpoint = 'resolve'; body.decision = action === 'reject' ? 'reject' : 'approve';
          if (currentRun.pending.kind === 'information') body.answer = $('#task-answer')?.value || '';
          if (action === 'resolve' && currentRun.pending.kind !== 'information' && !window.confirm('确认处理页面中展示的这一次具体变更？此操作不扩大后续任务的授权。')) return true;
        }
        if (action === 'retry' && !window.confirm('保留已成功动作，只重试未完成部分？')) return true;
        if (action === 'cancel' && !window.confirm('停止剩余动作？已经完成的操作不会自动撤销。')) return true;
        currentRun = await api(`${base()}/runs/${currentRun.id}/${endpoint}`, { method: 'POST', body: JSON.stringify(body) });
      }
      button.blur();
      await load();
    } catch (error) { toast(error.message, true); }
    finally { if (button) button.disabled = false; }
    return true;
  }
  return { load, stop, handleClick, showRun, reset() { stop(); currentRun = null; tasks = []; taskPage = 1; runPage = 1; taskSignature = ''; memberNames.clear(); tableLabels.clear(); labelsLoadedAt = 0;  $('#task-run-detail')?.replaceChildren(); } };
}
