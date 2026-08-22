import Fastify from 'fastify';
import fastifyStatic from '@fastify/static';
import multipart from '@fastify/multipart';
import fs from 'node:fs';
import path from 'node:path';
import { pipeline } from 'node:stream/promises';
import { uploadsDir, id, now, hashPassword, verifyPassword, parseJson, publicUser, publicApp, addEvent, addTrace, collections, initDb, client } from './db.js';
import { compileSource, starterSource } from './manifest.js';
import { manifestOf, findObjectDefinition, findActionDefinition, createObject, searchObjects, getObject, relatedObjects, applyAction } from './ontology-runtime.js';
import ExcelJS from 'exceljs';
import mammoth from 'mammoth';
import { PDFParse } from 'pdf-parse';

const app = Fastify({ logger: process.env.NODE_ENV !== 'test' });
await app.register(multipart, { limits: { fileSize: 25 * 1024 * 1024, files: 1 } });
await app.register(fastifyStatic, { root: path.resolve('public'), prefix: '/' });
const c = (name) => collections[name];
const body = (request) => request.body || {};
const safeFilename = (name) => name.replace(/[^\w\-.\u4e00-\u9fa5 ]/g, '_').slice(0, 160);
const sortDesc = { created_at: -1 };

const auth = async (request, reply) => {
  const token = request.headers.authorization?.replace(/^Bearer\s+/i, '');
  const session = token && await c('sessions').findOne({ token, expires_at: { $gt: now() } });
  if (!session) return reply.code(401).send({ error: '请先登录' });
  const user = await c('users').findOne({ id: session.user_id });
  const tenant = await c('tenants').findOne({ owner_id: user.id }, { sort: { created_at: 1 } });
  if (!user || !tenant) return reply.code(401).send({ error: '账号工作区不存在' });
  request.user = publicUser(user); request.tenant = { id: tenant.id, name: tenant.name, slug: tenant.slug }; request.token = token;
};
const ownedApp = (request, appId) => c('apps').findOne({ id: appId, tenant_id: request.tenant.id });
const requireApp = async (request, reply) => { const record = await ownedApp(request, request.params.id || body(request).app_id || request.query?.app_id); if (!record) return reply.code(404).send({ error: '应用不存在' }); request.appRecord = record; };

app.get('/api/health', async () => ({ ok: true, service: 'agent-native-runtime', persistence: 'mongodb', database: process.env.MONGODB_DB || 'agent_native_runtime', time: now() }));
app.post('/api/auth/register', async (request, reply) => {
  const { email, password, name } = body(request); const normalizedEmail = String(email || '').toLowerCase();
  if (!/^\S+@\S+\.\S+$/.test(normalizedEmail)) return reply.code(400).send({ error: '请输入有效邮箱' });
  if (!password || String(password).length < 8) return reply.code(400).send({ error: '密码至少 8 位' });
  if (!name?.trim()) return reply.code(400).send({ error: '请输入姓名' });
  if (await c('users').findOne({ email: normalizedEmail })) return reply.code(409).send({ error: '该邮箱已注册' });
  const userId = id(); const tenantId = id(); const timestamp = now(); const tenantName = `${name.trim()} 的工作区`; const slugBase = name.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'workspace'; const tenantSlug = `${slugBase}-${userId.slice(0, 6)}`;
  await c('users').insertOne({ id: userId, email: normalizedEmail, password_hash: hashPassword(password), name: name.trim(), created_at: timestamp });
  await c('tenants').insertOne({ id: tenantId, name: tenantName, slug: tenantSlug, owner_id: userId, created_at: timestamp });
  const token = id(); await c('sessions').insertOne({ token, user_id: userId, created_at: timestamp, expires_at: new Date(Date.now() + 30 * 86400000).toISOString() });
  await addEvent({ tenantId, type: 'tenant.created', message: '工作区已创建', actor: 'human', payload: { email: normalizedEmail } });
  return reply.code(201).send({ token, user: { id: userId, email: normalizedEmail, name: name.trim() }, tenant: { id: tenantId, name: tenantName, slug: tenantSlug }, needs_onboarding: true });
});
app.post('/api/auth/login', async (request, reply) => {
  const { email, password } = body(request); const user = await c('users').findOne({ email: String(email || '').toLowerCase() });
  if (!user || !verifyPassword(String(password || ''), user.password_hash)) return reply.code(401).send({ error: '邮箱或密码不正确' });
  const tenant = await c('tenants').findOne({ owner_id: user.id }, { sort: { created_at: 1 } }); const token = id();
  await c('sessions').insertOne({ token, user_id: user.id, created_at: now(), expires_at: new Date(Date.now() + 30 * 86400000).toISOString() });
  return { token, user: publicUser(user), tenant: { id: tenant.id, name: tenant.name, slug: tenant.slug }, needs_onboarding: !(await c('apps').findOne({ tenant_id: tenant.id })) };
});
app.post('/api/auth/logout', { preHandler: auth }, async (request) => { await c('sessions').deleteOne({ token: request.token }); return { ok: true }; });
app.get('/api/me', { preHandler: auth }, async (request) => ({ user: request.user, tenant: request.tenant, apps: (await c('apps').find({ tenant_id: request.tenant.id }).sort({ updated_at: -1 }).toArray()).map(publicApp) }));

