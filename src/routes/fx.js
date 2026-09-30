import { Readable } from 'node:stream';
import { pocketbase } from '../store.js';
import { readAIConfig } from '../ai-settings.js';

const gatewayOrigin = 'https://ai-gateway.vercel.sh';
const gatewayPaths = new Set([
  '/v3/ai/language-model',
  '/v4/ai/language-model',
  '/coding-agent/v1/models',
]);
const forwardedRequestHeaders = [
  'accept',
  'content-type',
  'x-vercel-ai-data-stream',
  'x-vercel-ai-sdk-version',
];
const forwardedResponseHeaders = [
  'cache-control',
  'content-type',
  'retry-after',
  'x-ai-gateway-provider',
  'x-vercel-ai-gateway-provider',
  'x-vercel-ai-data-stream',
];
const persistentRateLimited = async (tenantId, userId) => {
  const start = new Date(Date.now() - 60_000).toISOString();
  const [tenant, user, global] = await Promise.all([
    pocketbase.collection('ai_usage').getList(1, 1, { filter: pocketbase.filter('tenant_id = {:tenantId} && created >= {:start}', { tenantId, start }) }),
    pocketbase.collection('ai_usage').getList(1, 1, { filter: pocketbase.filter('tenant_id = {:tenantId} && user_id = {:userId} && created >= {:start}', { tenantId, userId, start }) }),
    pocketbase.collection('ai_usage').getList(1, 1, { filter: pocketbase.filter('created >= {:start}', { start }) })
  ]);
  return tenant.totalItems >= 120 || user.totalItems >= 30 || global.totalItems >= 500;
};

const dailyUsage = async (tenantId) => {
  const start = new Date();
  start.setUTCHours(0, 0, 0, 0);
  const filter = pocketbase.filter('tenant_id = {:tenantId} && created >= {:start}', { tenantId, start: start.toISOString() });
  return pocketbase.collection('ai_usage').getList(1, 1, { filter });
};

const tokenUsage = (value) => {
  if (!value || typeof value !== 'object') return null;
  if (value.usage && typeof value.usage === 'object') {
    const usage = value.usage;
    const input = Number(usage.inputTokens ?? usage.promptTokens ?? usage.prompt_tokens ?? 0);
    const output = Number(usage.outputTokens ?? usage.completionTokens ?? usage.completion_tokens ?? 0);
    if (input || output) return { input: Math.max(0, input), output: Math.max(0, output) };
  }
  for (const child of Object.values(value)) {
    const result = tokenUsage(child);
    if (result) return result;
  }
  return null;
};

const parseTokenUsage = (text) => {
  for (const line of text.split(/\r?\n/).reverse()) {
    const payload = line.startsWith('data:') ? line.slice(5).trim() : line.trim();
    if (!payload || payload === '[DONE]') continue;
    try {
      const found = tokenUsage(JSON.parse(payload));
      if (found) return found;
    } catch {}
  }
  return { input: 0, output: 0 };
};

const textParts = (content) => {
  if (typeof content === 'string') return content;
  if (!Array.isArray(content)) return '';
  return content.filter((part) => part?.type === 'text' && typeof part.text === 'string').map((part) => part.text).join('');
};

const normalizeToolInput = (value) => {
  if (typeof value === 'string') return value;
  try { return JSON.stringify(value ?? {}); } catch { return '{}'; }
};

