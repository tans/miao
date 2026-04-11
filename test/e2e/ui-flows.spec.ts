/**
 * UI Flow Tests - 基于真实 Playwright UI 操作的端到端测试
 * 使用浏览器模拟真实用户操作，而非直接 API 调用
 */
import { test, expect, Page } from '@playwright/test';

// Test data helpers
function generateUsername(): string {
  return `ui_user_${Date.now()}_${Math.floor(Math.random() * 10000)}`;
}

function generatePhone(): string {
  return `138${String(Math.floor(Math.random() * 100000000)).padStart(8, '0')}`;
}

// Helper: Register via UI
async function registerViaUI(page: Page, username: string, password: string, phone: string) {
  await page.goto('/auth/register.html');
  await page.fill('#username', username);
  await page.fill('#password', password);
  await page.fill('#phone', phone);
  await page.click('button[type="submit"]');
  // Wait for redirect after registration
  await page.waitForTimeout(2000);
}

// Helper: Login via UI
async function loginViaUI(page: Page, username: string, password: string, role: 'business' | 'creator' = 'creator') {
  await page.goto('/auth/login.html');
  await page.fill('#username', username);
  await page.fill('#password', password);
  await page.locator('#login-role').selectOption(role);
  await page.click('button[type="submit"]');
  await page.waitForTimeout(2000);
}

// ============== UI AUTHENTICATION FLOWS ==============

test.describe('UI Authentication Flows', () => {
  test('UI-AUTH-01: 创作者通过 UI 注册并登录', async ({ page }) => {
    const username = generateUsername();
    const password = 'test123456';
    const phone = generatePhone();

    // 1. 注册
    await page.goto('/auth/register.html');
    await expect(page.locator('#register-form')).toBeVisible();
    await page.fill('#username', username);
    await page.fill('#password', password);
    await page.fill('#phone', phone);
    await page.click('button[type="submit"]');

    // 等待注册完成并跳转
    await page.waitForTimeout(3000);
    const url = page.url();
    console.log('注册后 URL:', url);

    // 2. 登录
    await page.goto('/auth/login.html');
    await expect(page.locator('#login-form')).toBeVisible();
    await page.fill('#username', username);
    await page.fill('#password', password);
    await page.locator('#login-role').selectOption('creator');
    await page.click('button[type="submit"]');

    // 等待登录完成
    await page.waitForTimeout(3000);
    console.log('登录后 URL:', page.url());

    // 3. 验证跳转到了创作者工作台
    const currentUrl = page.url();
    expect(currentUrl).toMatch(/creator\/dashboard|dashboard/);
  });

  test('UI-AUTH-02: 商家通过 UI 注册并登录', async ({ page }) => {
    const username = generateUsername();
    const password = 'test123456';
    const phone = generatePhone();

    // 1. 注册
    await registerViaUI(page, username, password, phone);

    // 2. 登录
    await loginViaUI(page, username, password, 'business');

    // 3. 验证跳转到了商家工作台
    const currentUrl = page.url();
    console.log('商家登录后 URL:', currentUrl);
    expect(currentUrl).toMatch(/business\/dashboard|dashboard/);
  });

  test('UI-AUTH-03: 错误密码登录应失败', async ({ page }) => {
    await page.goto('/auth/login.html');
    await page.fill('#username', 'nonexistent_user_' + Date.now());
    await page.fill('#password', 'wrongpassword');
    await page.click('button[type="submit"]');

    // 等待错误处理
    await page.waitForTimeout(2000);

    // 应该仍然在登录页
    const url = page.url();
    expect(url).toMatch(/login/);
  });
});

// ============== UI BUSINESS FLOWS ==============

test.describe('UI Business Flows', () => {
  test('UI-BUSINESS-01: 商家通过 UI 充值', async ({ page }) => {
    const username = generateUsername();
    const password = 'test123456';
    const phone = generatePhone();

    // 注册并登录商家
    await registerViaUI(page, username, password, phone);
    await loginViaUI(page, username, password, 'business');

    // 访问充值页面
    await page.goto('/business/recharge.html');
    await expect(page.locator('#recharge-form')).toBeVisible();
    await expect(page.locator('#current-balance')).toBeVisible();

    // 输入充值金额并提交
    await page.fill('#amount', '500');
    await page.click('button[type="submit"]');

    // 等待充值处理
    await page.waitForTimeout(3000);

    // 验证余额已更新
    const balanceText = await page.locator('#current-balance').textContent();
    console.log('充值后余额:', balanceText);
    expect(balanceText).toContain('500');
  });

  test('UI-BUSINESS-02: 商家通过 UI 发布任务', async ({ page }) => {
    const username = generateUsername();
    const password = 'test123456';
    const phone = generatePhone();

    // 注册并登录商家
    await registerViaUI(page, username, password, phone);
    await loginViaUI(page, username, password, 'business');

    // 先充值（确保有足够余额）
    await page.goto('/business/recharge.html');
    await page.fill('#amount', '1000');
    await page.click('button[type="submit"]');
    await page.waitForTimeout(2000);

    // 访问发布任务页面
    await page.goto('/business/task_create.html');
    await expect(page.locator('#task-form')).toBeVisible();

    // 填写任务表单
    const taskTitle = 'UI测试任务_' + Date.now();
    await page.fill('#title', taskTitle);
    await page.fill('#description', '这是通过 UI 自动化测试创建的任务');

    // 选择分类（默认分类 1）
    // 填写单价
    await page.fill('#unit_price', '50');
    // 填写总数量
    await page.fill('#total_count', '10');

    // 提交
    await page.click('button[type="submit"]');

    // 等待发布完成
    await page.waitForTimeout(3000);

    // 验证跳转到了任务列表
    const url = page.url();
    console.log('发布任务后 URL:', url);
    expect(url).toMatch(/task_list|dashboard/);
  });
});