app.post('/api/onboard', { preHandler: auth }, async (request, reply) => {
  const { name, goal, concepts } = body(request); if (!name?.trim() || !goal?.trim()) return reply.code(400).send({ error: '请填写应用名称和目标' });
  const source = starterSource({ name: name.trim(), goal: goal.trim(), concepts: Array.isArray(concepts) ? concepts.filter(Boolean).slice(0, 12) : [] }); const manifest = compileSource(source, { description: goal.trim(), concepts }); const appId = id(); const timestamp = now();
  await c('apps').insertOne({ id: appId, tenant_id: request.tenant.id, name: name.trim(), description: goal.trim(), source, manifest_json: manifest, published_version: 1, draft_version: 1, created_at: timestamp, updated_at: timestamp });
  await c('app_versions').insertOne({ id: id(), app_id: appId, version: 1, source, manifest_json: manifest, status: 'published', created_at: timestamp, published_at: timestamp, previous_version: null });
  await addEvent({ tenantId: request.tenant.id, appId, type: 'app.published', message: `应用「${name.trim()}」已创建并发布 v1`, actor: 'human' });
  return reply.code(201).send({ app: publicApp(await c('apps').findOne({ id: appId })) });
});
app.get('/api/apps', { preHandler: auth }, async (request) => (await c('apps').find({ tenant_id: request.tenant.id }).sort({ updated_at: -1 }).toArray()).map(publicApp));
app.get('/api/apps/:id', { preHandler: [auth, requireApp] }, async (request) => publicApp(request.appRecord));
app.get('/api/apps/:id/source', { preHandler: [auth, requireApp] }, async (request) => ({ source: request.appRecord.source, manifest: parseJson(request.appRecord.manifest_json) }));
app.put('/api/apps/:id/source', { preHandler: [auth, requireApp] }, async (request, reply) => {
  const source = body(request).source; if (!source?.trim()) return reply.code(400).send({ error: 'APP.md 不能为空' }); const current = request.appRecord; const nextVersion = Math.max(current.published_version, current.draft_version) + 1; const manifest = compileSource(source, { description: current.description }); if (manifest.diagnostics.some((item) => item.level === 'error')) return reply.code(422).send({ error: manifest.diagnostics.map((item) => item.message).join('；'), diagnostics: manifest.diagnostics }); const timestamp = now();
  await c('apps').updateOne({ id: current.id }, { $set: { source, manifest_json: manifest, draft_version: nextVersion, updated_at: timestamp } }); await c('app_versions').insertOne({ id: id(), app_id: current.id, version: nextVersion, source, manifest_json: manifest, status: 'draft', created_at: timestamp, published_at: null, previous_version: current.published_version }); await addEvent({ tenantId: request.tenant.id, appId: current.id, type: 'app.draft', message: `已生成 v${nextVersion} 草稿`, actor: 'builder' }); return { version: nextVersion, manifest };
});
app.post('/api/apps/:id/compile', { preHandler: [auth, requireApp] }, async (request) => { const manifest = compileSource(request.appRecord.source, { description: request.appRecord.description }); return { ok: !manifest.diagnostics.some((item) => item.level === 'error'), manifest, draft_version: request.appRecord.draft_version }; });
app.post('/api/apps/:id/publish', { preHandler: [auth, requireApp] }, async (request, reply) => { const current = request.appRecord; const manifest = manifestOf(current); if (manifest.diagnostics?.some((item) => item.level === 'error')) return reply.code(422).send({ error: '当前 Ontology 有编译错误', diagnostics: manifest.diagnostics }); if (!current.draft_version || current.draft_version <= current.published_version) return reply.code(400).send({ error: '没有待发布草稿' }); const timestamp = now(); await c('app_versions').updateMany({ app_id: current.id, status: 'published' }, { $set: { status: 'archived' } }); await c('app_versions').updateOne({ app_id: current.id, version: current.draft_version }, { $set: { status: 'published', published_at: timestamp } }); await c('apps').updateOne({ id: current.id }, { $set: { published_version: current.draft_version, updated_at: timestamp } }); await addEvent({ tenantId: request.tenant.id, appId: current.id, type: 'app.published', message: `已发布 v${current.draft_version}`, actor: 'builder' }); return { ok: true, version: current.draft_version }; });
app.post('/api/apps/:id/rollback', { preHandler: [auth, requireApp] }, async (request, reply) => { const target = Number(body(request).version); const version = await c('app_versions').findOne({ app_id: request.appRecord.id, version: target }); if (!version) return reply.code(404).send({ error: '版本不存在' }); const timestamp = now(); await c('app_versions').updateMany({ app_id: request.appRecord.id, status: 'published' }, { $set: { status: 'archived' } }); await c('app_versions').updateOne({ id: version.id }, { $set: { status: 'published', published_at: timestamp } }); await c('apps').updateOne({ id: request.appRecord.id }, { $set: { source: version.source, manifest_json: version.manifest_json, published_version: target, draft_version: target, updated_at: timestamp } }); await addEvent({ tenantId: request.tenant.id, appId: request.appRecord.id, type: 'app.rollback', message: `已回滚到 v${target}`, actor: 'builder' }); return { ok: true, version: target }; });
app.get('/api/apps/:id/versions', { preHandler: [auth, requireApp] }, async (request) => c('app_versions').find({ app_id: request.appRecord.id }, { projection: { _id: 0, version: 1, status: 1, created_at: 1, published_at: 1, previous_version: 1 } }).sort({ version: -1 }).toArray());

