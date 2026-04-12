import { test, expect, Page, request } from '@playwright/test';
import { randomInt } from 'crypto';

// Test data helpers
function generateUsername(): string {
  return `test_user_${Date.now()}_${randomInt(10000)}`;
}

function generatePhone(): string {
  return `138${String(randomInt(100000000)).padStart(8, '0')}`;
}

// API helpers
const API_BASE = 'http://localhost:8888/api/v1';

async function apiRegister(username: string, password: string, phone: string, role: string) {
  const context = await request.newContext();
  const response = await context.post(`${API_BASE}/auth/register`, {
    data: { username, password, phone, role }
  });
  const data = await response.json();
  await context.dispose();
  return data;
}

async function apiLogin(username: string, password: string) {
  const context = await request.newContext();
  const response = await context.post(`${API_BASE}/auth/login`, {
    data: { username, password }
  });
  const data = await response.json();
  await context.dispose();
  return data;
}

async function apiGetProfile(token: string) {
  const context = await request.newContext();
  const response = await context.get(`${API_BASE}/user/profile`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  const data = await response.json();
  await context.dispose();
  return data;
}

async function apiUpdateProfile(token: string, profileData: { nickname?: string; phone?: string; avatar?: string }) {
  const context = await request.newContext();
  const response = await context.put(`${API_BASE}/user/profile`, {
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    data: profileData
  });
  const data = await response.json();
  await context.dispose();
  return data;
}

async function apiChangePassword(token: string, oldPassword: string, newPassword: string) {
  const context = await request.newContext();
  const response = await context.put(`${API_BASE}/user/password`, {
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    data: { old_password: oldPassword, new_password: newPassword }
  });
  const data = await response.json();
  await context.dispose();
  return data;
}

// ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂ¼ÃÂ¯ÃÂ¼ÃÂÃÂ¥ÃÂ¸ÃÂ¦ÃÂ©ÃÂÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¯ÃÂ¼ÃÂ
async function apiBusinessRecharge(token: string, amount: number, retries = 3) {
  const context = await request.newContext();
  for (let i = 0; i < retries; i++) {
    const response = await context.post(`${API_BASE}/business/recharge`, {
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      data: { amount, payment_method: 'alipay' }
    });
    const data = await response.json();
    if (data.code === 0 || i === retries - 1) {
      await context.dispose();
      return data;
    }
    await new Promise(r => setTimeout(r, 1000 * (i + 1)));
  }
  await context.dispose();
  return { code: -1, message: 'ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂ¼ÃÂ¥ÃÂ¤ÃÂ±ÃÂ¨ÃÂ´ÃÂ¥' };
}

// ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¸ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¯ÃÂ¼ÃÂÃÂ¥ÃÂ¸ÃÂ¦ÃÂ©ÃÂÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¯ÃÂ¼ÃÂ
async function apiCreateTask(token: string, taskData: {
  title: string;
  description: string;
  category: number;
  unit_price: number;
  total_count: number;
  deadline?: string;
}, retries = 3) {
  const context = await request.newContext();
  for (let i = 0; i < retries; i++) {
    const response = await context.post(`${API_BASE}/business/tasks`, {
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      data: taskData