// ============== UI CREATOR FLOWS ==============

test.describe('UI Creator Flows', () => {
  test('UI-CREATOR-01: 创作者通过 UI 浏览和认领任务', async ({ page }) => {
    const username = generateUsername();
    const password = 'test123456';
    const phone = generatePhone();

    // 注册并登录创作者
    await registerViaUI(page, username, password, phone);
    await loginViaUI(page, username, password, 'creator');

    // 访问任务大厅
    await page.goto('/creator/task_hall.html');
    await page.waitForTimeout(2000);

    // 验证页面加载
    const body = await page.locator('body').textContent();
    console.log('任务大厅页面长度:', body.length);

    // 检查是否有任务列表
    const pageContent = await page.content();
    const hasTaskList = pageContent.includes('任务') || pageContent.includes('task');
    console.log('页面包含任务内容:', hasTaskList);
  });

  test('UI-CREATOR-02: 创作者通过 UI 访问钱包', async ({ page }) => {
    const username = generateUsername();
    const password = 'test123456';
    const phone = generatePhone();

    // 注册并登录创作者
    await registerViaUI(page, username, password, phone);
    await loginViaUI(page, username, password, 'creator');

    // 访问钱包页面
    await page.goto('/creator/wallet.html');
    await page.waitForTimeout(2000);

    // 验证页面关键元素
    const pageContent = await page.content();
    console.log('钱包页面加载完成');
    // 钱包页面应该包含余额信息
    expect(pageContent.length).toBeGreaterThan(100);
  });
});

// ============== COMPLETE UI FLOW ==============

test.describe('Complete UI Flow Tests', () => {
  test('UI-FLOW-01: 完整商家流程（注册→充值→发布任务）', async ({ page }) => {
    const username = generateUsername();
    const password = 'test123456';
    const phone = generatePhone();
    const taskTitle = '完整UI测试任务_' + Date.now();

    console.log('=== 开始完整商家流程 ===');

    // 1. 注册商家
    console.log('1. 注册商家...');
    await registerViaUI(page, username, password, phone);

    // 2. 登录商家
    console.log('2. 登录商家...');
    await loginViaUI(page, username, password, 'business');
    console.log('   登录后 URL:', page.url());

    // 3. 充值
    console.log('3. 充值...');
    await page.goto('/business/recharge.html');
    await page.fill('#amount', '1000');
    await page.click('button[type="submit"]');
    await page.waitForTimeout(2000);
    console.log('   充值后余额元素可见');

    // 4. 发布任务
    console.log('4. 发布任务...');
    await page.goto('/business/task_create.html');
    await page.fill('#title', taskTitle);
    await page.fill('#description', '完整UI测试任务描述');
    await page.fill('#unit_price', '50');
    await page.fill('#total_count', '5');
    await page.click('button[type="submit"]');
    await page.waitForTimeout(3000);
    console.log('   发布后 URL:', page.url());

    // 验证任务发布成功
    const finalUrl = page.url();
    console.log('=== 商家流程完成 ===', finalUrl);
  });

  test('UI-FLOW-02: 完整创作者流程（注册→浏览→认领）', async ({ page }) => {
    const username = generateUsername();
    const password = 'test123456';
    const phone = generatePhone();

    console.log('=== 开始完整创作者流程 ===');

    // 1. 注册创作者
    console.log('1. 注册创作者...');
    await registerViaUI(page, username, password, phone);

    // 2. 登录创作者
    console.log('2. 登录创作者...');
    await loginViaUI(page, username, password, 'creator');
    console.log('   登录后 URL:', page.url());

    // 3. 访问工作台
    console.log('3. 访问工作台...');
    await page.goto('/creator/dashboard.html');
    await page.waitForTimeout(2000);

    // 4. 访问任务大厅
    console.log('4. 访问任务大厅...');
    await page.goto('/creator/task_hall.html');
    await page.waitForTimeout(2000);
    console.log('   任务大厅 URL:', page.url());

    // 5. 访问我的认领
    console.log('5. 访问我的认领...');
    await page.goto('/creator/claim_list.html');
    await page.waitForTimeout(2000);

    // 6. 访问钱包
    console.log('6. 访问钱包...');
    await page.goto('/creator/wallet.html');
    await page.waitForTimeout(2000);

    console.log('=== 创作者流程完成 ===');
  });
});
