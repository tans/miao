import { Readable } from 'node:stream';

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
const windows = new Map();
let globalWindow = { startedAt: 0, count: 0 };

function isRateLimited(userId) {
  const now = Date.now();
  if (now - globalWindow.startedAt >= 60_000) globalWindow = { startedAt: now, count: 0 };
  let userWindow = windows.get(userId);
  if (!userWindow || now - userWindow.startedAt >= 60_000) {
    userWindow = { startedAt: now, count: 0 };
    windows.set(userId, userWindow);
  }
  if (windows.size > 5000) {
    for (const [id, window] of windows) if (now - window.startedAt >= 60_000) windows.delete(id);
  }
  if (globalWindow.count >= 120 || userWindow.count >= 30) return true;
  globalWindow.count += 1;
  userWindow.count += 1;
  return false;
}

export const registerFxRoutes = (app, { auth }) => {
  app.route({
    method: ['GET', 'POST'],
    url: '/api/fx/gateway',
    preHandler: auth,
    handler: async (request, reply) => {
      const apiKey = process.env.AI_GATEWAY_API_KEY;
      if (!apiKey) return reply.code(503).send({ error: '企业尚未配置 AI Gateway' });
      if (isRateLimited(request.user.id)) return reply.code(429).send({ error: 'AI 请求次数过多，请稍后再试' });

      const targetPath = request.headers['x-fx-path'];
      const method = request.method;
      if (!gatewayPaths.has(targetPath) || !['GET', 'POST'].includes(method)) {
        return reply.code(400).send({ error: 'AI 请求路径无效' });
      }

      const headers = new Headers();
      for (const name of forwardedRequestHeaders) {
        const value = request.headers[name];
        if (value) headers.set(name, value);
      }
      headers.set('authorization', `Bearer ${apiKey}`);

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
        for (const name of forwardedResponseHeaders) {
          const value = upstream.headers.get(name);
          if (value) reply.header(name, value);
        }
        reply.header('cache-control', 'no-store');
        reply.code(upstream.status);
        if (!upstream.body) return reply.send();
        return reply.send(Readable.fromWeb(upstream.body));
      } catch (error) {
        request.log.error({ err: error }, 'fx gateway request failed');
        return reply.code(502).send({ error: 'AI 服务暂时不可用' });
      }
    }
  });
};
