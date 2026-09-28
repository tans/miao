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
