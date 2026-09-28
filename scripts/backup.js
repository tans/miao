import PocketBase from 'pocketbase';
import { mkdir, readdir, rename, stat, unlink, writeFile } from 'node:fs/promises';
import path from 'node:path';

const base = process.env.POCKETBASE_URL || 'http://127.0.0.1:8090';
const email = process.env.POCKETBASE_SUPERUSER_EMAIL;
const password = process.env.POCKETBASE_SUPERUSER_PASSWORD;
if (!email || !password) throw new Error('Configure PocketBase superuser credentials before running backups');

const root = process.env.MIAO_DATA_DIR || path.resolve('data');
const backupDir = path.resolve(process.env.MIAO_BACKUP_DIR || path.join(root, 'backups'));
const configuredRetention = Number(process.env.MIAO_BACKUP_RETENTION_DAYS || 30);
const retentionDays = Number.isFinite(configuredRetention) ? Math.max(1, Math.floor(configuredRetention)) : 30;
await mkdir(backupDir, { recursive: true, mode: 0o700 });

const pb = new PocketBase(base);
pb.autoCancellation(false);
await pb.collection('_superusers').authWithPassword(email, password);
const stamp = new Date().toISOString().replace(/[-:T]/g, '').replace(/\.\d+Z$/, 'Z').toLowerCase();
const name = `miao_backup_${stamp}.zip`;
await pb.backups.create(name);
const token = await pb.files.getToken();
const response = await fetch(pb.backups.getDownloadURL(token, name));
if (!response.ok) throw new Error(`PocketBase backup download failed: ${response.status}`);
const target = path.join(backupDir, name);
const temporary = `${target}.partial`;
await writeFile(temporary, Buffer.from(await response.arrayBuffer()), { mode: 0o600 });
await rename(temporary, target);
await pb.backups.delete(name);

const cutoff = Date.now() - retentionDays * 24 * 60 * 60 * 1000;
for (const entry of await readdir(backupDir, { withFileTypes: true })) {
  if (!entry.isFile() || !/^miao_backup_[a-z0-9_-]+\.zip$/.test(entry.name)) continue;
  const file = path.join(backupDir, entry.name);
  const info = await stat(file);
  if (info.mtimeMs < cutoff) await unlink(file);
}
console.log(`PocketBase backup saved: ${target}`);
