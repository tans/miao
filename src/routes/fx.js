import { Readable } from 'node:stream';
import { pocketbase } from '../store.js';
import { readAIKey } from '../ai-settings.js';

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

export const registerFxRoutes = (app, { auth }) => {
  app.route({
    method: ['GET', 'POST'],
    url: '/api/fx/gateway',
    preHandler: auth,
    handler: async (request, reply) => {
      const { key: apiKey } = await readAIKey();
      if (!apiKey) return reply.code(503).send({ error: '企业尚未配置 AI Gateway' });
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
