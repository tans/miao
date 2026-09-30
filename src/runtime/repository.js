import crypto from 'node:crypto';
import { serialized } from '../business/locks.js';
export { serialized } from '../business/locks.js';

export const missing = (error) => { if (error.status === 404) return null; throw error; };

export async function enqueueRun(pocketbase, task, eventKey, input = {}) {
  return serialized(`enqueue:${task.id}`, async () => {
    const existing = await pocketbase.collection('miao_runs').getFirstListItem(pocketbase.filter('task_id = {:id} && event_key = {:key}', { id: task.id, key: eventKey })).catch(missing);
    if (existing) return existing;
    try {
      return await pocketbase.collection('miao_runs').create({ tenant_id: task.tenant_id, app_id: task.app_id, task_id: task.id, created_by: task.created_by, event_key: eventKey,
        snapshot: { ...task.definition, name: task.name, revision: task.revision, input }, status: 'queued', attempts: 0, model_requests: 0, delivery_status: task.definition.mode === 'preview' ? 'suppressed' : 'pending' });
    } catch (error) {
      const duplicate = await pocketbase.collection('miao_runs').getFirstListItem(pocketbase.filter('task_id = {:id} && event_key = {:key}', { id: task.id, key: eventKey })).catch(missing);
      if (duplicate) return duplicate;
      throw error;
    }
  });
}

export const actionKey = (tool, input) => {
  const stable = (value) => Array.isArray(value) ? value.map(stable) : value && typeof value === 'object' ? Object.fromEntries(Object.keys(value).sort().map((key) => [key, stable(value[key])])) : value;
  return crypto.createHash('sha256').update(JSON.stringify({ tool, input: stable(input) })).digest('hex');
};

export const publicRun = ({ checkpoint, snapshot, ...row }) => ({ ...row, task_name: snapshot?.name, revision: snapshot?.revision, trigger: snapshot?.trigger, mode: snapshot?.mode || 'live' });
