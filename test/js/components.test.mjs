import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';

const componentsPath = path.resolve(process.cwd(), 'web/static/js/components.js');
const source = fs.readFileSync(componentsPath, 'utf8');
const templatesRoot = path.resolve(process.cwd(), 'web/templates');

function createLocalStorage() {
  const store = new Map();
  return {
    getItem(key) {
      return store.has(key) ? store.get(key) : null;
    },
    setItem(key, value) {
      store.set(key, String(value));
    },
    removeItem(key) {
      store.delete(key);
    }
  };
}

function loadComponents(fetchImpl) {
  const context = {
    console,
    setTimeout,
    clearTimeout,
    fetch: fetchImpl,
    window: { location: { pathname: '/', href: '/' } },
    document: {},
    bootstrap: { Toast: function Toast() {}, Modal: function Modal() {} },
    localStorage: createLocalStorage()
  };

  vm.createContext(context);
  vm.runInContext(
    `${source}
    globalThis.__test_apiRequest = apiRequest;
    globalThis.__test_storeAuthSession = typeof storeAuthSession === 'function' ? storeAuthSession : undefined;
    globalThis.__test_redirectToDashboard = typeof redirectToDashboard === 'function' ? redirectToDashboard : undefined;`,
    context
  );
  return {
    apiRequest: context.__test_apiRequest,
    storeAuthSession: context.__test_storeAuthSession,
    redirectToDashboard: context.__test_redirectToDashboard,
    context
  };
}

function walkHtmlFiles(dir) {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  const files = [];

  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...walkHtmlFiles(fullPath));
    } else if (entry.isFile() && entry.name.endsWith('.html')) {
      files.push(fullPath);
    }
  }

  return files;
}

test('apiRequest does not duplicate the /api/v1 prefix', async () => {
  let requestedUrl = null;
  const { apiRequest } = loadComponents((url) => {
    requestedUrl = url;
    return Promise.resolve({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ code: 0 })
    });
  });

  await apiRequest('/api/v1/auth/login', 'POST', { username: 'demo', password: 'demo' }, false);

  assert.equal(requestedUrl, '/api/v1/auth/login');
});

test('apiRequest still prefixes relative API endpoints', async () => {
  let requestedUrl = null;
  const { apiRequest } = loadComponents((url) => {
    requestedUrl = url;
    return Promise.resolve({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ code: 0 })
    });
  });

  await apiRequest('/creator/tasks', 'GET', null, false);

  assert.equal(requestedUrl, '/api/v1/creator/tasks');
});

test('storeAuthSession persists a successful auth response for later page loads', () => {
  const { storeAuthSession, context } = loadComponents(() => Promise.reject(new Error('fetch not expected')));

  assert.equal(typeof storeAuthSession, 'function');

  storeAuthSession({
    token: 'token-123',
    user: {
      id: 7,
      username: 'new_user',
      is_admin: false
    }
  }, 'creator');

  assert.equal(context.localStorage.getItem('token'), 'token-123');
  assert.equal(context.localStorage.getItem('user_id'), '7');
  assert.equal(context.localStorage.getItem('username'), 'new_user');
  assert.equal(context.localStorage.getItem('role'), 'creator');
  assert.equal(context.localStorage.getItem('roles'), 'business,creator');
  assert.equal(context.localStorage.getItem('current_role'), 'creator');
  assert.equal(context.localStorage.getItem('is_admin'), 'false');
});

test('redirectToDashboard routes creators to the creator dashboard', () => {
  const { redirectToDashboard, context } = loadComponents(() => Promise.reject(new Error('fetch not expected')));

  assert.equal(typeof redirectToDashboard, 'function');

  redirectToDashboard('creator');

  assert.equal(context.window.location.href, '/creator/dashboard.html');
});

test('templates using components.js do not hardcode the /api/v1 prefix in apiRequest calls', () => {
  const offenders = walkHtmlFiles(templatesRoot)
    .filter((file) => {
      const content = fs.readFileSync(file, 'utf8');
      return content.includes('/static/js/components.js') && /apiRequest\(\s*['"`]\/api\/v1/.test(content);
    })
    .map((file) => path.relative(process.cwd(), file));

  assert.deepEqual(offenders, []);
});
