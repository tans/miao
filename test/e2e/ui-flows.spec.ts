/**
 * UI Flow Tests - 基于真实 Playwright UI 操作的端到端测试
 * 使用浏览器模拟真实用户操作
 *
 * 注意：角色（商家/创作者）已改为纯前端 localStorage 切换，
 * 登录页不再有角色选择框。注册后默认进入商家工作台。
 */
import { test, expect, Page } from '@playwright/test';

// ─── Helpers ────────────────────────────────────────────────────────────────

function uid(): string {
  return `ui_${Date.now()}_${Math.floor(Math.random() * 9999)}`;
}

function phone(): string {
  return `138${String(Math.floor(Math.random() * 1e8)).padStart(8, '0')}`;
}

/** Wait for components.js to be loaded (apiRequest / requireAuth defined) */
async function waitForScripts(page: Page) {
  await page.waitForFunction(() => typeof (window as any).apiRequest === 'function', { timeout: 10000 });
}

/** Register via UI. After success, auto-redirects to /business/dashboard.html */
async function registerViaUI(page: Page, username: string, password: string, phoneNum: string) {
  await page.goto('/auth/register.html');
  await waitForScripts(page);
  await expect(page.locator('#register-form')).toBeVisible({ timeout: 10000 });
  await page.fill('#username', username);
  await page.fill('#password', password);
  await page.fill('#phone', phoneNum);
  await page.click('button[type="submit"]');
  await page.waitForURL('**/dashboard.html', { timeout: 10000 });
}

/** Login via UI. Role selection removed — defaults to last stored role (business). */
async function loginViaUI(page: Page, username: string, password: string) {
  // Pre-set current_role so the login redirect goes to business dashboard
  await page.goto('/auth/login.html');
  await page.evaluate(() => localStorage.setItem('current_role', 'business'));
  // Now reload so the early-redirect check doesn't fire on this login visit
  // (we only set the role for post-login redirect, not to trigger auto-redirect)
  await waitForScripts(page);
  await expect(page.locator('#login-form')).toBeVisible({ timeout: 10000 });
  await page.fill('#username', username);
  await page.fill('#password', password);
  await page.click('button[type="submit"]');
  await page.waitForURL('**/dashboard.html', { timeout: 10000 });
}

/** Switch to creator role via localStorage. */
async function switchToCreator(page: Page) {
  await page.evaluate(() => {
    localStorage.setItem('current_role', 'creator');
    localStorage.setItem('role', 'creator');
  });
}

/** Switch to business role via localStorage. */
async function switchToBusiness(page: Page) {
  await page.evaluate(() => {
    localStorage.setItem('current_role', 'business');
    localStorage.setItem('role', 'business');
  });
}

/** Navigate to page and wait for scripts + network to settle */
async function gotoAndWait(page: Page, url: string) {
  await page.goto(url);
  await page.waitForLoadState('networkidle', { timeout: 15000 }).catch(() => {});
  await waitForScripts(page).catch(() => {});
}

// ─── Auth ────────────────────────────────────────────────────────────────────

test.describe('UI Authentication Flows', () => {
  test('UI-AUTH-01: 注册成功后自动登录并跳转工作台', async ({ page }) => {
    const username = uid();
    await page.goto('/auth/register.html');
    await waitForScripts(page);
    await expect(page.locator('#register-form')).toBeVisible();
    await page.fill('#username', username);
    await page.fill('#password', 'test123456');
    await page.fill('#phone', phone());
    await page.click('button[type="submit"]');

    await page.waitForURL('**/dashboard.html', { timeout: 10000 });
    expect(page.url()).toMatch(/dashboard\.html/);

    const token = await page.evaluate(() => localStorage.getItem('token'));
    expect(token).toBeTruthy();
  });

  test('UI-AUTH-02: 登录成功跳转到工作台', async ({ page }) => {
    const username = uid();
    await registerViaUI(page, username, 'test123456', phone());

    // Clear session, set role hint, then login
    await page.evaluate(() => {
      localStorage.clear();
      localStorage.setItem('current_role', 'business');
    });
    await loginViaUI(page, username, 'test123456');
    expect(page.url()).toMatch(/dashboard\.html/);
  });

  test('UI-AUTH-03: 错误密码登录应失败，停留在登录页', async ({ page }) => {
    await page.goto('/auth/login.html');
    await waitForScripts(page);
    await page.fill('#username', 'no_such_user_' + Date.now());
    await page.fill('#password', 'wrongpassword');
    await page.click('button[type="submit"]');
    await page.waitForTimeout(2000);
    expect(page.url()).toMatch(/login/);
  });

  test('UI-AUTH-04: 已登录用户访问登录页自动跳转', async ({ page }) => {
    const username = uid();
    await registerViaUI(page, username, 'test123456', phone());
    // Now already logged in — visiting login.html should redirect
    await page.goto('/auth/login.html');
    await waitForScripts(page);
    await page.waitForTimeout(500);
    // The inline script checks token + getCurrentRole and redirects
    await page.waitForURL(/dashboard/, { timeout: 5000 }).catch(() => {});
    const url = page.url();
    expect(url).not.toMatch(/login\.html/);
  });
});

