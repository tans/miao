import { formatUIChanges } from './ui-changes.js';
import { mount } from './ui-renderer.bundle.js';
import { createServerConversationStore, authorizationScope } from '/modules/agent-conversation-store.js';

export function createAgentAssistant({ state, api, $, esc, toast, renderWorkspace }) {
  const conversations = createServerConversationStore(api);
  state.agentConversationMessages ||= [];
  state.agentConversationRevision ||= 0;
  state.agentRun ||= null;
  const activeRunKey = () => `miao-agent-run:${state.user?.id || ''}:${state.tenant?.id || ''}`;
  const conversationScope = () => authorizationScope(state.tenant, state.apps || [], state.user?.id);
  const terminalStates = new Set(['completed', 'failed', 'cancelled', 'budget_exhausted', 'unsupported', 'unavailable']);
  const operationLabels = { 'apps.create': '创建应用', 'backend_plan.apply': '应用后端变更', 'ui.compose': '创建界面草稿', 'ui.publish': '发布正式界面', 'ui.publish.public': '发布并开启匿名公开', 'requirements.collect': '整理需求', 'records.query': '查询记录', 'business_actions.execute': '执行业务动作' };
  const phaseLabels = { observe: '读取资源', enumerate: '枚举候选', decide: '模型判断', validate: '校验方案', execute: '执行变更', record: '记录回执', reconciled: '恢复核对', confirmation: '等待确认', execution_waiting: '等待补充信息', complete: '收尾完成' };
  const eventLabels = { confirmation_required: '等待确认', waiting: '等待补充信息', confirmed: '已确认，继续执行', completed: '已完成', failed: '运行失败', cancelled: '已取消', budget_exhausted: '预算耗尽', unsupported: '暂不支持' };
  const runStateLabels = { completed: '已完成', failed: '运行失败', cancelled: '已取消', budget_exhausted: '预算耗尽', unsupported: '暂不支持', unknown: '待核实', waiting_confirmation: '等待确认' };

  function runTraceElement(runID, open) {
    const details = document.createElement('details');
    details.className = 'agent-run-trace';
    details.dataset.agentTrace = runID;
    if (open) details.open = true;
    details.innerHTML = '<summary>开始处理</summary><ol class="agent-run-phases"></ol>';
    return details;
  }

  function traceGoalText(goal, appId) {
    goal = String(goal || '').trim();
    if (!goal) return '';
    const trimmed = goal.length > 48 ? `${goal.slice(0, 48)}…` : goal;
    const appName = appId ? state.apps?.find((item) => item.id === appId)?.name || '当前应用' : '';
    return `目标：${trimmed} · ${appName || '工作台'}`;
  }

  function appendTraceItem(details, text) {
    if (!text) return;
    const item = document.createElement('li');
    item.textContent = text;
    details.querySelector('.agent-run-phases').append(item);
    const messages = $('#chat-messages');
    if (messages) messages.scrollTop = messages.scrollHeight;
  }

  function consumeTraceEvent(details, event) {
    const type = event.type;
    if (type === 'phase') {
      appendTraceItem(details, phaseLabels[event.data?.phase]);
    } else if (type === 'candidate') {
      const label = operationLabels[event.data?.capability] || event.data?.capability;
      appendTraceItem(details, label ? `候选操作：${label}${event.data?.write ? '（写入，需确认）' : ''}` : '');
    } else if (eventLabels[type]) {
      appendTraceItem(details, eventLabels[type]);
    }
  }

  async function attachArchivedTrace(bubble, message) {
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
      finish(`运行记录 · ${runStateLabels[run.state] || run.state}`);
      appendTraceItem(details, traceGoalText(run.prompt, run.app_id));
      for (const event of eventsResponse.events || []) consumeTraceEvent(details, event);
    } catch (error) {
      finish('运行记录 · 明细已归档');
    }
    const clear = document.createElement('button');
    clear.type = 'button';
    clear.className = 'btn btn-ghost btn-xs agent-run-trace-clear';
    clear.textContent = '清除记录';
    clear.onclick = async () => {
      if (!window.confirm('清除这条运行记录的展示？运行回执仍按审计要求保留在服务端。')) return;
      const index = state.agentConversationMessages.indexOf(message);
      if (index >= 0) {
        state.agentConversationMessages.splice(index, 1);
        await persistConversation();
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
    try {
      const template = await api('/api/build/templates').then((result) => (result.items || []).find((item) => item.id === templateId));
      if (!template) throw new Error('应用模板不存在');
      state.workspaceView = 'home';
      state.app = null;
      state.appPanel = 'runtime';
      state.table = null;
      await renderWorkspace();
      const prompt = `${template.command}：`;
      appendChat(`创建应用模板：${template.name}`, 'user');
      state.agentConversationMessages.push({ role: 'user', content: `创建应用模板：${template.name}` });
      await persistConversation();
      output = appendChat('', 'assistant');
      trace = runTraceElement('', true);
      appendTraceItem(trace, traceGoalText(`创建应用模板：${template.name}`, ''));
      $('#chat-messages').append(trace);
      const response = await api('/api/agent/runs', { method: 'POST', body: JSON.stringify({ app_id: '', prompt, context: { template: templateId } }) });
      const run = response.run || response;
      trace.dataset.agentTrace = run.id;
      state.agentRun = run.id;
      state.agentRunKey = activeRunKey();
      localStorage.setItem(activeRunKey(), run.id);
      await pollRun(run.id, output);
      await rememberOutput(output);
      await renderWorkspace();
    } catch (error) {
      if (trace && !trace.dataset.agentTrace) trace.remove();
      if (output && !output.textContent) output.closest('.chat')?.remove();
      throw error;
    } finally {
      state.agentBusy = false;
    }
  }

  function appendChat(message, role) {
    const node = document.createElement('div');
    node.className = `chat chat-${role === 'user' ? 'end' : 'start'}`;
    node.innerHTML = `<div class="chat-bubble">${esc(message).replace(/\n/g, '<br>')}</div>`;
    $('#chat-messages').append(node);
    $('#chat-messages').scrollTop = $('#chat-messages').scrollHeight;
    return node.querySelector('.chat-bubble');
  }

  function forgetRun() {
    state.agentRun = null;
    localStorage.removeItem(activeRunKey());
  }

  async function enterConversation() {
    const key = activeRunKey();
    if (state.agentRunKey !== key) {
      state.agentRun = localStorage.getItem(key);
      state.agentRunKey = key;
    }
    const scope = conversationScope();
    if (!scope || state.agentConversationLoadedKey === scope) return;
    const saved = await conversations.load(scope);
    if (conversationScope() !== scope) return;
    state.agentConversationMessages = saved?.messages || [];
    state.agentConversationRevision = saved?.revision || 0;
    for (const message of state.agentConversationMessages) {
      const bubble = appendChat(message.content, message.role);
      if (message.run_id) attachArchivedTrace(bubble, message);
    }
    state.agentConversationLoadedKey = scope;
    if (state.agentRun) await resumeRun().catch((error) => toast(error.message || '运行状态恢复失败', true));
  }

  async function persistConversation() {
    const scope = conversationScope();
    if (!scope) return;
    const result = await conversations.save({ scope, expectedRevision: state.agentConversationRevision || 0, messages: state.agentConversationMessages });
    if (conversationScope() !== scope) return;
    if (result.saved) state.agentConversationRevision = result.revision;
    if (result.conflict) toast('对话已在其他窗口更新，请刷新后继续。', true);
    if (result.changed) toast('对话权限范围已变化，请刷新后继续。', true);
  }

  async function rememberOutput(output) {
    if (!output.textContent) return;
    const last = state.agentConversationMessages.at(-1);
    const message = last?.role === 'assistant' ? last : { role: 'assistant', content: '' };
    message.content = output.textContent;
    if (output.dataset.runId) message.run_id = output.dataset.runId;
    if (!last || last.role !== 'assistant') state.agentConversationMessages.push(message);
    await persistConversation();
  }

  async function refreshCreatedApp(run) {
    if (!run.app_id || state.apps?.some((app) => app.id === run.app_id)) return;
    const previousApps = state.apps || [];
    const apps = await api('/api/apps');
    const app = apps.find((item) => item.id === run.app_id);
    if (!app) return;
    const priorScope = authorizationScope(state.tenant, previousApps, state.user?.id);
    const retainedScope = authorizationScope(state.tenant, apps.filter((item) => item.id !== run.app_id), state.user?.id);
    state.apps = apps;
    state.app = app;
    state.table = null;
    state.appRuntime = null;
    // Only the app just created may expand the saved conversation's scope.
    if (priorScope !== retainedScope) {
      state.agentConversationMessages = [];
      $('#chat-messages').replaceChildren();
    }
    const saved = await conversations.load(conversationScope());
    state.agentConversationRevision = saved?.revision || 0;
    state.agentConversationLoadedKey = conversationScope();
  }

  function planSummary(candidate) {
    const lines = [];
    if (candidate?.capability === 'apps.create') {
      lines.push(`应用：${candidate.input?.name || ''}`);
      for (const table of candidate.evidence?.definition?.tables || []) {
        lines.push(`${table.name}：${table.fields.map((field) => field.label || field.name).join('、')}`);
      }
    }
    for (const operation of candidate?.evidence?.plan?.operations || []) {
      const input = operation.input || {};
      const fields = (input.fields || []).map((field) => field.label || field.name).join('、');
      lines.push(`${operation.capability === 'collections.create' ? '创建数据表' : '修改数据表'} ${input.name || input.slug || ''}${fields ? `：${fields}` : ''}`);
    }
    return lines.join('\n');
  }

  async function cancelRun(runID) {
    return api(`/api/agent/runs/${encodeURIComponent(runID)}/cancel`, { method: 'POST', body: JSON.stringify({}) });
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
      try { await handler(wasBusy); } catch (error) { toast(error.message || '运行操作失败', true); }
      finally { button.disabled = false; state.agentControlBusy = false; state.agentBusy = wasBusy; }
    };
    return button;
  }

  async function pollRun(runID, output, goalText) {
    const runScopeKey = activeRunKey();
    const assertScope = () => { if (activeRunKey() !== runScopeKey) throw new Error('工作区上下文已切换；原运行已保留，可切回后继续。'); };
    for (const element of document.querySelectorAll('[data-agent-run]')) {
      if (element.dataset.agentRun === runID) element.remove();
    }
    output.dataset.runId = runID;
    const controls = document.createElement('div');
    controls.dataset.agentRun = runID;
    controls.className = 'flex flex-wrap gap-2';
    $('#chat-messages').append(controls);
    let trace = document.querySelector(`[data-agent-trace="${CSS.escape(runID)}"]`);
    if (!trace || !trace.isConnected) {
      trace = runTraceElement(runID, true);
      if (goalText) appendTraceItem(trace, goalText);
      $('#chat-messages').insertBefore(trace, controls);
    }
    controls.append(actionButton('取消运行', async (wasBusy) => {
      await cancelRun(runID);
      if (!wasBusy) { await pollRun(runID, output); await rememberOutput(output); }
    }, true));
    let after = 0;
    for (;;) {
      assertScope();
      const events = await api(`/api/agent/runs/${encodeURIComponent(runID)}/events?after=${after}`);
      assertScope();
      for (const event of events.events || []) {
        after = Math.max(after, Number(event.sequence) || after);
        if (['text', 'assistant_message', 'message'].includes(event.type)) output.textContent += String(event.data?.text || event.data?.content || '');
        else consumeTraceEvent(trace, event);
      }
      const response = await api(`/api/agent/runs/${encodeURIComponent(runID)}`);
      const run = response.run || response;
      assertScope();
      await refreshCreatedApp(run);
      if (terminalStates.has(run.state)) {
        controls.remove();
        if (run.result?.items || run.result?.record) {
          const result = document.createElement('details'); result.className = 'preview-change-list'; result.open = true;
          result.innerHTML = `<summary>${run.result.items ? '查询记录' : '保存回执'}</summary><pre>${esc(JSON.stringify(run.result.items || run.result.record, null, 2))}</pre>`;
          $('#chat-messages').append(result);
        }
        forgetRun();
        if (!eventLabels[run.state]) appendTraceItem(trace, runStateLabels[run.state] || run.state);
        trace.querySelector('summary').textContent = `运行记录 · ${runStateLabels[run.state] || run.state}`;
        output.textContent = run.error || run.result?.message || (run.state === 'completed' ? (run.result?.version ? `界面草稿 v${run.result.version_number || ''} 已生成。` : run.result?.published ? `已发布正式界面${run.result.version_number ? ` v${run.result.version_number}` : ''}。` : '已完成本轮操作。') : '本轮已停止。');
        return run;
      }
      if (run.state === 'waiting_confirmation' && run.phase === 'confirmation') {
        if (run.candidate?.capability === 'ui.publish.public' && run.candidate.evidence?.public_scope) {
          const scope = document.createElement('div');
          scope.className = 'alert alert-vertical sm:alert-horizontal';
          scope.setAttribute('role', 'status');
          scope.textContent = `匿名公开范围：${run.candidate.evidence.public_scope}`;
          controls.prepend(scope);
        }
        if (run.candidate?.capability === 'ui.compose') {
          const diff = document.createElement('details'); diff.className = 'preview-change-list'; diff.open = true;
          diff.innerHTML = `<summary>界面变更 · ${(run.candidate.evidence?.changes || []).length} 项</summary>${formatUIChanges(run.candidate.evidence?.changes || [], esc)}`;
          controls.prepend(diff);
          controls.append(actionButton('预览待保存界面', async () => {
            const preview = await api(`/api/apps/${encodeURIComponent(run.app_id)}/versions/preview`, { method:'POST',body:JSON.stringify({ definition:run.candidate.input.definition }) });
            const host = document.createElement('div'); host.className = 'agent-ui-preview'; diff.append(host);
            mount(host, preview.definition.pages.find((page) => page.id === preview.ui_page).spec, { sources:preview.sources || {},members:preview.members || [],readOnly:true });
          }));
        }
        output.textContent = `请审阅后确认${operationLabels[run.candidate?.capability] || '本次操作'}。`;
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
        for (const [label, decision] of [['确认以上变更', 'approve'], ['拒绝变更', 'reject']]) {
          controls.append(actionButton(label, async () => {
            await api(`/api/agent/runs/${encodeURIComponent(runID)}/confirm`, { method: 'POST', body: JSON.stringify({ expected_version: run.candidate?.version || 0, decision }) });
            await pollRun(runID, output);
            await rememberOutput(output);
          }));
        }
        return run;
      }
      if (run.phase === 'execution_waiting') {
        output.textContent = run.result?.question || '请补充信息后继续本轮运行。';
        return run;
      }
      if (run.state === 'unknown') {
        output.textContent = '执行效果需要核实。';
        controls.append(actionButton('核实执行回执', async () => {
          await api(`/api/agent/runs/${encodeURIComponent(runID)}/resume`, { method: 'POST', body: JSON.stringify({}) });
          await pollRun(runID, output);
          await rememberOutput(output);
        }));
        return run;
      }
      await new Promise((resolve) => setTimeout(resolve, 700));
    }
  }

  async function resumeRun() {
    if (!state.agentRun) return null;
    const response = await api(`/api/agent/runs/${encodeURIComponent(state.agentRun)}`);
    const run = response.run || response;
    return pollRun(run.id, appendChat('', 'assistant'), traceGoalText(run.prompt, run.app_id));
  }

  async function attachmentInput(form, appId) {
    const file = form.elements.attachment?.files?.[0];
    if (!file) return {};
    if (!appId) {
      if (file.size > 64000 || !/\.(txt|md|csv)$/i.test(file.name)) throw new Error('未选择应用时支持不超过 64 KB 的文本、Markdown 或 CSV；其他附件请先选择应用。');
      return { attachment:{ name:file.name,text:await file.text() } };
    }
    if (file.size > 5 * 1024 * 1024) throw new Error('附件不能超过 5 MB。');
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
    let output = null;
    let trace = null;
    try {
      await enterConversation();
      let pending = null;
      if (state.agentRun) {
        const response = await api(`/api/agent/runs/${encodeURIComponent(state.agentRun)}`);
        pending = response.run || response;
        if (pending.phase !== 'execution_waiting') throw new Error('请先确认、核实或取消当前运行，再开始新的需求。');
      }
      const appId = pending?.app_id || state.app?.id || '', tenantId = state.tenant?.id;
      if (pending && pending.app_id !== (state.app?.id || '')) throw new Error('补充需求时请先切换回当前运行的应用。');
      const attachments = await attachmentInput(form, appId);
      if (state.tenant?.id !== tenantId || !pending && (state.app?.id || '') !== appId) throw new Error('工作区或应用已切换，请重新发送。');
      appendChat(prompt, 'user');
      if (form.elements.attachment?.files?.[0]) appendChat(`附件：${form.elements.attachment.files[0].name}`, 'user');
      state.agentConversationMessages.push({ role: 'user', content: prompt });
      await persistConversation();
      output = appendChat('', 'assistant');
      trace = runTraceElement('', true);
      appendTraceItem(trace, traceGoalText(prompt, appId));
      $('#chat-messages').append(trace);
      const response = pending
        ? await api(`/api/agent/runs/${encodeURIComponent(pending.id)}/continue`, { method: 'POST', body: JSON.stringify({ answer: prompt, expected_version: pending.version,...attachments }) })
        : await api('/api/agent/runs', { method: 'POST', body: JSON.stringify({ app_id:appId,prompt,context:{},...attachments }) });
      const run = response.run || response;
      trace.dataset.agentTrace = run.id;
      state.agentRun = run.id;
      state.agentRunKey = activeRunKey();
      localStorage.setItem(activeRunKey(), run.id);
      const result = await pollRun(run.id, output);
      await rememberOutput(output);
      form.reset();
      const attachmentLabel = form.querySelector('[data-attachment-name]'); if (attachmentLabel) attachmentLabel.textContent = '';
      await renderWorkspace();
      return result;
    } catch (error) {
      if (trace && !trace.dataset.agentTrace) trace.remove();
      if (output && !output.textContent) output.closest('.chat')?.remove();
      toast(error.message || '小助手暂时无法响应。', true);
    }
    finally { state.agentBusy = false; }
  }

  async function clearSavedConversation() {
    if (state.agentRun) throw new Error('请先取消当前运行，再清理对话。');
    if ($('#chat-messages').children.length && !window.confirm('清除整个小助手会话？消息与运行记录展示将一并移除，且无法恢复。')) return;
    state.agentConversationMessages = [];
    state.agentConversationLoadedKey = null;
    forgetRun();
    $('#chat-messages').replaceChildren();
    await conversations.clear();
  }

  async function selectApp(appId) {
    if (appId) {
      const app = state.apps.find((item) => item.id === appId);
      if (!app) throw new Error('当前工作区找不到这个应用');
      state.app = app;
    } else {
      state.app = null;
      state.appPanel = 'runtime';
      state.editingApp = false;
      state.workspaceView = 'home';
    }
    state.appRuntime = null; state.table = null;
    await renderWorkspace();
  }

  return { submitPrompt, enterConversation, clearSavedConversation, clearSavedConversations: clearSavedConversation, selectApp, resumeRun, cancelRun, createTemplate };
}
