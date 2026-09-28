import Fastify from 'fastify';
import fastifyStatic from '@fastify/static';
import path from 'node:path';
import { existsSync } from 'node:fs';
import { connectPocketBase, now, pocketbase } from '../store.js';
import { registerAppRoutes } from './apps.js';
import { registerFxRoutes } from './fx.js';
import { createAuth } from '../auth.js';

const app = Fastify({ logger: process.env.NODE_ENV !== 'test', bodyLimit: 8 * 1024 * 1024 });
const body = (request) => request.body || {};
const staticRoot = existsSync(path.resolve('dist')) ? path.resolve('dist') : path.resolve('public');
await app.register(fastifyStatic, { root: staticRoot, prefix: '/' });

const { auth, registerRoutes: registerAuthRoutes } = createAuth();

app.get('/api/health', async () => ({ ok: true, service: 'miao', persistence: 'pocketbase', time: now() }));

registerAuthRoutes(app, { body });
registerAppRoutes(app, { auth, body, pocketbase });
registerFxRoutes(app, { auth });

app.addHook('onResponse', async (request, reply) => {
  if (!request.user || !request.tenant || !['POST', 'PUT', 'PATCH', 'DELETE'].includes(request.method) || reply.statusCode < 200 || reply.statusCode >= 400) return;
  const route = request.routeOptions?.url || request.routerPath || request.url.split('?')[0];
  const targetId = String(request.params?.recordId || request.params?.slug || request.params?.id || '').slice(0, 80);
  await pocketbase.collection('audit_logs').create({
    tenant_id: request.tenant.id, actor_id: request.user.id, actor_email: request.user.email,
    action: request.method, route: String(route).slice(0, 200), target_id: targetId, status: reply.statusCode
  }).catch((error) => request.log.error({ err: error }, 'failed to save audit log'));
});

app.setNotFoundHandler((request, reply) => {
  if (request.url.startsWith('/api/')) return reply.code(404).send({ error: '接口不存在' });
  return reply.sendFile('index.html');
});

export const start = async () => {
  const port = Number(process.env.PORT || 41874);
  await connectPocketBase();
  await app.listen({ port, host: process.env.HOST || '0.0.0.0' });
  console.log(`Miao listening on http://localhost:${port} (PocketBase)`);
};

export { app };