app.get('/api/apps/:id/records', { preHandler: [auth, requireApp] }, async (request) => { const query = { app_id: request.appRecord.id, deleted_at: null }; if (request.query.collection && request.query.collection !== 'all') query.collection = request.query.collection; const rows = await c('records').find(query, { projection: { _id: 0 } }).sort({ updated_at: -1 }).limit(500).toArray(); const q = String(request.query.q || '').toLowerCase(); return rows.map((row) => ({ id: row.id, collection: row.collection, data: row.data_json, created_at: row.created_at, updated_at: row.updated_at, provenance: row.provenance_json })).filter((row) => !q || JSON.stringify(row.data).toLowerCase().includes(q)); });
app.post('/api/apps/:id/records', { preHandler: [auth, requireApp] }, async (request, reply) => { const { collection = 'records', data = {}, provenance = { type: 'human' } } = body(request); const manifest = manifestOf(request.appRecord); const definition = findObjectDefinition(manifest, collection); if (definition) { const errors = Object.entries(definition.properties || {}).filter(([key, field]) => field.required && (data[key] === undefined || data[key] === '')).map(([key]) => `字段 ${key} 必填`); if (errors.length) return reply.code(422).send({ error: errors.join('；') }); } const timestamp = now(); const objectType = definition?.slug || collection; const record = { id: id(), tenant_id: request.tenant.id, app_id: request.appRecord.id, object_type: objectType, collection: objectType, data_json: data, created_at: timestamp, updated_at: timestamp, deleted_at: null, provenance_json: provenance }; await c('records').insertOne(record); await addEvent({ tenantId: request.tenant.id, appId: request.appRecord.id, type: 'object.created', message: `新增 ${collection} 对象`, actor: provenance.type || 'human', payload: { record_id: record.id, object_type: objectType } }); return reply.code(201).send({ id: record.id, object_type: objectType, data, created_at: timestamp, updated_at: timestamp, provenance }); });
app.post('/api/apps/:id/records/bulk', { preHandler: [auth, requireApp] }, async (request, reply) => { const { collection = 'records', records = [], provenance = { type: 'human_bulk' } } = body(request); if (!Array.isArray(records) || !records.length) return reply.code(400).send({ error: 'records 必须是非空数组' }); const timestamp = now(); const docs = records.map((data) => ({ id: id(), app_id: request.appRecord.id, collection, data_json: data, created_at: timestamp, updated_at: timestamp, deleted_at: null, provenance_json: provenance })); await c('records').insertMany(docs); await addEvent({ tenantId: request.tenant.id, appId: request.appRecord.id, type: 'records.bulk_created', message: `批量新增 ${docs.length} 条 ${collection} 记录`, actor: provenance.type, payload: { count: docs.length } }); return reply.code(201).send({ inserted: docs.length, ids: docs.map((item) => item.id) }); });
app.patch('/api/apps/:id/records/:recordId', { preHandler: [auth, requireApp] }, async (request, reply) => { const row = await c('records').findOne({ id: request.params.recordId, app_id: request.appRecord.id, deleted_at: null }); if (!row) return reply.code(404).send({ error: '记录不存在' }); const data = { ...row.data_json, ...(body(request).data || {}) }; const timestamp = now(); await c('records').updateOne({ id: row.id }, { $set: { data_json: data, updated_at: timestamp, provenance_json: body(request).provenance || row.provenance_json } }); await addEvent({ tenantId: request.tenant.id, appId: request.appRecord.id, type: 'record.updated', message: `更新 ${row.collection} 记录`, actor: body(request).provenance?.type || 'human', payload: { record_id: row.id } }); return { id: row.id, collection: row.collection, data, updated_at: timestamp }; });
app.delete('/api/apps/:id/records/:recordId', { preHandler: [auth, requireApp] }, async (request, reply) => { const result = await c('records').updateOne({ id: request.params.recordId, app_id: request.appRecord.id, deleted_at: null }, { $set: { deleted_at: now() } }); if (!result.modifiedCount) return reply.code(404).send({ error: '记录不存在' }); await addEvent({ tenantId: request.tenant.id, appId: request.appRecord.id, type: 'record.deleted', message: '记录已软删除', actor: 'human', payload: { record_id: request.params.recordId } }); return { ok: true }; });
app.post('/api/apps/:id/records/aggregate', { preHandler: [auth, requireApp] }, async (request, reply) => {
  const { collection, group_by: groupBy, sum, count = true } = body(request);
  if (!collection || !groupBy) return reply.code(400).send({ error: 'collection 和 group_by 必填' });
  const match = { app_id: request.appRecord.id, collection, deleted_at: null };
  const group = { _id: `$data_json.${groupBy}` }; if (count) group.count = { $sum: 1 }; if (sum) group.sum = { $sum: { $convert: { input: `$data_json.${sum}`, to: 'double', onError: 0, onNull: 0 } } };
  const result = await c('records').aggregate([{ $match: match }, { $group: group }, { $sort: { count: -1 } }]).toArray();
  return { collection, group_by: groupBy, results: result.map(({ _id, ...values }) => ({ value: _id, ...values })) };
});
app.post('/api/apps/:id/records/transform', { preHandler: [auth, requireApp] }, async (request, reply) => {
  const { collection, set = {}, filter = {} } = body(request); if (!collection || typeof set !== 'object') return reply.code(400).send({ error: 'collection 和 set 必填' });
  const query = { app_id: request.appRecord.id, collection, deleted_at: null }; for (const [key, value] of Object.entries(filter)) query[`data_json.${key}`] = value;
  const rows = await c('records').find(query).toArray(); const timestamp = now(); await Promise.all(rows.map((row) => { const data = { ...row.data_json }; for (const [key, value] of Object.entries(set)) data[key] = typeof value === 'string' ? value.replace(/\{([^}]+)\}/g, (_, field) => row.data_json[field] ?? '') : value; return c('records').updateOne({ id: row.id }, { $set: { data_json: data, updated_at: timestamp, provenance_json: { type: 'builder_transform' } } }); }));
  await addEvent({ tenantId: request.tenant.id, appId: request.appRecord.id, type: 'data.transformed', message: `已转换 ${rows.length} 条 ${collection} 记录`, actor: 'builder', payload: { collection, count: rows.length } }); return { ok: true, updated: rows.length };
});
app.post('/api/apps/:id/actions/test', { preHandler: [auth, requireApp] }, async (request, reply) => { const action = String(body(request).action || ''); const manifest = parseJson(request.appRecord.manifest_json); const definition = (manifest.actions || []).find((item) => item.name === action || item.slug === action); if (!definition) return reply.code(422).send({ ok: false, error: '未找到动作定义' }); return { ok: true, action, definition }; });
app.post('/api/apps/:id/files/rules/test', { preHandler: [auth, requireApp] }, async (request, reply) => { const file = await c('files').findOne({ id: body(request).file_id, app_id: request.appRecord.id }); if (!file) return reply.code(404).send({ error: '文件不存在' }); return { ok: true, file_id: file.id, original_name: file.original_name, extracted_rows: Array.isArray(file.extracted_text) ? file.extracted_text.length : 0 }; });

