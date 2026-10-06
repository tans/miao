import { createServerConversationStore, authorizationScope } from '/modules/agent-conversation-store.js';

export function createAgentAssistant({ state, api, $, esc, toast, renderWorkspace }) {
  const conversations = createServerConversationStore(api);
  state.agentConversationMessages ||= [];
  state.agentConversationRevision ||= 0;
  state.agentRun ||= null;
  const activeRunKey = () => `miao-agent-run:${state.user?.id || ''}:${state.tenant?.id || ''}`;
  const conversationScope = () => authorizationScope(state.tenant, state.apps || []);
  const appendChat = (message, role) => { const node = document.createElement('div'); node.className = `chat chat-${role === 'user' ? 'end' : 'start'}`; node.innerHTML = `<div class="chat-bubble">${esc(message).replace(/\n/g, '<br>')}</div>`; $('#chat-messages').append(node); $('#chat-messages').scrollTop = $('#chat-messages').scrollHeight; return node.querySelector('.chat-bubble'); };
  async function enterConversation() {
    if (!state.agentRun) state.agentRun = localStorage.getItem(activeRunKey());
    const scope = conversationScope();
    if (!scope || state.agentConversationLoadedKey === scope) return;
    const saved = await conversations.load(scope).catch(() => null);
    if (saved?.messages?.length) { state.agentConversationMessages = saved.messages; for (const message of saved.messages) appendChat(message.content, message.role); state.agentConversationRevision = saved.revision; }
    else state.agentConversationRevision = 0;
    state.agentConversationLoadedKey = scope;
    if (state.agentRun) await resumeRun().catch((error) => toast(error.message || '运行状态恢复失败', true));
  }
  async function persistConversation() {
    const scope = conversationScope();
    if (!scope) return;
    const result = await conversations.save({ scope, expectedRevision: state.agentConversationRevision || 0, checkpoint: new TextEncoder().encode(JSON.stringify(state.agentConversationMessages)), messages: state.agentConversationMessages });
    if (result.saved) state.agentConversationRevision = result.revision;
  }
  async function clearSavedConversation() { state.agentConversationMessages = []; state.agentConversationLoadedKey = null; state.agentRun = null; localStorage.removeItem(activeRunKey()); $('#chat-messages').innerHTML = ''; await conversations.clear(); }
  async function clearSavedConversations() { return clearSavedConversation(); }
  async function selectApp(appId) { const app = state.apps.find((item) => item.id === appId); if (!app) throw new Error('当前工作区找不到这个工具'); state.app = app; state.appRuntime = null; state.table = null; await renderWorkspace(); }
  async function candidates() {
    if (!state.app) return [];
    const result = await api(`/api/apps/${encodeURIComponent(state.app.id)}/backend/candidates`);
    return (result.candidates || []).filter((item) => item.available !== false).map((item) => ({ id: item.id, description: item.description || '', write: item.impact !== 'read' }));
  }
  async function cancelRun(runID) { await api(`/api/agent/runs/${encodeURIComponent(runID)}/cancel`, { method: 'POST', body: JSON.stringify({}) }); }
  async function pollRun(runID, output) {
    let after = 0;
    const cancel = document.createElement('button'); cancel.className = 'btn btn-ghost btn-sm'; cancel.textContent = '取消运行';
    cancel.onclick = async () => { cancel.disabled = true; try { await cancelRun(runID); } catch (error) { cancel.disabled = false; toast(error.message || '取消失败', true); } };
    $('#chat-messages').append(cancel);
    for (;;) {
      const events = await api(`/api/agent/runs/${encodeURIComponent(runID)}/events?after=${after}`);
      for (const event of events.events || []) {
        after = Math.max(after, Number(event.sequence) || after);
        const data = event.data || {};
        if (event.type === 'text' || event.type === 'assistant_message' || event.type === 'message') output.textContent += String(data.text || data.content || '');
        if (event.type === 'confirmation_required') { const button = document.createElement('button'); button.className = 'btn btn-warning btn-sm'; button.textContent = '确认写入'; button.onclick = async () => { button.disabled = true; try { await api(`/api/agent/runs/${encodeURIComponent(runID)}/confirm`, { method: 'POST', body: JSON.stringify({ expected_version: data.version || 0, decision: 'confirm' }) }); button.replaceWith(document.createTextNode('已提交确认')); } catch (error) { button.disabled = false; toast(error.message || '确认失败', true); } }; $('#chat-messages').append(button); }
      }
      const response = await api(`/api/agent/runs/${encodeURIComponent(runID)}`); const run = response.run || response;
      if (['completed', 'failed', 'cancelled', 'unavailable'].includes(run.state || run.status)) { cancel.remove(); state.agentRun = null; localStorage.removeItem(activeRunKey()); if (!output.textContent) output.textContent = run.error || (run.state === 'completed' ? JSON.stringify(run.result || '') : '本轮已结束。'); return run; }
      await new Promise((resolve) => setTimeout(resolve, 700));
    }
  }
  async function resumeRun() { if (!state.agentRun) return null; const response = await api(`/api/agent/runs/${encodeURIComponent(state.agentRun)}`); const run = response.run || response; return pollRun(run.id, appendChat('', 'assistant')); }
  async function submitPrompt(event) {
    event.preventDefault(); if (state.agentBusy) return;
    const form = event.currentTarget; const prompt = String(new FormData(form).get('prompt') || '').trim(); if (!prompt) return;
    state.agentBusy = true;
    try { await enterConversation(); if (event.currentTarget.id === 'home-agent-form') { state.workspaceView = 'assistant'; await renderWorkspace(); }
      appendChat(prompt, 'user'); const output = appendChat('', 'assistant');
      const response = await api('/api/agent/runs', { method: 'POST', body: JSON.stringify({ app_id: state.app?.id || '', prompt, context: { app_id: state.app?.id || null, candidate_ids: (await candidates()).map((candidate) => candidate.id) } }) });
      const run = response.run || response; state.agentRun = run.id; localStorage.setItem(activeRunKey(), run.id); const result = await pollRun(run.id, output);
      state.agentConversationMessages.push({ role: 'user', content: prompt }, { role: 'assistant', content: output.textContent }); await persistConversation(); form.reset(); await renderWorkspace(); return result;
    } catch (error) { toast(error.message || '小助手暂时无法响应。', true); } finally { state.agentBusy = false; }
  }
  return { submitPrompt, enterConversation, clearSavedConversation, clearSavedConversations, selectApp, resumeRun, cancelRun };
}
