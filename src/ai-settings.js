import crypto from 'node:crypto';
import { pocketbase } from './store.js';

const settingName = 'ai_gateway_api_key';
const encryptionKey = () => {
  const secret = process.env.MIAO_SETTINGS_ENCRYPTION_KEY || '';
  if (secret.length < 32) throw new Error('MIAO_SETTINGS_ENCRYPTION_KEY must contain at least 32 characters');
  return crypto.createHash('sha256').update(secret).digest();
};

const encrypt = (value) => {
  const iv = crypto.randomBytes(12);
  const cipher = crypto.createCipheriv('aes-256-gcm', encryptionKey(), iv);
  const encrypted = Buffer.concat([cipher.update(value, 'utf8'), cipher.final()]);
  return `v1.${iv.toString('base64url')}.${cipher.getAuthTag().toString('base64url')}.${encrypted.toString('base64url')}`;
};

const decrypt = (value) => {
  const [version, iv, tag, ciphertext] = String(value || '').split('.');
  if (version !== 'v1' || !iv || !tag || !ciphertext) throw new Error('Stored AI key has an unsupported format');
  const decipher = crypto.createDecipheriv('aes-256-gcm', encryptionKey(), Buffer.from(iv, 'base64url'));
  decipher.setAuthTag(Buffer.from(tag, 'base64url'));
  return Buffer.concat([decipher.update(Buffer.from(ciphertext, 'base64url')), decipher.final()]).toString('utf8');
};

export const isPlatformAdmin = (email) => String(process.env.MIAO_ADMIN_EMAILS || '').split(',').map((item) => item.trim().toLowerCase()).filter(Boolean).includes(String(email || '').toLowerCase());

export const readAIKey = async () => {
  const saved = await pocketbase.collection('platform_settings').getFirstListItem(
    pocketbase.filter('name = {:name}', { name: settingName })
  ).catch(() => null);
  if (saved) return { key: decrypt(saved.value), source: 'admin' };
  return { key: process.env.AI_GATEWAY_API_KEY || '', source: process.env.AI_GATEWAY_API_KEY ? 'environment' : 'none' };
};

export const readAIConfig = async () => {
  const saved = await pocketbase.collection('platform_settings').getFirstListItem(
    pocketbase.filter('name = {:name}', { name: settingName })
  ).catch(() => null);
  const environmentKey = process.env.AI_GATEWAY_API_KEY || '';
  const key = saved ? decrypt(saved.value) : environmentKey;
  const provider = process.env.MIAO_AI_PROVIDER === 'capi' ? 'capi' : 'vercel';
  return {
    key,
    source: saved ? 'admin' : environmentKey ? 'environment' : 'none',
    provider,
    baseUrl: provider === 'capi' ? (process.env.MIAO_AI_BASE_URL || 'http://127.0.0.1:3210/api/v1') : 'https://ai-gateway.vercel.sh',
    model: process.env.MIAO_AI_MODEL || 'gpt-5.2',
  };
};

export const writeAIKey = async (key, userId) => {
  const value = encrypt(key);
  const saved = await pocketbase.collection('platform_settings').getFirstListItem(
    pocketbase.filter('name = {:name}', { name: settingName })
  ).catch(() => null);
  if (saved) await pocketbase.collection('platform_settings').update(saved.id, { value, updated_by: userId });
  else await pocketbase.collection('platform_settings').create({ name: settingName, value, updated_by: userId });
};

export const useEnvironmentAIKey = async () => {
  const saved = await pocketbase.collection('platform_settings').getFirstListItem(
    pocketbase.filter('name = {:name}', { name: settingName })
  ).catch(() => null);
  if (saved) await pocketbase.collection('platform_settings').delete(saved.id);
};