const parseTabular = async (filePath, originalName) => { const ext = path.extname(originalName).toLowerCase(); if (['.csv', '.tsv', '.txt', '.md'].includes(ext)) { const text = fs.readFileSync(filePath, 'utf8'); if (['.txt', '.md'].includes(ext)) return { headers: ['content'], rows: text.split(/\r?\n/).filter(Boolean).map((content) => ({ content })) }; const lines = text.split(/\r?\n/).filter((line) => line.trim()); const delimiter = ext === '.tsv' ? '\t' : ','; const parse = (line) => line.split(delimiter).map((cell) => cell.trim().replace(/^"|"$/g, '')); const headers = parse(lines.shift() || '内容'); return { headers, rows: lines.map((line) => Object.fromEntries(parse(line).map((value, index) => [headers[index] || `字段${index + 1}`, value]))) }; } if (['.xlsx', '.xls'].includes(ext)) { const workbook = new ExcelJS.Workbook(); await workbook.xlsx.readFile(filePath); const sheet = workbook.worksheets[0]; const headers = (sheet.getRow(1).values || []).slice(1).map((value, index) => String(value ?? `字段${index + 1}`)); const rows = []; sheet.eachRow((row, rowNumber) => { if (rowNumber === 1) return; const values = row.values.slice(1); const data = Object.fromEntries(headers.map((header, index) => [header, values[index] ?? ''])); if (Object.values(data).some(Boolean)) rows.push(data); }); return { headers, rows }; } if (ext === '.docx') { const result = await mammoth.extractRawText({ path: filePath }); return { headers: ['content'], rows: result.value.split(/\r?\n/).filter(Boolean).map((content) => ({ content })) }; } if (ext === '.pdf') { const parser = new PDFParse({ data: fs.readFileSync(filePath) }); const result = await parser.getText(); await parser.destroy(); return { headers: ['content'], rows: result.text.split(/\r?\n/).map((content) => content.trim()).filter(Boolean).map((content) => ({ content })) }; } return { headers: [], rows: [] }; };
app.get('/api/apps/:id/files', { preHandler: [auth, requireApp] }, async (request) => (await c('files').find({ app_id: request.appRecord.id }, { projection: { _id: 0, path: 0 } }).sort(sortDesc).toArray()).map((row) => ({ ...row, provenance: row.provenance_json })));
app.post('/api/apps/:id/files', { preHandler: [auth, requireApp] }, async (request, reply) => { const part = await request.file(); if (!part) return reply.code(400).send({ error: '请选择文件' }); const fileId = id(); const originalName = safeFilename(part.filename || 'upload'); const destination = path.join(uploadsDir, `${fileId}-${originalName}`); await pipeline(part.file, fs.createWriteStream(destination)); const stat = fs.statSync(destination); let preview = { headers: [], rows: [] }; try { preview = await parseTabular(destination, originalName); } catch { /* 保留原文件 */ } const file = { id: fileId, app_id: request.appRecord.id, original_name: originalName, path: destination, mime: part.mimetype || 'application/octet-stream', size: stat.size, status: 'ready', extracted_text: preview.rows.slice(0, 5), created_at: now(), provenance_json: { type: 'file', file_id: fileId, actor: request.user.email } }; await c('files').insertOne(file); await addEvent({ tenantId: request.tenant.id, appId: request.appRecord.id, type: 'file.uploaded', message: `已上传 ${originalName}`, actor: 'human', payload: { file_id: fileId, size: stat.size } }); return reply.code(201).send({ id: fileId, original_name: originalName, mime: file.mime, size: stat.size, status: 'ready', preview: { headers: preview.headers, rows: preview.rows.slice(0, 5), total: preview.rows.length } }); });
app.post('/api/apps/:id/files/:fileId/import', { preHandler: [auth, requireApp] }, async (request, reply) => { const file = await c('files').findOne({ id: request.params.fileId, app_id: request.appRecord.id }); if (!file) return reply.code(404).send({ error: '文件不存在' }); let parsed; try { parsed = await parseTabular(file.path, file.original_name); } catch (error) { return reply.code(422).send({ error: `文件解析失败：${error.message}` }); } if (!parsed.rows.length) return reply.code(422).send({ error: '文件没有可导入的数据' }); const collection = body(request).collection || path.basename(file.original_name, path.extname(file.original_name)).replace(/[^\w\u4e00-\u9fa5]+/g, '_') || 'records'; const timestamp = now(); await c('records').insertMany(parsed.rows.map((data) => ({ id: id(), app_id: request.appRecord.id, collection, data_json: data, created_at: timestamp, updated_at: timestamp, deleted_at: null, provenance_json: { type: 'file', file_id: file.id } }))); await addEvent({ tenantId: request.tenant.id, appId: request.appRecord.id, type: 'file.imported', message: `${file.original_name} 已导入 ${parsed.rows.length} 条记录`, actor: 'human', payload: { file_id: file.id, collection, count: parsed.rows.length } }); return { ok: true, collection, imported: parsed.rows.length, headers: parsed.headers }; });
app.get('/api/apps/:id/files/:fileId/download', { preHandler: [auth, requireApp] }, async (request, reply) => { const file = await c('files').findOne({ id: request.params.fileId, app_id: request.appRecord.id }); if (!file || !fs.existsSync(file.path)) return reply.code(404).send({ error: '文件不存在' }); reply.header('Content-Disposition', `attachment; filename*=UTF-8''${encodeURIComponent(file.original_name)}`); reply.type(file.mime); return reply.send(fs.createReadStream(file.path)); });
app.get('/api/apps/:id/records/export', { preHandler: [auth, requireApp] }, async (request, reply) => { const query = { app_id: request.appRecord.id, deleted_at: null }; if (request.query.collection) query.collection = request.query.collection; const rows = await c('records').find(query, { projection: { _id: 0, data_json: 1 } }).sort({ updated_at: -1 }).limit(5000).toArray(); const headers = [...new Set(rows.flatMap((row) => Object.keys(row.data_json || {})))]; const csvCell = (value) => `"${String(value ?? '').replaceAll('"', '""')}"`; const csv = [headers.map(csvCell).join(','), ...rows.map((row) => headers.map((header) => csvCell(row.data_json?.[header])).join(','))].join('\n'); reply.header('Content-Disposition', `attachment; filename*=UTF-8''${encodeURIComponent(request.query.collection || 'records')}.csv`); return reply.type('text/csv; charset=utf-8').send(`\uFEFF${csv}`); });

