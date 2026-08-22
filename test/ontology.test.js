import test from 'node:test';
import assert from 'node:assert/strict';
import { compileSource } from '../src/manifest.js';
import { blockingPublishDiagnostics, publishSnapshot, rollbackSnapshot } from '../src/app-runtime.js';
import { resolveAppCapability } from '../src/capability.js';
import { hashToken } from '../src/db.js';
import { manifestOf, validateProperties, buildObjectRecord, mapImportRows, applyAction } from '../src/ontology-runtime.js';

const source = `# 报价

## 目标
管理报价确认。

## Objects

### Quote
Properties:
- status: enum[待确认, 已确认] required
- unit_price: number required

### Supplier
Properties:
- name: string required

## Links
- Quote from Supplier

## Actions

### confirm_quote
Input:
- quote_id: object_ref Quote required
Rules:
- required: unit_price
- greater_than: unit_price 0
Mutations:
- set: Quote.status = 已确认
`;

const matches = (row, query) => Object.entries(query).every(([key, value]) => {
  if (key === '$or') return value.some((part) => matches(row, part));
  if (value && typeof value === 'object' && '$in' in value) return value.$in.includes(row[key]);
  if (value && typeof value === 'object' && '$gt' in value) return row[key] > value.$gt;
  return row[key] === value;
});
const collection = (rows = []) => ({
  rows,
  findOne: async (query) => rows.find((row) => matches(row, query)) || null,
  find: (query) => ({ toArray: async () => rows.filter((row) => matches(row, query)) }),
  insertOne: async (row) => { rows.push(row); return { insertedId: row.id }; },
  insertMany: async (items) => { rows.push(...items); return { insertedCount: items.length }; },
  updateOne: async (query, update, options = {}) => {
    let row = rows.find((item) => matches(item, query));
    if (!row && options.upsert && update.$setOnInsert) { row = { ...update.$setOnInsert }; rows.push(row); return { modifiedCount: 0, upsertedCount: 1 }; }
    if (!row) return { modifiedCount: 0 };
    for (const [key, value] of Object.entries(update.$set || {})) {
      if (key.startsWith('data_json.')) row.data_json[key.slice(10)] = value;
      else row[key] = value;
    }
    return { modifiedCount: 1 };
  }
});

test('compiler produces object, link, action and compatibility collections', () => {
  const manifest = compileSource(source);
  assert.equal(manifest.schema_version, 2);
  assert.deepEqual(manifest.objects.map((item) => item.slug), ['quote', 'supplier']);
  assert.deepEqual(manifest.collections[0].fields, ['status', 'unit_price']);
  assert.deepEqual(manifest.links[0], { slug: 'quote_supplier', name: 'Quote from Supplier', from: 'quote', to: 'supplier', cardinality: 'many' });
  assert.equal(manifest.actions[0].mutations[0].op, 'set');
  assert.deepEqual(manifest.diagnostics, []);
});

test('compiler reports links to unknown objects', () => {
  const manifest = compileSource(source.replace('Quote from Supplier', 'Quote from Customer'));
  assert.equal(manifest.diagnostics[0].code, 'unknown_link_object');
});

test('object validation enforces required and enum properties', () => {
  const quote = compileSource(source).objects[0];
  assert.deepEqual(validateProperties(quote, { status: '待确认', unit_price: 10 }), []);
  assert.match(validateProperties(quote, { status: '未知' }).join('；'), /unit_price.*必填/);
  assert.match(validateProperties(quote, { status: '未知', unit_price: 1 }).join('；'), /必须是/);
});

test('draft and published manifests stay isolated', () => {
  const published = compileSource(source);
  const draft = compileSource(source.replace('### Supplier', '### Customer'));
  const app = { published_manifest_json: published, draft_manifest_json: draft };
  assert.deepEqual(manifestOf(app).objects.map((item) => item.slug), ['quote', 'supplier']);
  assert.deepEqual(manifestOf(app, 'draft').objects.map((item) => item.slug), ['quote', 'customer']);
});

test('publish copies only draft fields and rollback switches only published fields', () => {
  const app = { source: 'legacy', manifest_json: { version: 0 }, draft_source: 'draft-v2', draft_manifest_json: { version: 2 }, draft_version: 2 };
  assert.deepEqual(publishSnapshot(app, 'now'), { published_source: 'draft-v2', published_manifest_json: { version: 2 }, published_version: 2, updated_at: 'now' });
  assert.deepEqual(rollbackSnapshot({ source: 'v1', manifest_json: { version: 1 }, version: 1 }, 'later'), { published_source: 'v1', published_manifest_json: { version: 1 }, published_version: 1, updated_at: 'later' });
  assert.equal(app.draft_source, 'draft-v2');
});

test('publish rejects uncompiled mutations even when compile reports a warning', () => {
  const manifest = compileSource(source.replace('set: Quote.status = 已确认', 'send an email somehow'));
  assert.equal(manifest.diagnostics[0].level, 'warning');
  assert.equal(blockingPublishDiagnostics(manifest)[0].code, 'uncompiled_mutation');
});

