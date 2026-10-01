import ExcelJS from 'exceljs';
import { parse } from 'csv-parse/sync';
import { resolveAppAccess, canEditRecords } from './apps.js';

export function normalizeSheetRows(rows) {
  if (!rows.length) throw Object.assign(new Error('文件没有内容'), { statusCode: 400 });
  const headers = rows[0].map((value) => String(value ?? '').trim());
  if (!headers.length || headers.length > 100 || headers.some((name) => !name) || new Set(headers).size !== headers.length) throw Object.assign(new Error('表头必须非空且不重复，最多 100 列'), { statusCode: 400 });
  return { headers, rows: rows.slice(1, 101).map((values) => Object.fromEntries(headers.map((name, index) => [name, values[index] ?? '']))), truncated: rows.length > 101 };
}

export async function readSpreadsheet(bytes, name, sheetName) {
  if (/\.csv$/i.test(name)) return normalizeSheetRows(parse(bytes.toString('utf8').replace(/^\uFEFF/, ''), { skip_empty_lines: true, bom: true, relax_column_count: false, max_record_size: 100000 }));
  if (!/\.xlsx$/i.test(name)) return { text: bytes.toString('utf8').slice(0, 30000), truncated: bytes.length > 30000 };
  // Bound decompressed XLSX size before ExcelJS loads its shared strings and XML.
  const { default: yauzl } = await import('yauzl');
  await new Promise((resolve, reject) => yauzl.fromBuffer(bytes, { lazyEntries: true }, (error, zip) => {
    if (error) return reject(error);
    let total = 0, count = 0;
    zip.on('error', reject); zip.on('end', resolve);
    zip.on('entry', (entry) => {
      total += entry.uncompressedSize; count++;
      if (total > 32 * 1024 * 1024 || count > 2000) { zip.close(); reject(Object.assign(new Error('Excel 解压后超过读取限制'), { statusCode: 400 })); return; }
      zip.readEntry();
    }); zip.readEntry();
  }));
  const workbook = new ExcelJS.Workbook();
  await workbook.xlsx.load(bytes);
  const sheets = workbook.worksheets.map((sheet) => sheet.name);
  const sheet = sheetName ? workbook.getWorksheet(sheetName) : workbook.worksheets[0];
  if (!sheet) throw Object.assign(new Error('工作表不存在'), { statusCode: 400 });
  if (sheet.columnCount > 100) throw Object.assign(new Error('工作表最多 100 列'), { statusCode: 400 });
  const value = (cell) => cell == null ? '' : cell instanceof Date ? cell.toISOString() : typeof cell === 'object' ? cell.result ?? cell.text ?? cell.richText?.map((part) => part.text).join('') ?? '' : cell;
  const rows = [];
  for (let index = 1; index <= Math.min(sheet.rowCount, 102); index++) rows.push(Array.from({ length: sheet.columnCount }, (_, col) => value(sheet.getRow(index).getCell(col + 1).value)));
  return { ...normalizeSheetRows(rows), sheet: sheet.name, sheets, truncated: sheet.rowCount > 101 };
}

export async function scopedFile(pocketbase, request, reply) {
  const application = await resolveAppAccess(request, reply, pocketbase);
  if (!application) return null;
  const file = await pocketbase.collection('app_files').getOne(request.params.fileId).catch(() => null);
  if (!file || file.tenant_id !== request.tenant.id || file.app_id !== application.id) { reply.code(404).send({ error: '文件不存在' }); return null; }
  return file;
}

export async function fileBytes(pocketbase, file) {
  const response = await fetch(pocketbase.files.getURL(file, file.file, { token: await pocketbase.files.getToken() }), { headers: { Authorization: pocketbase.authStore.token } });
  if (!response.ok) throw Object.assign(new Error('文件读取失败'), { statusCode: 503 });
  return Buffer.from(await response.arrayBuffer());
}

