import { createServerFxConversationStore, fxAuthorizationScope } from '/modules/fx-conversation-store.js';

export function createFxAssistant({ state, api, $, esc, toast, renderWorkspace, runtime, tokenKey, resetAgentConversation }) {
  const conversations = createServerFxConversationStore(api);
  state.fxConversationMessages ||= [];
  state.fxTurnNumber ||= 0;
  state.fxPreviewedVersions ||= new Map();
  state.fxHarnessRun ||= null;
  const activeRunKey = () => `miao-harness-run:${state.user?.id || ''}:${state.tenant?.id || ''}`;
  const conversationScope = () => fxAuthorizationScope(state.tenant, state.apps || []);
  const appendChat = (message, role) => { const node = document.createElement('div'); node.className = `chat chat-${role === 'user' ? 'end' : 'start'}`; node.innerHTML = `<div class="chat-bubble">${esc(message).replace(/\n/g, '<br>')}</div>`; $('#chat-messages').append(node); $('#chat-messages').scrollTop = $('#chat-messages').scrollHeight; return node.querySelector('.chat-bubble'); };
  async function enterConversation() {
    if (!state.fxHarnessRun) state.fxHarnessRun = localStorage.getItem(activeRunKey());
    const scope = conversationScope();
    if (!scope || state.fxConversationLoadedKey === scope) return;
    const saved = await conversations.load(scope).catch(() => null);
    if (saved?.messages?.length) { state.fxConversationMessages = saved.messages; for (const message of saved.messages) appendChat(message.content, message.role); state.fxConversationRevision = saved.revision; }
    else state.fxConversationRevision = 0;
    state.fxConversationLoadedKey = scope;
    if (state.fxHarnessRun) await resumeRun().catch((error) => toast(error.message || '运行状态恢复失败', true));
  }
  async function persistConversation() {
    const scope = conversationScope();
    if (!scope) return;
    const messages = state.fxConversationMessages;
    const checkpoint = new TextEncoder().encode(JSON.stringify(messages));
    const result = await conversations.save({ scope, expectedRevision: state.fxConversationRevision || 0, checkpoint, messages });
    if (result.saved) state.fxConversationRevision = result.revision;
  }
  async function clearSavedConversation() { state.fxConversationMessages = []; state.fxConversationLoadedKey = null; state.fxHarnessRun = null; localStorage.removeItem(activeRunKey()); $('#chat-messages').innerHTML = ''; await conversations.clear(); }
  async function clearSavedConversations() { return clearSavedConversation(); }
  async function selectApp(appId) { const app = state.apps.find((item) => item.id === appId); if (!app) throw new Error('当前工作区找不到这个工具'); state.app = app; state.appRuntime = null; state.table = null; await renderWorkspace(); }
  async function candidates() {
    if (!state.app) return [];
    const result = await api(`/api/apps/${encodeURIComponent(state.app.id)}/backend/candidates`);
    return (result.candidates || []).filter((item) => item.available !== false).map((item) => ({ id: item.id, capability: item.capability, description: item.description || '', write: item.impact !== 'read' }));
  }
  async function cancelRun(runID) {
    await api(`/api/agent/runs/${encodeURIComponent(runID)}/cancel`, { method: 'POST', body: JSON.stringify({}) });
  }
  async function pollRun(runID, output) {
    let after = 0;
    const cancel = document.createElement('button');
    cancel.className = 'btn btn-ghost btn-sm';
    cancel.textContent = '取消运行';
    cancel.onclick = async () => { cancel.disabled = true; try { await cancelRun(runID); } catch (error) { cancel.disabled = false; toast(error.message || '取消失败', true); } };
    $('#chat-messages').append(cancel);
    for (;;) {
      const events = await api(`/api/agent/runs/${encodeURIComponent(runID)}/events?after=${after}`);
      for (const event of events.events || []) {
        after = Math.max(after, Number(event.sequence) || after);
        const data = event.data || {};
        if (event.type === 'text' || event.type === 'assistant_message' || event.type === 'message') output.textContent += String(data.text || data.content || '');
        if (event.type === 'confirmation_required') {
          const button = document.createElement('button'); button.className = 'btn btn-warning btn-sm'; button.textContent = '确认写入'; button.onclick = async () => { button.disabled = true; try { await api(`/api/agent/runs/${encodeURIComponent(runID)}/confirm`, { method: 'POST', body: JSON.stringify({ expected_version: data.version || 0, decision: 'confirm' }) }); button.replaceWith(document.createTextNode('已提交确认')); } catch (error) { button.disabled = false; toast(error.message || '确认失败', true); } }; $('#chat-messages').append(button);
        }
      }
      const response = await api(`/api/agent/runs/${encodeURIComponent(runID)}`);
      const run = response.run || response;
      const stateName = run.state || run.status;
      if (['completed', 'failed', 'cancelled', 'unavailable'].includes(stateName)) { cancel.remove(); state.fxHarnessRun = null; localStorage.removeItem(activeRunKey()); if (!output.textContent) output.textContent = run.error || (stateName === 'completed' ? JSON.stringify(run.result || '') : '本轮已结束。'); return run; }
      await new Promise((resolve) => setTimeout(resolve, 700));
    }
  }
  async function resumeRun() {
    if (!state.fxHarnessRun) return null;
    const response = await api(`/api/agent/runs/${encodeURIComponent(state.fxHarnessRun)}`);
    const run = response.run || response;
    const output = appendChat('', 'assistant');
    return pollRun(run.id, output);
  }
  async function submitPrompt(event) {
    event.preventDefault();
    if (state.fxBusy) return;
    const form = event.currentTarget;
    const prompt = String(new FormData(form).get('prompt') || '').trim();
    if (!prompt) return;
    state.fxBusy = true;
    try {
      await enterConversation();
      if (event.currentTarget.id === 'home-agent-form') { state.workspaceView = 'assistant'; await renderWorkspace(); }
      appendChat(prompt, 'user');
      const output = appendChat('', 'assistant');
      state.fxTurnNumber += 1;
      const candidate_ids = (await candidates()).map((candidate) => candidate.id);
      const context = { app_id: state.app?.id || null, page: state.appRuntime?.ui_page || null, record_id: state.runtimeSelectedRecord || null, candidate_ids };
      const response = await api('/api/agent/runs', { method: 'POST', body: JSON.stringify({ app_id: state.app?.id || '', prompt, context }) });
      const run = response.run || response;
      state.fxHarnessRun = run.id;
      localStorage.setItem(activeRunKey(), run.id);
      const result = await pollRun(run.id, output);
      state.fxConversationMessages.push({ role: 'user', content: prompt }, { role: 'assistant', content: output.textContent });
      await persistConversation();
      form.reset();
      await renderWorkspace();
      return result;
    } catch (error) { toast(error.message || '小助手暂时无法响应。', true); }
    finally { state.fxBusy = false; }
  }
  return { submitPrompt, enterConversation, clearSavedConversation, clearSavedConversations, selectApp, resumeRun, cancelRun };
}
