import { t, fmtDateTime } from '/modules/i18n.js';

export function createAppTasks({ state, api, $, esc, toast }) {
  let generation = 0;
  let polling;
  let tasks = [];
  let currentRun = null;
  let detailGeneration = 0;
  let taskPage = 1;
  let runPage = 1;
  let taskSignature = '';
  let memberNames = new Map();
  let tableLabels = new Map();
  let labelsLoadedAt = 0;
  const memberLabel = (id) => memberNames.get(id) || t('已离开工作区的成员');
  const tableLabel = (slug) => tableLabels.get(slug)?.name || slug;
  const fieldLabel = (slug, name) => tableLabels.get(slug)?.fields?.find((field) => field.name === name)?.label || name;
  const status = (value) => ({ draft: t('待启用'), enabled: t('已启用'), paused: t('已暂停'), archived: t('已归档'), queued: t('排队中'), running: t('执行中'), waiting: t('等待处理'), completed: t('已完成'), partial: t('部分完成'), failed: t('失败'), cancelled: t('已取消') })[value] || value;
  const date = (value) => value ? fmtDateTime(value) : '—';
  const active = () => state.workspaceView === 'app' && state.appPanel === 'tasks' && state.app;
  const manage = (record) => state.tenant?.role === 'owner' || (state.app?.permission === 'publisher' && record.created_by === state.user.id);
  const base = () => `/api/apps/${encodeURIComponent(state.app.id)}`;
  function stop() { generation++; detailGeneration++; clearTimeout(polling); }
  function triggerText(trigger) {
    if (trigger.type === 'manual') return t('手动运行');
    if (trigger.type === 'once') return t('一次性 · {time}', { time: date(trigger.at) });
    if (trigger.type === 'daily') return t('每天 {time} · {timezone}', { time: trigger.time, timezone: trigger.timezone });
    if (trigger.type === 'weekly') return t('每周 {days} {time} · {timezone}', { days: trigger.weekdays.map((day) => ['日', '一', '二', '三', '四', '五', '六'][day]).join(t('、')), time: trigger.time, timezone: trigger.timezone });
    return trigger.type === 'record_created' ? t('{table} 新增记录', { table: tableLabel(trigger.table) }) : t('{table} · {field}：{from} → {to}', { table: tableLabel(trigger.table), field: fieldLabel(trigger.table, trigger.field), from: trigger.from, to: trigger.to });
  }
  function scopeText(task) {
    const { scope, limits } = task.definition;
    return t('执行方式：{execution}\n{tables}\n通知接收人：{recipients}\n{limits}', {
      execution: task.definition.execution === 'agent' ? t('后台 Agent') : t('固定第一页数据快照'),
      tables: scope.tables.map((table) => t('{table}：读取 {read}；自动写入 {write}', { table: tableLabel(table.table), read: table.read_fields.map((field) => fieldLabel(table.table, field)).join(t('、')), write: table.write_fields.map((field) => fieldLabel(table.table, field)).join(t('、')) || t('无') })).join('\n'),
      recipients: scope.recipient_ids.map(memberLabel).join(t('、')) || t('仅负责人'),
      limits: t('单次最多写入 {writes} 条；最多 {requests} 次模型请求；超时 {timeout} 秒；待处理期限 {hours} 小时', { writes: limits.max_writes, requests: limits.max_requests, timeout: limits.timeout_seconds, hours: limits.confirmation_timeout_hours || 72 })
    });
  }
  async function showRun(runId) {
    const revision = ++detailGeneration;
    const prefix = base();
    const context = `${state.tenant.id}:${state.app.id}`;
    const run = await api(`${prefix}/runs/${encodeURIComponent(runId)}`);
    if (revision !== detailGeneration || !active() || context !== `${state.tenant?.id}:${state.app?.id}`) return;
    currentRun = run;
    const pending = run.status === 'waiting' && run.pending;
    $('#task-run-detail').innerHTML = `<div class="task-detail-heading"><h3>${esc(run.task_name)}${run.mode === 'preview' ? t(' · 只读试运行') : ''} · ${esc(status(run.status))}</h3><button class="btn btn-ghost btn-sm" data-task-action="close-run">${esc(t('收起'))}</button></div>
      <p class="task-muted">${esc(t('版本 {revision} · 开始 {start} · 执行段 {attempts} 次 · 重试 {retries} 次 · 模型请求 {requests}', { revision: run.revision, start: date(run.started_at), attempts: run.attempts, retries: run.retry_count || 0, requests: run.model_requests }))}</p>
      ${run.error ? `<p class="task-error">${esc(run.error)}</p>` : ''}
      ${pending ? `<div class="task-pending"><strong>${esc(pending.reason)}</strong><p class="task-muted">${esc(t('处理期限 {time}', { time: date(pending.expires_at) }))}</p>${pending.evidence ? `<div class="task-diff"><div><small>${esc(t('修改前'))}</small><pre>${esc(JSON.stringify(pending.evidence.before, null, 2))}</pre></div><div><small>${esc(t('具体变更'))}</small><pre>${esc(JSON.stringify(pending.input, null, 2))}</pre></div></div>` : ''}
        ${manage(run) ? `${pending.kind === 'information' ? `<label>${esc(t('补充信息'))}<textarea id="task-answer" class="textarea" rows="3" maxlength="6000"></textarea></label>` : ''}<div class="task-buttons"><button class="btn btn-primary btn-sm" data-task-action="resolve">${pending.kind === 'information' ? esc(t('提交并继续')) : pending.kind === 'uncertain' ? esc(t('核实当前记录并继续')) : esc(t('批准本次具体变更'))}</button><button class="btn btn-ghost btn-sm" data-task-action="reject">${esc(t('拒绝并结束'))}</button></div>` : `<p>${esc(t('等待任务负责人或工作区所有者处理。'))}</p>`}</div>` : ''}
      ${run.output ? `<pre class="task-output">${esc(run.output)}</pre>` : `<p class="task-muted">${esc(t('尚无文本结果。'))}</p>`}
      <details><summary>${esc(t('动作记录 · {count}', { count: run.actions.length }))}</summary>${run.actions.map((action) => `<div class="task-action-log"><strong>${esc(action.tool)} · ${esc(action.status)}</strong><pre>${esc(JSON.stringify({ input: action.input, result: action.result, evidence: action.evidence }, null, 2))}</pre></div>`).join('') || `<p>${esc(t('暂无写入动作。'))}</p>`}</details>
      <details><summary>${esc(t('执行历史 · {count}', { count: (run.attempt_history || []).length }))}</summary>${(run.attempt_history || []).map((attempt) => `<div class="task-action-log"><strong>${esc(t('第 {n} 段', { n: attempt.sequence }))} · ${esc(status(attempt.status))}</strong><p class="task-muted">${esc(date(attempt.started_at))} → ${esc(date(attempt.finished_at))} · ${esc(t('{count} 次模型请求', { count: attempt.model_requests }))}</p><pre>${esc(attempt.error || attempt.output || t('尚无结果'))}</pre></div>`).join('')}</details>
      ${manage(run) ? `<div class="task-buttons">${['queued', 'running', 'waiting'].includes(run.status) ? `<button class="btn btn-ghost btn-sm" data-task-action="cancel">${esc(t('取消运行'))}</button>` : ''}${['failed', 'partial'].includes(run.status) && Number(run.retry_count || 0) < 2 ? `<button class="btn btn-outline btn-sm" data-task-action="retry">${esc(t('重试未完成部分'))}</button>` : ''}</div>` : ''}`;
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
    $('#task-list').innerHTML = tasks.map((task) => `<article class="task-card"><div class="task-detail-heading"><strong>${esc(task.name)}</strong><span class="badge badge-ghost">${esc(status(task.status))}</span></div><p>${esc(task.definition.goal)}</p><p class="task-muted">${esc(triggerText(task.definition.trigger))} · ${esc(t('负责人 {owner} · 版本 {revision}{next}', { owner: memberLabel(task.created_by), revision: task.revision, next: task.next_run_at ? t(' · 下次 {time}', { time: date(task.next_run_at) }) : '' }))}</p>
      <details><summary>${esc(t('查看执行范围与授权'))}</summary><pre>${esc(scopeText(task))}</pre></details>${task.pause_reason ? `<p class="task-muted">${esc(task.pause_reason)}</p>` : ''}
      ${manage(task) ? `<div class="task-buttons">${task.status === 'archived' ? `<button class="btn btn-outline btn-sm" data-task-action="restore" data-task-id="${esc(task.id)}">${esc(t('恢复为草稿'))}</button>` : `<button class="btn btn-ghost btn-sm" data-task-action="preview" data-task-id="${esc(task.id)}">${esc(t('只读试运行'))}</button>${task.status === 'enabled' ? `<button class="btn btn-outline btn-sm" data-task-action="run" data-task-id="${esc(task.id)}">${esc(t('立即运行'))}</button><button class="btn btn-ghost btn-sm" data-task-action="pause" data-task-id="${esc(task.id)}">${esc(t('暂停'))}</button>` : `<button class="btn btn-primary btn-sm" data-task-action="enable" data-task-id="${esc(task.id)}">${esc(t('审阅并启用'))}</button>`}<button class="btn btn-ghost btn-sm" data-task-action="archive" data-task-id="${esc(task.id)}">${esc(t('归档'))}</button>`}</div>` : ''}</article>`).join('') || `<div class="dashboard-empty"><strong>${esc(t('还没有后台任务'))}</strong></div>`;
    taskSignature = signature;
    }
    const pager = (kind, result, page) => `<button class="btn btn-ghost btn-sm" data-task-action="${kind}-page" data-page="${page - 1}" ${page <= 1 ? 'disabled' : ''}>${esc(t('上一页'))}</button><span>${t('{page} / {pages} · {total} 项', { page, pages: Math.max(1, result.totalPages), total: result.totalItems })}</span><button class="btn btn-ghost btn-sm" data-task-action="${kind}-page" data-page="${page + 1}" ${page >= result.totalPages ? 'disabled' : ''}>${esc(t('下一页'))}</button>`;
    $('#task-list-pager').innerHTML = pager('tasks', result[0], taskPage);
    $('#task-runs-pager').innerHTML = pager('runs', result[1], runPage);
    $('#task-runs').innerHTML = result[1].items.map((run) => `<button class="task-run-row" data-task-run="${esc(run.id)}"><span><strong>${esc(run.task_name)}${run.mode === 'preview' ? t(' · 试运行') : ''}</strong><small>${esc(date(run.created))}${run.error ? ` · ${esc(run.error)}` : ''}</small></span><span class="badge badge-ghost">${esc(status(run.status))}</span></button>`).join('') || `<p class="task-muted">${esc(t('暂无运行记录。'))}</p>`;
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
      if (action === 'close-run') { detailGeneration++; currentRun = null; $('#task-run-detail').replaceChildren(); return true; }
      const task = tasks.find((item) => item.id === button.dataset.taskId);
      if (task) {
        if (action === 'archive' && !window.confirm(t('归档 {name} 并取消未结束运行？已完成的写入不会撤销。', { name: task.name }))) return true;
        if (action === 'enable' && !window.confirm(t('启用 {name}？\n{goal}\n{trigger}\n{scope}\n浏览器关闭后仍运行；授权范围内自动执行，退出登录不会停用。', { name: task.name, goal: task.definition.goal, trigger: triggerText(task.definition.trigger), scope: scopeText(task) }))) return true;
        const body = { confirm: true, expected_revision: task.revision, request_id: crypto.randomUUID() };
        const result = await api(`${base()}/tasks/${task.id}/${action}`, { method: 'POST', body: JSON.stringify(body) });
        if (['run', 'preview'].includes(action)) { currentRun = result; runPage = 1; }
      } else if (currentRun) {
        const body = { confirm: true, expected_updated_at: currentRun.updated };
        let endpoint = action;
        if (['resolve', 'reject'].includes(action)) {
          endpoint = 'resolve'; body.decision = action === 'reject' ? 'reject' : 'approve';
          if (currentRun.pending.kind === 'information') body.answer = $('#task-answer')?.value || '';
          if (action === 'resolve' && currentRun.pending.kind !== 'information' && !window.confirm(t('确认处理页面中展示的这一次具体变更？此操作不扩大后续任务的授权。'))) return true;
        }
        if (action === 'retry' && !window.confirm(t('保留已成功动作，只重试未完成部分？'))) return true;
        if (action === 'cancel' && !window.confirm(t('停止剩余动作？已经完成的操作不会自动撤销。'))) return true;
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
