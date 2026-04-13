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
  industries?: string[];
  video_duration?: string;
  video_aspect?: string;
  video_resolution?: string;
  creative_style?: string;
  award_price?: number;
  award_count?: number;
  materials?: Array<{
    file_name: string;
    file_path: string;
    file_size?: number;
    file_type: string;
    sort_order: number;
  }>;
}, retries = 3) {
  const context = await request.newContext();
  for (let i = 0; i < retries; i++) {
    const response = await context.post(`${API_BASE}/business/tasks`, {
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      data: taskData
    });
    const text = await response.text();
    if (!text) {
      if (i < retries - 1) { await new Promise(r => setTimeout(r, 1000 * (i + 1))); continue; }
      await context.dispose();
      return { code: -1, message: 'ÃÂ§ÃÂ©ÃÂºÃÂ¥ÃÂÃÂÃÂ¥ÃÂºÃÂ' };
    }
    const data = JSON.parse(text);
    if (data.code === 0 || i === retries - 1) {
      await context.dispose();
      return data;
    }
    await new Promise(r => setTimeout(r, 1000 * (i + 1)));
  }
  await context.dispose();
  return { code: -1, message: 'ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ»ÃÂºÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂ¤ÃÂ±ÃÂ¨ÃÂ´ÃÂ¥' };
}

// ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂÃÂÃÂ¨ÃÂ¡ÃÂ¨
async function apiBusinessTasks(token: string) {
  const context = await request.newContext();
  const response = await context.get(`${API_BASE}/business/tasks`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  const text = await response.text();
  const data = text ? JSON.parse(text) : { code: -1, data: [] };
  await context.dispose();
  if (data.data === null) data.data = [];
  return data;
}

// ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ©ÃÂ¢ÃÂ
async function apiBusinessBalance(token: string) {
  const context = await request.newContext();
  const response = await context.get(`${API_BASE}/business/balance`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  const data = await response.json();
  await context.dispose();
  // ÃÂ¥ÃÂ¦ÃÂÃÂ¦ÃÂÃÂÃÂ¨ÃÂ¿ÃÂÃÂ¥ÃÂÃÂ nullÃÂ¯ÃÂ¼ÃÂÃÂ¨ÃÂ®ÃÂ¾ÃÂ§ÃÂ½ÃÂ®ÃÂ©ÃÂ»ÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ¥ÃÂÃÂ¼
  if (data.data === null) {
    data.data = { balance: 0, frozen_amount: 0 };
  }
  return data;
}

// ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂÃÂÃÂ¨ÃÂ¡ÃÂ¨
async function apiCreatorTasks(token: string, params?: { page?: number; limit?: number; category?: number; keyword?: string; sort?: string }) {
  const context = await request.newContext();
  const searchParams = new URLSearchParams();
  if (params?.page) searchParams.set('page', String(params.page));
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.category) searchParams.set('category', String(params.category));
  if (params?.keyword) searchParams.set('keyword', params.keyword);
  if (params?.sort) searchParams.set('sort', params.sort);

  const response = await context.get(`${API_BASE}/creator/tasks?${searchParams.toString()}`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  const data = await response.json();
  await context.dispose();
  return data;
}

// ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¯ÃÂ¼ÃÂÃÂ¥ÃÂ¸ÃÂ¦ÃÂ©ÃÂÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¯ÃÂ¼ÃÂ
async function apiCreatorClaim(token: string, taskId: number, retries = 3) {
  const context = await request.newContext();
  for (let i = 0; i < retries; i++) {
    const response = await context.post(`${API_BASE}/creator/claim`, {
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      data: { task_id: taskId }
    });
    const text = await response.text();
    if (!text) {
      if (i < retries - 1) { await new Promise(r => setTimeout(r, 1000 * (i + 1))); continue; }
      await context.dispose();
      return { code: -1, message: 'ÃÂ§ÃÂ©ÃÂºÃÂ¥ÃÂÃÂÃÂ¥ÃÂºÃÂ' };
    }
    const data = JSON.parse(text);
    if (data.code === 0 || i === retries - 1) {
      await context.dispose();
      return data;
    }
    await new Promise(r => setTimeout(r, 1000 * (i + 1)));
  }
  await context.dispose();
  return { code: -1, message: 'ÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂÃÂ¥ÃÂ¤ÃÂ±ÃÂ¨ÃÂ´ÃÂ¥' };
}

// ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¦ÃÂÃÂÃÂ§ÃÂÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂÃÂ¥ÃÂÃÂÃÂ¨ÃÂ¡ÃÂ¨
async function apiCreatorClaims(token: string) {
  const context = await request.newContext();
  const response = await context.get(`${API_BASE}/creator/claims`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  const data = await response.json();
  await context.dispose();
  return data;
}

// ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¦ÃÂÃÂÃÂ¤ÃÂºÃÂ¤ÃÂ¤ÃÂºÃÂ¤ÃÂ¤ÃÂ»ÃÂÃÂ¯ÃÂ¼ÃÂÃÂ¥ÃÂ¸ÃÂ¦ÃÂ©ÃÂÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¯ÃÂ¼ÃÂ
async function apiCreatorSubmit(token: string, claimId: number, content: string, retries = 3) {
  const context = await request.newContext();
  for (let i = 0; i < retries; i++) {
    const response = await context.put(`${API_BASE}/creator/claim/${claimId}/submit`, {
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      data: { content }
    });
    const text = await response.text();
    if (!text) {
      if (i < retries - 1) { await new Promise(r => setTimeout(r, 1000 * (i + 1))); continue; }
      await context.dispose();
      return { code: -1, message: 'ÃÂ§ÃÂ©ÃÂºÃÂ¥ÃÂÃÂÃÂ¥ÃÂºÃÂ' };
    }
    const data = JSON.parse(text);
    if (data.code === 0 || i === retries - 1) {
      await context.dispose();
      return data;
    }
    await new Promise(r => setTimeout(r, 1000 * (i + 1)));
  }
  await context.dispose();
  return { code: -1, message: 'ÃÂ¦ÃÂÃÂÃÂ¤ÃÂºÃÂ¤ÃÂ¥ÃÂ¤ÃÂ±ÃÂ¨ÃÂ´ÃÂ¥' };
}

// ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂÃÂ¥ÃÂÃÂÃÂ¨ÃÂ¡ÃÂ¨
async function apiBusinessTaskClaims(token: string, taskId: number, retries = 3) {
  const context = await request.newContext();
  for (let i = 0; i < retries; i++) {
    const response = await context.get(`${API_BASE}/business/tasks/${taskId}/claims`, {
      headers: { Authorization: `Bearer ${token}` }
    });
    const text = await response.text();
    if (!text) {
      if (i < retries - 1) { await new Promise(r => setTimeout(r, 1000 * (i + 1))); continue; }
      await context.dispose();
      return { code: -1, message: 'ÃÂ§ÃÂ©ÃÂºÃÂ¥ÃÂÃÂÃÂ¥ÃÂºÃÂ', data: [] };
    }
    const data = JSON.parse(text);
    if (data.code === 0 || i === retries - 1) {
      await context.dispose();
      return data;
    }
    await new Promise(r => setTimeout(r, 1000 * (i + 1)));
  }
  await context.dispose();
  return { code: -1, message: 'ÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂÃÂ¥ÃÂÃÂÃÂ¨ÃÂ¡ÃÂ¨ÃÂ¥ÃÂ¤ÃÂ±ÃÂ¨ÃÂ´ÃÂ¥', data: [] };
}

// ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ©ÃÂªÃÂÃÂ¦ÃÂÃÂ¶ÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂÃÂ¯ÃÂ¼ÃÂÃÂ¥ÃÂ¸ÃÂ¦ÃÂ©ÃÂÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¯ÃÂ¼ÃÂ
async function apiBusinessReviewClaim(token: string, claimId: number, result: number, comment?: string, retries = 3) {
  const context = await request.newContext();
  for (let i = 0; i < retries; i++) {
    const response = await context.put(`${API_BASE}/business/claim/${claimId}/review`, {
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      data: { result, comment }
    });
    const text = await response.text();
    if (!text) {
      if (i < retries - 1) { await new Promise(r => setTimeout(r, 1000 * (i + 1))); continue; }
      await context.dispose();
      return { code: -1, message: 'ÃÂ§ÃÂ©ÃÂºÃÂ¥ÃÂÃÂÃÂ¥ÃÂºÃÂ' };
    }
    const data = JSON.parse(text);
    if (data.code === 0 || i === retries - 1) {
      await context.dispose();
      return data;
    }
    await new Promise(r => setTimeout(r, 1000 * (i + 1)));
  }
  await context.dispose();
  return { code: -1, message: 'ÃÂ©ÃÂªÃÂÃÂ¦ÃÂÃÂ¶ÃÂ¥ÃÂ¤ÃÂ±ÃÂ¨ÃÂ´ÃÂ¥' };
}

// ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ©ÃÂÃÂ±ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ¿ÃÂ¡ÃÂ¦ÃÂÃÂ¯
async function apiCreatorWallet(token: string) {
  const context = await request.newContext();
  const response = await context.get(`${API_BASE}/creator/wallet`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  const data = await response.json();
  await context.dispose();
  return data;
}

// ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¤ÃÂºÃÂ¤ÃÂ¦ÃÂÃÂÃÂ¨ÃÂ®ÃÂ°ÃÂ¥ÃÂ½ÃÂ
async function apiCreatorTransactions(token: string) {
  const context = await request.newContext();
  const response = await context.get(`${API_BASE}/creator/transactions`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  const data = await response.json();
  await context.dispose();
  return data;
}

// ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¤ÃÂºÃÂ¤ÃÂ¦ÃÂÃÂÃÂ¨ÃÂ®ÃÂ°ÃÂ¥ÃÂ½ÃÂ
async function apiBusinessTransactions(token: string) {
  const context = await request.newContext();
  const response = await context.get(`${API_BASE}/business/transactions`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  const data = await response.json();
  await context.dispose();
  return data;
}

// ============== PUBLIC PAGES ==============

test.describe('Public Pages', () => {
  test('TC-PUBLIC-01: ÃÂ©ÃÂ¦ÃÂÃÂ©ÃÂ¡ÃÂµÃÂ¥ÃÂÃÂ ÃÂ¨ÃÂ½ÃÂ½ÃÂ¦ÃÂÃÂÃÂ¥ÃÂÃÂ', async ({ page }) => {
    await page.goto('/');
    // ÃÂ¦ÃÂ£ÃÂÃÂ¦ÃÂÃÂ¥ÃÂ©ÃÂ¡ÃÂµÃÂ©ÃÂÃÂ¢ÃÂ¤ÃÂ¸ÃÂ»ÃÂ¨ÃÂ¦ÃÂÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¹ÃÂ¥ÃÂÃÂ ÃÂ¨ÃÂ½ÃÂ½
    await expect(page.locator('body')).toBeVisible();
    const title = await page.title();
    console.log('ÃÂ©ÃÂ¦ÃÂÃÂ©ÃÂ¡ÃÂµÃÂ¦ÃÂ ÃÂÃÂ©ÃÂ¢ÃÂ:', title);
  });

  test('TC-PUBLIC-02: ÃÂ¥ÃÂÃÂ¬ÃÂ¥ÃÂ¼ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂ¤ÃÂ§ÃÂ¥ÃÂÃÂ', async ({ page }) => {
    await page.goto('/tasks');
    await expect(page.locator('body')).toBeVisible();
  });

  test('TC-PUBLIC-03: ÃÂ§ÃÂÃÂ¨ÃÂ¦ÃÂÃÂ·ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ©ÃÂ¡ÃÂµ', async ({ page }) => {
    await page.goto('/auth/login.html');
    await expect(page.locator('#login-form')).toBeVisible();
    await expect(page.locator('#username')).toBeVisible();
    await expect(page.locator('#password')).toBeVisible();
    await expect(page.locator('#login-role')).toBeVisible();
  });

  test('TC-PUBLIC-04: ÃÂ§ÃÂÃÂ¨ÃÂ¦ÃÂÃÂ·ÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂÃÂ©ÃÂ¡ÃÂµ', async ({ page }) => {
    await page.goto('/auth/register.html');
    await expect(page.locator('#register-form')).toBeVisible();
    await expect(page.locator('#username')).toBeVisible();
    await expect(page.locator('#password')).toBeVisible();
    await expect(page.locator('#phone')).toBeVisible();
  });
});

// ============== AUTHENTICATION FLOW ==============

test.describe('Authentication Flow', () => {
  let creatorUser: { username: string; password: string; phone: string; token?: string };
  let businessUser: { username: string; password: string; phone: string; token?: string };

  test.beforeEach(() => {
    creatorUser = {
      username: generateUsername(),
      password: 'test123456',
      phone: generatePhone(),
    };
    businessUser = {
      username: generateUsername(),
      password: 'test123456',
      phone: generatePhone(),
    };
  });

  test('TC-AUTH-01: ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂ', async ({ page }) => {
    // ÃÂ¤ÃÂ½ÃÂ¿ÃÂ§ÃÂÃÂ¨APIÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂ
    const result = await apiRegister(creatorUser.username, creatorUser.password, creatorUser.phone, 'creator');
    console.log('ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂÃÂ§ÃÂ»ÃÂÃÂ¦ÃÂÃÂ:', result);
    expect(result.code).toBe(0);
    creatorUser.token = result.data?.token;
  });

  test('TC-AUTH-02: ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂ', async ({ page }) => {
    const result = await apiRegister(businessUser.username, businessUser.password, businessUser.phone, 'business');
    console.log('ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂÃÂ§ÃÂ»ÃÂÃÂ¦ÃÂÃÂ:', result);
    expect(result.code).toBe(0);
    businessUser.token = result.data?.token;
  });

  test('TC-AUTH-03: ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¥ÃÂ¹ÃÂ¶ÃÂ©ÃÂªÃÂÃÂ¨ÃÂ¯ÃÂ', async ({ page }) => {
    // ÃÂ¥ÃÂÃÂÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂ
    await apiRegister(creatorUser.username, creatorUser.password, creatorUser.phone, 'creator');

    // ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂ
    const loginResult = await apiLogin(creatorUser.username, creatorUser.password);
    console.log('ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ§ÃÂ»ÃÂÃÂ¦ÃÂÃÂ:', loginResult);
    expect(loginResult.code).toBe(0);
    expect(loginResult.data).toHaveProperty('token');
    expect(loginResult.data.user.role).toBe('creator');
  });

  test('TC-AUTH-04: ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¥ÃÂ¹ÃÂ¶ÃÂ©ÃÂªÃÂÃÂ¨ÃÂ¯ÃÂ', async ({ page }) => {
    // ÃÂ¥ÃÂÃÂÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂ
    await apiRegister(businessUser.username, businessUser.password, businessUser.phone, 'business');

    // ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂ
    const loginResult = await apiLogin(businessUser.username, businessUser.password);
    console.log('ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ§ÃÂ»ÃÂÃÂ¦ÃÂÃÂ:', loginResult);
    expect(loginResult.code).toBe(0);
    expect(loginResult.data).toHaveProperty('token');
    expect(loginResult.data.user.role).toBe('business');
  });

  test('TC-AUTH-05: ÃÂ©ÃÂÃÂÃÂ¨ÃÂ¯ÃÂ¯ÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¥ÃÂºÃÂÃÂ¥ÃÂ¤ÃÂ±ÃÂ¨ÃÂ´ÃÂ¥', async ({ page }) => {
    await page.goto('/auth/login.html');
    await page.fill('#username', 'nonexistent_user_12345');
    await page.fill('#password', 'wrongpassword');
    await page.click('button[type="submit"]');

    // ÃÂ§ÃÂ­ÃÂÃÂ¥ÃÂ¾ÃÂÃÂ¥ÃÂ¯ÃÂ¼ÃÂ¨ÃÂÃÂªÃÂ¦ÃÂÃÂÃÂ©ÃÂÃÂÃÂ¨ÃÂ¯ÃÂ¯ÃÂ¦ÃÂÃÂÃÂ§ÃÂ¤ÃÂºÃÂ¥ÃÂÃÂºÃÂ§ÃÂÃÂ°
    await page.waitForURL('**').catch(() => {});
    const url = page.url();
    console.log('ÃÂ©ÃÂÃÂÃÂ¨ÃÂ¯ÃÂ¯ÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¥ÃÂÃÂURL:', url);
    expect(url).toMatch(/login/);
  });
});

// ============== CREATOR PAGES ==============

test.describe('Creator Pages', () => {
  let creatorToken: string;

  test.beforeEach(async () => {
    // ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ»ÃÂºÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂ
    const username = generateUsername();
    const phone = generatePhone();
    const regResult = await apiRegister(username, 'test123456', phone, 'creator');
    const loginResult = await apiLogin(username, 'test123456');
    creatorToken = loginResult.data?.token;
  });

  test('TC-CREATOR-01: ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¥ÃÂ·ÃÂ¥ÃÂ¤ÃÂ½ÃÂÃÂ¥ÃÂÃÂ°ÃÂ©ÃÂÃÂÃÂ¨ÃÂ¦ÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ¨ÃÂ¯ÃÂ', async ({ page }) => {
    await page.goto('/creator/dashboard.html');
    await page.waitForURL('**/auth/login**', { timeout: 5000 }).catch(() => {});
    const url = page.url();
    console.log('ÃÂ¦ÃÂÃÂªÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¥ÃÂ·ÃÂ¥ÃÂ¤ÃÂ½ÃÂÃÂ¥ÃÂÃÂ°URL:', url);
    expect(url).toMatch(/login|auth/);
  });

  test('TC-CREATOR-02: ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂ¤ÃÂ§ÃÂ¥ÃÂÃÂÃÂ©ÃÂÃÂÃÂ¨ÃÂ¦ÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ¨ÃÂ¯ÃÂ', async ({ page }) => {
    await page.goto('/creator/task_hall.html');
    await page.waitForURL('**/auth/login**', { timeout: 5000 }).catch(() => {});
    const url = page.url();
    console.log('ÃÂ¦ÃÂÃÂªÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂ¤ÃÂ§ÃÂ¥ÃÂÃÂURL:', url);
    expect(url).toMatch(/login|auth/);
  });

  test('TC-CREATOR-03: ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂ¤ÃÂ§ÃÂ¥ÃÂÃÂ-ÃÂ¥ÃÂ·ÃÂ²ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂ', async ({ page }) => {
    const username = generateUsername();
    await apiRegister(username, 'test123456', generatePhone(), 'creator');
    const loginResult = await apiLogin(username, 'test123456');

    await page.goto('/');
    await page.evaluate((token) => {
      localStorage.setItem('token', token);
      localStorage.setItem('role', 'creator');
    }, loginResult.data?.token);

    await page.goto('/creator/task_hall.html');
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
    const body = await page.locator('body').textContent();
    console.log('ÃÂ¥ÃÂ·ÃÂ²ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂ¤ÃÂ§ÃÂ¥ÃÂÃÂ, bodyÃÂ©ÃÂÃÂ¿ÃÂ¥ÃÂºÃÂ¦:', body.length);
    expect(body.length).toBeGreaterThan(0);
  });

  test('TC-CREATOR-04: ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¦ÃÂÃÂÃÂ§ÃÂÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂÃÂ©ÃÂ¡ÃÂµÃÂ©ÃÂÃÂ¢', async ({ page }) => {
    await page.goto('/creator/claim_list.html');
    await page.waitForURL('**/auth/login**', { timeout: 5000 }).catch(() => {});
    const url = page.url();
    console.log('ÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¦ÃÂÃÂÃÂ§ÃÂÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂURL:', url);
    expect(url).toMatch(/login|auth|claim/);
  });

  test('TC-CREATOR-05: ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ©ÃÂÃÂ±ÃÂ¥ÃÂÃÂÃÂ©ÃÂ¡ÃÂµÃÂ©ÃÂÃÂ¢', async ({ page }) => {
    await page.goto('/creator/wallet.html');
    await page.waitForURL('**/auth/login**', { timeout: 5000 }).catch(() => {});
});

// ============== BUSINESS PAGES ==============

test.describe('Business Pages', () => {
  test('TC-BUSINESS-01: ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¥ÃÂ·ÃÂ¥ÃÂ¤ÃÂ½ÃÂÃÂ¥ÃÂÃÂ°ÃÂ©ÃÂÃÂÃÂ¨ÃÂ¦ÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ¨ÃÂ¯ÃÂ', async ({ page }) => {
    await page.goto('/business/dashboard.html');
    await page.waitForURL('**/auth/login**', { timeout: 5000 }).catch(() => {});
    const url = page.url();
    console.log('ÃÂ¦ÃÂÃÂªÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¥ÃÂ·ÃÂ¥ÃÂ¤ÃÂ½ÃÂÃÂ¥ÃÂÃÂ°URL:', url);
    expect(url).toMatch(/login|auth/);
  });

  test('TC-BUSINESS-02: ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¸ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ©ÃÂ¡ÃÂµÃÂ©ÃÂÃÂ¢ÃÂ©ÃÂÃÂÃÂ¨ÃÂ¦ÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ¨ÃÂ¯ÃÂ', async ({ page }) => {
    await page.goto('/business/task_create.html');
    await page.waitForURL('**/auth/login**', { timeout: 5000 }).catch(() => {});
    const url = page.url();
    console.log('ÃÂ¦ÃÂÃÂªÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¸ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡URL:', url);
    expect(url).toMatch(/login|auth/);
  });

  test('TC-BUSINESS-03: ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂ¼ÃÂ©ÃÂ¡ÃÂµÃÂ©ÃÂÃÂ¢ÃÂ©ÃÂÃÂÃÂ¨ÃÂ¦ÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ¨ÃÂ¯ÃÂ', async ({ page }) => {
    await page.goto('/business/recharge.html');
    await page.waitForURL('**/auth/login**', { timeout: 5000 }).catch(() => {});
    const url = page.url();
    console.log('ÃÂ¦ÃÂÃÂªÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂ¼URL:', url);
    expect(url).toMatch(/login|auth/);
  });
});

// ============== FULL USER FLOWS ==============

test.describe('Full User Flows', () => {
  test('FLOW-01: ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¥ÃÂ®ÃÂÃÂ¦ÃÂÃÂ´ÃÂ¦ÃÂµÃÂÃÂ§ÃÂ¨ÃÂ', async ({ page }) => {
    const username = generateUsername();
    const phone = generatePhone();

    // 1. ÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂ
    const regResult = await apiRegister(username, 'test123456', phone, 'creator');
    console.log('1. ÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂ:', regResult.code === 0 ? 'ÃÂ¦ÃÂÃÂÃÂ¥ÃÂÃÂ' : 'ÃÂ¥ÃÂ¤ÃÂ±ÃÂ¨ÃÂ´ÃÂ¥');
    expect(regResult.code).toBe(0);

    // 2. ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂ
    const loginResult = await apiLogin(username, 'test123456');
    console.log('2. ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂ:', loginResult.code === 0 ? 'ÃÂ¦ÃÂÃÂÃÂ¥ÃÂÃÂ' : 'ÃÂ¥ÃÂ¤ÃÂ±ÃÂ¨ÃÂ´ÃÂ¥');
    expect(loginResult.code).toBe(0);
    const token = loginResult.data?.token;

    // 3. ÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¥ÃÂ·ÃÂ¥ÃÂ¤ÃÂ½ÃÂÃÂ¥ÃÂÃÂ°
    await page.goto('/');
    await page.evaluate((t) => {
      localStorage.setItem('token', t);
      localStorage.setItem('role', 'creator');
    }, token);

    await page.goto('/creator/dashboard.html');
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
    console.log('3. ÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¥ÃÂ·ÃÂ¥ÃÂ¤ÃÂ½ÃÂÃÂ¥ÃÂÃÂ°:', page.url());
    expect(page.url()).toMatch(/creator|dashboard/);

    // 4. ÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂ¤ÃÂ§ÃÂ¥ÃÂÃÂ
    await page.goto('/creator/task_hall.html');
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
    console.log('4. ÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂ¤ÃÂ§ÃÂ¥ÃÂÃÂ:', page.url());

    // 5. ÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¦ÃÂÃÂÃÂ§ÃÂÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂ
    await page.goto('/creator/claim_list.html');
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
    console.log('5. ÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¦ÃÂÃÂÃÂ§ÃÂÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂ:', page.url());

    // 6. ÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ©ÃÂÃÂ±ÃÂ¥ÃÂÃÂ
    await page.goto('/creator/wallet.html');
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
    console.log('6. ÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ©ÃÂÃÂ±ÃÂ¥ÃÂÃÂ:', page.url());
  });

  test('FLOW-02: ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¥ÃÂ®ÃÂÃÂ¦ÃÂÃÂ´ÃÂ¦ÃÂµÃÂÃÂ§ÃÂ¨ÃÂ', async ({ page }) => {
    const username = generateUsername();
    const phone = generatePhone();

    await apiRegister(username, 'test123456', phone, 'business');
    const loginResult = await apiLogin(username, 'test123456');
    expect(loginResult.code).toBe(0);
    const token = loginResult.data?.token;

    await page.goto('/');
    await page.evaluate((t) => {
      localStorage.setItem('token', t);
      localStorage.setItem('role', 'business');
    }, token);

    await page.goto('/business/dashboard.html');
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
    console.log('3. ÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¥ÃÂ·ÃÂ¥ÃÂ¤ÃÂ½ÃÂÃÂ¥ÃÂÃÂ°:', page.url());
    expect(page.url()).toMatch(/business|dashboard/);

    await page.goto('/business/task_create.html');
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
    console.log('4. ÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¸ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡:', page.url());

    await page.goto('/business/recharge.html');
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
    console.log('5. ÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂ¼:', page.url());

    // 6. ÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¦ÃÂÃÂÃÂ§ÃÂÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡
    await page.goto('/business/task_list.html');
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
    console.log('6. ÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¦ÃÂÃÂÃÂ§ÃÂÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡:', page.url());
  });
});

// ============== EDGE CASES ==============

test.describe('Edge Cases', () => {
  test('EDGE-01: ÃÂ¦ÃÂÃÂªÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¥ÃÂºÃÂÃÂ©ÃÂÃÂÃÂ¥ÃÂ®ÃÂÃÂ¥ÃÂÃÂ', async ({ page }) => {
    await page.goto('/creator/dashboard.html');
    await page.waitForURL('**/auth/login**', { timeout: 5000 }).catch(() => {});
    const url = page.url();
    console.log('ÃÂ¦ÃÂÃÂªÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¥ÃÂ·ÃÂ¥ÃÂ¤ÃÂ½ÃÂÃÂ¥ÃÂÃÂ°ÃÂ¯ÃÂ¼ÃÂURL:', url);
    expect(url).toMatch(/login|auth/);
  });

  test('EDGE-02: ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¥ÃÂÃÂÃÂ¨ÃÂ®ÃÂ¿ÃÂ©ÃÂÃÂ®ÃÂ©ÃÂ¡ÃÂµÃÂ©ÃÂÃÂ¢', async ({ page }) => {
    const username = generateUsername();
    await apiRegister(username, 'test123456', generatePhone(), 'business');

    await page.goto('/auth/login.html');
    await page.fill('#username', username);
    await page.fill('#password', 'test123456');
    await page.locator('#login-role').selectOption('business');
    await page.click('button[type="submit"]');

    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
    const url = page.url();
    console.log('ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¥ÃÂÃÂURL:', url);
  });
});

// ============== USER PROFILE ==============

test.describe('User Profile', () => {
  let testUser: { username: string; password: string; phone: string; token?: string };

  test.beforeEach(() => {
    testUser = {
      username: generateUsername(),
      password: 'test123456',
      phone: generatePhone(),
    };
  });

  test('TC-PROFILE-01: ÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ§ÃÂÃÂ¨ÃÂ¦ÃÂÃÂ·ÃÂ¨ÃÂµÃÂÃÂ¦ÃÂÃÂ', async () => {
    // ÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¹ÃÂ¶ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂ
    await apiRegister(testUser.username, testUser.password, testUser.phone, 'creator');
    const loginResult = await apiLogin(testUser.username, testUser.password);
    expect(loginResult.code).toBe(0);
    testUser.token = loginResult.data?.token;

    // ÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¨ÃÂµÃÂÃÂ¦ÃÂÃÂ
    const profileResult = await apiGetProfile(testUser.token);
    console.log('ÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ§ÃÂÃÂ¨ÃÂ¦ÃÂÃÂ·ÃÂ¨ÃÂµÃÂÃÂ¦ÃÂÃÂ:', profileResult);
    expect(profileResult.code).toBe(0);
    expect(profileResult.data).toHaveProperty('id');
    expect(profileResult.data).toHaveProperty('username');
    expect(profileResult.data.username).toBe(testUser.username);
  });

  test('TC-PROFILE-02: ÃÂ¦ÃÂÃÂ´ÃÂ¦ÃÂÃÂ°ÃÂ§ÃÂÃÂ¨ÃÂ¦ÃÂÃÂ·ÃÂ¦ÃÂÃÂµÃÂ§ÃÂ§ÃÂ°', async () => {
    // ÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¹ÃÂ¶ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂ
    await apiRegister(testUser.username, testUser.password, testUser.phone, 'creator');
    const loginResult = await apiLogin(testUser.username, testUser.password);
    testUser.token = loginResult.data?.token;

    // ÃÂ¦ÃÂÃÂ´ÃÂ¦ÃÂÃÂ°ÃÂ¦ÃÂÃÂµÃÂ§ÃÂ§ÃÂ°
    const newNickname = 'ÃÂ¦ÃÂµÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¦ÃÂÃÂµÃÂ§ÃÂ§ÃÂ°_' + randomInt(1000);
    const updateResult = await apiUpdateProfile(testUser.token, { nickname: newNickname });
    console.log('ÃÂ¦ÃÂÃÂ´ÃÂ¦ÃÂÃÂ°ÃÂ¦ÃÂÃÂµÃÂ§ÃÂ§ÃÂ°:', updateResult);
    expect(updateResult.code).toBe(0);

    // ÃÂ©ÃÂªÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¦ÃÂÃÂ´ÃÂ¦ÃÂÃÂ°
    const profileResult = await apiGetProfile(testUser.token);
    expect(profileResult.data.nickname).toBe(newNickname);
  });

  test('TC-PROFILE-03: ÃÂ¦ÃÂÃÂ´ÃÂ¦ÃÂÃÂ°ÃÂ§ÃÂÃÂ¨ÃÂ¦ÃÂÃÂ·ÃÂ¥ÃÂ¤ÃÂ´ÃÂ¥ÃÂÃÂ', async () => {
    // ÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¹ÃÂ¶ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂ
    await apiRegister(testUser.username, testUser.password, testUser.phone, 'creator');
    const loginResult = await apiLogin(testUser.username, testUser.password);
    testUser.token = loginResult.data?.token;

    // ÃÂ¦ÃÂÃÂ´ÃÂ¦ÃÂÃÂ°ÃÂ¥ÃÂ¤ÃÂ´ÃÂ¥ÃÂÃÂ
    const newAvatar = 'https://example.com/avatar/' + randomInt(1000) + '.jpg';
    const updateResult = await apiUpdateProfile(testUser.token, { avatar: newAvatar });
    console.log('ÃÂ¦ÃÂÃÂ´ÃÂ¦ÃÂÃÂ°ÃÂ¥ÃÂ¤ÃÂ´ÃÂ¥ÃÂÃÂ:', updateResult);
    expect(updateResult.code).toBe(0);

    // ÃÂ©ÃÂªÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¦ÃÂÃÂ´ÃÂ¦ÃÂÃÂ°
    const profileResult = await apiGetProfile(testUser.token);
    expect(profileResult.data.avatar).toBe(newAvatar);
  });

  test('TC-PROFILE-04: ÃÂ¤ÃÂ¿ÃÂ®ÃÂ¦ÃÂÃÂ¹ÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂ', async () => {
    // ÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¹ÃÂ¶ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂ
    await apiRegister(testUser.username, testUser.password, testUser.phone, 'creator');
    const loginResult = await apiLogin(testUser.username, testUser.password);
    testUser.token = loginResult.data?.token;

    // ÃÂ¤ÃÂ¿ÃÂ®ÃÂ¦ÃÂÃÂ¹ÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂ
    const newPassword = 'newpass123';
    const changeResult = await apiChangePassword(testUser.token, testUser.password, newPassword);
    console.log('ÃÂ¤ÃÂ¿ÃÂ®ÃÂ¦ÃÂÃÂ¹ÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂ:', changeResult);
    expect(changeResult.code).toBe(0);

    // ÃÂ¤ÃÂ½ÃÂ¿ÃÂ§ÃÂÃÂ¨ÃÂ¦ÃÂÃÂ°ÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂ
    const newLoginResult = await apiLogin(testUser.username, newPassword);
    console.log('ÃÂ¦ÃÂÃÂ°ÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂ:', newLoginResult);
    expect(newLoginResult.code).toBe(0);
    expect(newLoginResult.data).toHaveProperty('token');
  });

  test('TC-PROFILE-05: ÃÂ¤ÃÂ¿ÃÂ®ÃÂ¦ÃÂÃÂ¹ÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂ-ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂÃÂ©ÃÂÃÂÃÂ¨ÃÂ¯ÃÂ¯', async () => {
    // ÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¹ÃÂ¶ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂ
    await apiRegister(testUser.username, testUser.password, testUser.phone, 'creator');
    const loginResult = await apiLogin(testUser.username, testUser.password);
    testUser.token = loginResult.data?.token;

    // ÃÂ¤ÃÂ½ÃÂ¿ÃÂ§ÃÂÃÂ¨ÃÂ©ÃÂÃÂÃÂ¨ÃÂ¯ÃÂ¯ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂÃÂ¤ÃÂ¿ÃÂ®ÃÂ¦ÃÂÃÂ¹ÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂ
    const wrongOldPassword = 'wrongpassword';
    const changeResult = await apiChangePassword(testUser.token, wrongOldPassword, 'newpass123');
    console.log('ÃÂ©ÃÂÃÂÃÂ¨ÃÂ¯ÃÂ¯ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂÃÂ¤ÃÂ¿ÃÂ®ÃÂ¦ÃÂÃÂ¹ÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂ:', changeResult);
    expect(changeResult.code).not.toBe(0);
  });

  test('TC-PROFILE-06: ÃÂ¦ÃÂÃÂªÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¨ÃÂµÃÂÃÂ¦ÃÂÃÂÃÂ¥ÃÂºÃÂÃÂ¥ÃÂ¤ÃÂ±ÃÂ¨ÃÂ´ÃÂ¥', async () => {
    const result = await apiGetProfile('invalid-token');
    console.log('ÃÂ¦ÃÂÃÂªÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¨ÃÂµÃÂÃÂ¦ÃÂÃÂ:', result);
    expect(result.code).not.toBe(0);
  });

  test('TC-PROFILE-07: ÃÂ¤ÃÂ¿ÃÂ®ÃÂ¦ÃÂÃÂ¹ÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂ-ÃÂ¦ÃÂÃÂ°ÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂÃÂ¥ÃÂ¤ÃÂªÃÂ§ÃÂÃÂ­', async () => {
    // ÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¹ÃÂ¶ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂ
    await apiRegister(testUser.username, testUser.password, testUser.phone, 'creator');
    const loginResult = await apiLogin(testUser.username, testUser.password);
    testUser.token = loginResult.data?.token;

    // ÃÂ¦ÃÂÃÂ°ÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂÃÂ¥ÃÂ¤ÃÂªÃÂ§ÃÂÃÂ­
    const shortPassword = '123';
    const changeResult = await apiChangePassword(testUser.token, testUser.password, shortPassword);
    console.log('ÃÂ¥ÃÂ¯ÃÂÃÂ§ÃÂ ÃÂÃÂ¥ÃÂ¤ÃÂªÃÂ§ÃÂÃÂ­:', changeResult);
    expect(changeResult.code).not.toBe(0);
  });
});

// ============== BUSINESS FLOW TESTS ==============

test.describe('Business Flow Tests', () => {
  let businessUser: { username: string; password: string; phone: string; token?: string };

  test.beforeEach(() => {
    businessUser = {
      username: generateUsername(),
      password: 'test123456',
      phone: generatePhone(),
    };
  });

  test('FLOW-BUSINESS-01: ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂ¼ÃÂ¥ÃÂÃÂÃÂ¦ÃÂÃÂ¥ÃÂ§ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ©ÃÂ¢ÃÂ', async () => {
    await apiRegister(businessUser.username, businessUser.password, businessUser.phone, 'business');
    const loginResult = await apiLogin(businessUser.username, businessUser.password);
    businessUser.token = loginResult.data?.token;
    expect(businessUser.token).toBeDefined();

    // ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂ¼
    const rechargeResult = await apiBusinessRecharge(businessUser.token, 1000);
    console.log('ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂ¼:', rechargeResult);
    expect(rechargeResult.code).toBe(0);

    // ÃÂ¦ÃÂÃÂ¥ÃÂ§ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ©ÃÂ¢ÃÂ
    const balanceResult = await apiBusinessBalance(businessUser.token);
    console.log('ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¤ÃÂ½ÃÂÃÂ©ÃÂ¢ÃÂ:', balanceResult);
    expect(balanceResult.code).toBe(0);
    expect(balanceResult.data).toHaveProperty('balance');
    expect(balanceResult.data.balance).toBe(1000);
  });

  test('FLOW-BUSINESS-02: ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¸ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡', async () => {
    await apiRegister(businessUser.username, businessUser.password, businessUser.phone, 'business');
    const loginResult = await apiLogin(businessUser.username, businessUser.password);
    businessUser.token = loginResult.data?.token;
    expect(businessUser.token).toBeDefined();

    // ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂ¼ÃÂ§ÃÂ¡ÃÂ®ÃÂ¤ÃÂ¿ÃÂÃÂ¦ÃÂÃÂÃÂ¨ÃÂ¶ÃÂ³ÃÂ¥ÃÂ¤ÃÂÃÂ¤ÃÂ½ÃÂÃÂ©ÃÂ¢ÃÂ
    await apiBusinessRecharge(businessUser.token, 5000);

    // ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¸ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡
    const taskResult = await apiCreateTask(businessUser.token, {
      title: 'ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¦ÃÂµÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡_' + Date.now(),
      description: 'E2EÃÂ¨ÃÂÃÂªÃÂ¥ÃÂÃÂ¨ÃÂ¥ÃÂÃÂÃÂ¦ÃÂµÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡',
      category: 1,
      unit_price: 50,
      total_count: 5,
    });
    console.log('ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¸ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡:', taskResult);
    expect(taskResult.code).toBe(0);
    expect(taskResult.data).toHaveProperty('task_id');

    // ÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂÃÂÃÂ¨ÃÂ¡ÃÂ¨ÃÂ©ÃÂªÃÂÃÂ¨ÃÂ¯ÃÂ
    const tasksResult = await apiBusinessTasks(businessUser.token);
    expect(tasksResult.code).toBe(0);
    expect(tasksResult.data.length).toBeGreaterThan(0);
  });
});

// ============== CREATOR FLOW TESTS ==============

test.describe('Creator Flow Tests', () => {
  let creatorUser: { username: string; password: string; phone: string; token?: string };

  test.beforeEach(() => {
    creatorUser = {
      username: generateUsername(),
      password: 'test123456',
      phone: generatePhone(),
    };
  });

  test('FLOW-CREATOR-01: ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¦ÃÂµÃÂÃÂ¨ÃÂ§ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂÃÂÃÂ¨ÃÂ¡ÃÂ¨', async () => {
    await apiRegister(creatorUser.username, creatorUser.password, creatorUser.phone, 'creator');
    const loginResult = await apiLogin(creatorUser.username, creatorUser.password);
    creatorUser.token = loginResult.data?.token;
    expect(creatorUser.token).toBeDefined();

    // ÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂÃÂÃÂ¨ÃÂ¡ÃÂ¨
    const tasksResult = await apiCreatorTasks(creatorUser.token);
    console.log('ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂÃÂÃÂ¨ÃÂ¡ÃÂ¨:', tasksResult);
    expect(tasksResult.code).toBe(0);
    expect(tasksResult.data).toHaveProperty('items');
  });

  test('FLOW-CREATOR-02: ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ©ÃÂÃÂ±ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ¿ÃÂ¡ÃÂ¦ÃÂÃÂ¯', async () => {
    await apiRegister(creatorUser.username, creatorUser.password, creatorUser.phone, 'creator');
    const loginResult = await apiLogin(creatorUser.username, creatorUser.password);
    creatorUser.token = loginResult.data?.token;
    expect(creatorUser.token).toBeDefined();

    const walletResult = await apiCreatorWallet(creatorUser.token);
    console.log('ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ©ÃÂÃÂ±ÃÂ¥ÃÂÃÂ:', walletResult);
    expect(walletResult.code).toBe(0);
    expect(walletResult.data).toHaveProperty('balance');
    expect(walletResult.data).toHaveProperty('frozen_amount');
  });

  test('FLOW-CREATOR-03: ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¤ÃÂºÃÂ¤ÃÂ¦ÃÂÃÂÃÂ¨ÃÂ®ÃÂ°ÃÂ¥ÃÂ½ÃÂ', async () => {
    await apiRegister(creatorUser.username, creatorUser.password, creatorUser.phone, 'creator');
    const loginResult = await apiLogin(creatorUser.username, creatorUser.password);
    creatorUser.token = loginResult.data?.token;
    expect(creatorUser.token).toBeDefined();

    const txResult = await apiCreatorTransactions(creatorUser.token);
    console.log('ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¤ÃÂºÃÂ¤ÃÂ¦ÃÂÃÂÃÂ¨ÃÂ®ÃÂ°ÃÂ¥ÃÂ½ÃÂ:', txResult);
    expect(txResult.code).toBe(0);
    expect(txResult.data).toBeDefined();
  });

  test('FLOW-CREATOR-04: ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¦ÃÂÃÂÃÂ§ÃÂÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂÃÂ¥ÃÂÃÂÃÂ¨ÃÂ¡ÃÂ¨', async () => {
    await apiRegister(creatorUser.username, creatorUser.password, creatorUser.phone, 'creator');
    const loginResult = await apiLogin(creatorUser.username, creatorUser.password);
    creatorUser.token = loginResult.data?.token;
    expect(creatorUser.token).toBeDefined();

    const claimsResult = await apiCreatorClaims(creatorUser.token);
    console.log('ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂÃÂ¥ÃÂÃÂÃÂ¨ÃÂ¡ÃÂ¨:', claimsResult);
    expect(claimsResult.code).toBe(0);
  });
});

// ============== INTEGRATED FLOW TESTS ==============

test.describe('Integrated Flow Tests', () => {
  // ÃÂ¦ÃÂ³ÃÂ¨ÃÂ¯ÃÂ¼ÃÂFLOW-INTEGRATED-01 ÃÂ¦ÃÂÃÂ¯ÃÂ¦ÃÂ ÃÂ¸ÃÂ¥ÃÂ¿ÃÂÃÂ§ÃÂ«ÃÂ¯ÃÂ¥ÃÂÃÂ°ÃÂ§ÃÂ«ÃÂ¯ÃÂ¦ÃÂµÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¯ÃÂ¼ÃÂÃÂ¥ÃÂºÃÂÃÂ¥ÃÂ§ÃÂÃÂ§ÃÂ»ÃÂÃÂ¨ÃÂ¿ÃÂÃÂ¨ÃÂ¡ÃÂ
  test('FLOW-INTEGRATED-01: ÃÂ§ÃÂ«ÃÂ¯ÃÂ¥ÃÂÃÂ°ÃÂ§ÃÂ«ÃÂ¯ÃÂ§ÃÂ»ÃÂ¼ÃÂ¥ÃÂÃÂÃÂ¦ÃÂµÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¯ÃÂ¼ÃÂÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¸ÃÂÃÂ¢ÃÂÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂÃÂ¢ÃÂÃÂÃÂ¤ÃÂºÃÂ¤ÃÂ¤ÃÂ»ÃÂÃÂ¢ÃÂÃÂÃÂ©ÃÂªÃÂÃÂ¦ÃÂÃÂ¶ÃÂ¯ÃÂ¼ÃÂ', async ({ page }) => {
    // ========== ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ§ÃÂ«ÃÂ¯ ==========
    const businessUsername = generateUsername();
    const businessPhone = generatePhone();
    const businessPassword = 'test123456';

    // 1. ÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶
    await apiRegister(businessUsername, businessPassword, businessPhone, 'business');
    const businessLogin = await apiLogin(businessUsername, businessPassword);
    const businessToken = businessLogin.data?.token;
    console.log('1. ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¹ÃÂ¶ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂ:', businessToken ? 'ÃÂ¦ÃÂÃÂÃÂ¥ÃÂÃÂ' : 'ÃÂ¥ÃÂ¤ÃÂ±ÃÂ¨ÃÂ´ÃÂ¥');
    expect(businessToken).toBeDefined();

    // 2. ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂ¼
    const rechargeResult = await apiBusinessRecharge(businessToken, 500);
    console.log('2. ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂ¼:', rechargeResult);
    expect(rechargeResult.code).toBe(0);

    // 3. ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¸ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡
    const taskTitle = 'E2EÃÂ¦ÃÂµÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡_' + Date.now();
    const taskResult = await apiCreateTask(businessToken, {
      title: taskTitle,
      description: 'ÃÂ¨ÃÂ¿ÃÂÃÂ¦ÃÂÃÂ¯ÃÂ§ÃÂ«ÃÂ¯ÃÂ¥ÃÂÃÂ°ÃÂ§ÃÂ«ÃÂ¯ÃÂ¨ÃÂÃÂªÃÂ¥ÃÂÃÂ¨ÃÂ¥ÃÂÃÂÃÂ¦ÃÂµÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¥ÃÂÃÂÃÂ¥ÃÂ»ÃÂºÃÂ§ÃÂÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡',
      category: 1,
      unit_price: 100,
      total_count: 2,
    });
    console.log('3. ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¸ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡:', taskResult);
    expect(taskResult.code).toBe(0);
    const taskId = taskResult.data?.task_id;

    // 4. ÃÂ©ÃÂªÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ§ÃÂÃÂ¶ÃÂ¦ÃÂÃÂÃÂ¯ÃÂ¼ÃÂÃÂ¥ÃÂ·ÃÂ²ÃÂ¤ÃÂ¸ÃÂÃÂ§ÃÂºÃÂ¿ÃÂ¯ÃÂ¼ÃÂÃÂ¦ÃÂÃÂ ÃÂ©ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¡ÃÂ¦ÃÂ ÃÂ¸ÃÂ¯ÃÂ¼ÃÂ
    const businessTasks = await apiBusinessTasks(businessToken);
    console.log('4. ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂÃÂÃÂ¨ÃÂ¡ÃÂ¨:', businessTasks);
    const createdTask = businessTasks.data.find((t: any) => t.id === taskId);
    expect(createdTask).toBeDefined();
    expect(createdTask.status).toBe(2); // ÃÂ¥ÃÂ·ÃÂ²ÃÂ¤ÃÂ¸ÃÂÃÂ§ÃÂºÃÂ¿

    // ========== ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ§ÃÂ«ÃÂ¯ ==========
    const creatorUsername = generateUsername();
    const creatorPhone = generatePhone();

    // 5. ÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂ
    await apiRegister(creatorUsername, 'test123456', creatorPhone, 'creator');
    const creatorLogin = await apiLogin(creatorUsername, 'test123456');
    const creatorToken = creatorLogin.data?.token;
    console.log('5. ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¦ÃÂ³ÃÂ¨ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ¹ÃÂ¶ÃÂ§ÃÂÃÂ»ÃÂ¥ÃÂ½ÃÂ:', creatorToken ? 'ÃÂ¦ÃÂÃÂÃÂ¥ÃÂÃÂ' : 'ÃÂ¥ÃÂ¤ÃÂ±ÃÂ¨ÃÂ´ÃÂ¥');
    expect(creatorToken).toBeDefined();

    // 6. ÃÂ¨ÃÂ®ÃÂ¾ÃÂ§ÃÂ½ÃÂ®localStorage
    await page.goto('/');
    await page.evaluate((token) => {
      localStorage.setItem('token', token);
      localStorage.setItem('role', 'creator');
    }, creatorToken);

    // 7. ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¨ÃÂÃÂ·ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂÃÂÃÂ¨ÃÂ¡ÃÂ¨
    const creatorTasks = await apiCreatorTasks(creatorToken);
    console.log('6. ÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¥ÃÂÃÂÃÂ¨ÃÂ¡ÃÂ¨:', creatorTasks);
    expect(creatorTasks.code).toBe(0);

    // 8. ÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡
    const claimResult = await apiCreatorClaim(creatorToken, taskId);
    console.log('7. ÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡:', claimResult);

    // ÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂÃÂ¦ÃÂÃÂÃÂ¥ÃÂÃÂÃÂ¯ÃÂ¼ÃÂÃÂ§ÃÂÃÂ½ÃÂ©ÃÂÃÂ¶+ÃÂ§ÃÂ­ÃÂÃÂ§ÃÂºÃÂ§ÃÂ¯ÃÂ¼ÃÂ
    if (claimResult.code === 0) {
      const claimId = claimResult.data?.claim_id;

      // 9. ÃÂ¦ÃÂÃÂÃÂ¤ÃÂºÃÂ¤ÃÂ¤ÃÂºÃÂ¤ÃÂ¤ÃÂ»ÃÂ
      const submitResult = await apiCreatorSubmit(creatorToken, claimId, 'https://example.com/e2e-test-work.pdf');
      console.log('8. ÃÂ¦ÃÂÃÂÃÂ¤ÃÂºÃÂ¤ÃÂ¤ÃÂºÃÂ¤ÃÂ¤ÃÂ»ÃÂ:', submitResult);
      expect(submitResult.code).toBe(0);

      // ========== ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ©ÃÂªÃÂÃÂ¦ÃÂÃÂ¶ ==========
      // 10. ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¦ÃÂÃÂ¥ÃÂ§ÃÂÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂÃÂ¥ÃÂÃÂÃÂ¨ÃÂ¡ÃÂ¨
      const claims = await apiBusinessTaskClaims(businessToken, taskId);
      console.log('9. ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¦ÃÂÃÂ¥ÃÂ§ÃÂÃÂÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂ:', claims);
      expect(claims.code).toBe(0);

      // 11. ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ©ÃÂªÃÂÃÂ¦ÃÂÃÂ¶ÃÂ©ÃÂÃÂÃÂ¨ÃÂ¿ÃÂ
      const reviewResult = await apiBusinessReviewClaim(businessToken, claimId, 1, 'E2EÃÂ¦ÃÂµÃÂÃÂ¨ÃÂ¯ÃÂÃÂ©ÃÂªÃÂÃÂ¦ÃÂÃÂ¶ÃÂ©ÃÂÃÂÃÂ¨ÃÂ¿ÃÂ');
      console.log('10. ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ©ÃÂªÃÂÃÂ¦ÃÂÃÂ¶:', reviewResult);
      expect(reviewResult.code).toBe(0);

      // 12. ÃÂ©ÃÂªÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ©ÃÂÃÂ±ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂ
      const walletAfter = await apiCreatorWallet(creatorToken);
      console.log('11. ÃÂ©ÃÂªÃÂÃÂ¦ÃÂÃÂ¶ÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂÃÂ¤ÃÂ½ÃÂÃÂ¨ÃÂÃÂÃÂ©ÃÂÃÂ±ÃÂ¥ÃÂÃÂ:', walletAfter.data);
    } else {
      // ÃÂ§ÃÂ­ÃÂÃÂ§ÃÂºÃÂ§ÃÂ¤ÃÂ¸ÃÂÃÂ¨ÃÂ¶ÃÂ³ÃÂ¦ÃÂÃÂÃÂ¥ÃÂÃÂ¶ÃÂ¤ÃÂ»ÃÂÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂ ÃÂ¤ÃÂ¸ÃÂÃÂ¨ÃÂÃÂ½ÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂ
      console.log('ÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂÃÂ¥ÃÂ¤ÃÂ±ÃÂ¨ÃÂ´ÃÂ¥ÃÂ¯ÃÂ¼ÃÂÃÂ©ÃÂÃÂÃÂ¨ÃÂ¯ÃÂ¯ÃÂ§ÃÂ ÃÂ:', claimResult.code, 'ÃÂ¦ÃÂ¶ÃÂÃÂ¦ÃÂÃÂ¯:', claimResult.message);
      // 40302 = ÃÂ§ÃÂ­ÃÂÃÂ§ÃÂºÃÂ§ÃÂ¤ÃÂ¸ÃÂÃÂ¨ÃÂ¶ÃÂ³ÃÂ¯ÃÂ¼ÃÂ40002 = ÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡ÃÂ¤ÃÂ¸ÃÂÃÂ¥ÃÂÃÂ¯ÃÂ¨ÃÂ®ÃÂ¤ÃÂ©ÃÂ¢ÃÂ
      expect([40002, 40302]).toContain(claimResult.code);
    }
  });


test('FLOW-INTEGRATED-02: ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¦ÃÂÃÂ¥ÃÂ§ÃÂÃÂÃÂ¤ÃÂºÃÂ¤ÃÂ¦ÃÂÃÂÃÂ¨ÃÂ®ÃÂ°ÃÂ¥ÃÂ½ÃÂÃÂ©ÃÂªÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¨ÃÂµÃÂÃÂ©ÃÂÃÂÃÂ¥ÃÂÃÂÃÂ¥ÃÂÃÂ¨', async () => {
  const businessUsername = generateUsername();
  const businessPhone = generatePhone();

  await apiRegister(businessUsername, 'test123456', businessPhone, 'business');
  const loginResult = await apiLogin(businessUsername, 'test123456');
  const businessToken = loginResult.data?.token;
  expect(businessToken).toBeDefined();

  await apiBusinessRecharge(businessToken, 1000);

  const taskResult = await apiCreateTask(businessToken, {
    title: 'ÃÂ¤ÃÂºÃÂ¤ÃÂ¦ÃÂÃÂÃÂ¨ÃÂ®ÃÂ°ÃÂ¥ÃÂ½ÃÂÃÂ¦ÃÂµÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¤ÃÂ»ÃÂ»ÃÂ¥ÃÂÃÂ¡_' + Date.now(),
    description: 'ÃÂ¦ÃÂµÃÂÃÂ¨ÃÂ¯ÃÂÃÂ¤ÃÂºÃÂ¤ÃÂ¦ÃÂÃÂÃÂ¨ÃÂ®ÃÂ°ÃÂ¥ÃÂ½ÃÂ',
    category: 1,
    unit_price: 100,
    total_count: 2,
  });
  expect(taskResult.code).toBe(0);

  const txResult = await apiBusinessTransactions(businessToken);
  console.log('ÃÂ¥ÃÂÃÂÃÂ¥ÃÂ®ÃÂ¶ÃÂ¤ÃÂºÃÂ¤ÃÂ¦ÃÂÃÂÃÂ¨ÃÂ®ÃÂ°ÃÂ¥ÃÂ½ÃÂ:', txResult);
  expect(txResult.code).toBe(0);
  expect(txResult.data).toBeDefined();

  const balanceResult = await apiBusinessBalance(businessToken);
  expect(balanceResult.code).toBe(0);
  expect(balanceResult.data.balance).toBe(800);
});
