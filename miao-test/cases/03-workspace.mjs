// 工作区：成员、邀请、角色、审计、AI 用量。
import { expectStatus, expectField } from "../lib.mjs";

const WS2_NAME = "miao-test 项目空间";

export default [
  {
    name: "workspace: 创建第二个工作区并切换上下文",
    run: async (t) => {
      const owner = t.state.owner;
      const me = await owner.get("/api/me");
      expectStatus(me, 200);
      const existing = me.body.workspaces.find((w) => w.name === WS2_NAME);
      if (existing) {
        t.state.tenant2Id = existing.id;
        owner.setTenant(t.state.tenant2Id);
        const back = await owner.get("/api/me");
        expectField(back, "body.tenant.id", (v) => v === t.state.tenant2Id, "切换到已有项目空间");
        owner.setTenant(t.state.tenantId);
        return;
      }
      const created = await owner.post("/api/workspaces", { name: WS2_NAME });
      expectStatus(created, 201);
      expectField(created, "body.id", (v) => typeof v === "string" && v.length > 0, "新空间 id");
      t.state.tenant2Id = created.body.id;
      owner.setTenant(t.state.tenant2Id);
      const switched = await owner.get("/api/me");
      expectField(switched, "body.tenant.name", (v) => v === WS2_NAME, "当前空间切换");
      owner.setTenant(t.state.tenantId);
    },
  },
  {
    name: "workspace: 列出成员，owner 在列",
    run: async (t) => {
      const res = await t.state.owner.get("/api/workspace/members");
      expectStatus(res, 200);
      const members = res.body.members ?? [];
      if (!members.some((m) => m.role === "owner" && m.email === t.accounts.owner.email)) {
        throw new Error(`成员列表中找不到 owner: ${JSON.stringify(members).slice(0, 300)}`);
      }
    },
  },
  {
    name: "workspace: 邀请 editor 和 viewer 加入并成功接受",
    run: async (t) => {
      const { owner } = t.state;
      for (const role of ["editor", "viewer"]) {
        const member = t.state[role];
        const me = await member.get("/api/me");
        expectStatus(me, 200);
        if (me.body.workspaces.some((w) => w.id === t.state.tenantId)) {
          continue; // 已加入（重复运行）
        }
        // 可能残留上一轮未接受的邀请：先撤销再重新邀请（邀请令牌只在创建时返回）。
        const invites = await owner.get("/api/workspace/invites");
        expectStatus(invites, 200);
        const pending = (invites.body ?? []).find((inv) => inv.email === t.accounts[role].email);
        if (pending) {
          const revoke = await owner.delete(`/api/workspace/invites/${pending.id}`);
          expectStatus(revoke, [200, 204], `撤销残留邀请（${role}）`);
        }
        const invite = await owner.post("/api/workspace/invites", { email: t.accounts[role].email });
        expectStatus(invite, 201, `邀请 ${role}`);
        const token = invite.body.invite_url?.split("invite=")[1];
        if (!token) throw new Error(`邀请响应缺少 invite_url: ${JSON.stringify(invite.body).slice(0, 300)}`);
        const accept = await member.post("/api/invites/accept", { token: decodeURIComponent(token) });
        expectStatus(accept, 200, `${role} 接受邀请`);
        expectField(accept, "body.tenant.name", (v) => typeof v === "string" && v.length > 0, "接受邀请后的空间");
      }
      const editor = t.state.editor;
      editor.setTenant(t.state.tenantId);
      const after = await editor.get("/api/me");
      expectStatus(after, 200, "editor 可切换到被邀请的工作区");
      editor.setTenant(null);
    },
  },
  {
    name: "workspace: 重复邀请已有待接受邀请的邮箱返回 409",
    run: async (t) => {
      const owner = t.state.owner;
      const tempEmail = `miao-test-pending-${Date.now()}@example.test`;
      const first = await owner.post("/api/workspace/invites", { email: tempEmail });
      expectStatus(first, 201, "首次邀请");
      const second = await owner.post("/api/workspace/invites", { email: tempEmail });
      expectStatus(second, 409, "重复邀请待接受邮箱");
    },
  },
  {
    name: "workspace: 再次邀请已加入成员返回 409",
    run: async (t) => {
      const res = await t.state.owner.post("/api/workspace/invites", { email: t.accounts.editor.email });
      expectStatus(res, 409, "已在工作区中的用户不能再次被邀请");
    },
  },
  {
    name: "workspace: 邀请无效邮箱返回 400",
    run: async (t) => {
      const res = await t.state.owner.post("/api/workspace/invites", { email: "not-an-email" });
      expectStatus(res, 400);
    },
  },
  {
    name: "workspace: 调整 editor 为 admin 成员，非法角色被拒绝",
    run: async (t) => {
      const { owner } = t.state;
      const list = await owner.get("/api/workspace/members");
      expectStatus(list, 200);
      const editorRow = list.body.members.find((m) => m.email === t.accounts.editor.email);
      if (!editorRow?.membership_id) throw new Error("找不到 editor 成员记录");
      const bad = await owner.patch(`/api/workspace/members/${editorRow.membership_id}`, { role: "boss" });
      expectStatus(bad, 400, "非法成员角色");
      const ok = await owner.patch(`/api/workspace/members/${editorRow.membership_id}`, { role: "admin" });
      expectStatus(ok, 200);
    },
  },
  {
    name: "workspace: 审计与 AI 用量接口可读",
    run: async (t) => {
      const audit = await t.state.owner.get("/api/workspace/audit");
      expectStatus(audit, 200);
      const usage = await t.state.owner.get("/api/workspace/ai-usage");
      expectStatus(usage, 200);
    },
  },
  {
    name: "workspace: 外部用户无权读取成员列表",
    run: async (t) => {
      const outsider = t.state.outsider;
      outsider.setTenant(t.state.tenant2Id);
      const res = await outsider.get("/api/workspace/members");
      expectStatus(res, 403, "非成员读取其他工作区成员");
      outsider.setTenant(t.state.outsiderHomeTenantId);
    },
  },
];
