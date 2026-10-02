const maxCheckpointBytes = 4 * 1024 * 1024;
const maxTranscriptCharacters = 256 * 1024;
const maxTranscriptMessages = 240;

export function fxAuthorizationScope(tenant, apps) {
  const permissions = (apps || [])
    .map((app) => [String(app.id), String(app.permission || '')])
    .sort(([left], [right]) => left.localeCompare(right));
  return JSON.stringify({ role: String(tenant?.role || ''), apps: permissions });
}

function visibleMessages(messages) {
  let remaining = maxTranscriptCharacters;
  const result = [];
  for (const message of [...(messages || [])].reverse()) {
    if (!['user', 'assistant'].includes(message?.role) || typeof message.content !== 'string') continue;
    const content = message.content.slice(0, Math.min(8192, remaining));
    if (!content) continue;
    result.push({ role: message.role, content });
    remaining -= content.length;
    if (remaining <= 0 || result.length >= maxTranscriptMessages) break;
  }
  return result.reverse();
}

export function createServerFxConversationStore(api) {
  return {
    async load(scope) {
      const { conversation } = await api('/api/agent/conversation');
      if (!conversation || conversation.scope !== scope) return null;
      return { ...conversation, checkpoint: Uint8Array.from(atob(conversation.checkpoint), (char) => char.charCodeAt(0)) };
    },
    async save({ scope, expectedRevision, checkpoint, messages }) {
      const bytes = checkpoint instanceof Uint8Array ? checkpoint : new Uint8Array(checkpoint);
      if (!bytes.byteLength || bytes.byteLength > maxCheckpointBytes) throw new Error('fx 会话检查点为空或超过 4 MB 保存上限');
      let binary = '';
      for (let index = 0; index < bytes.length; index += 32768) binary += String.fromCharCode(...bytes.subarray(index, index + 32768));
      return api('/api/agent/conversation', { method: 'PUT', body: JSON.stringify({ scope, expected_revision: expectedRevision, checkpoint: btoa(binary), messages: visibleMessages(messages) }) });
    },
    clear() { return api('/api/agent/conversation', { method: 'DELETE' }); },
  };
}
