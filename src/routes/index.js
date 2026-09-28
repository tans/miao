import Fastify from 'fastify';
import fastifyStatic from '@fastify/static';
import multipart from '@fastify/multipart';
import path from 'node:path';
import { pipeline } from 'node:stream/promises';
import { compileDefinition, starterDefinition, serializeDefinition, blockingPublishDiagnostics, publishSnapshot, rollbackSnapshot, manifestOf } from '@miao/core';
import { collections, id, now, publicApp, addEvent, addTrace, pocketbase, connectPocketBase } from '../store.js';
import { registerAppRoutes } from './apps.js';
import { createAuth } from '../auth.js';

const app = Fastify({ logger: process.env.NODE_ENV !== 'test' });
await app.register(multipart, { limits: { fileSize: 25 * 1024 * 1024, files: 1 } });
await app.register(fastifyStatic, { root: path.resolve('public'), prefix: '/' });
const c = (name) => collections[name];
const body = (request) => request.body || {};
const sortDesc = { created_at: -1 };

const { auth, registerRoutes: registerAuthRoutes } = createAuth({ id, addEvent });

const requireApp = async (request, reply) => {
  const appId = request.params.id || body(request).app_id || request.query?.app_id;
  const filter = request.tenant ? { id: appId, tenant_id: request.tenant.id } : { id: appId };
  const record = await c('apps').findOne(filter);
  if (!record) return reply.code(404).send({ error: '应用不存在' });
  request.appRecord = record;
};

app.get('/api/health', async () => ({ ok: true, service: 'miao', persistence: 'pocketbase', time: now() }));

const safeFilename = (name) => name.replace(/[^\w\-.\u4e00-\u9fa5 ]/g, '_').slice(0, 160);
const resourcePath = (value) => {
  const normalized = String(value || '').replaceAll('\\', '/').replace(/^\/+/, '');
  if (!normalized || normalized.split('/').some((part) => !part || part === '.' || part === '..')) throw new Error('resource.path 必须是安全的相对路径');
  return normalized.slice(0, 240);
};
const resourcePublic = (row, request = null) => ({
  id: row.id,
  app_id: row.app_id,
  path: row.path,
  version: row.version,
  mime: row.mime,
  size: row.size,
  created_at: row.created_at,
  url: request ? `${request.protocol}://${request.host}/assets/${encodeURIComponent(row.app_id)}/${row.path.split('/').map(encodeURIComponent).join('/')}` : `/assets/${encodeURIComponent(row.app_id)}/${row.path.split('/').map(encodeURIComponent).join('/')}`
});
const findResource = async ({ appId, path: requestedPath, version = null }) => {
  const query = { app_id: appId, path: resourcePath(requestedPath), deleted_at: null };
  if (version !== null && version !== undefined) query.version = Number(version);
  return c('static_resources').findOne(query);
};

app.get('/assets/:appId/*', async (request, reply) => {
  let requestedPath;
  try { requestedPath = resourcePath(request.params['*']); } catch { return reply.code(404).send({ error: '资源不存在' }); }
  const resource = await c('static_resources').findOne({ app_id: request.params.appId, path: requestedPath, deleted_at: null });
  if (!resource || !resource.content) return reply.code(404).send({ error: '资源不存在' });
  reply.header('Cache-Control', 'public, max-age=31536000, immutable'); reply.type(resource.mime || 'application/octet-stream');
  const client = await connectPocketBase();
  const response = await fetch(client.files.getURL(resource, resource.content));
  if (!response.ok) return reply.code(404).send({ error: '资源不存在' });
  return reply.send(Buffer.from(await response.arrayBuffer()));
});

