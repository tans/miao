import test from 'node:test';
import assert from 'node:assert/strict';
import { compileSource } from '../src/manifest.js';
import { validateProperties, applyAction } from '../src/ontology-runtime.js';

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

const collection = (rows = []) => ({
  rows,
  findOne: async (query) => rows.find((row) => Object.entries(query).every(([key, value]) => row[key] === value)) || null,
  updateOne: async (query, update) => {
    const row = rows.find((item) => Object.entries(query).every(([key, value]) => item[key] === value));
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
