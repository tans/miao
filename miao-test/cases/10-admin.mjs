// 平台管理员控制台接口。依赖 miao.env 中 MIAO_ADMIN_EMAILS 包含 miao-test-admin@example.test。
import { expectStatus, expectField, ensureAccount } from "../lib.mjs";

const ADMIN_EMAIL = "miao-test-admin@example.test";

export default [
  {
    name: "admin: 平台管理员标志生效",
    run: async (t) => {
      const me = await t.state.admin.get("/api/me");
      expectStatus(me, 200);
      expectField(me, "body.user.email", (v) => v === ADMIN_EMAIL);
      expectField(me, "body.is_platform_admin", (v) => v === true, "is_platform_admin");
    },
  },
  {
    name: "admin: 概览 / 用户 / 工作区 / 应用 / 审计 / 用量可读",
    run: async (t) => {
      const admin = t.state.admin;
      const overview = await admin.get("/api/admin/overview");
      expectStatus(overview, 200, "平台概览");
      const users = await admin.get("/api/admin/users");
      expectStatus(users, 200);
      const rows = users.body.items ?? users.body.users ?? users.body;
      if (!Array.isArray(rows) || !rows.some((u) => u.email === t.accounts.owner.email)) {
        throw new Error(`管理员用户列表缺少测试 owner: ${JSON.stringify(users.body).slice(0, 300)}`);
      }
      t.state.outsiderUserId = rows.find((u) => u.email === t.accounts.outsider.email)?.id;
      for (const path of [
        "/api/admin/workspaces",
        "/api/admin/apps",
        "/api/admin/audit",
        "/api/admin/runtime",
        "/api/admin/settings",
        "/api/admin/usage",
        "/api/admin/usage/requests",
      ]) {
        const res = await admin.get(path);
        expectStatus(res, 200, `管理员接口 ${path}`);
      }
    },
  },
  {
    name: "admin: 普通用户访问管理员接口被拒绝",
    run: async (t) => {
      const res = await t.state.owner.get("/api/admin/overview");
      expectStatus(res, [403, 404], "普通用户访问 /api/admin/*");
    },
  },
  {
    name: "admin: 停用账号后其请求被拒绝，再启用恢复",
    run: async (t) => {
      const { admin, outsider, outsiderUserId } = t.state;
      if (!outsiderUserId) throw new Error("找不到外部测试账号");
      const disable = await admin.patch(`/api/admin/users/${outsiderUserId}/status`, {
        disabled: true,
        reason: "miao-test 停用演练",
      });
      expectStatus(disable, 200, "停用账号");
      const blocked = await outsider.get("/api/me");
      expectStatus(blocked, 403, "停用账号访问被拒绝");
      const enable = await admin.patch(`/api/admin/users/${outsiderUserId}/status`, {
        disabled: false,
        reason: "miao-test 恢复演练",
      });
      expectStatus(enable, 200, "恢复账号");
      let restored = await outsider.get("/api/me");
      if (restored.status !== 200) {
        // 停用/恢复或租户头残留时重新登录并回到自己的默认工作区。
        t.state.outsider = await ensureAccount(t.base, t.accounts.outsider);
        restored = await t.state.outsider.get("/api/me");
      }
      expectStatus(restored, 200, "恢复后访问正常");
    },
  },
  {
    name: "admin: 缺少操作原因的账号状态变更被拒绝",
    run: async (t) => {
      const res = await t.state.admin.patch(`/api/admin/users/${t.state.outsiderUserId}/status`, {
        disabled: true,
        reason: "短",
      });
      expectStatus(res, 400, "原因长度校验");
    },
  },
];
