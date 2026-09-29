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

export const registerFxRoutes = (app, { auth }) => {
  app.route({
    method: ['GET', 'POST'],
    url: '/api/fx/gateway',
    preHandler: auth,
    handler: async (request, reply) => {
      const config = await readAIConfig();
      const { key: apiKey } = config;
      if (!apiKey) return reply.code(503).send({ error: '企业尚未配置 AI 服务密钥' });
      const targetPath = request.headers['x-fx-path'];
      const method = request.method;
      if (!gatewayPaths.has(targetPath) || !['GET', 'POST'].includes(method)) {
        return reply.code(400).send({ error: 'AI 请求路径无效' });
      }
      const limit = Number(request.tenant.ai_daily_limit || 0);
      if (limit > 0) {
        const usage = await dailyUsage(request.tenant.id);
        if (usage.totalItems >= limit) return reply.code(429).send({ error: '工作区已达到今日 AI 请求预算，请联系所有者调整预算' });
      }
      if (await persistentRateLimited(request.tenant.id, request.user.id)) return reply.code(429).send({ error: 'AI 请求次数过多，请稍后再试' });

      const headers = new Headers();
      for (const name of forwardedRequestHeaders) {
        const value = request.headers[name];
        if (value) headers.set(name, value);
      }
      headers.set('authorization', `Bearer ${apiKey}`);

      let usageRecord;
      try {
        const requestedApp = String(request.headers['x-miao-app-id'] || '');
        const appRecord = requestedApp ? await pocketbase.collection('apps').getOne(requestedApp).catch(() => null) : null;
        const appId = appRecord?.tenant_id === request.tenant.id ? appRecord.id : '';
        usageRecord = await pocketbase.collection('ai_usage').create({ tenant_id: request.tenant.id, user_id: request.user.id, app_id: appId, status: 100, input_tokens: 0, output_tokens: 0 });
      } catch (error) {
        request.log.error({ err: error }, 'failed to create AI usage record');
        return reply.code(503).send({ error: 'AI 用量服务暂不可用' });
      }

      if (config.provider === 'capi') {
        const baseUrl = config.baseUrl.replace(/\/+$/, '');
        try {
          if (targetPath === '/coding-agent/v1/models' && method === 'GET') {
            const upstream = await fetch(`${baseUrl}/models?modality=text`, { headers: { authorization: `Bearer ${apiKey}` }, redirect: 'error' });
            await pocketbase.collection('ai_usage').update(usageRecord.id, { status: upstream.status });
            if (!upstream.ok) return reply.code(upstream.status).send({ error: '无法读取 CAPI 模型列表' });
            const catalog = await upstream.json();
            return { ...catalog, data: (catalog.data || []).map((model) => ({ ...model, type: 'language' })) };
          }
          const modelId = config.model || request.headers['ai-language-model-id'] || 'gpt-5.2';
          const controller = new AbortController();
          const abort = () => controller.abort();
          reply.raw.once('close', abort);
          reply.raw.once('finish', () => reply.raw.off('close', abort));
          const upstream = await fetch(`${baseUrl}/chat/completions`, {
            method: 'POST',
            headers: { authorization: `Bearer ${apiKey}`, 'content-type': 'application/json', accept: 'text/event-stream' },
            body: JSON.stringify(toOpenAIRequest(request.body || {}, modelId)),
            signal: controller.signal,
            redirect: 'error',
          });
          await pocketbase.collection('ai_usage').update(usageRecord.id, { status: upstream.status });
          if (!upstream.ok) return reply.code(upstream.status).send({ error: 'CAPI 模型请求失败' });
          reply.header('content-type', 'text/event-stream; charset=utf-8');
          reply.header('cache-control', 'no-cache, no-transform');
          reply.header('x-vercel-ai-data-stream', 'v1');
          reply.code(upstream.status);
          const stream = languageModelStream(upstream);
          if (!stream) return reply.send();
          const decoder = new TextDecoder();
          let captured = '';
          const monitored = stream.pipeThrough(new TransformStream({
            transform(chunk, controller) {
              captured = (captured + decoder.decode(chunk, { stream: true })).slice(-250_000);
              controller.enqueue(chunk);
            },
            async flush() {
              captured += decoder.decode();
              const usage = parseTokenUsage(captured);
              await pocketbase.collection('ai_usage').update(usageRecord.id, { input_tokens: usage.input, output_tokens: usage.output }).catch((error) => request.log.error({ err: error }, 'failed to save AI token usage'));
            },
          }));
          return reply.send(Readable.fromWeb(monitored));
        } catch (error) {
          await pocketbase.collection('ai_usage').update(usageRecord.id, { status: 502 }).catch(() => {});
          request.log.error({ err: error }, 'CAPI adapter request failed');
          return reply.code(502).send({ error: 'CAPI 服务暂时不可用' });
        }
      }

      try {
        const controller = new AbortController();
        const abort = () => controller.abort();
        reply.raw.once('close', abort);
        reply.raw.once('finish', () => reply.raw.off('close', abort));
        const upstream = await fetch(`${gatewayOrigin}${targetPath}`, {
          method,
          headers,
          body: method === 'POST' ? JSON.stringify(request.body ?? {}) : undefined,
          signal: controller.signal,
        });
        await pocketbase.collection('ai_usage').update(usageRecord.id, { status: upstream.status });
        for (const name of forwardedResponseHeaders) {
          const value = upstream.headers.get(name);
          if (value) reply.header(name, value);
        }
        reply.header('cache-control', 'no-store');
        reply.code(upstream.status);
        if (!upstream.body) return reply.send();
        const decoder = new TextDecoder();
        let captured = '';
        const monitored = upstream.body.pipeThrough(new TransformStream({
          transform(chunk, controller) {
            captured = (captured + decoder.decode(chunk, { stream: true })).slice(-250_000);
            controller.enqueue(chunk);
          },
          async flush() {
            captured += decoder.decode();
            const usage = parseTokenUsage(captured);
            await pocketbase.collection('ai_usage').update(usageRecord.id, { input_tokens: usage.input, output_tokens: usage.output }).catch((error) => request.log.error({ err: error }, 'failed to save AI token usage'));
          }
        }));
        return reply.send(Readable.fromWeb(monitored));
      } catch (error) {
        if (usageRecord) await pocketbase.collection('ai_usage').update(usageRecord.id, { status: 502 }).catch(() => {});
        request.log.error({ err: error }, 'fx gateway request failed');
        return reply.code(502).send({ error: 'AI 服务暂时不可用' });
      }
    }
  });
};
