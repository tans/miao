import { hashToken } from './db.js';

export async function resolveAppCapability({ tokens, apps, token, scope, requestedAppId = null, timestamp }) {
  if (!token || !['builder', 'user'].includes(scope)) return null;
  const capability = await tokens.findOne({ token_hash: hashToken(token), scope, revoked_at: null, $or: [{ expires_at: null }, { expires_at: { $gt: timestamp } }, { expires_at: { $exists: false } }] });
  if (!capability || (requestedAppId && requestedAppId !== capability.app_id)) return null;
  const app = await apps.findOne({ id: capability.app_id, tenant_id: capability.tenant_id });
  return app ? { capability, app } : null;
}

export async function resolveMcpSession({ sessions, apps, token, scope, requestedAppId = null, timestamp }) {
  if (!token || !['builder', 'user'].includes(scope)) return null;
  // DSH session tokens are long-lived credentials. Their lifetime is the
  // DSH session lifecycle and is controlled by explicit revocation.
  const session = await sessions.findOne({ token_hash: hashToken(token), scope, revoked_at: null });
  if (!session || (requestedAppId && requestedAppId !== session.app_id)) return null;
  const app = await apps.findOne({ id: session.app_id, tenant_id: session.tenant_id });
  return app ? { session, app } : null;
}