app.get('/api/apps/:id/resources', { preHandler: [auth, requireApp] }, async (request) => (await c('static_resources').find({ app_id: request.appRecord.id, deleted_at: null }).sort({ path: 1, version: -1 }).toArray()).map((row) => resourcePublic(row, request)));
app.post('/api/apps/:id/resources', { preHandler: [auth, requireApp] }, async (request, reply) => {
  const part = await request.file(); if (!part) return reply.code(400).send({ error: '请选择资源文件' });
  let resourcePathValue; try { resourcePathValue = resourcePath(part.fields?.path?.value || part.filename); } catch (error) { return reply.code(400).send({ error: error.message }); }
  const previous = await c('static_resources').findOne({ app_id: request.appRecord.id, path: resourcePathValue, deleted_at: null });
  const version = (previous?.version || 0) + 1; const resourceId = id(); const raw = Buffer.from(await part.toBuffer());
  const resource = { id: resourceId, tenant_id: request.appRecord.tenant_id, app_id: request.appRecord.id, path: resourcePathValue, version, content: raw, mime: part.mimetype || 'application/octet-stream', size: raw.length, created_at: now(), updated_at: now(), deleted_at: null };
  await c('static_resources').insertOne(resource); await addEvent({ tenantId: request.appRecord.tenant_id, appId: request.appRecord.id, type: 'resource.uploaded', message: `已上传静态资源 ${resourcePathValue} v${version}`, actor: 'human', payload: { resource_id: resourceId } });
  return reply.code(201).send(resourcePublic(resource, request));
});
app.delete('/api/apps/:id/resources/:resourceId', { preHandler: [auth, requireApp] }, async (request, reply) => {
  const resource = await c('static_resources').findOne({ id: request.params.resourceId, app_id: request.appRecord.id, deleted_at: null });
  if (!resource) return reply.code(404).send({ error: '资源不存在' });
  await c('static_resources').updateOne({ id: resource.id }, { $set: { deleted_at: now(), updated_at: now() } });
  await addEvent({ tenantId: request.appRecord.tenant_id, appId: request.appRecord.id, type: 'resource.deleted', message: `已删除静态资源 ${resource.path}`, actor: 'human', payload: { resource_id: resource.id } });
  return { ok: true };
});
app.get('/api/apps/:id/resources/url', { preHandler: [auth, requireApp] }, async (request, reply) => { try { const resource = await findResource({ appId: request.appRecord.id, path: request.query.path, version: request.query.version }); if (!resource) return reply.code(404).send({ error: '资源不存在' }); return resourcePublic(resource, request); } catch (error) { return reply.code(400).send({ error: error.message }); } });

app.get('/api/apps/:id/history', { preHandler: [auth, requireApp] }, async (request) => (await c('events').find({ app_id: request.appRecord.id }).sort(sortDesc).limit(100).toArray()).map((row) => ({ ...row, payload: row.payload_json })));
app.get('/api/apps/:id/traces', { preHandler: [auth, requireApp] }, async (request) => (await c('traces').find({ app_id: request.appRecord.id }).sort(sortDesc).limit(100).toArray()).map((row) => ({ ...row, input: row.input_json, output: row.output_json })));

const mcpTools = {
  builder: [
    { name: 'app.get_definition', description: '读取单一 APP.md 应用定义', inputSchema: { type: 'object', properties: {} } },
    { name: 'app.update_definition', description: '更新单一 APP.md 应用定义并编译', inputSchema: { type: 'object', required: ['definition'], properties: { definition: { type: 'string' } } } },
    { name: 'app.compile', description: '重新编译并返回诊断', inputSchema: { type: 'object', properties: {} } },
    { name: 'app.publish', description: '发布当前应用草稿', inputSchema: { type: 'object', properties: {} } },
    { name: 'resource.list', description: '列出应用公开静态资源', inputSchema: { type: 'object', properties: {} } },
    { name: 'resource.upload', description: '上传应用公开静态资源', inputSchema: { type: 'object', required: ['path', 'content_base64'], properties: { path: { type: 'string' }, content_base64: { type: 'string' }, mime: { type: 'string' } } } },
    { name: 'resource.delete', description: '删除应用公开静态资源', inputSchema: { type: 'object', required: ['resource_id'], properties: { resource_id: { type: 'string' } } } },
    { name: 'resource.url', description: '获取静态资源公开 URL', inputSchema: { type: 'object', required: ['path'], properties: { path: { type: 'string' }, version: { type: 'integer' } } } },
    { name: 'history.search', description: '查询业务历史', inputSchema: { type: 'object', properties: { q: { type: 'string' } } } },
    { name: 'trace.search', description: '查询系统执行轨迹', inputSchema: { type: 'object', properties: { status: { type: 'string' } } } }
  ]
};

