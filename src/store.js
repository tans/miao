import crypto from 'node:crypto';
import PocketBase from 'pocketbase';

const pocketbaseUrl = process.env.POCKETBASE_URL || 'http://127.0.0.1:8090';
export const pocketbase = new PocketBase(pocketbaseUrl);
pocketbase.autoCancellation(false);

let connection;
export const connectPocketBase = () => {
  if (!connection) {
    connection = pocketbase.collection('_superusers').authWithPassword(
      process.env.POCKETBASE_SUPERUSER_EMAIL || '',
      process.env.POCKETBASE_SUPERUSER_PASSWORD || ''
    ).then(() => pocketbase);
    connection.catch(() => { connection = null; });
  }
  return connection;
};

export const createPocketBaseClient = (token = '') => {
  const client = new PocketBase(pocketbaseUrl);
  client.autoCancellation(false);
  if (token) client.authStore.save(token);
  return client;
};

export const id = () => {
  const alphabet = 'abcdefghijklmnopqrstuvwxyz0123456789';
  return [...crypto.randomBytes(15)].map((byte) => alphabet[byte % alphabet.length]).join('');
};
export const now = () => new Date().toISOString();

export const parseJson = (value, fallback = {}) => {
  if (value && typeof value === 'object') return value;
  try { return JSON.parse(value); } catch { return fallback; }
};

export const publicApp = (row) => row && ({
  id: row.id,
  tenant_id: row.tenant_id,
  name: row.name,
  description: row.description,
  definition: row.draft_definition ?? row.definition,
  manifest: parseJson(row.draft_manifest_json ?? row.manifest_json),
  published_definition: row.published_definition ?? row.definition,
  published_manifest: parseJson(row.published_manifest_json ?? row.manifest_json),
  draft_definition: row.draft_definition ?? row.definition,
  draft_manifest: parseJson(row.draft_manifest_json ?? row.manifest_json),
  published_version: row.published_version,
  draft_version: row.draft_version,
  created_at: row.created_at,
  updated_at: row.updated_at
});

export async function addEvent({ tenantId, appId = null, type, message, payload = {}, actor = 'system' }) {
  await collections.events.insertOne({ id: id(), tenant_id: tenantId, app_id: appId, type, message, payload_json: payload, actor, created_at: now() });
}

export async function addTrace({ tenantId, appId = null, sessionId = null, userId = null, agentId = null, permissions = [], tool, status = 'ok', input = {}, output = {}, error = null, durationMs = 0 }) {
  await collections.traces.insertOne({ id: id(), tenant_id: tenantId, app_id: appId, session_id: sessionId, user_id: userId, agent_id: agentId, permissions, tool, status, input_json: input, output_json: output, error, created_at: now(), duration_ms: durationMs });
}

const match = (doc, filter) => Object.entries(filter || {}).every(([key, expected]) => {
  if (expected === null) return doc[key] === undefined || doc[key] === null || doc[key] === '';
  if (expected && typeof expected === 'object' && !Array.isArray(expected)) {
    if ('$ne' in expected) return doc[key] !== expected.$ne;
    if ('$gt' in expected) return doc[key] > expected.$gt;
    if ('$in' in expected) return expected.$in.includes(doc[key]);
    if ('$exists' in expected) return expected.$exists ? doc[key] !== undefined : doc[key] === undefined;
  }
  return doc[key] === expected;
});

const sortRows = (list, spec) => {
  const keys = Object.entries(spec || {});
  return [...list].sort((a, b) => {
    for (const [key, dir] of keys) {
      const cmp = a[key] > b[key] ? 1 : a[key] < b[key] ? -1 : 0;
      if (cmp) return dir === -1 ? -cmp : cmp;
    }
    return 0;
  });
};

const asRecord = (doc) => {
  const record = Object.fromEntries(Object.entries(doc).filter(([, value]) => value !== null && value !== undefined));
  if (Buffer.isBuffer(record.content)) {
    const filename = String(record.path || 'resource').split('/').pop().replace(/[^\w.-]/g, '_').slice(0, 160);
    record.content = new File([record.content], filename, { type: record.mime || 'application/octet-stream' });
  }
  return record;
};

function collection(name) {
  return {
    name,
    async insertOne(doc) {
      const record = await connectPocketBase().then((client) => client.collection(name).create(asRecord(doc)));
      return { insertedId: record.id, modifiedCount: 1 };
    },
    async updateOne(filter, update) {
      const row = await this.findOne(filter);
      if (!row) return { modifiedCount: 0 };
      await connectPocketBase().then((client) => client.collection(name).update(row.id, update.$set || {}));
      return { modifiedCount: 1 };
    },
    async updateMany(filter, update) {
      const rows = await this.find(filter).toArray();
      const client = await connectPocketBase();
      await Promise.all(rows.map((row) => client.collection(name).update(row.id, update.$set || {})));
      return { modifiedCount: rows.length };
    },
    async deleteOne(filter) {
      const row = await this.findOne(filter);
      if (!row) return { deletedCount: 0 };
      await connectPocketBase().then((client) => client.collection(name).delete(row.id));
      return { deletedCount: 1 };
    },
    async findOne(filter) {
      const rows = await this.find(filter).limit(1).toArray();
      return rows[0] || null;
    },
    find(filter = {}) {
      const query = { sortSpec: {}, max: null };
      const api = {
        sort(spec) { query.sortSpec = spec; return api; },
        limit(n) { query.max = n; return api; },
        project() { return api; },
        async toArray() {
          const client = await connectPocketBase();
          const exactFilters = {};
          const residualFilters = {};
          for (const [key, value] of Object.entries(filter)) {
            if (value === null || typeof value !== 'object' || Array.isArray(value)) exactFilters[key] = value;
            else residualFilters[key] = value;
          }
          const clauses = Object.entries(exactFilters).map(([key, value], index) => `${key} = {:p${index}}`);
          const params = Object.fromEntries(Object.entries(exactFilters).map(([key, value], index) => [`p${index}`, value ?? '']));
          const options = clauses.length ? { filter: client.filter(clauses.join(' && '), params) } : {};
          let rows = (await client.collection(name).getFullList({ batch: 200, ...options })).filter((row) => match(row, residualFilters));
          rows = sortRows(rows, query.sortSpec);
          if (query.max) rows = rows.slice(0, query.max);
          return rows;
        }
      };
      return api;
    }
  };
}

const collections = Object.fromEntries(['apps', 'app_versions', 'static_resources', 'events', 'traces', 'tenants'].map((name) => [name, collection(name)]));
export { collections };
