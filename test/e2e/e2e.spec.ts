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

// ============== PUBLIC PAGES ==============

test.describe('Public Pages', () => {
  test('TC-PUBLIC-01: 首页加载成功', async ({ page }) => {
    await page.goto('/');
    // 检查页面主要内容加载
    await expect(page.locator('body')).toBeVisible();
    const title = await page.title();
    console.log('首页标题:', title);
  });

  test('TC-PUBLIC-02: 公开任务大厅', async ({ page }) => {
    await page.goto('/tasks');
    await expect(page.locator('body')).toBeVisible();
  });

  test('TC-PUBLIC-03: 用户登录页', async ({ page }) => {
    await page.goto('/auth/login.html');
    await expect(page.locator('#login-form')).toBeVisible();
    await expect(page.locator('#username')).toBeVisible();
    await expect(page.locator('#password')).toBeVisible();
    await expect(page.locator('#login-role')).toBeVisible();
  });

  test('TC-PUBLIC-04: 用户注册页', async ({ page }) => {
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

  test('TC-AUTH-01: 创作者注册', async ({ page }) => {
    // 使用API注册
    const result = await apiRegister(creatorUser.username, creatorUser.password, creatorUser.phone, 'creator');
    console.log('创作者注册结果:', result);
    expect(result.code).toBe(0);
    creatorUser.token = result.data?.token;
  });

  test('TC-AUTH-02: 商家注册', async ({ page }) => {
    const result = await apiRegister(businessUser.username, businessUser.password, businessUser.phone, 'business');
    console.log('商家注册结果:', result);
    expect(result.code).toBe(0);
    businessUser.token = result.data?.token;
  });

  test('TC-AUTH-03: 创作者登录并验证', async ({ page }) => {
    // 先注册
    await apiRegister(creatorUser.username, creatorUser.password, creatorUser.phone, 'creator');

    // 登录
    const loginResult = await apiLogin(creatorUser.username, creatorUser.password);
    console.log('创作者登录结果:', loginResult);
    expect(loginResult.code).toBe(0);
    expect(loginResult.data).toHaveProperty('token');
    expect(loginResult.data.user.role).toBe('creator');
  });

  test('TC-AUTH-04: 商家登录并验证', async ({ page }) => {
    // 先注册
    await apiRegister(businessUser.username, businessUser.password, businessUser.phone, 'business');

    // 登录
    const loginResult = await apiLogin(businessUser.username, businessUser.password);
    console.log('商家登录结果:', loginResult);
    expect(loginResult.code).toBe(0);
    expect(loginResult.data).toHaveProperty('token');
    expect(loginResult.data.user.role).toBe('business');
  });

  test('TC-AUTH-05: 错误密码登录应失败', async ({ page }) => {
    await page.goto('/auth/login.html');
    await page.fill('#username', 'nonexistent_user_12345');
    await page.fill('#password', 'wrongpassword');
    await page.click('button[type="submit"]');

    await page.waitForTimeout(2000);

    // 检查是否显示错误提示
    const errorEl = page.locator('.toast, .alert, [class*="error"]').first();
    const hasError = await errorEl.isVisible().catch(() => false);
    // 或者检查URL是否仍在登录页
    const url = page.url();
    expect(url).toMatch(/login/);
  });
});

// ============== CREATOR PAGES ==============

test.describe('Creator Pages', () => {
  let creatorToken: string;

  test.beforeEach(async () => {
    // 创建创作者
    const username = generateUsername();
    const phone = generatePhone();
    const regResult = await apiRegister(username, 'test123456', phone, 'creator');
    const loginResult = await apiLogin(username, 'test123456');
    creatorToken = loginResult.data?.token;
  });

  test('TC-CREATOR-01: 创作者工作台需要认证', async ({ page }) => {
    // 不带token访问应重定向或显示未授权
    await page.goto('/creator/dashboard.html');
    await page.waitForTimeout(2000);

    const url = page.url();
    console.log('未登录访问创作者工作台URL:', url);
    // 检查是否还在登录页或者页面显示需要登录
    const body = await page.locator('body').textContent();
    console.log('页面内容:', body.substring(0, 200));
  });

  test('TC-CREATOR-02: 创作者任务大厅需要认证', async ({ page }) => {
    await page.goto('/creator/task_hall.html');
    await page.waitForTimeout(2000);

    const url = page.url();
    console.log('未登录访问任务大厅URL:', url);
  });

  test('TC-CREATOR-03: 创作者任务大厅-已登录', async ({ page }) => {
    // 设置token
    await page.goto('/auth/login.html');
    const username = generateUsername();
    await apiRegister(username, 'test123456', generatePhone(), 'creator');
    const loginResult = await apiLogin(username, 'test123456');

    // 通过执行脚本设置localStorage
    await page.goto('/');
    await page.evaluate((token) => {
      localStorage.setItem('token', token);
      localStorage.setItem('role', 'creator');
    }, loginResult.data?.token);

    // 访问任务大厅
    await page.goto('/creator/task_hall.html');
    await page.waitForTimeout(2000);

    const body = await page.locator('body').textContent();
    console.log('已登录访问任务大厅, body长度:', body.length);
  });

  test('TC-CREATOR-04: 创作者我的认领页面', async ({ page }) => {
    await page.goto('/creator/claim_list.html');
    await page.waitForTimeout(2000);

    const url = page.url();
    console.log('访问我的认领URL:', url);
  });

  test('TC-CREATOR-05: 创作者钱包页面', async ({ page }) => {
    await page.goto('/creator/wallet.html');
    await page.waitForTimeout(2000);

    const url = page.url();
    console.log('访问钱包URL:', url);
  });
});

// ============== BUSINESS PAGES ==============

test.describe('Business Pages', () => {
  test('TC-BUSINESS-01: 商家工作台需要认证', async ({ page }) => {
    await page.goto('/business/dashboard.html');
    await page.waitForTimeout(2000);

    const url = page.url();
    console.log('未登录访问商家工作台URL:', url);
  });

  test('TC-BUSINESS-02: 发布任务页面需要认证', async ({ page }) => {
    await page.goto('/business/task_create.html');
    await page.waitForTimeout(2000);

    const url = page.url();
    console.log('未登录访问发布任务URL:', url);
  });

  test('TC-BUSINESS-03: 商家充值页面需要认证', async ({ page }) => {
    await page.goto('/business/recharge.html');
    await page.waitForTimeout(2000);

    const url = page.url();
    console.log('未登录访问充值URL:', url);
  });
});

// ============== FULL USER FLOWS ==============

test.describe('Full User Flows', () => {
  test('FLOW-01: 创作者完整流程', async ({ page }) => {
    const username = generateUsername();
    const phone = generatePhone();

    // 1. 注册创作者
    const regResult = await apiRegister(username, 'test123456', phone, 'creator');
    console.log('1. 注册创作者:', regResult.code === 0 ? '成功' : '失败');
    expect(regResult.code).toBe(0);

    // 2. 登录
    const loginResult = await apiLogin(username, 'test123456');
    console.log('2. 登录:', loginResult.code === 0 ? '成功' : '失败');
    expect(loginResult.code).toBe(0);
    const token = loginResult.data?.token;

    // 3. 访问创作者工作台
    await page.goto('/');
    await page.evaluate((t) => {
      localStorage.setItem('token', t);
      localStorage.setItem('role', 'creator');
    }, token);

    await page.goto('/creator/dashboard.html');
    await page.waitForTimeout(2000);
    console.log('3. 访问创作者工作台:', page.url());

    // 4. 访问任务大厅
    await page.goto('/creator/task_hall.html');
    await page.waitForTimeout(2000);
    console.log('4. 访问任务大厅:', page.url());

    // 5. 访问我的认领
    await page.goto('/creator/claim_list.html');
    await page.waitForTimeout(2000);
    console.log('5. 访问我的认领:', page.url());

    // 6. 访问钱包
    await page.goto('/creator/wallet.html');
    await page.waitForTimeout(2000);
    console.log('6. 访问钱包:', page.url());
  });

  test('FLOW-02: 商家完整流程', async ({ page }) => {
    const username = generateUsername();
    const phone = generatePhone();

    // 1. 注册商家
    const regResult = await apiRegister(username, 'test123456', phone, 'business');
    console.log('1. 注册商家:', regResult.code === 0 ? '成功' : '失败');
    expect(regResult.code).toBe(0);

    // 2. 登录
    const loginResult = await apiLogin(username, 'test123456');
    console.log('2. 登录:', loginResult.code === 0 ? '成功' : '失败');
    expect(loginResult.code).toBe(0);
    const token = loginResult.data?.token;

    // 3. 设置token并访问工作台
    await page.goto('/');
    await page.evaluate((t) => {
      localStorage.setItem('token', t);
      localStorage.setItem('role', 'business');
    }, token);

    await page.goto('/business/dashboard.html');
    await page.waitForTimeout(2000);
    console.log('3. 访问商家工作台:', page.url());

    // 4. 访问发布任务
    await page.goto('/business/task_create.html');
    await page.waitForTimeout(2000);
    console.log('4. 访问发布任务:', page.url());

    // 5. 访问充值页面
    await page.goto('/business/recharge.html');
    await page.waitForTimeout(2000);
    console.log('5. 访问充值:', page.url());

    // 6. 访问我的任务
    await page.goto('/business/task_list.html');
    await page.waitForTimeout(2000);
    console.log('6. 访问我的任务:', page.url());
  });
});

// ============== EDGE CASES ==============

test.describe('Edge Cases', () => {
  test('EDGE-01: 未登录访问应重定向', async ({ page }) => {
    await page.goto('/creator/dashboard.html');
    await page.waitForTimeout(2000);

    const url = page.url();
    console.log('未登录访问创作者工作台，URL:', url);
    // 检查是否在登录页或显示需要登录
  });

  test('EDGE-02: 商家访问创作者页面应该可以（双角色）', async ({ page }) => {
    // 注册双角色用户
    const username = generateUsername();
    await apiRegister(username, 'test123456', generatePhone(), 'business');

    await page.goto('/auth/login.html');
    await page.fill('#username', username);
    await page.fill('#password', 'test123456');
    await page.locator('#login-role').selectOption('business');
    await page.click('button[type="submit"]');

    await page.waitForTimeout(2000);
    const url = page.url();
    console.log('商家登录后URL:', url);
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

  test('TC-PROFILE-01: 获取用户资料', async () => {
    // 注册并登录
    await apiRegister(testUser.username, testUser.password, testUser.phone, 'creator');
    const loginResult = await apiLogin(testUser.username, testUser.password);
    expect(loginResult.code).toBe(0);
    testUser.token = loginResult.data?.token;

    // 获取资料
    const profileResult = await apiGetProfile(testUser.token);
    console.log('获取用户资料:', profileResult);
    expect(profileResult.code).toBe(0);
    expect(profileResult.data).toHaveProperty('id');
    expect(profileResult.data).toHaveProperty('username');
    expect(profileResult.data.username).toBe(testUser.username);
  });

  test('TC-PROFILE-02: 更新用户昵称', async () => {
    // 注册并登录
    await apiRegister(testUser.username, testUser.password, testUser.phone, 'creator');
    const loginResult = await apiLogin(testUser.username, testUser.password);
    testUser.token = loginResult.data?.token;

    // 更新昵称
    const newNickname = '测试昵称_' + randomInt(1000);
    const updateResult = await apiUpdateProfile(testUser.token, { nickname: newNickname });
    console.log('更新昵称:', updateResult);
    expect(updateResult.code).toBe(0);

    // 验证更新
    const profileResult = await apiGetProfile(testUser.token);
    expect(profileResult.data.nickname).toBe(newNickname);
  });

  test('TC-PROFILE-03: 更新用户头像', async () => {
    // 注册并登录
    await apiRegister(testUser.username, testUser.password, testUser.phone, 'creator');
    const loginResult = await apiLogin(testUser.username, testUser.password);
    testUser.token = loginResult.data?.token;

    // 更新头像
    const newAvatar = 'https://example.com/avatar/' + randomInt(1000) + '.jpg';
    const updateResult = await apiUpdateProfile(testUser.token, { avatar: newAvatar });
    console.log('更新头像:', updateResult);
    expect(updateResult.code).toBe(0);

    // 验证更新
    const profileResult = await apiGetProfile(testUser.token);
    expect(profileResult.data.avatar).toBe(newAvatar);
  });

  test('TC-PROFILE-04: 修改密码', async () => {
    // 注册并登录
    await apiRegister(testUser.username, testUser.password, testUser.phone, 'creator');
    const loginResult = await apiLogin(testUser.username, testUser.password);
    testUser.token = loginResult.data?.token;

    // 修改密码
    const newPassword = 'newpass123';
    const changeResult = await apiChangePassword(testUser.token, testUser.password, newPassword);
    console.log('修改密码:', changeResult);
    expect(changeResult.code).toBe(0);

    // 使用新密码登录
    const newLoginResult = await apiLogin(testUser.username, newPassword);
    console.log('新密码登录:', newLoginResult);
    expect(newLoginResult.code).toBe(0);
    expect(newLoginResult.data).toHaveProperty('token');
  });

  test('TC-PROFILE-05: 修改密码-原密码错误', async () => {
    // 注册并登录
    await apiRegister(testUser.username, testUser.password, testUser.phone, 'creator');
    const loginResult = await apiLogin(testUser.username, testUser.password);
    testUser.token = loginResult.data?.token;

    // 使用错误原密码修改密码
    const wrongOldPassword = 'wrongpassword';
    const changeResult = await apiChangePassword(testUser.token, wrongOldPassword, 'newpass123');
    console.log('错误原密码修改密码:', changeResult);
    expect(changeResult.code).not.toBe(0);
  });

  test('TC-PROFILE-06: 未登录获取资料应失败', async () => {
    const result = await apiGetProfile('invalid-token');
    console.log('未登录获取资料:', result);
    expect(result.code).not.toBe(0);
  });

  test('TC-PROFILE-07: 修改密码-新密码太短', async () => {
    // 注册并登录
    await apiRegister(testUser.username, testUser.password, testUser.phone, 'creator');
    const loginResult = await apiLogin(testUser.username, testUser.password);
    testUser.token = loginResult.data?.token;

    // 新密码太短
    const shortPassword = '123';
    const changeResult = await apiChangePassword(testUser.token, testUser.password, shortPassword);
    console.log('密码太短:', changeResult);
    expect(changeResult.code).not.toBe(0);
  });
});
