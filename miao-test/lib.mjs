// Minimal HTTP client + helpers for the miao-test local API suite.
// The miao API rotates account tokens: every authenticated response carries a
// fresh token in the X-PocketBase-Token header, and this client adopts it.

export class Client {
  constructor(base, { token = "", tenantId = null, name = "client" } = {}) {
    this.base = base.replace(/\/+$/, "");
    this.name = name;
    this._token = token;
    this._tenantId = tenantId;
  }

  get token() {
    return this._token;
  }

  setToken(token) {
    this._token = token;
  }

  setTenant(id) {
    this._tenantId = id;
  }

  async req(method, path, body, { expectAuth = true } = {}) {
    const headers = { "Content-Type": "application/json" };
    if (this._token) headers.Authorization = `Bearer ${this._token}`;
    if (this._tenantId) headers["X-Miao-Tenant-Id"] = this._tenantId;
    const res = await fetch(this.base + path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      redirect: "manual",
    });
    const text = await res.text();
    let data = null;
    if (text) {
      try {
        data = JSON.parse(text);
      } catch {
        data = text;
      }
    }
    const fresh = res.headers.get("x-pocketbase-token");
    if (fresh && this._token) this._token = fresh;
    return { status: res.status, body: data, headers: res.headers };
  }

  get(path, opts) {
    return this.req("GET", path, undefined, opts);
  }
  post(path, body, opts) {
    return this.req("POST", path, body, opts);
  }
  put(path, body, opts) {
    return this.req("PUT", path, body, opts);
  }
  patch(path, body, opts) {
    return this.req("PATCH", path, body, opts);
  }
  delete(path, body, opts) {
    return this.req("DELETE", path, body, opts);
  }
}

export async function ensureAccount(base, { email, name, password, asAdmin = null }) {
  // Login first so re-runs reuse a stable account.
  const login = await new Client(base).post("/api/auth/login", { email, password });
  if (login.status === 200) {
    return new Client(base, { token: login.body.token, name: email });
  }
  if (login.status !== 401) throw new Error(`login unexpected for ${email}: ${login.status} ${JSON.stringify(login.body)}`);
  const reg = await new Client(base).post("/api/auth/register", { email, password, name });
  if (reg.status !== 201) throw new Error(`register failed for ${email}: ${reg.status} ${JSON.stringify(reg.body)}`);
  return new Client(base, { token: reg.body.token, name: email });
}

export class AssertionError extends Error {}

function describe(res) {
  return `status=${res.status} body=${JSON.stringify(res.body)?.slice(0, 400)}`;
}

export function expectStatus(res, expected, what = "expected status") {
  const allowed = Array.isArray(expected) ? expected : [expected];
  if (!allowed.includes(res.status)) {
    throw new AssertionError(`${what}: expected ${allowed.join(" or ")} but got ${describe(res)}`);
  }
}

export function expectOk(res, what = "expected 2xx") {
  if (res.status < 200 || res.status >= 300) {
    throw new AssertionError(`${what}: ${describe(res)}`);
  }
}

export function expectField(res, path, predicate, what) {
  let value = res;
  for (const part of path.split(".")) value = value?.[part];
  if (predicate !== undefined && !predicate(value)) {
    throw new AssertionError(`${what ?? `field ${path} mismatch`}: got ${JSON.stringify(value)}`);
  }
  return value;
}

// Extract a record's stable "updated_at" timestamp from list/get results.
export function updatedAt(record) {
  return record?.updated_at ?? record?.updated ?? record?.data?.updated_at;
}
