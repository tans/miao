import { MongoClient } from 'mongodb';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';

const dataDir = path.resolve(process.env.DATA_DIR || 'data');
fs.mkdirSync(dataDir, { recursive: true });
export const uploadsDir = path.join(dataDir, 'uploads');
fs.mkdirSync(uploadsDir, { recursive: true });
export const extractedDir = path.join(dataDir, 'extracted');
fs.mkdirSync(extractedDir, { recursive: true });
const uri = process.env.MONGODB_URI || 'mongodb://127.0.0.1:27017';
const databaseName = process.env.MONGODB_DB || 'agent_native_runtime';
export const client = new MongoClient(uri, { serverSelectionTimeoutMS: 5000 });
export let mongo;
export const collections = {};
export async function initDb() {
  await client.connect();
  mongo = client.db(databaseName);
  for (const name of ['users', 'sessions', 'agent_sessions', 'app_tokens', 'tenants', 'apps', 'app_versions', 'records', 'links', 'files', 'events', 'traces']) collections[name] = mongo.collection(name);
  await Promise.all([
    collections.users.createIndex({ email: 1 }, { unique: true }),
    collections.tenants.createIndex({ slug: 1 }, { unique: true }),
    collections.sessions.createIndex({ token: 1 }, { unique: true }),
    collections.sessions.createIndex({ expires_at: 1 }, { expireAfterSeconds: 0 }),
    collections.agent_sessions.createIndex({ tenant_id: 1, app_id: 1, created_at: -1 }),
    collections.agent_sessions.createIndex({ token: 1 }, { unique: true }),
    collections.app_tokens.createIndex({ token_hash: 1 }, { unique: true }),
    collections.app_tokens.createIndex({ tenant_id: 1, app_id: 1, scope: 1, revoked_at: 1 }),
    collections.records.createIndex({ tenant_id: 1, app_id: 1, object_type: 1, deleted_at: 1 }),
    collections.links.createIndex({ tenant_id: 1, app_id: 1, link_type: 1, from_object_id: 1 }),
    collections.links.createIndex({ tenant_id: 1, app_id: 1, link_type: 1, to_object_id: 1 }),
    collections.app_versions.createIndex({ app_id: 1, version: 1 }, { unique: true }),
    collections.events.createIndex({ app_id: 1, created_at: -1 }),
    collections.traces.createIndex({ app_id: 1, created_at: -1 })
  ]);
}
export const id = () => crypto.randomUUID();
export const now = () => new Date().toISOString();
export const hashToken = (token) => crypto.createHash('sha256').update(String(token)).digest('hex');
export const hashPassword = (password) => { const salt = crypto.randomBytes(16).toString('hex'); return `${salt}:${crypto.scryptSync(password, salt, 64).toString('hex')}`; };
export const verifyPassword = (password, stored) => { const [salt, hash] = String(stored).split(':'); if (!salt || !hash) return false; const candidate = crypto.scryptSync(password, salt, 64).toString('hex'); return crypto.timingSafeEqual(Buffer.from(candidate, 'hex'), Buffer.from(hash, 'hex')); };
export const parseJson = (value, fallback = {}) => { if (value && typeof value === 'object') return value; try { return JSON.parse(value); } catch { return fallback; } };
export const publicUser = (row) => row && ({ id: row.id, email: row.email, name: row.name, created_at: row.created_at });
export const publicApp = (row) => row && ({
  id: row.id,
  tenant_id: row.tenant_id,
  name: row.name,
  description: row.description,
  source: row.draft_source ?? row.source,
  manifest: parseJson(row.draft_manifest_json ?? row.manifest_json),
  published_source: row.published_source ?? row.source,
  published_manifest: parseJson(row.published_manifest_json ?? row.manifest_json),
  draft_source: row.draft_source ?? row.source,
  draft_manifest: parseJson(row.draft_manifest_json ?? row.manifest_json),
  published_version: row.published_version,
  draft_version: row.draft_version,
  created_at: row.created_at,
  updated_at: row.updated_at
});
export async function addEvent({ tenantId, appId = null, type, message, payload = {}, actor = 'system' }) { await collections.events.insertOne({ id: id(), tenant_id: tenantId, app_id: appId, type, message, payload_json: payload, actor, created_at: now() }); }
export async function addTrace({ tenantId, appId = null, tool, status = 'ok', input = {}, output = {}, error = null, durationMs = 0 }) { await collections.traces.insertOne({ id: id(), tenant_id: tenantId, app_id: appId, tool, status, input_json: input, output_json: output, error, created_at: now(), duration_ms: durationMs }); }
