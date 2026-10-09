// 账号与身份：注册、登录、token 轮换、me、登出。
import { ensureAccount, Client, expectStatus, expectField } from "../lib.mjs";

const WEAK_EMAIL = `miao-test-weak-${Date.now()}@example.test`;

export default [
  {
    name: "identity: 注册新账号并自动获得默认工作区",
    run: async (t) => {
      const owner = await ensureAccount(t.base, { ...t.accounts.owner });
      t.state.owner = owner;
      const me = await owner.get("/api/me");
      expectStatus(me, 200);
      expectField(me, "body.user.email", (v) => v === t.accounts.owner.email, "me email");
      expectField(me, "body.tenant", (v) => v && v.id, "me tenant");
      expectField(me, "body.workspaces", (v) => Array.isArray(v) && v.length >= 1, "me workspaces");
      t.state.tenantId = me.body.tenant.id;
      owner.setTenant(t.state.tenantId);
    },
  },
  {
    name: "identity: 短密码注册被拒绝",
    run: async (t) => {
      const res = await t.anon.post("/api/auth/register", {
        email: WEAK_EMAIL,
        password: "1234567",
        name: "弱密码",
      });
      expectStatus(res, 400);
    },
  },
  {
    name: "identity: 错误密码登录返回 401",
    run: async (t) => {
      const res = await t.anon.post("/api/auth/login", {
        email: t.accounts.owner.email,
        password: "WrongPassword!99",
      });
      expectStatus(res, 401);
    },
  },
  {
    name: "identity: 正确密码登录成功",
    run: async (t) => {
      const res = await t.anon.post("/api/auth/login", { ...t.accounts.owner });
      expectStatus(res, 200);
      expectField(res, "body.token", (v) => typeof v === "string" && v.length > 10, "login token");
    },
  },
  {
    name: "identity: PATCH /api/me 更新界面语言",
    run: async (t) => {
      const owner = t.state.owner;
      const patch = await owner.patch("/api/me", { language: "zh" });
      expectStatus(patch, 200);
      expectField(patch, "body.user.language", (v) => String(v).startsWith("zh"), "语言设置");
      const bad = await owner.patch("/api/me", { language: "xx" });
      expectStatus(bad, 400, "非法语言代码");
    },
  },
  {
    name: "identity: 响应中的 X-PocketBase-Token 可用于新客户端（token 轮换）",
    run: async (t) => {
      const owner = t.state.owner;
      const me = await owner.get("/api/me");
      expectStatus(me, 200);
      const fresh = me.headers.get("x-pocketbase-token");
      if (!fresh) throw new Error("响应未携带 X-PocketBase-Token 头");
      const rotated = new Client(t.base, { token: fresh, name: "rotated" });
      rotated.setTenant(t.state.tenantId);
      const viaRotated = await rotated.get("/api/me");
      expectStatus(viaRotated, 200);
    },
  },
  {
    name: "identity: 登出撤销该账号全部会话，旧 token 立即失效",
    run: async (t) => {
      const login = await t.anon.post("/api/auth/login", { ...t.accounts.owner });
      expectStatus(login, 200);
      const scratch = new Client(t.base, { token: login.body.token, name: "scratch" });
      const before = await scratch.get("/api/me");
      expectStatus(before, 200);
      const logout = await scratch.post("/api/auth/logout");
      expectStatus(logout, 200);
      const after = await scratch.get("/api/me");
      expectStatus(after, 401, "登出后旧 token 应失效");
      // 登出会撤销该账号的所有会话（含 state.owner），重新登录恢复。
      t.state.owner = await ensureAccount(t.base, t.accounts.owner);
      const restored = await t.state.owner.get("/api/me");
      expectStatus(restored, 200, "重新登录后恢复会话");
    },
  },
];