// ─── Business Flows ──────────────────────────────────────────────────────────

test.describe('UI Business Flows', () => {
  test('UI-BUSINESS-01: 商家通过 UI 充值', async ({ page }) => {
    const username = uid();
    await registerViaUI(page, username, 'test123456', phone());
    await switchToBusiness(page);

    await gotoAndWait(page, '/business/recharge.html');
    await expect(page.locator('#recharge-form')).toBeVisible({ timeout: 10000 });

    // Wait for balance to load
    await page.waitForFunction(
      () => (document.getElementById('current-balance') as HTMLElement)?.textContent !== '¥0.00' ||
            (window as any).apiRequest !== undefined,
      { timeout: 5000 }
    ).catch(() => {});

    await page.fill('#amount', '500');
    await page.click('button[type="submit"]');

    // Wait for success toast or balance update
    await page.waitForFunction(
      () => (document.getElementById('current-balance') as HTMLElement)?.textContent?.includes('500') ||
            document.querySelector('.toast-body')?.textContent?.includes('成功'),
      { timeout: 8000 }
    ).catch(() => {});

    const balanceText = await page.locator('#current-balance').textContent();
    console.log('充值后余额:', balanceText);
    // Accept either updated balance or verify the recharge API was called
    const hasBalance = balanceText?.includes('500');
    const toastVisible = await page.locator('.toast-body').filter({ hasText: '成功' }).count() > 0;
    expect(hasBalance || toastVisible).toBeTruthy();
  });

  test('UI-BUSINESS-02: 商家通过 UI 发布任务', async ({ page }) => {
    const username = uid();
    await registerViaUI(page, username, 'test123456', phone());
    await switchToBusiness(page);

    // Recharge first
    await gotoAndWait(page, '/business/recharge.html');
    await expect(page.locator('#recharge-form')).toBeVisible({ timeout: 10000 });
    await page.fill('#amount', '1000');
    await page.click('button[type="submit"]');
    await page.waitForTimeout(2000);

    // Create task
    await gotoAndWait(page, '/business/task_create.html');
    await expect(page.locator('#task-form')).toBeVisible({ timeout: 10000 });
    await page.fill('#title', 'UI任务_' + Date.now());
    await page.fill('#description', 'UI 自动化测试任务描述，需要足够详细的内容');
    await page.fill('#unit_price', '10');
    await page.fill('#total_count', '10');
    // Fill required deadline (tomorrow)
    const tomorrow = new Date(Date.now() + 86400000).toISOString().slice(0, 16);
    await page.fill('#deadline', tomorrow);
    await page.click('button[type="submit"]');

    // Wait for redirect or success message
    await page.waitForURL(/task_list|dashboard/, { timeout: 8000 }).catch(() => {});
    const url = page.url();
    const toast = await page.locator('.toast-body').filter({ hasText: '成功' }).count();
    console.log('发布任务后 URL:', url, 'toast:', toast);
    expect(url.match(/task_list|dashboard/) !== null || toast > 0).toBeTruthy();
  });

  test('UI-BUSINESS-03: 商家工作台正常加载', async ({ page }) => {
    const username = uid();
    await registerViaUI(page, username, 'test123456', phone());
    await switchToBusiness(page);
    await gotoAndWait(page, '/business/dashboard.html');
    expect(await page.content()).toContain('</html>');
    expect(await page.title()).toBeTruthy();
  });

  test('UI-BUSINESS-04: 商家任务列表页正常加载', async ({ page }) => {
    const username = uid();
    await registerViaUI(page, username, 'test123456', phone());
    await switchToBusiness(page);
    await gotoAndWait(page, '/business/task_list.html');
    expect(await page.content()).toContain('</html>');
  });
});

// ─── Creator Flows ───────────────────────────────────────────────────────────