app.get('/api/apps/:id/history', { preHandler: [auth, requireApp] }, async (request) => (await c('events').find({ app_id: request.appRecord.id }, { projection: { _id: 0 } }).sort(sortDesc).limit(100).toArray()).map((row) => ({ ...row, payload: row.payload_json })));
app.get('/api/apps/:id/traces', { preHandler: [auth, requireApp] }, async (request) => (await c('traces').find({ app_id: request.appRecord.id }, { projection: { _id: 0 } }).sort(sortDesc).limit(100).toArray()).map((row) => ({ ...row, input: row.input_json, output: row.output_json })));
const dshUrl = () => process.env.DSH_PUBLIC_URL || process.env.DSH_URL || null;
const agentProfile = (mode, appRecord, request) => ({
  name: `miaozao-${mode}`,
  runtime: 'deepseek-harness',
  model: process.env.DSH_MODEL || 'deepseek-chat',
  system_prompt: '你是秒造企业助手。优先使用秒造业务能力，禁止访问系统文件，所有业务数据通过秒造工具获取。',
  app_id: appRecord.id,
  mcp_url: `${request.protocol}://${request.hostname}/api/mcp/${mode}?app_id=${encodeURIComponent(appRecord.id)}`,
  mcp_headers: { Authorization: `Bearer ${request.token}` },
  dsh_url: dshUrl(),
  capabilities: ['file', 'ontology', 'action', 'code']
});
app.post('/api/apps/:id/agent/sessions', { preHandler: [auth, requireApp] }, async (request, reply) => {
  const mode = body(request).mode === 'builder' ? 'builder' : 'user';
  const sessionId = id(); const timestamp = now();
  const session = { id: sessionId, token: id(), tenant_id: request.tenant.id, app_id: request.appRecord.id, user_id: request.user.id, mode, runtime: 'deepseek-harness', status: 'ready', workspace_id: `session-${sessionId}`, created_at: timestamp, last_used_at: timestamp };
  await c('agent_sessions').insertOne(session);
  await addEvent({ tenantId: request.tenant.id, appId: request.appRecord.id, type: 'agent.session.created', message: `已创建 ${mode === 'builder' ? 'Builder' : 'User'} Agent 会话`, actor: 'human', payload: { session_id: sessionId, mode } });
  return reply.code(201).send({ session: { id: sessionId, mode, runtime: session.runtime, status: session.status, workspace_id: session.workspace_id, created_at: timestamp }, profile: agentProfile(mode, request.appRecord, request), launch_url: dshUrl() ? `${dshUrl()}?session_id=${encodeURIComponent(sessionId)}` : null });
});
app.get('/api/apps/:id/agent/sessions', { preHandler: [auth, requireApp] }, async (request) => c('agent_sessions').find({ tenant_id: request.tenant.id, app_id: request.appRecord.id }, { projection: { _id: 0, token: 0 } }).sort({ created_at: -1 }).limit(50).toArray());
app.get('/api/apps/:id/agent/profile', { preHandler: [auth, requireApp] }, async (request) => agentProfile(request.query?.mode === 'builder' ? 'builder' : 'user', request.appRecord, request));
app.get('/api/apps/:id/capabilities', { preHandler: [auth, requireApp] }, async () => ({ capabilities: ['file.search', 'file.read', 'file.extract', 'file.save', 'file.export', 'ontology.query', 'ontology.get', 'ontology.search', 'action.execute', 'code.execute'], code_runtime: Boolean(process.env.CODE_EXECUTOR_URL), dsh_runtime: Boolean(dshUrl()) }));
app.get('/api/openapi.json', async () => ({ openapi: '3.0.3', info: { title: 'Agent Native App Runtime', version: '1.0.0' }, servers: [{ url: '/api' }], paths: { '/auth/register': { post: { summary: '创建账号和租户' } }, '/onboard': { post: { summary: '创建应用' } }, '/apps/{id}/records': { get: { summary: '查询记录' }, post: { summary: '写入记录' } }, '/apps/{id}/files': { post: { summary: '上传文件' } }, '/apps/{id}/agent/sessions': { get: { summary: '查询内置 Agent 会话' }, post: { summary: '创建内置 Agent 会话' } }, '/apps/{id}/agent/profile': { get: { summary: '读取 DSH Profile' } }, '/apps/{id}/capabilities': { get: { summary: '读取秒造 Capability 清单' } }, '/mcp/user': { post: { summary: 'User Agent MCP' } }, '/mcp/builder': { post: { summary: 'Builder Agent MCP' } } } }));

const mcpTools = {
  user: [
    { name: 'ontology.describe', description: '读取应用的对象、关系和动作定义', inputSchema: { type: 'object', properties: {} } },
    { name: 'object.search', description: '搜索业务对象', inputSchema: { type: 'object', properties: { object_type: { type: 'string' }, q: { type: 'string' }, limit: { type: 'integer' } } } },
    { name: 'object.get', description: '读取一个业务对象', inputSchema: { type: 'object', required: ['object_id'], properties: { object_id: { type: 'string' } } } },
    { name: 'object.create', description: '按对象定义创建业务对象', inputSchema: { type: 'object', required: ['object_type', 'properties'], properties: { object_type: { type: 'string' }, properties: { type: 'object' } } } },
    { name: 'object.related', description: '读取对象关系', inputSchema: { type: 'object', required: ['object_id'], properties: { object_id: { type: 'string' }, link_type: { type: 'string' }, direction: { type: 'string', enum: ['out', 'in'] } } } },
    { name: 'action.list', description: '列出可执行的业务动作', inputSchema: { type: 'object', properties: {} } },
    { name: 'action.describe', description: '读取动作规则和变更定义', inputSchema: { type: 'object', required: ['action'], properties: { action: { type: 'string' } } } },
    { name: 'action.apply', description: '执行一个受规则约束的业务动作', inputSchema: { type: 'object', required: ['action'], properties: { action: { type: 'string' }, object_id: { type: 'string' }, target_id: { type: 'string' }, from_object_id: { type: 'string' }, to_object_id: { type: 'string' } }, additionalProperties: true } },
    { name: 'file.list', description: '列出应用文件', inputSchema: { type: 'object', properties: {} } },
    { name: 'file.read', description: '读取文件元数据和提取预览', inputSchema: { type: 'object', required: ['file_id'], properties: { file_id: { type: 'string' } } } },
    { name: 'history.search', description: '查询业务历史', inputSchema: { type: 'object', properties: { q: { type: 'string' } } } },
    { name: 'trace.search', description: '查询系统执行轨迹', inputSchema: { type: 'object', properties: { status: { type: 'string' } } } }
  ],
  builder: [
    { name: 'ontology.describe', description: '读取当前 Ontology', inputSchema: { type: 'object', properties: {} } },
    { name: 'app.get_source', description: '读取 APP.md', inputSchema: { type: 'object', properties: {} } },
    { name: 'app.update_source', description: '更新 APP.md 草稿并编译', inputSchema: { type: 'object', required: ['source'], properties: { source: { type: 'string' } } } },
    { name: 'app.compile', description: '重新编译并返回诊断', inputSchema: { type: 'object', properties: {} } },
    { name: 'app.publish', description: '发布当前 Ontology 草稿', inputSchema: { type: 'object', properties: {} } },
    { name: 'action.list', description: '列出动作定义', inputSchema: { type: 'object', properties: {} } },
    { name: 'action.describe', description: '读取动作定义', inputSchema: { type: 'object', required: ['action'], properties: { action: { type: 'string' } } } },
    { name: 'action.test', description: '检查动作是否已定义', inputSchema: { type: 'object', required: ['action'], properties: { action: { type: 'string' } } } },
    { name: 'history.search', description: '查询业务历史', inputSchema: { type: 'object', properties: {} } },
    { name: 'trace.search', description: '查询系统执行轨迹', inputSchema: { type: 'object', properties: { status: { type: 'string' } } } }
  ]
};

