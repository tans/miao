import PocketBase from 'pocketbase';
import { readFile, stat } from 'node:fs/promises';
import path from 'node:path';

const [archive, confirmation] = process.argv.slice(2);
if (confirmation !== '--confirm') throw new Error('Restore replaces the running PocketBase data. Re-run with --confirm after making a current backup.');
if (!archive || !/^miao_backup_[a-z0-9_-]+\.zip$/i.test(path.basename(archive))) throw new Error('Select an archive named miao_backup_*.zip');
const filePath = path.resolve(archive);
if (!(await stat(filePath)).isFile()) throw new Error('Backup file does not exist');
const email = process.env.POCKETBASE_SUPERUSER_EMAIL;
const password = process.env.POCKETBASE_SUPERUSER_PASSWORD;
if (!email || !password) throw new Error('Configure PocketBase superuser credentials before restoring');

if (process.env.MIAO_RESTORE_PLATFORM_STOPPED !== 'true') throw new Error('Use bun run restore so MIAO is stopped before restoring historical tasks');

const pb = new PocketBase(process.env.POCKETBASE_URL || 'http://127.0.0.1:8090');
pb.autoCancellation(false);
await pb.collection('_superusers').authWithPassword(email, password);
const bytes = await readFile(filePath);
await pb.backups.upload({ file: new File([bytes], path.basename(filePath), { type: 'application/zip' }) });
await pb.backups.restore(path.basename(filePath));

let restored = false;
for (let attempt = 0; attempt < 60; attempt += 1) {
  await new Promise((resolve) => setTimeout(resolve, 1000));
  try {
    const response = await fetch(`${process.env.POCKETBASE_URL || 'http://127.0.0.1:8090'}/api/health`);
    if (response.ok) { restored = true; break; }
  } catch {}
}
if (!restored) throw new Error('PocketBase did not become ready; MIAO remains stopped.');
await pb.collection('_superusers').authWithPassword(email, password);
const existingCollections = new Set((await pb.collections.getFullList()).map((collection) => collection.name));
const pause = async (collection, filter, data) => {
  if (!existingCollections.has(collection)) return;
  const rows = await pb.collection(collection).getFullList({ filter });
  for (const row of rows) await pb.collection(collection).update(row.id, data);
};
await pause('miao_tasks', 'status = "enabled"', { status: 'paused', next_run_at: '', pause_reason: '历史备份恢复，核实恢复点后已发生的动作再重新授权' });
await pause('miao_runs', 'status = "queued" || status = "running" || status = "waiting"', { status: 'cancelled', cancel_requested: true, finished_at: new Date().toISOString(), delivery_status: 'suppressed', error: '历史备份恢复：保留原动作证据，不自动恢复或重放' });
await pause('miao_run_attempts', 'status = "running"', { status: 'interrupted', finished_at: new Date().toISOString(), error: '历史备份恢复，原运行已隔离' });
await pause('automation_rules', 'enabled = true', { enabled: false, pause_reason: '历史备份恢复，等待负责人重新核实启用' });
if (existingCollections.has('miao_runtime_locks')) for (const lock of await pb.collection('miao_runtime_locks').getFullList()) await pb.collection('miao_runtime_locks').delete(lock.id);
console.log(`PocketBase restored from ${filePath}. Tasks and unfinished runs are isolated; MIAO remains stopped. Review historical effects before server:start and selectively enabling tasks.`);