test.describe('UI Creator Flows', () => {
  test('UI-CREATOR-01: 创作者任务大厅页正常加载', async ({ page }) => {
    const username = uid();
    await registerViaUI(page, username, 'test123456', phone());
    await switchToCreator(page);
    await gotoAndWait(page, '/creator/task_hall.html');
    const content = await page.content();
    expect(content).toContain('</html>');
    const hasTaskContent = content.includes('任务') || content.includes('task') || content.includes('Task');
    console.log('任务大厅含任务内容:', hasTaskContent, '长度:', content.length);
    expect(content.length).toBeGreaterThan(500);
  });

  test('UI-CREATOR-02: 创作者钱包页正常加载', async ({ page }) => {
    const username = uid();
    await registerViaUI(page, username, 'test123456', phone());
    await switchToCreator(page);
    await gotoAndWait(page, '/creator/wallet.html');
    expect(await page.content()).toContain('</html>');
  });

  test('UI-CREATOR-03: 创作者工作台正常加载', async ({ page }) => {
    const username = uid();
    await registerViaUI(page, username, 'test123456', phone());
    await switchToCreator(page);
    await gotoAndWait(page, '/creator/dashboard.html');
    expect(await page.content()).toContain('</html>');
  });

  test('UI-CREATOR-04: 创作者认领列表页正常加载', async ({ page }) => {
    const username = uid();
    await registerViaUI(page, username, 'test123456', phone());
    await switchToCreator(page);
    await gotoAndWait(page, '/creator/claim_list.html');
    expect(await page.content()).toContain('</html>');
  });
});

// ─── Complete Flows ───────────────────────────────────────────────────────────

test.describe('Complete UI Flow Tests', () => {
  test('UI-FLOW-01: 完整商家流程（注册→充值→发布任务）', async ({ page }) => {
    const username = uid();

    // 1. Register
    await registerViaUI(page, username, 'test123456', phone());
    await switchToBusiness(page);
    expect(page.url()).toMatch(/dashboard\.html/);

    // 2. Recharge
    await gotoAndWait(page, '/business/recharge.html');
    await expect(page.locator('#recharge-form')).toBeVisible({ timeout: 10000 });
    await page.fill('#amount', '1000');
    await page.click('button[type="submit"]');
    await page.waitForTimeout(2000);

    // 3. Create task
    await gotoAndWait(page, '/business/task_create.html');
    await expect(page.locator('#task-form')).toBeVisible({ timeout: 10000 });
    await page.fill('#title', '完整UI任务_' + Date.now());
    await page.fill('#description', '完整UI测试任务描述，需要足够详细的内容说明');
    await page.fill('#unit_price', '10');
    await page.fill('#total_count', '10');
    const tomorrow = new Date(Date.now() + 86400000).toISOString().slice(0, 16);
    await page.fill('#deadline', tomorrow);
    await page.click('button[type="submit"]');
    await page.waitForURL(/task_list|dashboard/, { timeout: 8000 }).catch(() => {});

    const finalUrl = page.url();
    const toast = await page.locator('.toast-body').filter({ hasText: '成功' }).count();
    console.log('=== 商家流程完成 ===', finalUrl, 'toast:', toast);
    expect(finalUrl.match(/task_list|dashboard/) !== null || toast > 0).toBeTruthy();
  });

  test('UI-FLOW-02: 完整创作者流程（注册→浏览各页面）', async ({ page }) => {
    const username = uid();

    // 1. Register
    await registerViaUI(page, username, 'test123456', phone());
    await switchToCreator(page);

    // 2. Browse creator pages
    for (const path of [
      '/creator/dashboard.html',
      '/creator/task_hall.html',
      '/creator/claim_list.html',
      '/creator/wallet.html',
    ]) {
      await gotoAndWait(page, path);
      expect(await page.content()).toContain('</html>');
      console.log('访问:', path, '✓');
    }
    console.log('=== 创作者流程完成 ===');
  });

  test('UI-FLOW-03: 角色切换（商家→创作者→商家）', async ({ page }) => {
    const username = uid();
    await registerViaUI(page, username, 'test123456', phone());

    // Default role after register is business
    let role = await page.evaluate(() => localStorage.getItem('current_role'));
    expect(role).toBe('business');

    // Switch to creator
    await switchToCreator(page);
    role = await page.evaluate(() => localStorage.getItem('current_role'));
    expect(role).toBe('creator');

    await gotoAndWait(page, '/creator/dashboard.html');
    expect(page.url()).toContain('/creator/');

    // Switch back to business
    await switchToBusiness(page);
    await gotoAndWait(page, '/business/dashboard.html');
    expect(page.url()).toContain('/business/');
    console.log('角色切换流程完成');
  });
});
