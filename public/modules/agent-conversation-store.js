const maxTranscriptBytes = 256 * 1024;
const maxTranscriptMessages = 240;

export function authorizationScope(tenant, apps, userId) {
  const permissions = (apps || []).map((app) => [String(app.id), String(app.permission || '')]).sort(([left], [right]) => left < right ? -1 : left > right ? 1 : 0);
  return JSON.stringify({ tenant_id: String(tenant?.id || ''), user_id: String(userId || ''), role: String(tenant?.role || ''), apps: permissions });
}

function visibleMessages(messages) {
  let remaining = maxTranscriptBytes - 2;
  const result = [];
  const encoder = new TextEncoder();
  let previous = null;
  const sizeOf = (message) => encoder.encode(JSON.stringify(message).replace(/[<>&\u2028\u2029]/g, (character) => `\\u${character.charCodeAt(0).toString(16).padStart(4, '0')}`)).length;
  for (const message of [...(messages || [])].reverse()) {
    if (!['user', 'assistant'].includes(message?.role) || typeof message.content !== 'string') continue;
    let content = message.content.slice(0, 8192);
    const record = { role: message.role, content };
    if (typeof message.run_id === 'string' && /^[0-9a-z]{15,64}$/i.test(message.run_id)) record.run_id = message.run_id;
    if (previous && previous.role === record.role && previous.content === record.content && !previous.run_id && !record.run_id) continue;
    let serializedBytes = sizeOf(record) + (result.length ? 1 : 0);
    if (serializedBytes > remaining) {
      // JSON escapes can use six bytes per character; retain a bounded excerpt.
      content = content.slice(0, Math.max(0, Math.floor((remaining - 40) / 6)));
      serializedBytes = sizeOf(record) + (result.length ? 1 : 0);
    }
    if (!content) continue;
    record.content = content;
    result.push(record);
    previous = record;
    remaining -= serializedBytes;
    if (remaining <= 0 || result.length >= maxTranscriptMessages) break;
  }
  return result.reverse();
}

export function createServerConversationStore(api) {
  return {
    async load(scope) {
      const { conversation } = await api('/api/agent/conversation');
      if (!conversation || conversation.scope !== scope) return null;
      return conversation;
    },
    async save({ scope, expectedRevision, messages }) {
      return api('/api/agent/conversation', { method: 'PUT', body: JSON.stringify({ scope, expected_revision: expectedRevision, messages: visibleMessages(messages) }) });
    },
    clear() { return api('/api/agent/conversation', { method: 'DELETE' }); },
  };
}