const toolsForManifest = (mode) => (mode === 'builder' ? mcpTools.builder : []);

app.get('/api/apps/:id/capabilities', { preHandler: [auth, requireApp] }, async (request) => ({ capabilities: toolsForManifest('builder').map((tool) => tool.name) }));

registerAuthRoutes(app, { body, c, publicApp });
registerAppRoutes(app, { auth, body, c, id, now, addEvent, publicApp, starterDefinition, compileDefinition, serializeDefinition, manifestOf, blockingPublishDiagnostics, publishSnapshot, rollbackSnapshot, requireApp });

const mcpResult = (value) => ({ content: [{ type: 'text', text: JSON.stringify(value) }], structuredContent: value });
const appTokenAuth = async (request, reply) => {
  if (request.params.mode !== 'builder') return reply.code(404).send({ error: '仅支持 Builder MCP' });
  const appId = request.query?.app_id || body(request).app_id || body(request).arguments?.app_id;
  const record = appId && await c('apps').findOne({ id: appId });
  if (!record) return reply.code(401).send({ error: '缺少或无效的 app_id' });
  request.tenant = { id: record.tenant_id || 'local' };
  request.appRecord = record;
};

const mcp = async (request, reply, mode) => {
  const started = Date.now();
  const payload = body(request);
  const manifest = manifestOf(request.appRecord, 'draft');
  const tools = toolsForManifest(mode);
  if (payload.method === 'initialize') return { jsonrpc: '2.0', id: payload.id ?? null, result: { protocolVersion: '2025-06-18', capabilities: { tools: {} }, serverInfo: { name: `miao-${mode}`, version: '2.0.0' } } };
  if (payload.method === 'notifications/initialized') return reply.code(202).send();
  if (payload.method === 'tools/list') return { jsonrpc: '2.0', id: payload.id ?? null, result: { tools } };
  const call = payload.method === 'tools/call' ? payload.params || {} : { name: payload.tool || payload.name, arguments: payload.arguments || payload.params || {} };
  const args = call.arguments || {};
  let result; let error = null; let status = 'ok';
  try {
    if (!tools.some((tool) => tool.name === call.name)) throw new Error(`当前 ${mode} 无权调用工具：${call.name}`);
    const record = request.appRecord;
    switch (call.name) {
      case 'app.get_definition': { const definition = record.draft_definition ?? record.definition; result = { definition, files: serializeDefinition(definition), manifest, version: record.draft_version }; break; }
      case 'app.update_definition': {
        if (typeof args.definition !== 'string' || !args.definition.trim()) throw new Error('definition 必须是单一 APP.md 文本');
        const nextManifest = compileDefinition(args.definition);
        if (nextManifest.diagnostics.some((item) => item.level === 'error')) throw new Error(nextManifest.diagnostics.map((item) => item.message).join('；'));
        const version = Math.max(record.published_version, record.draft_version) + 1; const timestamp = now();
        await c('apps').updateOne({ id: record.id }, { $set: { draft_definition: args.definition, draft_manifest_json: nextManifest, draft_version: version, updated_at: timestamp } });
        await c('app_versions').insertOne({ id: id(), app_id: record.id, version, definition: args.definition, manifest_json: nextManifest, status: 'draft', created_at: timestamp, published_at: null, previous_version: record.published_version });
        await addEvent({ tenantId: record.tenant_id, appId: record.id, type: 'app.draft', message: `已生成应用定义 v${version} 草稿`, actor: 'builder' }); result = { version, definition: args.definition, files: serializeDefinition(args.definition), manifest: nextManifest }; break;
      }
      case 'app.compile': result = compileDefinition(record.draft_definition ?? record.definition); break;
      case 'app.publish': {
        const diagnostics = blockingPublishDiagnostics(manifest); if (diagnostics.length) throw new Error(`当前应用不满足发布条件：${diagnostics.map((item) => item.message).join('；')}`);
        if (!record.draft_version || record.draft_version <= record.published_version) throw new Error('没有待发布草稿');
        const timestamp = now();
        const update = await c('apps').updateOne({ id: record.id, draft_version: record.draft_version, published_version: record.published_version }, { $set: publishSnapshot(record, timestamp) }); if (!update.modifiedCount) throw new Error('应用版本已变化，请重新读取后发布');
        await c('app_versions').updateMany({ app_id: record.id, status: 'published' }, { $set: { status: 'archived' } });
        await c('app_versions').updateOne({ app_id: record.id, version: record.draft_version }, { $set: { status: 'published', published_at: timestamp } });
        await addEvent({ tenantId: record.tenant_id, appId: record.id, type: 'app.published', message: `已发布 v${record.draft_version}`, actor: 'builder' });
        result = { version: record.draft_version }; break;
      }
      case 'resource.list': result = { resources: (await c('static_resources').find({ app_id: record.id, deleted_at: null }).sort({ path: 1, version: -1 }).toArray()).map((row) => resourcePublic(row, request)) }; break;
      case 'resource.url': { const resource = await findResource({ appId: record.id, path: args.path, version: args.version }); if (!resource) throw new Error('资源不存在'); result = resourcePublic(resource, request); break; }
      case 'resource.upload': { const pathValue = resourcePath(args.path); const raw = Buffer.from(String(args.content_base64 || ''), 'base64'); if (!raw.length || raw.length > 25 * 1024 * 1024) throw new Error('资源内容为空或超过 25MB'); const previous = await c('static_resources').findOne({ app_id: record.id, path: pathValue, deleted_at: null }); const version = (previous?.version || 0) + 1; const resourceId = id(); const row = { id: resourceId, tenant_id: record.tenant_id, app_id: record.id, path: pathValue, version, content: raw, mime: args.mime || 'application/octet-stream', size: raw.length, created_at: now(), updated_at: now(), deleted_at: null }; await c('static_resources').insertOne(row); await addEvent({ tenantId: record.tenant_id, appId: record.id, type: 'resource.uploaded', message: `Agent 上传了静态资源 ${pathValue}`, actor: 'builder', payload: { resource_id: resourceId } }); result = resourcePublic(row, request); break; }
      case 'resource.delete': { const row = await c('static_resources').findOne({ id: args.resource_id, app_id: record.id, deleted_at: null }); if (!row) throw new Error('资源不存在'); await c('static_resources').updateOne({ id: row.id }, { $set: { deleted_at: now(), updated_at: now() } }); result = { ok: true, resource_id: row.id }; break; }
      case 'history.search': result = { events: await c('events').find({ app_id: record.id }).sort(sortDesc).limit(100).toArray() }; break;
      case 'trace.search': { const query = { app_id: record.id }; if (args.status) query.status = args.status; result = { traces: await c('traces').find(query).sort(sortDesc).limit(100).toArray() }; break; }
      default: throw new Error(`未知工具：${call.name}`);
    }
  } catch (e) { status = 'error'; error = e.message; }
  await addTrace({ tenantId: record.tenant_id, appId: record.id, tool: call.name || payload.method, status, input: args, output: result || {}, error, durationMs: Date.now() - started });
  if (error) return reply.code(422).send({ jsonrpc: '2.0', id: payload.id ?? null, error: { code: -32602, message: error } });
  return { jsonrpc: '2.0', id: payload.id ?? null, result: mcpResult(result) };
};
app.get('/api/mcp/:mode', { preHandler: appTokenAuth }, (request, reply) => reply.header('Allow', 'POST').code(405).send({ error: '此 MCP 连接使用无状态 POST' }));
app.post('/api/mcp/:mode', { preHandler: appTokenAuth }, (request, reply) => mcp(request, reply, request.params.mode));
app.get('/api/mcp/:mode/tools', { preHandler: appTokenAuth }, async (request, reply) => ({ mode: request.params.mode, tools: toolsForManifest(request.params.mode) }));

app.setNotFoundHandler((request, reply) => { if (request.url.startsWith('/api/')) return reply.code(404).send({ error: '接口不存在' }); return reply.sendFile('index.html'); });
export const start = async () => {
  const port = Number(process.env.PORT || 41874);
  await connectPocketBase();
  await app.listen({ port, host: process.env.HOST || '0.0.0.0' });
  console.log(`Miao listening on http://localhost:${port} (PocketBase)`);
};

export { app };
