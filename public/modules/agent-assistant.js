import { formatUIChanges } from './ui-changes.js';
import { createServerConversationStore, authorizationScope } from '/modules/agent-conversation-store.js';
import { t, fmtNumber, fmtDateTime } from '/modules/i18n.js';

export function createAgentAssistant({ state, api, stream, $, esc, toast, icon, renderWorkspace, openRecord }) {
  const conversations = createServerConversationStore(api);
  let rendererPromise = null;
  const loadRenderer = () => {
    if (!rendererPromise) {
      rendererPromise = import('./ui-renderer.bundle.js').catch((error) => {
        rendererPromise = null;
        throw error;
      });
    }
    return rendererPromise;
  };
  state.agentConversationMessages ||= [];
  state.agentConversationRevision ||= 0;
  state.agentRun ||= null;
  state.agentConversationCache ||= new Map();
  const workspaceRunKey = () => `miao-agent-run:${state.user?.id || ''}:${state.tenant?.id || ''}`;
  const conversationRunKey = (bucket) => `miao-agent-run:${state.user?.id || ''}:${state.tenant?.id || ''}:${bucket?.appId || ''}:${bucket?.id || ''}`;
  const activeConversationKey = (appId) => `miao-active-conversation:${state.user?.id || ''}:${state.tenant?.id || ''}:${appId || 'workspace'}`;
  const conversationScope = (appId = state.app?.id || '') => authorizationScope(state.tenant, state.apps || [], state.user?.id, appId);
  const conversationKey = (appId, id) => `${state.user?.id || ''}:${state.tenant?.id || ''}:${appId || ''}:${id || ''}`;
  const isCurrentApp = (appId) => (state.app?.id || '') === (appId || '');
  const isCurrentBucket = (bucket) => Boolean(bucket && state.agentConversationBucket === bucket && isCurrentApp(bucket.appId));
  const chatRoot = (bucket = state.agentConversationBucket) => isCurrentBucket(bucket) ? $('#chat-messages') : bucket?.root || $('#chat-messages');
  function getConversationBucket(appId, id, scope, messages = [], revision = 0) {
    const key = conversationKey(appId, id);
    let bucket = state.agentConversationCache.get(key);
    if (!bucket) {
      bucket = { key, appId: appId || '', id, scope, messages, revision, root: document.createElement('div'), rendered: false };
      state.agentConversationCache.set(key, bucket);
    } else {
      bucket.scope = scope;
      bucket.messages = messages;
      bucket.revision = revision;
    }
    return bucket;
  }
  function activateConversation(bucket) {
    const current = state.agentConversationBucket;
    const messages = $('#chat-messages');
    if (current && current !== bucket) while (messages.firstChild) current.root.append(messages.firstChild);
    state.agentConversationBucket = bucket;
    state.agentConversationId = bucket.id;
    state.agentConversationAppId = bucket.appId;
    state.agentConversationMessages = bucket.messages;
    state.agentConversationRevision = bucket.revision;
    state.agentConversationLoadedKey = bucket.key;
    if (current !== bucket) messages.append(...bucket.root.childNodes);
    state.agentRunKey = conversationRunKey(bucket);
    state.agentRun = localStorage.getItem(state.agentRunKey) || (!bucket.appId ? localStorage.getItem(workspaceRunKey()) : null);
    if (state.agentRun && !localStorage.getItem(state.agentRunKey)) localStorage.setItem(state.agentRunKey, state.agentRun);
  }
  const terminalStates = new Set(['completed', 'failed', 'cancelled', 'budget_exhausted', 'unsupported', 'unavailable']);
  const operationLabels = { 'apps.create': t('创建应用'), 'backend_plan.apply': t('应用后端变更'), 'ui.compose': t('创建界面草稿'), 'ui.publish': t('发布正式界面'), 'ui.publish.public': t('发布并开启匿名公开'), 'requirements.collect': t('整理需求'), 'records.query': t('查询记录'), 'business_actions.execute': t('执行业务动作') };
  const phaseLabels = { observe: t('读取资源'), enumerate: t('枚举候选'), decide: t('模型判断'), validate: t('校验方案'), execute: t('执行变更'), record: t('记录回执'), reconciled: t('恢复核对'), confirmation: t('等待确认'), execution_waiting: t('等待补充信息'), complete: t('收尾完成') };
  const eventLabels = { confirmation_required: t('等待确认'), waiting: t('等待补充信息'), confirmed: t('已确认，继续执行'), completed: t('已完成'), failed: t('运行失败'), cancelled: t('已取消'), budget_exhausted: t('预算耗尽'), unsupported: t('暂不支持') };
  const runStateLabels = { completed: t('已完成'), failed: t('运行失败'), cancelled: t('已取消'), budget_exhausted: t('预算耗尽'), unsupported: t('暂不支持'), unknown: t('待核实'), waiting_confirmation: t('等待确认') };

  function durationText(milliseconds) {
    const value = Math.max(0, Number(milliseconds) || 0);
    if (value < 1000) return `${Math.round(value)} ms`;
    if (value < 60000) return `${(value / 1000).toFixed(1)} s`;
    return `${Math.floor(value / 60000)} min ${Math.round((value % 60000) / 1000)} s`;
  }

  function stepDuration(step) {
    if (!step?.started_at) return '';
    const start = new Date(step.started_at).getTime();
    const end = step.completed_at ? new Date(step.completed_at).getTime() : Date.now();
    return Number.isFinite(start) && Number.isFinite(end) ? durationText(end - start) : '';
  }

  function renderRunSnapshot(details, run) {
    const summary = details.querySelector('.agent-run-summary');
    const steps = details.querySelector('.agent-run-steps');
    const loop = run?.loop || {};
    const usage = run?.usage?.totals;
    const started = run?.created_at ? new Date(run.created_at).getTime() : 0;
    const updated = run?.updated_at ? new Date(run.updated_at).getTime() : 0;
    const activeMs = Number(loop.active_duration || 0) / 1e6 || (started && updated ? updated - started : 0);
    const input = usage ? (usage.input_unknown ? t('未知') : fmtNumber(usage.input_tokens || 0)) : t('未知');
    const output = usage ? (usage.output_unknown ? t('未知') : fmtNumber(usage.output_tokens || 0)) : t('未知');
    if (summary) summary.textContent = t('耗时 {duration} · 模型请求 {requests} 次 · 输入 {input} / 输出 {output} tokens', { duration: durationText(activeMs), requests: fmtNumber(loop.model_requests || usage?.requests || 0), input, output });
    if (!steps) return;
    steps.replaceChildren();
    for (const [index, step] of (loop.steps || []).entries()) {
      const item = document.createElement('li');
      const capability = operationLabels[step.candidate?.capability] || step.candidate?.capability || t('执行步骤');
      const time = step.started_at ? `${fmtDateTime(step.started_at)} · ${t('耗时')} ${stepDuration(step)}` : '';
      item.textContent = `${index + 1}. ${capability}${time ? ` · ${time}` : ''}`;
      steps.append(item);
    }
  }

  function runTraceElement(runID, open, summary = t('开始处理')) {
    const details = document.createElement('details');
    details.className = 'agent-run-trace';
    details.dataset.agentTrace = runID;
    if (open) details.open = true;
    details.innerHTML = '<summary></summary><div class="agent-run-summary"></div><ol class="agent-run-steps"></ol><ol class="agent-run-phases"></ol>';
    details.querySelector('summary').textContent = summary;
    return details;
  }

  function traceGoalText(goal, appId) {
    goal = String(goal || '').trim();
    if (!goal) return '';
    const trimmed = goal.length > 48 ? `${goal.slice(0, 48)}…` : goal;
    const appName = appId ? state.apps?.find((item) => item.id === appId)?.name || t('当前应用') : '';
    return `${t('目标：{goal} · 应用：{app}', { goal: trimmed, app: appName || t('工作台') })}`;
  }

  function traceSummary(label, goal, appId) {
    const context = traceGoalText(goal, appId);
    return context ? `${label} · ${context}` : label;
  }

  function appendTraceItem(details, text) {
    if (!text) return;
    const item = document.createElement('li');
    item.textContent = text;
    details.querySelector('.agent-run-phases').append(item);
    const messages = details.closest('.chat-messages') || $('#chat-messages');
    if (messages) messages.scrollTop = messages.scrollHeight;
  }

  function consumeTraceEvent(details, event) {
    const type = event.type;
    if (type === 'phase') {
      appendTraceItem(details, phaseLabels[event.data?.phase]);
    } else if (type === 'candidate') {
      const label = operationLabels[event.data?.capability] || event.data?.capability;
      appendTraceItem(details, label ? t('候选操作：{label}{write}', { label, write: event.data?.write ? t('（写入，需确认）') : '' }) : '');
    } else if (eventLabels[type]) {
      appendTraceItem(details, eventLabels[type]);
    }
  }

  async function attachArchivedTrace(bubble, message, bucket) {
    const details = runTraceElement(message.run_id, false);
    details.querySelector('.agent-run-phases').dataset.pending = '1';
    bubble.after(details);
    const finish = (summary) => { details.querySelector('summary').textContent = summary; delete details.querySelector('.agent-run-phases').dataset.pending; };
    try {
      const [runResponse, eventsResponse] = await Promise.all([
        api(`/api/agent/runs/${encodeURIComponent(message.run_id)}`),
        api(`/api/agent/runs/${encodeURIComponent(message.run_id)}/events?after=0`),
      ]);
      const run = runResponse.run || runResponse;
      if (runResponse.usage) run.usage = runResponse.usage;
      finish(traceSummary(`${t('运行记录')} · ${runStateLabels[run.state] || run.state}`, run.prompt, run.app_id));
      renderRunSnapshot(details, run);
      appendTraceItem(details, traceGoalText(run.prompt, run.app_id));
      let after = 0;
      for (const event of eventsResponse.events || []) {
        after = Math.max(after, Number(event.sequence) || after);
        consumeTraceEvent(details, event);
      }
      details.dataset.after = String(Math.max(after, Number(eventsResponse.next_sequence) || 0));
    } catch (error) {
      finish(`${t('运行记录')} · ${t('明细已归档')}`);
    }
    const clear = document.createElement('button');
    clear.type = 'button';
    clear.className = 'btn btn-ghost btn-xs agent-run-trace-clear';
    clear.textContent = t('清除记录');
    clear.onclick = async () => {
      if (!window.confirm(t('清除这条运行记录的展示？运行回执仍按审计要求保留在服务端。'))) return;
      const index = bucket.messages.indexOf(message);
      if (index >= 0) {
        bucket.messages.splice(index, 1);
        if (isCurrentBucket(bucket)) state.agentConversationMessages = bucket.messages;
        await persistConversation(bucket);
      }
      bubble.remove();
      details.remove();
    };
    details.append(clear);
  }

  async function createTemplate(templateId) {
    if (state.agentBusy) return;
    state.agentBusy = true;
    let output = null;
    let trace = null;
    let sentMessage = null;
    let sentBubble = null;
    let runCreated = false;
    let bucket = null;
    try {
      const template = await api('/api/build/templates').then((result) => (result.items || []).find((item) => item.id === templateId));
      if (!template) throw new Error(t('应用模板不存在'));
      state.workspaceView = 'home';
      state.app = null;
      state.appPanel = 'runtime';
      state.table = null;
      await enterConversation();
      await renderWorkspace();
      bucket = state.agentConversationBucket;
      const prompt = `${template.command}：`;
      const message = t('创建应用模板：{name}', { name: template.name });
      sentBubble = appendChat(message, 'user', bucket);
      sentMessage = { role: 'user', content: message };
      bucket.messages.push(sentMessage);
      await persistConversation(bucket);
      output = pendingOutput(bucket);
      trace = runTraceElement('', false, traceSummary(t('开始处理'), t('创建应用模板：{name}', { name: template.name }), ''));
      appendTraceItem(trace, traceGoalText(t('创建应用模板：{name}', { name: template.name }), ''));
      chatRoot(bucket).append(trace);
      const response = await api('/api/agent/runs', { method: 'POST', body: JSON.stringify({ app_id: '', prompt, context: { template: templateId } }) });
      const run = response.run || response;
      runCreated = true;
      trace.dataset.agentTrace = run.id;
      state.agentRun = run.id;
      state.agentRunKey = conversationRunKey(bucket);
      localStorage.setItem(state.agentRunKey, run.id);
      await persistRunLink(sentMessage, run.id, bucket);
      await pollRun(run.id, output, '', bucket);
      await rememberOutput(output, bucket);
      if (isCurrentBucket(bucket)) await renderWorkspace();
    } catch (error) {
      if (!runCreated) {
        removeUnstartedMessage(sentMessage, sentBubble, bucket);
        await persistConversation(bucket).catch(() => {});
      }
      if (trace && !trace.dataset.agentTrace) trace.remove();
      if (output && !output.textContent) output.closest('.chat')?.remove();
      throw error;
    } finally {
      state.agentBusy = false;
    }
  }

  function appendChat(message, role, bucket = state.agentConversationBucket) {
    const node = document.createElement('div');
    node.className = `chat chat-${role === 'user' ? 'end' : 'start'}`;
    node.innerHTML = `<div class="chat-bubble">${esc(message).replace(/\n/g, '<br>')}</div>`;
    const root = chatRoot(bucket);
    root.append(node);
    root.scrollTop = root.scrollHeight;
    return node.querySelector('.chat-bubble');
  }

  function pendingOutput(bucket = state.agentConversationBucket) {
    const output = appendChat('', 'assistant', bucket);
    output.classList.add('chat-bubble-pending');
    return output;
  }

  function settleOutput(output) {
    output?.classList.remove('chat-bubble-pending');
  }

  function forgetRun(bucket = state.agentConversationBucket) {
    if (isCurrentBucket(bucket)) state.agentRun = null;
    localStorage.removeItem(conversationRunKey(bucket));
    if (!bucket?.appId) localStorage.removeItem(workspaceRunKey());
  }

  function renderConversationMenu(items, activeId) {
    const list = $('#conversation-list');
    if (!list) return;
    list.innerHTML = items.map((item) => `<div class="conversation-menu-item"><button type="button" class="conversation-menu-select${item.id === activeId ? ' active' : ''}" data-select-conversation="${esc(item.id)}" aria-current="${item.id === activeId ? 'true' : 'false'}"><span>${esc(item.title || t('新对话'))}</span><small>${item.updated_at ? esc(fmtDateTime(item.updated_at)) : ''}</small></button><details class="conversation-menu-actions"><summary class="btn btn-ghost btn-xs" aria-label="${esc(t('更多操作'))}">${icon('more-horizontal', 14)}</summary><div class="account-actions-menu"><button type="button" class="btn btn-ghost btn-sm" data-rename-conversation="${esc(item.id)}">${esc(t('重命名'))}</button><button type="button" class="btn btn-ghost btn-sm" data-delete-conversation="${esc(item.id)}">${esc(t('删除'))}</button></div></details></div>`).join('') || `<p class="conversation-menu-empty">${esc(t('暂无会话'))}</p>`;
  }

  async function enterConversation() {
    const appId = state.app?.id || '';
    const scope = conversationScope(appId);
    if (!scope) return;
    const storedId = localStorage.getItem(activeConversationKey(appId)) || '';
    const cached = storedId && state.agentConversationCache.get(conversationKey(appId, storedId));
    if (cached && isCurrentBucket(cached)) return;
    let items = await conversations.list(appId);
    if (!isCurrentApp(appId)) return;
    let conversation = items.find((item) => item.id === storedId) || items[0];
    if (!conversation) {
      conversation = await conversations.create(appId);
      items = [conversation];
    }
    localStorage.setItem(activeConversationKey(appId), conversation.id);
    let bucket = state.agentConversationCache.get(conversationKey(appId, conversation.id));
    if (!bucket) {
      const saved = await conversations.load(scope, appId, conversation.id);
      if (!isCurrentApp(appId)) return;
      if (!saved) {
        conversation = await conversations.create(appId);
        items.unshift(conversation);
        localStorage.setItem(activeConversationKey(appId), conversation.id);
        bucket = getConversationBucket(appId, conversation.id, scope);
      } else {
        bucket = getConversationBucket(appId, conversation.id, scope, compactConversationMessages(saved.messages || []), saved.revision || 0);
      }
    }
    activateConversation(bucket);
    renderConversationMenu(items, bucket.id);
    const activeTraceLoads = [];
    if (!bucket.rendered) {
      for (const message of bucket.messages) {
        const bubble = appendChat(message.content, message.role, bucket);
        if (message.run_id) {
          const load = attachArchivedTrace(bubble, message, bucket);
          if (message.run_id === state.agentRun) activeTraceLoads.push(load);
        }
      }
      bucket.rendered = true;
    }
    await Promise.all(activeTraceLoads);
    if (state.agentRun) await resumeRun(bucket).catch((error) => toast(error.message || t('运行状态恢复失败'), true));
  }

  async function newConversation() {
    const appId = state.app?.id || '';
    const created = await conversations.create(appId);
    localStorage.setItem(activeConversationKey(appId), created.id);
    await enterConversation();
    await renderWorkspace();
  }

  async function selectConversation(id) {
    const appId = state.app?.id || '';
    if (state.agentConversationBucket?.appId === appId && state.agentConversationBucket?.id === id) return;
    localStorage.setItem(activeConversationKey(appId), id);
    state.agentConversationId = '';
    await enterConversation();
    $('#conversation-switcher')?.removeAttribute('open');
    await renderWorkspace();
  }

  $('#conversation-list')?.addEventListener('click', async (event) => {
    const select = event.target.closest('[data-select-conversation]');
    const rename = event.target.closest('[data-rename-conversation]');
    const remove = event.target.closest('[data-delete-conversation]');
    try {
      if (select) await selectConversation(select.dataset.selectConversation);
      if (rename) {
        const value = window.prompt(t('对话名称'), rename.closest('.conversation-menu-item')?.querySelector('.conversation-menu-select span')?.textContent || '');
        if (value == null) return;
        const result = await conversations.rename(rename.dataset.renameConversation, value);
        const appId = state.app?.id || '';
        const items = await conversations.list(appId);
        renderConversationMenu(items, state.agentConversationId);
        const bucket = state.agentConversationBucket;
        if (bucket?.id === result.id) bucket.title = result.title;
      }
      if (remove) {
        if (!window.confirm(t('删除此会话及其消息？运行回执仍保留在服务端。'))) return;
        await conversations.remove(remove.dataset.deleteConversation);
        const appId = state.app?.id || '';
        const key = conversationKey(appId, remove.dataset.deleteConversation);
        state.agentConversationCache.delete(key);
        if (state.agentConversationId === remove.dataset.deleteConversation) {
          localStorage.removeItem(activeConversationKey(appId));
          state.agentConversationId = '';
          state.agentConversationLoadedKey = null;
          await enterConversation();
          await renderWorkspace();
        } else {
          renderConversationMenu(await conversations.list(appId), state.agentConversationId);
        }
      }
    } catch (error) { toast(error.message || t('会话操作失败'), true); }
  });
  $('#conversation-new')?.addEventListener('click', () => newConversation().catch((error) => toast(error.message || t('对话创建失败'), true)));

  function compactConversationMessages(messages) {
    const compacted = [];
    for (const message of messages) {
      const previous = compacted.at(-1);
      if (previous && previous.role === message.role && previous.content === message.content && !previous.run_id && !message.run_id) continue;
      compacted.push(message);
    }
    return compacted;
  }

  function removeUnstartedMessage(message, bubble, bucket = state.agentConversationBucket) {
    bubble?.closest('.chat')?.remove();
    const index = bucket?.messages.indexOf(message) ?? -1;
    if (index >= 0) bucket.messages.splice(index, 1);
    if (isCurrentBucket(bucket)) state.agentConversationMessages = bucket.messages;
  }

  async function persistConversation(bucket = state.agentConversationBucket) {
    if (!bucket?.id) return;
    const scope = bucket.scope;
    const result = await conversations.save({ scope, expectedRevision: bucket.revision || 0, messages: bucket.messages, appId: bucket.appId, conversationId: bucket.id });
    if (result.saved) {
      bucket.revision = result.revision;
      if (isCurrentBucket(bucket)) state.agentConversationRevision = result.revision;
      if (result.title) bucket.title = result.title;
    }
    if (result.conflict) toast(t('对话已在其他窗口更新，请刷新后继续。'), true);
    if (result.changed) toast(t('对话权限范围已变化，请刷新后继续。'), true);
  }

  async function rememberOutput(output, bucket = state.agentConversationBucket) {
    if (!output.textContent) return;
    const last = bucket.messages.at(-1);
    const message = last?.role === 'assistant' ? last : { role: 'assistant', content: '' };
    message.content = output.textContent;
    const linkedUserMessage = output.dataset.runId && bucket.messages.some((item) => item.role === 'user' && item.run_id === output.dataset.runId);
    if (output.dataset.runId && !linkedUserMessage) message.run_id = output.dataset.runId;
    if (!last || last.role !== 'assistant') bucket.messages.push(message);
    if (isCurrentBucket(bucket)) state.agentConversationMessages = bucket.messages;
    await persistConversation(bucket);
  }

  async function persistRunLink(message, runID, bucket = state.agentConversationBucket) {
    if (!message || !runID) return;
    message.run_id = runID;
    await persistConversation(bucket);
  }

  async function refreshCreatedApp(run, bucket) {
    if (!run.app_id || state.apps?.some((app) => app.id === run.app_id)) return;
    const apps = await api('/api/apps');
    const app = apps.find((item) => item.id === run.app_id);
    if (!app) return;
    state.apps = apps;
    if (isCurrentBucket(bucket) && !bucket.appId) {
      state.app = app;
      state.table = null;
      state.appRuntime = null;
      bucket.scope = conversationScope('');
    }
  }

  function planSummary(candidate) {
    const lines = [];
    if (candidate?.capability === 'apps.create') {
      lines.push(t('应用：{name}', { name: candidate.input?.name || '' }));
      for (const table of candidate.evidence?.definition?.tables || []) {
        lines.push(t('{name}：{fields}', { name: table.name, fields: table.fields.map((field) => field.label || field.name).join(t('、')) }));
      }
    }
    for (const operation of candidate?.evidence?.plan?.operations || []) {
      const input = operation.input || {};
      const fields = (input.fields || []).map((field) => field.label || field.name).join(t('、'));
      lines.push(t('{op} {name}{fields}', { op: operation.capability === 'collections.create' ? t('创建数据表') : t('修改数据表'), name: input.name || input.slug || '', fields: fields ? t('：{fields}', { fields }) : '' }));
    }
    return lines.join('\n');
  }

  function actionButton(label, handler, allowBusy = false) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'btn btn-sm';
    button.textContent = label;
    button.onclick = async () => {
      if (state.agentControlBusy || state.agentBusy && !allowBusy) return;
      const wasBusy = state.agentBusy;
      state.agentControlBusy = true;
      state.agentBusy = true;
      button.disabled = true;
      try { await handler(wasBusy); } catch (error) { toast(error.message || t('运行操作失败'), true); }
      finally { button.disabled = false; state.agentControlBusy = false; state.agentBusy = wasBusy; }
    };
    return button;
  }

  state.agentRunPolls ||= new Map();
  async function pollRun(runID, output, goalText, bucket = state.agentConversationBucket) {
    const active = state.agentRunPolls.get(runID);
    if (active) return active;
    const poll = pollRunLoop(runID, output, goalText, bucket);
    state.agentRunPolls.set(runID, poll);
    try { return await poll; }
    finally { if (state.agentRunPolls.get(runID) === poll) state.agentRunPolls.delete(runID); }
  }

  async function pollRunLoop(runID, output, goalText, bucket) {
    const ownerKey = `${state.user?.id || ''}:${state.tenant?.id || ''}`;
    const assertScope = () => { if (`${state.user?.id || ''}:${state.tenant?.id || ''}` !== ownerKey) throw new Error(t('工作区上下文已切换；原运行已保留，可切回后继续。')); };
    const messages = () => chatRoot(bucket);
    for (const element of messages().querySelectorAll('[data-agent-run]')) {
      if (element.dataset.agentRun === runID) element.remove();
    }
    output.dataset.runId = runID;
    const controls = document.createElement('div');
    controls.dataset.agentRun = runID;
    controls.className = 'flex flex-wrap gap-2';
    messages().append(controls);
    let trace = messages().querySelector(`[data-agent-trace="${CSS.escape(runID)}"]`);
    if (!trace || !trace.isConnected) {
      trace = runTraceElement(runID, false, goalText ? `${t('开始处理')} · ${goalText}` : t('开始处理'));
      if (goalText) appendTraceItem(trace, goalText);
      messages().insertBefore(trace, controls);
    }
    let after = Number(trace.dataset.after) || 0;
    const controller = new AbortController();
    let stoppedRun = null;

    const handleRunSnapshot = async (run) => {
      assertScope();
      await refreshCreatedApp(run, bucket);
      if (terminalStates.has(run.state) && !run.usage) {
        const response = await api(`/api/agent/runs/${encodeURIComponent(run.id)}`);
        run = response.run || response;
        if (response.usage) run.usage = response.usage;
      }
      renderRunSnapshot(trace, run);
      if (terminalStates.has(run.state)) {
        controls.remove();
        if (run.result?.items || run.result?.record) {
          const result = document.createElement('details'); result.className = 'preview-change-list'; result.open = true;
          result.innerHTML = `<summary>${run.result.items ? esc(t('查询记录')) : esc(t('保存回执'))}</summary><pre>${esc(JSON.stringify(run.result.items || run.result.record, null, 2))}</pre>`;
          if (run.app_id && run.result.table) {
            const open = document.createElement('button');
            open.type = 'button';
            open.className = 'btn btn-outline btn-sm';
            open.textContent = t('在应用中查看');
            open.onclick = async () => {
              const app = state.apps?.find((item) => item.id === run.app_id);
              if (!app) return;
              await openRecord({ app: run.app_id, table: run.result.table, record: run.result.items?.[0]?.id || '' });
            };
            result.append(open);
          }
          messages().append(result);
        }
        forgetRun(bucket);
        if (isCurrentBucket(bucket)) state.agentRun = null;
        if (!eventLabels[run.state]) appendTraceItem(trace, runStateLabels[run.state] || run.state);
        trace.querySelector('summary').textContent = traceSummary(`${t('运行记录')} · ${runStateLabels[run.state] || run.state}`, run.prompt, run.app_id);
        output.textContent = run.error || run.result?.message || (run.state === 'completed' ? (run.result?.published ? t('已发布正式界面{version}。', { version: run.result.version_number ? ` v${run.result.version_number}` : '' }) : run.result?.version ? t('界面草稿 v{version} 已生成。', { version: run.result.version_number || '' }) : t('已完成本轮操作。')) : t('本轮已停止。'));
        return true;
      }
      if (run.state === 'waiting_confirmation' && run.phase === 'confirmation') {
        if (run.candidate?.capability === 'ui.publish.public' && run.candidate.evidence?.public_scope) {
          const scope = document.createElement('div');
          scope.className = 'alert alert-vertical sm:alert-horizontal';
          scope.setAttribute('role', 'status');
          scope.textContent = t('匿名公开范围：{scope}', { scope: run.candidate.evidence.public_scope });
          controls.prepend(scope);
        }
        if (run.candidate?.capability === 'ui.compose') {
          const diff = document.createElement('details'); diff.className = 'preview-change-list'; diff.open = true;
          diff.innerHTML = `<summary>${esc(t('界面变更 · {count} 项', { count: fmtNumber((run.candidate.evidence?.changes || []).length) }))}</summary>${formatUIChanges(run.candidate.evidence?.changes || [], esc)}`;
          controls.prepend(diff);
          controls.append(actionButton(t('预览待保存界面'), async () => {
            const preview = await api(`/api/apps/${encodeURIComponent(run.app_id)}/versions/preview`, { method:'POST',body:JSON.stringify({ definition:run.candidate.input.definition }) });
            const host = document.createElement('div'); host.className = 'agent-ui-preview'; diff.append(host);
            const { mount } = await loadRenderer();
            mount(host, preview.definition.pages.find((page) => page.id === preview.ui_page).spec, { sources:preview.sources || {},members:preview.members || [],readOnly:true });
          }));
        }
        output.textContent = t('请审阅后确认{op}。', { op: operationLabels[run.candidate?.capability] || t('本次操作') });
        const summary = planSummary(run.candidate);
        if (summary) {
          const detail = document.createElement('div');
          detail.className = 'alert alert-vertical sm:alert-horizontal';
          detail.setAttribute('role', 'status');
          const text = document.createElement('p');
          text.style.whiteSpace = 'pre-wrap';
          text.textContent = summary;
          detail.append(text);
          controls.prepend(detail);
        }
        for (const [label, decision] of [[t('确认以上变更'), 'approve'], [t('拒绝变更'), 'reject']]) {
          controls.append(actionButton(label, async () => {
            await api(`/api/agent/runs/${encodeURIComponent(runID)}/confirm`, { method: 'POST', body: JSON.stringify({ expected_version: run.candidate?.version || 0, decision }) });
            await pollRun(runID, output, goalText, bucket);
            await rememberOutput(output, bucket);
          }));
        }
        return true;
      }
      if (run.phase === 'execution_waiting') {
        output.textContent = run.result?.question || t('请补充信息后继续本轮运行。');
        return true;
      }
      if (run.state === 'unknown') {
        output.textContent = t('执行效果需要核实。');
        controls.append(actionButton(t('核实执行回执'), async () => {
          await api(`/api/agent/runs/${encodeURIComponent(runID)}/resume`, { method: 'POST', body: JSON.stringify({}) });
          await pollRun(runID, output, goalText, bucket);
          await rememberOutput(output, bucket);
        }));
        return true;
      }
      return false;
    };

    const consumeEvent = async (event) => {
      assertScope();
      const sequence = Number(event.id);
      if (Number.isFinite(sequence) && sequence > after) {
        after = sequence;
        trace.dataset.after = String(after);
      }
      if (event.event === 'state') {
        const run = event.data?.run || event.data;
        if (run && await handleRunSnapshot(run)) {
          settleOutput(output);
          stoppedRun = run;
          controller.abort();
        }
        return;
      }
      if (['text', 'assistant_message', 'message'].includes(event.event)) output.textContent += String(event.data?.text || event.data?.content || '');
      else consumeTraceEvent(trace, { type: event.event, data: event.data });
      if (!output.textContent && event.event === 'phase' && phaseLabels[event.data?.phase]) output.textContent = `${phaseLabels[event.data.phase]}…`;
    };

    const pollFallback = async () => {
      for (;;) {
        assertScope();
        const events = await api(`/api/agent/runs/${encodeURIComponent(runID)}/events?after=${after}`);
        for (const event of events.events || []) await consumeEvent({ event: event.type, id: String(event.sequence), data: event.data });
        if (stoppedRun) return stoppedRun;
        const response = await api(`/api/agent/runs/${encodeURIComponent(runID)}`);
        const run = response.run || response;
        if (await handleRunSnapshot(run)) { settleOutput(output); return run; }
        await new Promise((resolve) => setTimeout(resolve, 700));
      }
    };

    try {
      if (!stream) throw new Error('SSE is unavailable');
      while (!stoppedRun && !controller.signal.aborted) {
        assertScope();
        await stream(`/api/agent/runs/${encodeURIComponent(runID)}/stream?after=${after}`, { signal: controller.signal, onEvent: consumeEvent });
        if (!stoppedRun && !controller.signal.aborted) await new Promise((resolve) => setTimeout(resolve, 250));
      }
    } catch (error) {
      if (controller.signal.aborted && stoppedRun) return stoppedRun;
      if (error?.name === 'AbortError') return stoppedRun;
      return pollFallback();
    }
    return stoppedRun;
  }

  async function resumeRun(bucket = state.agentConversationBucket) {
    if (!bucket) return null;
    const runID = localStorage.getItem(conversationRunKey(bucket)) || state.agentRun;
    if (!runID) return null;
    if (state.agentRunPolls.has(runID)) return state.agentRunPolls.get(runID);
    const response = await api(`/api/agent/runs/${encodeURIComponent(runID)}`);
    const run = response.run || response;
    const activeOutput = [...chatRoot(bucket).querySelectorAll('[data-run-id]')].find((item) => item.dataset.runId === run.id);
    return pollRun(run.id, activeOutput || pendingOutput(bucket), traceGoalText(run.prompt, run.app_id), bucket);
  }

  // Follow an app-template import run created outside the chat composer.
  async function resumeImportRun(run) {
    if (state.agentBusy || !run?.id) return;
    state.agentBusy = true;
    try {
      await enterConversation();
      const bucket = state.agentConversationBucket;
      const message = t('导入应用模板：{name}', { name: run.prompt.replace(/^导入应用模板：/, '') });
      appendChat(message, 'user', bucket);
      const userMessage = { role: 'user', content: message };
      bucket.messages.push(userMessage);
      await persistConversation(bucket);
      const output = pendingOutput(bucket);
      state.agentRun = run.id;
      state.agentRunKey = conversationRunKey(bucket);
      localStorage.setItem(state.agentRunKey, run.id);
      await persistRunLink(userMessage, run.id, bucket);
      await pollRun(run.id, output, traceGoalText(message, ''), bucket);
      await rememberOutput(output, bucket);
    } finally {
      state.agentBusy = false;
    }
  }

  async function attachmentPayload(file, appId) {
    if (!file) return {};
    if (!appId) {
      if (file.size > 64000 || !/\.(txt|md|csv)$/i.test(file.name)) throw new Error(t('未选择应用时支持不超过 64 KB 的文本、Markdown 或 CSV；其他附件请先选择应用。'));
      return { attachment:{ name:file.name,text:await file.text() } };
    }
    if (file.size > 5 * 1024 * 1024) throw new Error(t('附件不能超过 5 MB。'));
    const base64 = await new Promise((resolve,reject) => { const reader = new FileReader(); reader.onload = () => resolve(reader.result); reader.onerror = () => reject(reader.error); reader.readAsDataURL(file); });
    const uploaded = await api(`/api/apps/${encodeURIComponent(appId)}/files`, { method:'POST',body:JSON.stringify({ name:file.name,base64 }) });
    return { attachment_ids:[uploaded.id] };
  }

  async function submitPrompt(event) {
    event.preventDefault();
    if (state.agentBusy) return;
    const form = event.currentTarget;
    const prompt = String(new FormData(form).get('prompt') || '').trim();
    if (!prompt) return;
    state.agentBusy = true;
    // Clear the composer up front so the sent request is visible only as the
    // user bubble; the pending spinner lives on the assistant bubble.
    const attachmentFile = form.elements.attachment?.files?.[0] || null;
    form.reset();
    const attachmentLabel = form.querySelector('[data-attachment-name]'); if (attachmentLabel) attachmentLabel.textContent = '';
    let output = null;
    let trace = null;
    let sentMessage = null;
    let sentBubble = null;
    let runCreated = false;
    let bucket = null;
    try {
      await enterConversation();
      let pending = null;
      if (state.agentRun) {
        const response = await api(`/api/agent/runs/${encodeURIComponent(state.agentRun)}`);
        pending = response.run || response;
        if (pending.phase !== 'execution_waiting') throw new Error(t('请先确认、核实或取消当前运行，再开始新的需求。'));
      }
      const appId = pending?.app_id || state.app?.id || '', tenantId = state.tenant?.id;
      if (pending && pending.app_id !== (state.app?.id || '')) throw new Error(t('补充需求时请先切换回当前运行的应用。'));
      bucket = state.agentConversationBucket;
      if (!bucket || bucket.appId !== appId) throw new Error(t('会话上下文已切换，请重新发送。'));
      const attachments = await attachmentPayload(attachmentFile, appId);
      if (state.tenant?.id !== tenantId || !pending && (state.app?.id || '') !== appId) throw new Error(t('工作区或应用已切换，请重新发送。'));
      sentBubble = appendChat(prompt, 'user', bucket);
      if (attachmentFile) appendChat(t('附件：{name}', { name: attachmentFile.name }), 'user', bucket);
      sentMessage = { role: 'user', content: prompt };
      bucket.messages.push(sentMessage);
      if (isCurrentBucket(bucket)) state.agentConversationMessages = bucket.messages;
      await persistConversation(bucket);
      output = pendingOutput(bucket);
      trace = runTraceElement('', false, traceSummary(t('开始处理'), prompt, appId));
      appendTraceItem(trace, traceGoalText(prompt, appId));
      chatRoot(bucket).append(trace);
      const response = pending
        ? await api(`/api/agent/runs/${encodeURIComponent(pending.id)}/continue`, { method: 'POST', body: JSON.stringify({ answer: prompt, expected_version: pending.version,...attachments }) })
        : await api('/api/agent/runs', { method: 'POST', body: JSON.stringify({ app_id:appId,prompt,context:{},...attachments }) });
      const run = response.run || response;
      runCreated = true;
      trace.dataset.agentTrace = run.id;
      state.agentRun = run.id;
      state.agentRunKey = conversationRunKey(bucket);
      localStorage.setItem(state.agentRunKey, run.id);
      await persistRunLink(sentMessage, run.id, bucket);
      const result = await pollRun(run.id, output, traceGoalText(prompt, appId), bucket);
      await rememberOutput(output, bucket);
      if (isCurrentBucket(bucket)) await renderWorkspace();
      return result;
    } catch (error) {
      if (!runCreated) {
        removeUnstartedMessage(sentMessage, sentBubble, bucket);
        await persistConversation(bucket).catch(() => {});
      }
      if (trace && !trace.dataset.agentTrace) trace.remove();
      if (output && !output.textContent) output.closest('.chat')?.remove();
      settleOutput(output);
      toast(error.message || t('小助手暂时无法响应。'), true);
    }
    finally { settleOutput(output); state.agentBusy = false; }
  }

  async function clearSavedConversation({ skipConfirm = false } = {}) {
    if (state.agentRun) throw new Error(t('请先取消当前运行，再清理对话。'));
    if (!skipConfirm && $('#chat-messages').children.length && !window.confirm(t('清除整个小助手会话？消息与运行记录展示将一并移除，且无法恢复。'))) return;
    const bucket = state.agentConversationBucket;
    if (bucket) {
      const result = await conversations.save({ scope: bucket.scope, expectedRevision: bucket.revision || 0, messages: [], appId: bucket.appId, conversationId: bucket.id });
      if (!result.saved) throw new Error(result.conflict ? t('对话已在其他窗口更新，请刷新后继续。') : t('对话权限范围已变化，请刷新后继续。'));
      bucket.revision = result.revision;
      bucket.messages = [];
      bucket.rendered = true;
    }
    state.agentConversationMessages = [];
    state.agentConversationLoadedKey = null;
    forgetRun(bucket);
    $('#chat-messages').replaceChildren();
    await enterConversation();
  }

  return { submitPrompt, enterConversation, clearSavedConversation, clearSavedConversations: clearSavedConversation, resumeRun, createTemplate, resumeImportRun };
}