test('app capability tokens enforce scope, app and tenant together', async () => {
  const raw = 'user-secret'; const tokens = collection([{ token_hash: hashToken(raw), scope: 'user', revoked_at: null, expires_at: '2026-09-01', app_id: 'a1', tenant_id: 't1' }]); const apps = collection([{ id: 'a1', tenant_id: 't1' }, { id: 'a2', tenant_id: 't2' }]);
  assert.equal((await resolveAppCapability({ tokens, apps, token: raw, scope: 'user', requestedAppId: 'a1', timestamp: '2026-08-22' })).app.id, 'a1');
  assert.equal(await resolveAppCapability({ tokens, apps, token: raw, scope: 'builder', requestedAppId: 'a1', timestamp: '2026-08-22' }), null);
  assert.equal(await resolveAppCapability({ tokens, apps, token: raw, scope: 'user', requestedAppId: 'a2', timestamp: '2026-08-22' }), null);
  apps.rows[0].tenant_id = 't2';
  assert.equal(await resolveAppCapability({ tokens, apps, token: raw, scope: 'user', requestedAppId: 'a1', timestamp: '2026-08-22' }), null);
});

test('all object creation paths can build the canonical scoped record', () => {
  const manifest = compileSource(source);
  const record = buildObjectRecord({ manifest, tenantId: 't1', appId: 'a1', objectType: 'quote', data: { status: '待确认', unit_price: 12 }, provenance: { type: 'file', file_id: 'f1' }, timestamp: '2026-08-22T00:00:00.000Z' });
  assert.deepEqual(Object.keys(record).sort(), ['app_id', 'created_at', 'data_json', 'deleted_at', 'id', 'object_type', 'provenance_json', 'tenant_id', 'updated_at']);
  assert.equal(record.tenant_id, 't1');
  assert.equal(record.object_type, 'quote');
  assert.equal(record.deleted_at, null);
});

test('file import mapping keeps row provenance in canonical records', () => {
  const manifest = compileSource(source); const mapped = mapImportRows([{ 状态: '待确认', 金额: 12 }], { 状态: 'status', 金额: 'unit_price' });
  const record = buildObjectRecord({ manifest, tenantId: 't1', appId: 'a1', objectType: 'quote', data: mapped[0], provenance: { type: 'file', file_id: 'f1', source_row: 2 } });
  assert.deepEqual(record.data_json, { status: '待确认', unit_price: 12 });
  assert.deepEqual(record.provenance_json, { type: 'file', file_id: 'f1', source_row: 2 });
});

test('action rules reject invalid data and apply declared mutation', async () => {
  const manifest = compileSource(source);
  const quote = { id: 'q1', tenant_id: 't1', app_id: 'a1', object_type: 'quote', collection: 'quote', data_json: { status: '待确认', unit_price: 0 }, deleted_at: null };
  const records = collection([quote]);
  const events = [];
  const context = { collections: { records, links: collection() }, tenantId: 't1', appId: 'a1', manifest, actionName: 'confirm_quote', args: { quote_id: 'q1' }, actor: 'user', addEvent: async (event) => events.push(event) };
  await assert.rejects(applyAction(context), /必须大于 0/);
  quote.data_json.unit_price = 12;
  const result = await applyAction(context);
  assert.equal(quote.data_json.status, '已确认');
  assert.equal(result.changed[0].property, 'status');
  assert.equal(events[0].type, 'action.applied');
});

test('action validates every mutation before applying any write', async () => {
  const manifest = compileSource(source.replace('set: Quote.status = 已确认', 'set: Quote.status = 已确认\n- unsupported mutation'));
  const quote = { id: 'q1', tenant_id: 't1', app_id: 'a1', object_type: 'quote', data_json: { status: '待确认', unit_price: 12 }, deleted_at: null };
  const records = collection([quote]);
  await assert.rejects(applyAction({ collections: { records, links: collection() }, tenantId: 't1', appId: 'a1', manifest, actionName: 'confirm_quote', args: { quote_id: 'q1' }, actor: 'user', addEvent: async () => {} }), /未编译变更/);
  assert.equal(quote.data_json.status, '待确认');
});

test('action derives targets from object_ref inputs and creates declared links', async () => {
  const linkSource = `${source}\n### match_supplier\nInput:\n- quote: object_ref Quote required\n- supplier: object_ref Supplier required\nMutations:\n- link: quote_supplier from quote to supplier\n`;
  const manifest = compileSource(linkSource); const quote = { id: 'q1', app_id: 'a1', object_type: 'quote', data_json: { status: '待确认', unit_price: 12 }, deleted_at: null }; const supplier = { id: 's1', app_id: 'a1', object_type: 'supplier', data_json: { name: '供应商' }, deleted_at: null }; const links = collection();
  await applyAction({ collections: { records: collection([quote, supplier]), links }, tenantId: 't1', appId: 'a1', manifest, actionName: 'match_supplier', args: { quote: 'q1', supplier: 's1' }, actor: 'user', addEvent: async () => {} });
  assert.equal(links.rows[0].from_object_id, 'q1');
  assert.equal(links.rows[0].to_object_id, 's1');
});
