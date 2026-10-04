import { createServerFxConversationStore, fxAuthorizationScope } from '/modules/fx-conversation-store.js';

export function createFxAssistant({ state, api, $, esc, toast, renderWorkspace, runtime, tokenKey, resetAgentConversation }) {
  const conversations = createServerFxConversationStore(api);
  state.fxConversationMessages ||= [];
  state.fxTurnNumber ||= 0;
  state.fxPreviewedVersions ||= new Map();
  state.fxHarnessRun ||= null;
  const conversationKey = () => state.user?.id && state.tenant?.id ? `${state.user.id}:${state.tenant.id}` : null;
  const appendChat = (message, role) => { const node = document.createElement('div'); node.className = `chat chat-${role === 'user' ? 'end' : 'start'}`; node.innerHTML = `<div class="chat-bubble">${esc(message).replace(/\n/g, '<br>')}</div>`; $('#chat-messages').append(node); $('#chat-messages').scrollTop = $('#chat-messages').scrollHeight; return node.querySelector('.chat-bubble'); };
  async function enterConversation() {
    const key = conversationKey();
    if (!key || state.fxConversationLoadedKey === key) return;
    const saved = await conversations.load(state.user.id, state.tenant.id).catch(() => null);
    if (saved?.messages?.length) { state.fxConversationMessages = saved.messages; for (const message of saved.messages) appendChat(message.content, message.role); }
    state.fxConversationLoadedKey = key;
  }
  async function persistConversation() { if (state.user?.id && state.tenant?.id) await conversations.save(state.user.id, state.tenant.id, state.fxConversationMessages); }
  async function clearSavedConversation() { state.fxConversationMessages = []; state.fxConversationLoadedKey = null; $('#chat-messages').innerHTML = ''; await conversations.clear(state.user.id, state.tenant.id); }
  async function clearSavedConversations() { return clearSavedConversation(); }
  async function selectApp(appId) { const app = state.apps.find((item) => item.id === appId); if (!app) throw new Error('当前工作区找不到这个工具'); state.app = app; state.appRuntime = null; state.table = null; await renderWorkspace(); }
  async function candidates() {
    const catalog = state.app ? await api(`/api/apps/${encodeURIComponent(state.app.id)}/backend/catalog`) : await api('/api/backend/catalog');
    return (catalog.capabilities || []).map((item) => ({ id: item.id, capability: item.id, description: item.purpose || item.description || '', input: item.parameters || {}, write: item.impact !== 'read' }));
  }
  async function pollRun(runID, output) {
    let after = 0;
    for (;;) {
      const events = await api(`/api/agent/runs/${encodeURIComponent(runID)}/events?after=${after}`);
      for (const event of events.events || []) {
        after = Math.max(after, Number(event.sequence) || after);
        const data = event.data || {};
        if (event.type === 'text' || event.type === 'assistant_message' || event.type === 'message') output.textContent += String(data.text || data.content || '');
        if (event.type === 'confirmation_required' || event.type === 'candidate') {
          const label = data.description || data.capability || '需要确认的操作';
          const button = document.createElement('button'); button.className = 'btn btn-warning btn-sm'; button.textContent = `确认：${label}`; button.onclick = async () => { button.disabled = true; const run = await api(`/api/agent/runs/${encodeURIComponent(runID)}/confirm`, { method: 'POST', body: JSON.stringify({ expected_version: data.expected_version || data.version || 0, decision: 'confirm' }) }); button.replaceWith(document.createTextNode(run.status === 'completed' ? '已确认' : '已提交确认')); }; $('#chat-messages').append(button);
        }
      }
      const run = await api(`/api/agent/runs/${encodeURIComponent(runID)}`);
      if (['completed', 'failed', 'cancelled', 'unavailable'].includes(run.status)) { if (!output.textContent) output.textContent = run.error || '本轮已结束，请查看实际运行状态。'; return run; }
      await new Promise((resolve) => setTimeout(resolve, 700));
    }
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
      const context = { app_id: state.app?.id || null, page: state.appRuntime?.ui_page || null, record_id: state.runtimeSelectedRecord || null, candidates: await candidates() };
      const response = await api('/api/agent/runs', { method: 'POST', body: JSON.stringify({ app_id: state.app?.id || '', prompt, context }) });
      const run = response.run || response;
      state.fxHarnessRun = run.id;
      const result = await pollRun(run.id, output);
      state.fxConversationMessages.push({ role: 'user', content: prompt }, { role: 'assistant', content: output.textContent });
      await persistConversation();
      form.reset();
      await renderWorkspace();
      return result;
    } catch (error) { toast(error.message || '小助手暂时无法响应。', true); }
    finally { state.fxBusy = false; }
  }
  return { submitPrompt, enterConversation, clearSavedConversation, clearSavedConversations, selectApp };
}