// Stable capability names are intentionally aliases: DSH and external MCP clients
// can share one contract without knowing the internal object/action vocabulary.
const capabilityTools = [
  { name: 'miaozao.files.search', description: '搜索应用文件', inputSchema: { type: 'object', properties: { q: { type: 'string' }, limit: { type: 'integer' } } } },
  { name: 'miaozao.files.read', description: '读取文件元数据和提取预览', inputSchema: { type: 'object', required: ['file_id'], properties: { file_id: { type: 'string' } } } },
  { name: 'miaozao.files.extract', description: '重新解析文件并返回文本/表格预览', inputSchema: { type: 'object', required: ['file_id'], properties: { file_id: { type: 'string' } } } },
  { name: 'miaozao.files.save', description: '把 Agent 产物保存到秒造文件服务', inputSchema: { type: 'object', required: ['filename', 'content_base64'], properties: { filename: { type: 'string' }, content_base64: { type: 'string' }, mime: { type: 'string' } } } },
  { name: 'miaozao.files.export', description: '导出应用业务数据为 CSV', inputSchema: { type: 'object', properties: { object_type: { type: 'string' } } } },
  { name: 'miaozao.ontology.query', description: '查询业务对象', inputSchema: { type: 'object', properties: { object_type: { type: 'string' }, q: { type: 'string' }, limit: { type: 'integer' } } } },
  { name: 'miaozao.ontology.get', description: '读取业务对象', inputSchema: { type: 'object', required: ['object_id'], properties: { object_id: { type: 'string' } } } },
  { name: 'miaozao.ontology.search', description: '搜索 Ontology 定义', inputSchema: { type: 'object', properties: { q: { type: 'string' } } } },
  { name: 'miaozao.action.execute', description: '执行受规则约束的业务动作', inputSchema: { type: 'object', required: ['action'], properties: { action: { type: 'string' } }, additionalProperties: true } },
  { name: 'miaozao.code.execute', description: '在隔离的 Code Runtime 中执行 Python、Node 或 Shell', inputSchema: { type: 'object', required: ['language', 'code'], properties: { language: { type: 'string', enum: ['python', 'node', 'shell'] }, code: { type: 'string' }, timeout_ms: { type: 'integer' } } } }
];
mcpTools.user.push(...capabilityTools, ...capabilityTools.map((tool) => ({ ...tool, name: tool.name.replace(/^miaozao\./, '') })));
mcpTools.builder.push(...capabilityTools);

const executeSandboxedCode = async ({ language, code, timeoutMs = 30000, appId, sessionId }) => {
  const executorUrl = process.env.CODE_EXECUTOR_URL;
  if (!executorUrl) throw new Error('Code Runtime 未配置（请设置 CODE_EXECUTOR_URL）；秒造不会在宿主机执行代码');
  if (!['python', 'node', 'shell'].includes(language)) throw new Error('仅支持 python、node、shell');
  if (typeof code !== 'string' || !code.trim() || code.length > 100_000) throw new Error('代码不能为空且不能超过 100KB');
  const controller = new AbortController(); const timer = setTimeout(() => controller.abort(), Math.min(Math.max(Number(timeoutMs) || 30000, 1000), 120000));
  try {
    const response = await fetch(`${executorUrl.replace(/\/$/, '')}/execute`, { method: 'POST', headers: { 'content-type': 'application/json', ...(process.env.CODE_EXECUTOR_TOKEN ? { authorization: `Bearer ${process.env.CODE_EXECUTOR_TOKEN}` } : {}) }, body: JSON.stringify({ language, code, timeout_ms: Math.min(Math.max(Number(timeoutMs) || 30000, 1000), 120000), app_id: appId, session_id: sessionId }), signal: controller.signal });
    const result = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(result.error || `Code Runtime 返回 ${response.status}`);
    return result;
  } finally { clearTimeout(timer); }
};