export function registerFileRoutes(app, { auth, pocketbase }) {
  app.get('/api/apps/:id/files', { preHandler: auth }, async (request, reply) => {
    if (!await resolveAppAccess(request, reply, pocketbase)) return;
    const result = await pocketbase.collection('app_files').getList(Math.max(1, Number.parseInt(request.query.page, 10) || 1), 25, { filter: pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId}', { tenantId: request.tenant.id, appId: request.params.id }), sort: '-created' });
    return { ...result, items: result.items.map(({ id, name, created }) => ({ id, name, created_at: created })) };
  });
  app.post('/api/apps/:id/files', { preHandler: auth }, async (request, reply) => {
    const application = await resolveAppAccess(request, reply, pocketbase);
    if (!application) return;
    if (!canEditRecords(request) || application.archived) return reply.code(403).send({ error: '没有文件上传权限' });
    const { name, base64 } = request.body || {};
    if (typeof name !== 'string' || !/\.(csv|xlsx|txt|md|pdf|png|jpg|jpeg|webp)$/i.test(name) || typeof base64 !== 'string' || base64.length > 6990508) return reply.code(400).send({ error: '文件名称或内容无效，支持 CSV、XLSX、文本、PDF 和图片' });
    const bytes = Buffer.from(base64.replace(/^data:[^;,]+;base64,/, ''), 'base64');
    if (!bytes.length || bytes.length > 5 * 1024 * 1024) return reply.code(400).send({ error: '文件必须为 1 字节到 5 MB' });
    const filename = name.split(/[\\/]/).pop().replace(/[^\p{L}\p{N}._-]/gu, '_').slice(0, 120);
    const file = await pocketbase.collection('app_files').create({ tenant_id: request.tenant.id, app_id: application.id, user_id: request.user.id, name: filename, file: new File([bytes], filename) });
    return reply.code(201).send({ id: file.id, name: filename });
  });
  app.get('/api/apps/:id/files/:fileId/content', { preHandler: auth }, async (request, reply) => {
    const file = await scopedFile(pocketbase, request, reply); if (!file) return;
    if (!/\.(csv|xlsx|txt|md)$/i.test(file.name)) return reply.code(400).send({ error: '此文件只支持下载，未提供 OCR 或 PDF 文本提取' });
    try { return { id: file.id, name: file.name, ...await readSpreadsheet(await fileBytes(pocketbase, file), file.name, request.query.sheet) }; }
    catch (error) { return reply.code(error.statusCode || 400).send({ error: error.message }); }
  });
  app.post('/api/apps/:id/files/:fileId/attach', { preHandler: auth }, async (request, reply) => {
    const file = await scopedFile(pocketbase, request, reply); if (!file) return;
    if (!canEditRecords(request)) return reply.code(403).send({ error: '没有记录修改权限' });
    const { table, record_id, field, expected_updated_at } = request.body || {};
    if (![table, record_id, field, expected_updated_at].every((value) => typeof value === 'string' && value)) return reply.code(400).send({ error: '需要目标表、记录、附件字段和更新时间' });
    const bytes = await fileBytes(pocketbase, file);
    const extension = file.name.split('.').pop().toLowerCase();
    const mime = { png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', webp: 'image/webp', pdf: 'application/pdf', txt: 'text/plain', md: 'text/plain' }[extension];
    if (!mime) return reply.code(400).send({ error: '此文件类型不能作为业务附件；CSV/XLSX 请使用导入工具' });
    const response = await app.inject({ method: 'PATCH', url: `/api/apps/${encodeURIComponent(request.params.id)}/collections/${encodeURIComponent(table)}/records/${encodeURIComponent(record_id)}`, headers: { authorization: request.headers.authorization || '', 'x-miao-tenant-id': request.tenant.id }, payload: { data: {}, expected_updated_at, files: { [field]: { name: file.name, type: mime, base64: bytes.toString('base64') } } } });
    return reply.code(response.statusCode).send(response.json());
  });
  app.get('/api/apps/:id/files/:fileId/download', { preHandler: auth }, async (request, reply) => {
    const file = await scopedFile(pocketbase, request, reply); if (!file) return;
    return reply.header('Content-Type', 'application/octet-stream').header('Content-Disposition', `attachment; filename*=UTF-8''${encodeURIComponent(file.name)}`).send(await fileBytes(pocketbase, file));
  });
}