const openAIMessage = (message) => {
  if (!message || typeof message !== 'object') return null;
  const role = message.role;
  if (role === 'system' || role === 'developer') return { role: 'system', content: textParts(message.content) };
  if (role === 'user') {
    if (typeof message.content === 'string') return { role, content: message.content };
    const content = (message.content || []).flatMap((part) => {
      if (part?.type === 'text') return [{ type: 'text', text: part.text }];
      if (part?.type === 'image' && part.image) {
        if (part.image instanceof URL) return [{ type: 'image_url', image_url: { url: part.image.href } }];
        const image = typeof part.image === 'string' ? part.image : Buffer.from(part.image).toString('base64');
        if (/^https?:\/\//i.test(image)) return [{ type: 'image_url', image_url: { url: image } }];
        const mediaType = part.mediaType || 'image/png';
        return [{ type: 'image_url', image_url: { url: image.startsWith('data:') ? image : `data:${mediaType};base64,${image}` } }];
      }
      return [];
    });
    return { role, content };
  }
  if (role === 'assistant') {
    const parts = Array.isArray(message.content) ? message.content : [];
    const toolCalls = parts.filter((part) => part?.type === 'tool-call').map((part) => ({
      id: part.toolCallId,
      type: 'function',
      function: { name: part.toolName, arguments: normalizeToolInput(part.args) },
    }));
    return { role, content: textParts(message.content) || null, ...(toolCalls.length ? { tool_calls: toolCalls } : {}) };
  }
  if (role === 'tool') {
    const results = (message.content || []).filter((part) => part?.type === 'tool-result');
    return results.map((part) => ({
      role: 'tool', tool_call_id: part.toolCallId, name: part.toolName,
      content: typeof part.result === 'string' ? part.result : JSON.stringify(part.result ?? null),
    }));
  }
  return null;
};

const toOpenAIRequest = (body, model) => {
  const messages = (Array.isArray(body.prompt) ? body.prompt : []).flatMap((message) => {
    const converted = openAIMessage(message);
    return Array.isArray(converted) ? converted : converted ? [converted] : [];
  });
  const tools = (Array.isArray(body.tools) ? body.tools : []).filter((tool) => tool?.type === 'function' && tool.name).map((tool) => ({
    type: 'function',
    function: { name: tool.name, description: tool.description || '', parameters: tool.inputSchema || { type: 'object', properties: {} } },
  }));
  const choices = body.toolChoice;
  const toolChoice = choices === 'none' || choices === 'auto' || choices === 'required'
    ? choices
    : choices?.type === 'tool' ? { type: 'function', function: { name: choices.toolName } } : undefined;
  return {
    model,
    messages,
    stream: true,
    stream_options: { include_usage: true },
    ...(body.maxOutputTokens ? { max_tokens: body.maxOutputTokens } : {}),
    ...(body.temperature !== undefined ? { temperature: body.temperature } : {}),
    ...(body.topP !== undefined ? { top_p: body.topP } : {}),
    ...(body.stopSequences ? { stop: body.stopSequences } : {}),
    ...(tools.length ? { tools, ...(toolChoice ? { tool_choice: toolChoice } : {}) } : {}),
  };
};

const languageModelStream = (openAIResponse) => {
  if (!openAIResponse.body) return null;
  const reader = openAIResponse.body.getReader();
  const decoder = new TextDecoder();
  const encoder = new TextEncoder();
  const stream = new ReadableStream({
    async start(controller) {
      let pending = '';
      const tools = new Map();
      let textStarted = false;
      let finishReason = 'stop';
      let usage = {};
      const emit = (part) => controller.enqueue(encoder.encode(`data: ${JSON.stringify(part)}\n\n`));
      const handleLine = (line) => {
        if (!line.startsWith('data:')) return;
        const data = line.slice(5).trim();
        if (!data || data === '[DONE]') return;
        let chunk;
        try { chunk = JSON.parse(data); } catch { return; }
        const choice = chunk.choices?.[0];
        const delta = choice?.delta || {};
        if (chunk.usage) usage = chunk.usage;
        if (typeof delta.content === 'string' && delta.content) {
          if (!textStarted) { emit({ type: 'text-start', id: 'miao-text' }); textStarted = true; }
          emit({ type: 'text-delta', id: 'miao-text', delta: delta.content });
        }
        const reasoning = delta.reasoning_content || delta.reasoning;
        if (typeof reasoning === 'string' && reasoning) emit({ type: 'reasoning', textDelta: reasoning });
        for (const item of delta.tool_calls || []) {
          const existing = tools.get(item.index) || { id: '', name: '', args: '' };
          if (item.id) existing.id = item.id;
          if (item.function?.name) existing.name = item.function.name;
          if (item.function?.arguments) existing.args += item.function.arguments;
          tools.set(item.index, existing);
          if (existing.id && existing.name) emit({ type: 'tool-call-delta', toolCallType: 'function', toolCallId: existing.id, toolName: existing.name, argsTextDelta: item.function?.arguments || '' });
        }
        if (choice?.finish_reason === 'tool_calls') finishReason = 'tool-calls';
        else if (choice?.finish_reason === 'length') finishReason = 'length';
      };
      try {
        while (true) {
          const { value, done } = await reader.read();
          pending += decoder.decode(value || new Uint8Array(), { stream: !done });
          const lines = pending.split(/\r?\n/);
          pending = lines.pop() || '';
          for (const line of lines) handleLine(line);
          if (done) { if (pending) handleLine(pending); break; }
        }
        for (const item of tools.values()) emit({ type: 'tool-call', toolCallType: 'function', toolCallId: item.id, toolName: item.name, args: item.args || '{}' });
        emit({ type: 'finish', finishReason, usage: { promptTokens: usage.prompt_tokens, completionTokens: usage.completion_tokens } });
        controller.close();
      } catch (error) { controller.error(error); }
      finally { reader.releaseLock(); }
    },
    async cancel(reason) { await reader.cancel(reason).catch(() => {}); },
  });
  return stream;
};

// Shared model transport for browser fx and unattended fx. It never accepts a caller-supplied origin.
export const createGatewayTransport = ({ tenantId, userId, appId = '', authorize = async () => {}, beforeRequest = async () => {} }) => async (url, init = {}) => {
  await authorize();
  const path = new URL(typeof url === 'string' ? url : url.url || String(url)).pathname;
  const method = String(init.method || 'GET').toUpperCase();
  if (!gatewayPaths.has(path) || !['GET', 'POST'].includes(method)) throw new Error('AI 请求路径无效');
  const config = await readAIConfig();
  if (!config.key) throw Object.assign(new Error('企业尚未配置 AI 服务密钥'), { statusCode: 503 });
  const tenant = await pocketbase.collection('tenants').getOne(tenantId);
  if (Number(tenant.ai_daily_limit) > 0 && (await dailyUsage(tenantId)).totalItems >= tenant.ai_daily_limit) throw Object.assign(new Error('工作区已达到今日 AI 请求预算'), { statusCode: 429 });
  if (await persistentRateLimited(tenantId, userId)) throw Object.assign(new Error('AI 请求次数过多，请稍后重试'), { statusCode: 429 });
  await beforeRequest();
  const usage = await pocketbase.collection('ai_usage').create({ tenant_id: tenantId, user_id: userId, app_id: appId, status: 100, input_tokens: 0, output_tokens: 0 });
  try {
    const headers = new Headers();
    const supplied = new Headers(init.headers);
    for (const name of forwardedRequestHeaders) if (supplied.has(name)) headers.set(name, supplied.get(name));
    headers.set('authorization', `Bearer ${config.key}`);
    let upstream;
    if (config.provider === 'capi') {
      const base = config.baseUrl.replace(/\/+$/, '');
      if (path === '/coding-agent/v1/models' && method === 'GET') {
        upstream = await fetch(`${base}/models?modality=text`, { headers, signal: init.signal, redirect: 'error' });
        await pocketbase.collection('ai_usage').update(usage.id, { status: upstream.status });
        if (!upstream.ok) return upstream;
        const catalog = await upstream.json();
        return Response.json({ ...catalog, data: (catalog.data || []).map((model) => ({ ...model, type: 'language' })) });
      }
      if (method !== 'POST') throw new Error('模型请求必须使用 POST');
      const body = typeof init.body === 'string' ? JSON.parse(init.body) : init.body || {};
      const model = config.model || supplied.get('ai-language-model-id') || 'gpt-5.2';
      upstream = await fetch(`${base}/chat/completions`, { method, headers: { authorization: `Bearer ${config.key}`, 'content-type': 'application/json', accept: 'text/event-stream' }, body: JSON.stringify(toOpenAIRequest(body, model)), signal: init.signal, redirect: 'error' });
    } else {
      // Preserve SDK model selection headers while keeping credentials host-owned.
      for (const name of ['ai-language-model-id', 'ai-language-model-version']) if (supplied.has(name)) headers.set(name, supplied.get(name));
      upstream = await fetch(`${gatewayOrigin}${path}`, { ...init, method, headers, redirect: 'error' });
    }
    await pocketbase.collection('ai_usage').update(usage.id, { status: upstream.status });
    const responseHeaders = new Headers();
    for (const name of forwardedResponseHeaders) if (upstream.headers.has(name)) responseHeaders.set(name, upstream.headers.get(name));
    responseHeaders.set('cache-control', 'no-store');
    if (!upstream.ok || !upstream.body) return new Response(upstream.body, { status: upstream.status, headers: responseHeaders });
    const stream = config.provider === 'capi' ? languageModelStream(upstream) : upstream.body;
    if (config.provider === 'capi') { responseHeaders.set('content-type', 'text/event-stream; charset=utf-8'); responseHeaders.set('x-vercel-ai-data-stream', 'v1'); }
    const decoder = new TextDecoder();
    let captured = '';
    const monitored = stream.pipeThrough(new TransformStream({
      transform(chunk, controller) { captured = (captured + decoder.decode(chunk, { stream: true })).slice(-250000); controller.enqueue(chunk); },
      async flush() {
        captured += decoder.decode();
        const tokens = parseTokenUsage(captured);
        await pocketbase.collection('ai_usage').update(usage.id, { input_tokens: tokens.input, output_tokens: tokens.output });
      }
    }));
    return new Response(monitored, { status: upstream.status, headers: responseHeaders });
  } catch (error) {
    await pocketbase.collection('ai_usage').update(usage.id, { status: 502 }).catch(() => {});
    throw error;
  }
};

export const registerFxRoutes = (app, { auth }) => {
  app.route({ method: ['GET', 'POST'], url: '/api/fx/gateway', preHandler: auth, handler: async (request, reply) => {
    const path = request.headers['x-fx-path'];
    if (!gatewayPaths.has(path)) return reply.code(400).send({ error: 'AI 请求路径无效' });
    const requestedApp = String(request.headers['x-miao-app-id'] || '');
    const record = requestedApp ? await pocketbase.collection('apps').getOne(requestedApp).catch(() => null) : null;
    const appId = record?.tenant_id === request.tenant.id ? record.id : '';
    const controller = new AbortController();
    const abort = () => controller.abort();
    reply.raw.once('close', abort);
    reply.raw.once('finish', () => reply.raw.off('close', abort));
    try {
      const transport = createGatewayTransport({ tenantId: request.tenant.id, userId: request.user.id, appId });
      const response = await transport(`${gatewayOrigin}${path}`, { method: request.method, headers: request.headers, body: request.method === 'POST' ? JSON.stringify(request.body || {}) : undefined, signal: controller.signal });
      for (const [name, value] of response.headers) reply.header(name, value);
      reply.code(response.status);
      return reply.send(response.body ? Readable.fromWeb(response.body) : undefined);
    } catch (error) {
      request.log.error({ err: error }, 'fx transport failed');
      return reply.code(error.statusCode || 502).send({ error: error.statusCode ? error.message : 'AI 服务暂时不可用' });
    }
  } });
};
