import crypto from 'node:crypto';

// Minimal in-memory store for the local Builder. Miao is a creator/preview/
// publish tool — the runtime (Agent Runtime, DSH, data, auth) lives in
// WorkBuddy, so we deliberately do NOT ship a MongoDB adapter or a production
// database. This ephemeral store is enough for local authoring and preview.
const apps = new Collection('apps');
const appVersions = new Collection('app_versions');
const staticResources = new Collection('static_resources');
const events = new Collection('events');
const traces = new Collection('traces');

const collections = { apps, app_versions: appVersions, static_resources: staticResources, events, traces };

export const id = () => crypto.randomUUID();
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
  await events.insertOne({ id: id(), tenant_id: tenantId, app_id: appId, type, message, payload_json: payload, actor, created_at: now() });
}

export async function addTrace({ tenantId, appId = null, sessionId = null, userId = null, agentId = null, permissions = [], tool, status = 'ok', input = {}, output = {}, error = null, durationMs = 0 }) {
  await traces.insertOne({ id: id(), tenant_id: tenantId, app_id: appId, session_id: sessionId, user_id: userId, agent_id: agentId, permissions, tool, status, input_json: input, output_json: output, error, created_at: now(), duration_ms: durationMs });
}

export { collections };

function Collection(name) {
  const rows = [];
  const match = (doc, filter) => {
    for (const [key, expected] of Object.entries(filter || {})) {
      if (expected && typeof expected === 'object' && !Array.isArray(expected)) {
        if ('$ne' in expected) { if (doc[key] === expected.$ne) return false; continue; }
        if ('$gt' in expected) { if (!(doc[key] > expected.$gt)) return false; continue; }
        if ('$in' in expected) { if (!expected.$in.includes(doc[key])) return false; continue; }
        if ('$exists' in expected) { if (expected.$exists ? doc[key] === undefined : doc[key] !== undefined) return false; continue; }
      }
      if (doc[key] !== expected) return false;
    }
    return true;
  };
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
  return {
    name,
    async insertOne(doc) { const row = { ...doc }; rows.push(row); return { insertedId: row.id, modifiedCount: 1 }; },
    async updateOne(filter, update) {
      const row = rows.find((d) => match(d, filter));
      if (!row) return { modifiedCount: 0 };
      if (update.$set) Object.assign(row, update.$set);
      return { modifiedCount: 1 };
    },
    async updateMany(filter, update) {
      let n = 0;
      for (const row of rows) if (match(row, filter)) { if (update.$set) Object.assign(row, update.$set); n++; }
      return { modifiedCount: n };
    },
    async deleteOne(filter) { const i = rows.findIndex((d) => match(d, filter)); if (i < 0) return { deletedCount: 0 }; rows.splice(i, 1); return { deletedCount: 1 }; },
    async findOne(filter) { return rows.find((d) => match(d, filter)) || null; },
    find(filter = {}) {
      const base = rows.filter((d) => match(d, filter));
      const api = {
        sort: (spec) => { api._sorted = sortRows(base, spec); return api; },
        limit: (n) => { api._limit = n; return api; },
        project: () => api,
        toArray: () => Promise.resolve((api._sorted ? (api._limit ? api._sorted.slice(0, api._limit) : api._sorted) : (api._limit ? base.slice(0, api._limit) : base)).map((x) => ({ ...x })))
      };
      return api;
    }
  };
}
