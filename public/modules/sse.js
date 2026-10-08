function abortError() {
  const error = new Error('SSE connection aborted');
  error.name = 'AbortError';
  return error;
}

async function responseError(response) {
  const payload = await response.json().catch(() => ({}));
  const error = new Error(payload.error || payload.message || `SSE request failed (${response.status})`);
  error.status = response.status;
  return error;
}

export async function openSSE(url, { headers = {}, signal, onEvent, onToken } = {}) {
  const response = await fetch(url, { headers, signal, cache: 'no-store' });
  const nextToken = response.headers.get('X-PocketBase-Token');
  if (nextToken) onToken?.(nextToken);
  if (!response.ok) throw await responseError(response);
  if (!response.body) throw new Error('SSE response body is unavailable');

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let eventName = 'message';
  let eventId = '';
  let data = [];
  const dispatch = async () => {
    if (!data.length) return;
    const raw = data.join('\n');
    let payload = raw;
    try { payload = JSON.parse(raw); } catch {}
    await onEvent?.({ event: eventName, id: eventId, data: payload, raw });
    eventName = 'message';
    eventId = '';
    data = [];
  };
  const parseLine = async (line) => {
    if (line === '') return dispatch();
    if (line.startsWith(':')) return;
    const separator = line.indexOf(':');
    const field = separator < 0 ? line : line.slice(0, separator);
    const value = separator < 0 ? '' : line.slice(separator + 1).replace(/^ /, '');
    if (field === 'event') eventName = value || 'message';
    else if (field === 'id') eventId = value;
    else if (field === 'data') data.push(value);
  };

  try {
    while (true) {
      if (signal?.aborted) throw abortError();
      const { done, value } = await reader.read();
      buffer += decoder.decode(value || new Uint8Array(), { stream: !done });
      const lines = buffer.split(/\r?\n/);
      buffer = lines.pop() || '';
      for (const line of lines) await parseLine(line);
      if (done) {
        if (buffer) await parseLine(buffer);
        await dispatch();
        return;
      }
    }
  } finally {
    reader.releaseLock();
  }
}