const mcpResult = (value) => ({ content: [{ type: 'text', text: JSON.stringify(value) }], structuredContent: value });
const mcp = async (request, reply, mode) => {
  const started = Date.now();
  const payload = body(request);
  if (!mcpTools[mode]) return reply.code(404).send({ error: 'MCP 模式不存在' });
  const call = payload.method === 'tools/call' ? payload.params || {} : { name: payload.tool || payload.name, arguments: payload.arguments || payload.params || {} };
  const args = call.arguments || {};
  const appId = payload.app_id || args.app_id || request.query?.app_id;
  if (payload.method === 'initialize') return { jsonrpc: '2.0', id: payload.id ?? null, result: { protocolVersion: '2025-06-18', capabilities: { tools: {} }, serverInfo: { name: `miaozao-${mode}`, version: '2.0.0' } } };
  if (payload.method === 'notifications/initialized') return reply.code(202).send();
  if (payload.method === 'tools/list') return { jsonrpc: '2.0', id: payload.id ?? null, result: { tools: mcpTools[mode] } };
  const record = await ownedApp(request, appId);
  if (!record) return reply.code(404).send({ jsonrpc: '2.0', id: payload.id ?? null, error: { code: -32001, message: '应用不存在' } });
  let result; let error = null; let status = 'ok';
  try {
    const manifest = manifestOf(record);
    switch (call.name) {
      case 'ontology.describe': result = { app: publicApp(record), objects: manifest.objects || [], links: manifest.links || [], actions: manifest.actions || [], files: manifest.files || [] }; break;
      case 'app.get_source': result = { source: record.source, manifest, version: record.draft_version }; break;
      case 'app.update_source': {
        if (!args.source?.trim()) throw new Error('source is required');
        const nextManifest = compileSource(args.source, { description: record.description });
        if (nextManifest.diagnostics.some((item) => item.level === 'error')) throw new Error(nextManifest.diagnostics.map((item) => item.message).join('；'));
        const version = Math.max(record.published_version, record.draft_version) + 1; const timestamp = now();
        await c('apps').updateOne({ id: record.id }, { $set: { source: args.source, manifest_json: nextManifest, draft_version: version, updated_at: timestamp } });
        await c('app_versions').insertOne({ id: id(), app_id: record.id, version, source: args.source, manifest_json: nextManifest, status: 'draft', created_at: timestamp, published_at: null, previous_version: record.published_version });
        await addEvent({ tenantId: request.tenant.id, appId: record.id, type: 'app.draft', message: `已生成 Ontology v${version} 草稿`, actor: 'builder' }); result = { version, manifest: nextManifest }; break;
      }
      case 'app.compile': result = compileSource(record.source, { description: record.description }); break;
      case 'app.publish': {
        if (manifest.diagnostics?.some((item) => item.level === 'error')) throw new Error('当前 Ontology 有编译错误');
        const latest = await c('app_versions').find({ app_id: record.id }).sort({ version: -1 }).limit(1).next();
        if (!latest || latest.version <= record.published_version) throw new Error('没有待发布草稿');
        await c('app_versions').updateMany({ app_id: record.id, status: 'published' }, { $set: { status: 'archived' } }); await c('app_versions').updateOne({ id: latest.id }, { $set: { status: 'published', published_at: now() } }); await c('apps').updateOne({ id: record.id }, { $set: { published_version: latest.version, updated_at: now() } }); result = { version: latest.version }; break;
      }
      case 'object.search':
      case 'miaozao.ontology.query': result = { objects: await searchObjects({ collections, appId: record.id, objectType: args.object_type, q: args.q, limit: args.limit }) }; break;
      case 'ontology.query': result = { objects: await searchObjects({ collections, appId: record.id, objectType: args.object_type, q: args.q, limit: args.limit }) }; break;
      case 'object.get': result = { object: await getObject({ collections, appId: record.id, objectId: args.object_id }) }; if (!result.object) throw new Error('对象不存在'); break;
      case 'miaozao.ontology.get': result = { object: await getObject({ collections, appId: record.id, objectId: args.object_id }) }; if (!result.object) throw new Error('对象不存在'); break;
      case 'ontology.get': result = { object: await getObject({ collections, appId: record.id, objectId: args.object_id }) }; if (!result.object) throw new Error('对象不存在'); break;
      case 'miaozao.ontology.search': { const needle = String(args.q || '').toLowerCase(); result = { objects: (manifest.objects || []).filter((item) => !needle || JSON.stringify(item).toLowerCase().includes(needle)), actions: (manifest.actions || []).filter((item) => !needle || JSON.stringify(item).toLowerCase().includes(needle)), links: (manifest.links || []).filter((item) => !needle || JSON.stringify(item).toLowerCase().includes(needle)) }; break; }
      case 'ontology.search': { const needle = String(args.q || '').toLowerCase(); result = { objects: (manifest.objects || []).filter((item) => !needle || JSON.stringify(item).toLowerCase().includes(needle)), actions: (manifest.actions || []).filter((item) => !needle || JSON.stringify(item).toLowerCase().includes(needle)), links: (manifest.links || []).filter((item) => !needle || JSON.stringify(item).toLowerCase().includes(needle)) }; break; }
      case 'object.create': result = { object: await createObject({ collections, manifest, tenantId: request.tenant.id, appId: record.id, objectType: args.object_type, data: args.properties, provenance: { type: 'agent', mode } }) }; await addEvent({ tenantId: request.tenant.id, appId: record.id, type: 'object.created', message: `创建 ${args.object_type} 对象`, actor: mode, payload: { object_id: result.object.id } }); break;
      case 'object.related': result = { objects: await relatedObjects({ collections, appId: record.id, objectId: args.object_id, linkType: args.link_type, direction: args.direction }) }; break;
      case 'action.list': result = { actions: manifest.actions || [] }; break;
      case 'action.describe': { const definition = findActionDefinition(manifest, args.action); if (!definition) throw new Error(`未知业务动作：${args.action}`); result = definition; break; }
      case 'action.test': result = { ok: Boolean(findActionDefinition(manifest, args.action)), action: args.action }; if (!result.ok) throw new Error(`未知业务动作：${args.action}`); break;
      case 'action.apply': result = await applyAction({ collections, tenantId: request.tenant.id, appId: record.id, manifest, actionName: args.action, args, actor: mode, addEvent }); break;
      case 'miaozao.action.execute': result = await applyAction({ collections, tenantId: request.tenant.id, appId: record.id, manifest, actionName: args.action, args, actor: mode, addEvent }); break;
      case 'action.execute': result = await applyAction({ collections, tenantId: request.tenant.id, appId: record.id, manifest, actionName: args.action, args, actor: mode, addEvent }); break;
      case 'file.list': result = { files: await c('files').find({ app_id: record.id }, { projection: { _id: 0, path: 0 } }).sort(sortDesc).toArray() }; break;
      case 'file.read':
      case 'miaozao.files.read': { const file = await c('files').findOne({ id: args.file_id, app_id: record.id }, { projection: { _id: 0, path: 0 } }); if (!file) throw new Error('文件不存在'); result = { file: { ...file, provenance: file.provenance_json } }; break; }
      case 'miaozao.files.search': { const needle = String(args.q || '').toLowerCase(); const files = await c('files').find({ app_id: record.id }, { projection: { _id: 0, path: 0 } }).sort(sortDesc).limit(Math.min(Number(args.limit) || 100, 500)).toArray(); result = { files: files.filter((file) => !needle || JSON.stringify({ name: file.original_name, preview: file.extracted_text }).toLowerCase().includes(needle)) }; break; }
      case 'file.search': { const needle = String(args.q || '').toLowerCase(); const files = await c('files').find({ app_id: record.id }, { projection: { _id: 0, path: 0 } }).sort(sortDesc).limit(Math.min(Number(args.limit) || 100, 500)).toArray(); result = { files: files.filter((file) => !needle || JSON.stringify({ name: file.original_name, preview: file.extracted_text }).toLowerCase().includes(needle)) }; break; }
      case 'miaozao.files.extract': { const file = await c('files').findOne({ id: args.file_id, app_id: record.id }); if (!file || !fs.existsSync(file.path)) throw new Error('文件不存在'); const preview = await parseTabular(file.path, file.original_name); result = { file_id: file.id, headers: preview.headers, rows: preview.rows.slice(0, 500), total: preview.rows.length }; break; }
      case 'file.extract': { const file = await c('files').findOne({ id: args.file_id, app_id: record.id }); if (!file || !fs.existsSync(file.path)) throw new Error('文件不存在'); const preview = await parseTabular(file.path, file.original_name); result = { file_id: file.id, headers: preview.headers, rows: preview.rows.slice(0, 500), total: preview.rows.length }; break; }
      case 'miaozao.files.save': { const filename = safeFilename(args.filename || 'agent-output.txt'); const raw = Buffer.from(String(args.content_base64 || ''), 'base64'); if (!raw.length || raw.length > 25 * 1024 * 1024) throw new Error('文件内容为空或超过 25MB'); const fileId = id(); const destination = path.join(uploadsDir, `${fileId}-${filename}`); fs.writeFileSync(destination, raw, { flag: 'wx' }); const file = { id: fileId, app_id: record.id, original_name: filename, path: destination, mime: args.mime || 'application/octet-stream', size: raw.length, status: 'ready', extracted_text: [], created_at: now(), provenance_json: { type: 'agent', mode, session_id: args.session_id || null } }; await c('files').insertOne(file); await addEvent({ tenantId: request.tenant.id, appId: record.id, type: 'file.saved', message: `Agent 保存了 ${filename}`, actor: mode, payload: { file_id: fileId } }); result = { id: fileId, original_name: filename, size: raw.length, status: 'ready' }; break; }
      case 'file.save': { const filename = safeFilename(args.filename || 'agent-output.txt'); const raw = Buffer.from(String(args.content_base64 || ''), 'base64'); if (!raw.length || raw.length > 25 * 1024 * 1024) throw new Error('文件内容为空或超过 25MB'); const fileId = id(); const destination = path.join(uploadsDir, `${fileId}-${filename}`); fs.writeFileSync(destination, raw, { flag: 'wx' }); const file = { id: fileId, app_id: record.id, original_name: filename, path: destination, mime: args.mime || 'application/octet-stream', size: raw.length, status: 'ready', extracted_text: [], created_at: now(), provenance_json: { type: 'agent', mode, session_id: args.session_id || null } }; await c('files').insertOne(file); result = { id: fileId, original_name: filename, size: raw.length, status: 'ready' }; break; }
      case 'miaozao.files.export': { const query = { app_id: record.id, deleted_at: null }; if (args.object_type) query.object_type = args.object_type; const rows = await c('records').find(query, { projection: { _id: 0, data_json: 1 } }).sort({ updated_at: -1 }).limit(5000).toArray(); const headers = [...new Set(rows.flatMap((row) => Object.keys(row.data_json || {})))]; const csvCell = (value) => `"${String(value ?? '').replaceAll('"', '""')}"`; result = { filename: `${args.object_type || 'records'}.csv`, content_type: 'text/csv', content_base64: Buffer.from(`\uFEFF${[headers.map(csvCell).join(','), ...rows.map((row) => headers.map((header) => csvCell(row.data_json?.[header])).join(','))].join('\n')}`).toString('base64'), count: rows.length }; break; }
      case 'file.export': { const query = { app_id: record.id, deleted_at: null }; if (args.object_type) query.object_type = args.object_type; const rows = await c('records').find(query, { projection: { _id: 0, data_json: 1 } }).sort({ updated_at: -1 }).limit(5000).toArray(); const headers = [...new Set(rows.flatMap((row) => Object.keys(row.data_json || {})))]; const csvCell = (value) => `"${String(value ?? '').replaceAll('"', '""')}"`; result = { filename: `${args.object_type || 'records'}.csv`, content_type: 'text/csv', content_base64: Buffer.from(`\uFEFF${[headers.map(csvCell).join(','), ...rows.map((row) => headers.map((header) => csvCell(row.data_json?.[header])).join('\n'))].join('\n')}`).toString('base64'), count: rows.length }; break; }
      case 'miaozao.code.execute': result = await executeSandboxedCode({ language: args.language, code: args.code, timeoutMs: args.timeout_ms, appId: record.id, sessionId: args.session_id }); break;
      case 'code.execute': result = await executeSandboxedCode({ language: args.language, code: args.code, timeoutMs: args.timeout_ms, appId: record.id, sessionId: args.session_id }); break;
      case 'history.search': result = { events: await c('events').find({ app_id: record.id }, { projection: { _id: 0 } }).sort(sortDesc).limit(100).toArray() }; break;
      case 'trace.search': { const query = { app_id: record.id }; if (args.status) query.status = args.status; result = { traces: await c('traces').find(query, { projection: { _id: 0 } }).sort(sortDesc).limit(100).toArray() }; break; }
      default: throw new Error(`未知工具：${call.name}`);
    }
  } catch (e) { status = 'error'; error = e.message; }
  await addTrace({ tenantId: request.tenant.id, appId: record.id, tool: call.name || payload.method, status, input: args, output: result || {}, error, durationMs: Date.now() - started });
  if (error) return reply.code(422).send({ jsonrpc: '2.0', id: payload.id ?? null, error: { code: -32602, message: error } });
  return { jsonrpc: '2.0', id: payload.id ?? null, result: mcpResult(result) };
};
app.get('/api/mcp/:mode', { preHandler: auth }, (request, reply) => reply.header('Allow', 'POST').code(405).send({ error: '此 MCP 连接使用无状态 POST' }));
app.post('/api/mcp/:mode', { preHandler: auth }, (request, reply) => mcp(request, reply, request.params.mode));
app.get('/api/mcp/:mode/tools', { preHandler: auth }, async (request, reply) => { if (!mcpTools[request.params.mode]) return reply.code(404).send({ error: 'MCP 模式不存在' }); return { mode: request.params.mode, tools: mcpTools[request.params.mode] }; });

app.setNotFoundHandler((request, reply) => { if (request.url.startsWith('/api/')) return reply.code(404).send({ error: '接口不存在' }); return reply.sendFile('index.html'); });
try { await initDb(); const port = Number(process.env.PORT || 41874); await app.listen({ port, host: process.env.HOST || '0.0.0.0' }); console.log(`Agent Native Runtime listening on http://localhost:${port} (MongoDB)`); } catch (error) { app.log.error(error, 'MongoDB connection failed'); process.exitCode = 1; await client.close().catch(() => {}); }
