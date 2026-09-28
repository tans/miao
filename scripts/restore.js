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

const pb = new PocketBase(process.env.POCKETBASE_URL || 'http://127.0.0.1:8090');
pb.autoCancellation(false);
await pb.collection('_superusers').authWithPassword(email, password);
const bytes = await readFile(filePath);
await pb.backups.upload({ file: new File([bytes], path.basename(filePath), { type: 'application/zip' }) });
await pb.backups.restore(path.basename(filePath));

for (let attempt = 0; attempt < 60; attempt += 1) {
  await new Promise((resolve) => setTimeout(resolve, 1000));
  try {
    const response = await fetch(`${process.env.POCKETBASE_URL || 'http://127.0.0.1:8090'}/api/health`);
    if (response.ok) {
      console.log(`PocketBase restored from ${filePath}. Recheck MIAO login and record access.`);
      process.exit(0);
    }
  } catch {}
}
throw new Error('PocketBase did not become ready after restore. Check service logs before restarting MIAO.');
