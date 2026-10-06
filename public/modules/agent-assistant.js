import { formatUIChanges } from './ui-editor.js';
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
  const operationLabels = { 'apps.create': '创建应用', 'backend_plan.apply': '应用后端变更', 'ui.compose': '创建界面草稿', 'business_actions.execute': '执行业务动作' };

  async function loadTemplates() {
    const result = await api('/api/build/templates');
    return result.items || [];
  }

  async function showTemplateChoices() {
    const templates = await loadTemplates();
    const controls = document.createElement('div');
    controls.className = 'flex flex-wrap gap-2';
    for (const template of templates) {
      controls.append(actionButton(template.name, async () => {
        $('#agent-form [name=prompt]').value = `${template.command}：`;
        $('#agent-form [name=prompt]').focus();
        controls.remove();
      }));
    }
    $('#chat-messages').append(controls);
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
    for (const message of state.agentConversationMessages) appendChat(message.content, message.role);
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
    if (last?.role === 'assistant') last.content = output.textContent;
    else state.agentConversationMessages.push({ role: 'assistant', content: output.textContent });
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

  async function pollRun(runID, output) {
    const runScopeKey = activeRunKey();
    const assertScope = () => { if (activeRunKey() !== runScopeKey) throw new Error('工作区上下文已切换；原运行已保留，可切回后继续。'); };
    for (const element of document.querySelectorAll('[data-agent-run]')) {
      if (element.dataset.agentRun === runID) element.remove();
    }
    const controls = document.createElement('div');
    controls.dataset.agentRun = runID;
    controls.className = 'flex flex-wrap gap-2';
    $('#chat-messages').append(controls);
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
      output.textContent = run.error || run.result?.message || (run.state === 'completed' ? (run.result?.version ? `界面草稿 v${run.result.version_number || ''} 已生成。` : '已完成本轮操作。') : '本轮已停止。');
        return run;
      }
      if (run.state === 'waiting_confirmation' && run.phase === 'confirmation') {
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
      output.textContent = '正在读取资源、整理计划并保存执行结果…';
      await new Promise((resolve) => setTimeout(resolve, 700));
    }
  }

  async function resumeRun() {
    if (!state.agentRun) return null;
    const response = await api(`/api/agent/runs/${encodeURIComponent(state.agentRun)}`);
    const run = response.run || response;
    return pollRun(run.id, appendChat('', 'assistant'));
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
    try {
      await enterConversation();
      if (form.id === 'home-agent-form') { state.workspaceView = 'assistant'; await renderWorkspace(); }
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
      const output = appendChat('', 'assistant');
      const response = pending
        ? await api(`/api/agent/runs/${encodeURIComponent(pending.id)}/continue`, { method: 'POST', body: JSON.stringify({ answer: prompt, expected_version: pending.version,...attachments }) })
        : await api('/api/agent/runs', { method: 'POST', body: JSON.stringify({ app_id:appId,prompt,context:{},...attachments }) });
      const run = response.run || response;
      state.agentRun = run.id;
      state.agentRunKey = activeRunKey();
      localStorage.setItem(activeRunKey(), run.id);
      const result = await pollRun(run.id, output);
      await rememberOutput(output);
      form.reset();
      const attachmentLabel = form.querySelector('[data-attachment-name]'); if (attachmentLabel) attachmentLabel.textContent = '';
      await renderWorkspace();
      return result;
    } catch (error) { toast(error.message || '小助手暂时无法响应。', true); }
    finally { state.agentBusy = false; }
  }

  async function startUIEdit(request) {
    if (state.agentBusy || state.agentRun) throw new Error('请先完成或取消当前小助手运行，再开始界面修改。');
    const appId = state.app?.id, tenantId = state.tenant?.id;
    if (request.app_id !== appId) throw new Error('应用上下文已切换，请重新载入界面。');
    state.agentBusy = true;
    try {
      await enterConversation();
      if (state.app?.id !== appId || state.tenant?.id !== tenantId) throw new Error('工作区上下文已切换，请重新载入界面。');
      state.workspaceView = 'assistant'; await renderWorkspace();
      appendChat(request.prompt, 'user');
      state.agentConversationMessages.push({ role:'user',content:request.prompt });
      await persistConversation();
      const output = appendChat('', 'assistant');
      const response = await api('/api/agent/runs', { method:'POST',body:JSON.stringify(request) });
      const run = response.run || response;
      state.agentRun = run.id; state.agentRunKey = activeRunKey(); localStorage.setItem(activeRunKey(),run.id);
      await pollRun(run.id,output); await rememberOutput(output);
    } finally { state.agentBusy = false; }
  }

  async function clearSavedConversation() {
    if (state.agentRun) throw new Error('请先取消当前运行，再清理对话。');
    state.agentConversationMessages = [];
    state.agentConversationLoadedKey = null;
    forgetRun();
    $('#chat-messages').replaceChildren();
    await conversations.clear();
  }

  async function selectApp(appId) {
    const app = state.apps.find((item) => item.id === appId);
    if (!app) throw new Error('当前工作区找不到这个工具');
    state.app = app; state.appRuntime = null; state.table = null;
    await renderWorkspace();
  }

  return { submitPrompt, startUIEdit, enterConversation, clearSavedConversation, clearSavedConversations: clearSavedConversation, selectApp, resumeRun, cancelRun, showTemplateChoices };
}
