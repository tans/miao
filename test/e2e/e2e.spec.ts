/**
 * API-level E2E Tests for Miao Platform
 * Tests all major API flows using direct HTTP requests via Playwright's request fixture.
 */
import { test, expect, APIRequestContext } from '@playwright/test';

const BASE = 'http://localhost:8888/api/v1';

// ─── Helpers ───────────────────────────────────────────────────────────────

function uid(): string {
  return `t${Date.now()}_${Math.floor(Math.random() * 9999)}`;
}

function phone(): string {
  return `138${String(Math.floor(Math.random() * 1e8)).padStart(8, '0')}`;
}

async function post(req: APIRequestContext, path: string, body: object, token?: string) {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  const res = await req.post(`${BASE}${path}`, { headers, data: body });
  return res.json();
}

async function get(req: APIRequestContext, path: string, token?: string) {
  const headers: Record<string, string> = {};
  if (token) headers['Authorization'] = `Bearer ${token}`;
  const res = await req.get(`${BASE}${path}`, { headers });
  return res.json();
}

async function put(req: APIRequestContext, path: string, body: object, token: string) {
  const res = await req.put(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    data: body,
  });
  return res.json();
}

async function del(req: APIRequestContext, path: string, token: string) {
  const res = await req.delete(`${BASE}${path}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  return res.json();
}

// Register and return token (register already returns a token, no need to login again)
async function registerAndLogin(req: APIRequestContext, username?: string) {
  const u = username ?? uid();
  const p = phone();
  const r = await post(req, '/auth/register', { username: u, password: 'test123456', phone: p });
  expect(r.code).toBe(0);
  return { token: r.data.token as string, username: u };
}

// ─── AUTH ──────────────────────────────────────────────────────────────────

test.describe('Auth', () => {
  test('TC-AUTH-01: register new user', async ({ request }) => {
    const u = uid();
    const r = await post(request, '/auth/register', {
      username: u,
      password: 'test123456',
      phone: phone(),
    });
    expect(r.code).toBe(0);
    expect(r.data.token).toBeTruthy();
    expect(r.data.user.username).toBe(u);
  });

  test('TC-AUTH-02: duplicate username rejected', async ({ request }) => {
    const u = uid();
    await post(request, '/auth/register', { username: u, password: 'test123456', phone: phone() });
    const r = await post(request, '/auth/register', { username: u, password: 'test123456', phone: phone() });
    expect(r.code).not.toBe(0);
  });

  test('TC-AUTH-03: login returns token', async ({ request }) => {
    const u = uid();
    await post(request, '/auth/register', { username: u, password: 'test123456', phone: phone() });
    const r = await post(request, '/auth/login', { username: u, password: 'test123456' });
    expect(r.code).toBe(0);
    expect(r.data.token).toBeTruthy();
  });

  test('TC-AUTH-04: wrong password rejected', async ({ request }) => {
    const u = uid();
    await post(request, '/auth/register', { username: u, password: 'test123456', phone: phone() });
    const r = await post(request, '/auth/login', { username: u, password: 'wrongpass' });
    expect(r.code).not.toBe(0);
  });

  test('TC-AUTH-05: unknown user rejected', async ({ request }) => {
    const r = await post(request, '/auth/login', { username: 'no_such_user_' + uid(), password: 'pass' });
    expect(r.code).not.toBe(0);
  });

  test('TC-AUTH-06: get current user /users/me', async ({ request }) => {
    const { token, username } = await registerAndLogin(request);
    const r = await get(request, '/users/me', token);
    expect(r.code).toBe(0);
    expect(r.data.username).toBe(username);
  });

  test('TC-AUTH-07: /users/me requires auth', async ({ request }) => {
    const r = await get(request, '/users/me');
    expect(r.code).not.toBe(0);
  });
});

// ─── USER PROFILE ──────────────────────────────────────────────────────────

test.describe('User Profile', () => {
  test('TC-PROFILE-01: get profile', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/user/profile', token);
    expect(r.code).toBe(0);
  });

  test('TC-PROFILE-02: update display name', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const newName = 'DisplayName_' + uid();
    const r = await put(request, '/user/profile', { real_name: newName }, token);
    expect(r.code).toBe(0);
  });

  test('TC-PROFILE-03: change password with correct old password', async ({ request }) => {
    const u = uid();
    await post(request, '/auth/register', { username: u, password: 'test123456', phone: phone() });
    const login = await post(request, '/auth/login', { username: u, password: 'test123456' });
    const token = login.data.token;
    const r = await put(request, '/user/password', {
      old_password: 'test123456',
      new_password: 'newpass789',
    }, token);
    expect(r.code).toBe(0);
    // Can login with new password
    const r2 = await post(request, '/auth/login', { username: u, password: 'newpass789' });
    expect(r2.code).toBe(0);
  });

  test('TC-PROFILE-04: change password with wrong old password fails', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await put(request, '/user/password', {
      old_password: 'wrongold',
      new_password: 'newpass789',
    }, token);
    expect(r.code).not.toBe(0);
  });
});

// ─── PUBLIC TASKS ──────────────────────────────────────────────────────────

test.describe('Public Tasks', () => {
  test('TC-PUB-01: list available tasks (no auth)', async ({ request }) => {
    const r = await get(request, '/tasks');
    expect(r.code).toBe(0);
    // response structure: data.data (array) or data (array)
    const list = r.data?.data ?? r.data?.tasks ?? r.data;
    expect(Array.isArray(list)).toBeTruthy();
  });

  test('TC-PUB-02: get task detail (no auth)', async ({ request }) => {
    // Create a task first so there is something to fetch
    const { token } = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 1000 }, token);
    const created = await post(request, '/business/tasks', {
      title: 'pub_task_' + uid(),
      description: 'test',
      category: 1,
      unit_price: 50,
      total_count: 2,
    }, token);
    if (created.code !== 0) return; // skip if task creation unsupported
    const taskId = created.data.task_id ?? created.data.id;
    if (!taskId) return;
    const r = await get(request, `/tasks/${taskId}`);
    expect(r.code).toBe(0);
  });
});

// ─── BUSINESS FLOW ─────────────────────────────────────────────────────────

test.describe('Business Flow', () => {
  test('TC-BIZ-01: recharge increases balance', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const before = await get(request, '/wallet', token);
    const balBefore = before.data?.balance ?? 0;
    await post(request, '/business/recharge', { amount: 500 }, token);
    const after = await get(request, '/wallet', token);
    expect(after.data.balance).toBe(balBefore + 500);
  });

  test('TC-BIZ-02: create task deducts balance', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 1000 }, token);
    const r = await post(request, '/business/tasks', {
      title: 'biz_task_' + uid(),
      description: 'test task',
      category: 1,
      unit_price: 100,
      total_count: 3,
    }, token);
    expect(r.code).toBe(0);
    const wallet = await get(request, '/wallet', token);
    // 100 * 3 = 300 escrowed, balance should be 700
    expect(wallet.data.balance).toBe(700);
  });

  test('TC-BIZ-03: create task without balance fails', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await post(request, '/business/tasks', {
      title: 'no_balance_task_' + uid(),
      description: 'test',
      category: 1,
      unit_price: 100,
      total_count: 5,
    }, token);
    expect(r.code).not.toBe(0);
  });

  test('TC-BIZ-04: list my tasks', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 500 }, token);
    await post(request, '/business/tasks', {
      title: 'list_task_' + uid(),
      description: 'test',
      category: 1,
      unit_price: 50,
      total_count: 2,
    }, token);
    const r = await get(request, '/business/tasks', token);
    expect(r.code).toBe(0);
    const tasks = Array.isArray(r.data) ? r.data : (r.data?.tasks ?? r.data?.data ?? []);
    expect(Array.isArray(tasks)).toBeTruthy();
    expect(tasks.length).toBeGreaterThan(0);
  });

  test('TC-BIZ-05: cancel task refunds balance', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 1000 }, token);
    const created = await post(request, '/business/tasks', {
      title: 'cancel_task_' + uid(),
      description: 'test',
      category: 1,
      unit_price: 100,
      total_count: 2,
    }, token);
    expect(created.code).toBe(0);
    const taskId = created.data.task_id ?? created.data.id;
    const r = await del(request, `/business/tasks/${taskId}`, token);
    expect(r.code).toBe(0);
    const wallet = await get(request, '/wallet', token);
    expect(wallet.data.balance).toBe(1000);
  });

  test('TC-BIZ-06: get business transactions', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 200 }, token);
    const r = await get(request, '/business/transactions', token);
    expect(r.code).toBe(0);
  });

  test('TC-BIZ-07: get business stats', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/business/stats', token);
    expect(r.code).toBe(0);
  });
});

// ─── CREATOR FLOW ──────────────────────────────────────────────────────────

test.describe('Creator Flow', () => {
  test('TC-CREATOR-01: get creator wallet', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/creator/wallet', token);
    expect(r.code).toBe(0);
    expect(r.data).toBeDefined();
  });

  test('TC-CREATOR-02: list available tasks', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/creator/tasks', token);
    expect(r.code).toBe(0);
  });

  test('TC-CREATOR-03: list my claims', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/creator/claims', token);
    expect(r.code).toBe(0);
  });

  test('TC-CREATOR-04: get creator stats', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/creator/stats', token);
    expect(r.code).toBe(0);
  });

  test('TC-CREATOR-05: get creator transactions', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/creator/transactions', token);
    expect(r.code).toBe(0);
  });

  test('TC-CREATOR-06: claim a task', async ({ request }) => {
    // Business creates task
    const biz = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 1000 }, biz.token);
    const task = await post(request, '/business/tasks', {
      title: 'claim_me_' + uid(),
      description: 'test',
      category: 1,
      unit_price: 100,
      total_count: 3,
    }, biz.token);
    expect(task.code).toBe(0);
    const taskId = task.data.task_id ?? task.data.id;

    // Creator claims task
    const creator = await registerAndLogin(request);
    const r = await post(request, '/creator/claim', { task_id: taskId }, creator.token);
    expect(r.code).toBe(0);
  });

  test('TC-CREATOR-07: claim exhausts remaining slots', async ({ request }) => {
    // Task with total_count=1 can only be claimed once total
    const biz = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 1000 }, biz.token);
    const task = await post(request, '/business/tasks', {
      title: 'single_claim_' + uid(),
      description: 'test',
      category: 1,
      unit_price: 100,
      total_count: 1,
    }, biz.token);
    const taskId = task.data.task_id ?? task.data.id;

    // First creator claims the only slot
    const creator1 = await registerAndLogin(request);
    const r1 = await post(request, '/creator/claim', { task_id: taskId }, creator1.token);
    expect(r1.code).toBe(0);

    // Second creator should fail (no slots left)
    const creator2 = await registerAndLogin(request);
    const r2 = await post(request, '/creator/claim', { task_id: taskId }, creator2.token);
    expect(r2.code).not.toBe(0);
  });

  test('TC-CREATOR-08: submit work on claim', async ({ request }) => {
    const biz = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 1000 }, biz.token);
    const task = await post(request, '/business/tasks', {
      title: 'submit_task_' + uid(),
      description: 'test',
      category: 1,
      unit_price: 100,
      total_count: 3,
    }, biz.token);
    const taskId = task.data.task_id ?? task.data.id;

    const creator = await registerAndLogin(request);
    const claim = await post(request, '/creator/claim', { task_id: taskId }, creator.token);
    expect(claim.code).toBe(0);
    const claimId = claim.data.claim_id ?? claim.data.id;

    const r = await put(request, `/creator/claim/${claimId}/submit`, {
      content: 'My submission content',
      attachments: [],
    }, creator.token);
    expect(r.code).toBe(0);
  });
});

// ─── BUSINESS REVIEW ───────────────────────────────────────────────────────

test.describe('Business Review', () => {
  // Helper: set up a task with one submitted claim
  async function setupSubmittedClaim(request: APIRequestContext) {
    const biz = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 2000 }, biz.token);
    const task = await post(request, '/business/tasks', {
      title: 'review_task_' + uid(),
      description: 'test',
      category: 1,
      unit_price: 200,
      total_count: 3,
    }, biz.token);
    const taskId = task.data.task_id ?? task.data.id;

    const creator = await registerAndLogin(request);
    const claim = await post(request, '/creator/claim', { task_id: taskId }, creator.token);
    const claimId = claim.data.claim_id ?? claim.data.id;

    await put(request, `/creator/claim/${claimId}/submit`, {
      content: 'Submitted work',
    }, creator.token);

    return { bizToken: biz.token, creatorToken: creator.token, taskId, claimId };
  }

  test('TC-REVIEW-01: business can view task claims', async ({ request }) => {
    const { bizToken, taskId } = await setupSubmittedClaim(request);
    const r = await get(request, `/business/tasks/${taskId}/claims`, bizToken);
    expect(r.code).toBe(0);
  });

  test('TC-REVIEW-02: business approves claim, creator gets paid', async ({ request }) => {
    const { bizToken, creatorToken, claimId } = await setupSubmittedClaim(request);

    const walletBefore = await get(request, '/creator/wallet', creatorToken);
    const balBefore = walletBefore.data?.balance ?? 0;

    const r = await put(request, `/business/claim/${claimId}/review`, {
      result: 1,
      comment: 'Great work!',
    }, bizToken);
    expect(r.code).toBe(0);

    const walletAfter = await get(request, '/creator/wallet', creatorToken);
    expect(walletAfter.data.balance).toBeGreaterThan(balBefore);
  });

  test('TC-REVIEW-03: business rejects claim', async ({ request }) => {
    const { bizToken, claimId } = await setupSubmittedClaim(request);
    const r = await put(request, `/business/claim/${claimId}/review`, {
      result: 2,
      comment: 'Not good enough',
    }, bizToken);
    expect(r.code).toBe(0);
  });

  test('TC-REVIEW-04: get all claims as business', async ({ request }) => {
    const { bizToken } = await setupSubmittedClaim(request);
    const r = await get(request, '/business/claims', bizToken);
    expect(r.code).toBe(0);
  });
});

// ─── WALLET ────────────────────────────────────────────────────────────────

test.describe('Wallet', () => {
  test('TC-WALLET-01: shared wallet endpoint', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/wallet', token);
    expect(r.code).toBe(0);
    expect(typeof r.data.balance).toBe('number');
  });

  test('TC-WALLET-02: balance starts at 0 for new user', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/wallet', token);
    expect(r.data.balance).toBe(0);
  });

  test('TC-WALLET-03: multiple recharges accumulate', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 300 }, token);
    await post(request, '/business/recharge', { amount: 200 }, token);
    const r = await get(request, '/wallet', token);
    expect(r.data.balance).toBe(500);
  });
});

// ─── MESSAGES ──────────────────────────────────────────────────────────────

test.describe('Messages', () => {
  test('TC-MSG-01: list messages', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/messages', token);
    expect(r.code).toBe(0);
  });

  test('TC-MSG-02: get unread count', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/messages/unread-count', token);
    expect(r.code).toBe(0);
    expect(typeof r.data.count).toBe('number');
  });

  test('TC-MSG-03: mark all read', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await post(request, '/messages/read-all', {}, token);
    expect(r.code).toBe(0);
  });
});

// ─── EDGE CASES ────────────────────────────────────────────────────────────

test.describe('Edge Cases', () => {
  test('TC-EDGE-01: unauthenticated access to protected endpoint', async ({ request }) => {
    const r = await get(request, '/business/tasks');
    expect(r.code).not.toBe(0);
  });

  test('TC-EDGE-02: invalid token rejected', async ({ request }) => {
    const r = await get(request, '/users/me', 'invalid.token.here');
    expect(r.code).not.toBe(0);
  });

  test('TC-EDGE-03: recharge with zero amount fails', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await post(request, '/business/recharge', { amount: 0 }, token);
    expect(r.code).not.toBe(0);
  });

  test('TC-EDGE-04: recharge with negative amount fails', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await post(request, '/business/recharge', { amount: -100 }, token);
    expect(r.code).not.toBe(0);
  });

  test('TC-EDGE-05: claim nonexistent task fails', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await post(request, '/creator/claim', { task_id: 99999999 }, token);
    expect(r.code).not.toBe(0);
  });

  test('TC-EDGE-06: register with missing fields fails', async ({ request }) => {
    const r = await post(request, '/auth/register', { username: uid() }); // no password/phone
    expect(r.code).not.toBe(0);
  });
});

// ─── USER UPDATE ───────────────────────────────────────────────────────────

test.describe('User Update', () => {
  test('TC-USER-01: PUT /users/me updates nickname', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const newNick = 'Nick_' + uid();
    const r = await put(request, '/users/me', { nickname: newNick }, token);
    expect(r.code).toBe(0);
    expect(r.data.nickname).toBe(newNick);
  });

  test('TC-USER-02: PUT /users/me updates avatar', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await put(request, '/users/me', { avatar: 'https://example.com/avatar.png' }, token);
    expect(r.code).toBe(0);
    expect(r.data.avatar).toBe('https://example.com/avatar.png');
  });

  test('TC-USER-03: PUT /users/me requires auth', async ({ request }) => {
    const r = await put(request, '/users/me', { nickname: 'test' }, 'bad.token');
    expect(r.code).not.toBe(0);
  });
});

// ─── CREDITS ───────────────────────────────────────────────────────────────

test.describe('Credits', () => {
  test('TC-CREDIT-01: new user has default credits', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/users/credits', token);
    expect(r.code).toBe(0);
    expect(typeof r.data.total_score).toBe('number');
    expect(r.data.total_score).toBeGreaterThanOrEqual(0);
  });

  test('TC-CREDIT-02: credits include level info', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/users/credits', token);
    expect(r.code).toBe(0);
    expect(r.data.level).toBeDefined();
    expect(r.data.level_name).toBeTruthy();
  });

  test('TC-CREDIT-03: credits requires auth', async ({ request }) => {
    const r = await get(request, '/users/credits');
    expect(r.code).not.toBe(0);
  });
});

// ─── MESSAGES EXTENDED ─────────────────────────────────────────────────────

test.describe('Messages Extended', () => {
  test('TC-MSG-04: mark single message as read', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    // Mark message id=1 as read (returns success even if msg doesn't belong to user)
    const r = await post(request, '/messages/1/read', {}, token);
    expect(r.code).toBe(0);
  });

  test('TC-MSG-05: delete a message', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await del(request, '/messages/1', token);
    expect(r.code).toBe(0);
  });

  test('TC-MSG-06: list messages returns pagination info', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/messages', token);
    expect(r.code).toBe(0);
    expect(typeof r.data.total).toBe('number');
    expect(typeof r.data.page).toBe('number');
    expect(typeof r.data.limit).toBe('number');
  });
});

// ─── CHARTS ────────────────────────────────────────────────────────────────

test.describe('Charts', () => {
  test('TC-CHART-01: creator income chart returns code 0', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/creator/chart/income', token);
    expect(r.code).toBe(0);
  });

  test('TC-CHART-02: business expense chart returns code 0', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/business/chart/expense', token);
    expect(r.code).toBe(0);
  });

  test('TC-CHART-03: charts require auth', async ({ request }) => {
    const r1 = await get(request, '/creator/chart/income');
    expect(r1.code).not.toBe(0);
    const r2 = await get(request, '/business/chart/expense');
    expect(r2.code).not.toBe(0);
  });
});

// ─── APPEALS ───────────────────────────────────────────────────────────────

test.describe('Appeals', () => {
  // Helper: business creates task, creator claims it, returns IDs
  async function setupClaim(request: APIRequestContext) {
    const biz = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 500 }, biz.token);
    const task = await post(request, '/business/tasks', {
      title: 'appeal_task_' + uid(),
      description: 'test',
      category: 1,
      unit_price: 50,
      total_count: 2,
    }, biz.token);
    const taskId = task.data.task_id ?? task.data.id;
    const creator = await registerAndLogin(request);
    const claim = await post(request, '/creator/claim', { task_id: taskId }, creator.token);
    const claimId = claim.data.claim_id ?? claim.data.id;
    return { bizToken: biz.token, creatorToken: creator.token, taskId, claimId };
  }

  test('TC-APPEAL-01: create task appeal', async ({ request }) => {
    const { creatorToken, taskId } = await setupClaim(request);
    const r = await post(request, '/appeals', {
      type: 1,
      target_id: taskId,
      reason: 'Task requirements unclear',
    }, creatorToken);
    expect(r.code).toBe(0);
    expect(r.data.id).toBeTruthy();
  });

  test('TC-APPEAL-02: create submission appeal', async ({ request }) => {
    const { creatorToken, claimId } = await setupClaim(request);
    const r = await post(request, '/appeals', {
      type: 2,
      target_id: claimId,
      reason: 'Review result unfair',
      evidence: 'https://example.com/evidence.jpg',
    }, creatorToken);
    expect(r.code).toBe(0);
    expect(r.data.id).toBeTruthy();
  });

  test('TC-APPEAL-03: list my appeals', async ({ request }) => {
    const { creatorToken, taskId } = await setupClaim(request);
    await post(request, '/appeals', { type: 1, target_id: taskId, reason: 'test' }, creatorToken);
    const r = await get(request, '/appeals', creatorToken);
    expect(r.code).toBe(0);
    const list = Array.isArray(r.data) ? r.data : (r.data?.appeals ?? r.data?.data ?? []);
    expect(Array.isArray(list)).toBeTruthy();
    expect(list.length).toBeGreaterThan(0);
  });

  test('TC-APPEAL-04: get appeal detail', async ({ request }) => {
    const { creatorToken, taskId } = await setupClaim(request);
    const created = await post(request, '/appeals', { type: 1, target_id: taskId, reason: 'test' }, creatorToken);
    expect(created.code).toBe(0);
    const appealId = created.data.id;
    const r = await get(request, `/appeals/${appealId}`, creatorToken);
    expect(r.code).toBe(0);
    expect(r.data.id).toBe(appealId);
  });

  test('TC-APPEAL-05: appeal requires auth', async ({ request }) => {
    const r = await post(request, '/appeals', { type: 1, target_id: 1, reason: 'test' });
    expect(r.code).not.toBe(0);
  });

  test('TC-APPEAL-06: appeal without reason still handled', async ({ request }) => {
    const { creatorToken, taskId } = await setupClaim(request);
    const r = await post(request, '/appeals', { type: 1, target_id: taskId }, creatorToken);
    // Either succeeds (reason optional) or fails with validation error (reason required)
    expect([0, 40001]).toContain(r.code);
  });
});

// ─── BUSINESS CLAIM DETAIL ─────────────────────────────────────────────────

test.describe('Business Claim Detail', () => {
  test('TC-BIZ-CLAIM-01: business can get individual claim', async ({ request }) => {
    const biz = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 500 }, biz.token);
    const task = await post(request, '/business/tasks', {
      title: 'claim_detail_' + uid(),
      description: 'test',
      category: 1,
      unit_price: 50,
      total_count: 2,
    }, biz.token);
    const taskId = task.data.task_id ?? task.data.id;

    const creator = await registerAndLogin(request);
    const claim = await post(request, '/creator/claim', { task_id: taskId }, creator.token);
    expect(claim.code).toBe(0);
    const claimId = claim.data.claim_id ?? claim.data.id;

    const r = await get(request, `/business/claim/${claimId}`, biz.token);
    expect(r.code).toBe(0);
    expect(r.data).toBeDefined();
  });

  test('TC-BIZ-CLAIM-02: claim detail requires auth', async ({ request }) => {
    const r = await get(request, '/business/claim/1');
    expect(r.code).not.toBe(0);
  });
});

// ─── TASK PAGINATION ───────────────────────────────────────────────────────

test.describe('Task Pagination', () => {
  test('TC-PAGE-01: public tasks returns pagination metadata', async ({ request }) => {
    const r = await get(request, '/tasks?page=1&limit=5');
    expect(r.code).toBe(0);
    expect(typeof r.data.total).toBe('number');
    expect(typeof r.data.page).toBe('number');
    expect(typeof r.data.limit).toBe('number');
    expect(Array.isArray(r.data.data)).toBeTruthy();
  });

  test('TC-PAGE-02: business tasks supports pagination', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/business/tasks?page=1&limit=10', token);
    expect(r.code).toBe(0);
  });

  test('TC-PAGE-03: creator tasks supports pagination', async ({ request }) => {
    const { token } = await registerAndLogin(request);
    const r = await get(request, '/creator/tasks?page=1&limit=10', token);
    expect(r.code).toBe(0);
  });
});

// ─── INTEGRATED FLOWS ──────────────────────────────────────────────────────

test.describe('Integrated Flows', () => {
  test('FLOW-01: full lifecycle (register→recharge→task→claim→submit→approve→payment)', async ({ request }) => {
    // Business setup
    const biz = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 1000 }, biz.token);

    const walletBefore = await get(request, '/wallet', biz.token);
    expect(walletBefore.data.balance).toBe(1000);

    // Create task (escrows 200)
    const task = await post(request, '/business/tasks', {
      title: 'lifecycle_' + uid(),
      description: 'Full lifecycle test',
      category: 1,
      unit_price: 200,
      total_count: 2,
    }, biz.token);
    expect(task.code).toBe(0);
    const taskId = task.data.task_id ?? task.data.id;

    const walletAfterTask = await get(request, '/wallet', biz.token);
    expect(walletAfterTask.data.balance).toBe(600); // 1000 - 200*2

    // Creator claims
    const creator = await registerAndLogin(request);
    const claim = await post(request, '/creator/claim', { task_id: taskId }, creator.token);
    expect(claim.code).toBe(0);
    const claimId = claim.data.claim_id ?? claim.data.id;

    // Creator submits
    const submit = await put(request, `/creator/claim/${claimId}/submit`, {
      content: 'Final submission',
    }, creator.token);
    expect(submit.code).toBe(0);

    // Business approves
    const review = await put(request, `/business/claim/${claimId}/review`, {
      result: 1,
      comment: 'Approved!',
    }, biz.token);
    expect(review.code).toBe(0);

    // Creator was paid
    const creatorWallet = await get(request, '/creator/wallet', creator.token);
    expect(creatorWallet.data.balance).toBeGreaterThan(0);
  });

  test('FLOW-02: business transaction history reflects recharge + task creation', async ({ request }) => {
    const biz = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 500 }, biz.token);
    await post(request, '/business/tasks', {
      title: 'tx_task_' + uid(),
      description: 'test',
      category: 1,
      unit_price: 100,
      total_count: 2,
    }, biz.token);

    const txs = await get(request, '/business/transactions', biz.token);
    expect(txs.code).toBe(0);

    const wallet = await get(request, '/wallet', biz.token);
    expect(wallet.data.balance).toBe(300); // 500 - 200
  });

  test('FLOW-03: creator sees claim in list after claiming', async ({ request }) => {
    const biz = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 500 }, biz.token);
    const task = await post(request, '/business/tasks', {
      title: 'list_claim_' + uid(),
      description: 'test',
      category: 1,
      unit_price: 100,
      total_count: 3,
    }, biz.token);
    const taskId = task.data.task_id ?? task.data.id;

    const creator = await registerAndLogin(request);
    await post(request, '/creator/claim', { task_id: taskId }, creator.token);

    const claims = await get(request, '/creator/claims', creator.token);
    expect(claims.code).toBe(0);
    const list = Array.isArray(claims.data) ? claims.data : (claims.data?.claims ?? claims.data?.data ?? []);
    expect(Array.isArray(list)).toBeTruthy();
    expect(list.length).toBeGreaterThan(0);
  });

  test('FLOW-04: cancel task before any claims refunds full amount', async ({ request }) => {
    const biz = await registerAndLogin(request);
    await post(request, '/business/recharge', { amount: 1000 }, biz.token);
    const task = await post(request, '/business/tasks', {
      title: 'refund_task_' + uid(),
      description: 'test',
      category: 1,
      unit_price: 300,
      total_count: 2,
    }, biz.token);
    expect(task.code).toBe(0);
    const taskId = task.data.task_id ?? task.data.id;

    const walletMid = await get(request, '/wallet', biz.token);
    expect(walletMid.data.balance).toBe(400); // 1000 - 600

    const cancel = await del(request, `/business/tasks/${taskId}`, biz.token);
    expect(cancel.code).toBe(0);

    const walletFinal = await get(request, '/wallet', biz.token);
    expect(walletFinal.data.balance).toBe(1000);
  });
});
