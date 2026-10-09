// 应用与数据表：创建应用、成员授权、建表、字段约束、关联字段。
import { expectStatus, expectField } from "../lib.mjs";

const APP_NAME = "miao-test 客户台账";

export default [
  {
    name: "app: 创建应用并获得 owner 权限",
    run: async (t) => {
      const res = await t.state.owner.post("/api/apps", {
        name: APP_NAME,
        description: "miao-test 自动生成的客户台账应用",
      });
      expectStatus(res, 201);
      expectField(res, "body.permission", (v) => v === "owner", "创建者权限");
      t.state.appId = res.body.id;
    },
  },
  {
    name: "app: 缺少名称的创建请求返回 400",
    run: async (t) => {
      const res = await t.state.owner.post("/api/apps", { name: "  " });
      expectStatus(res, 400);
    },
  },
  {
    name: "app: 列表与详情一致，外部用户不可见",
    run: async (t) => {
      const { appId, owner } = t.state;
      const list = await owner.get("/api/apps");
      expectStatus(list, 200);
      const found = list.body.find((a) => a.id === appId);
      if (!found) throw new Error("应用列表中找不到新建应用");
      const detail = await owner.get(`/api/apps/${appId}`);
      expectField(detail, "body.name", (v) => v === found.name, "应用名称一致");
      const outsider = t.state.outsider;
      outsider.setTenant(t.state.tenant2Id);
      const denied = await outsider.get(`/api/apps/${appId}`);
      expectStatus(denied, [403, 404], "外部用户访问他人应用");
    },
  },
  {
    name: "app: 重命名应用",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.patch(`/api/apps/${appId}`, { name: "miao-test 客户台账 v2" });
      expectStatus(res, 200);
      expectField(res, "body.name", (v) => v === "miao-test 客户台账 v2");
    },
  },
  {
    name: "app: 为 editor / viewer 授权（editor 无批量权限）",
    run: async (t) => {
      const { appId, owner, editor, viewer } = t.state;
      editor.setTenant(t.state.tenantId);
      // user id 与所在工作区无关，viewer 无需切换到 owner 工作区。
      const [editorMe, viewerMe] = await Promise.all([editor.get("/api/me"), viewer.get("/api/me")]);
      expectStatus(editorMe, 200);
      expectStatus(viewerMe, 200);
      const editorId = editorMe.body.user.id;
      const viewerId = viewerMe.body.user.id;
      const res = await owner.put(`/api/apps/${appId}/access`, {
        restricted: false,
        permissions: [
          { user_id: editorId, role: "editor", can_batch: false },
          { user_id: viewerId, role: "viewer" },
        ],
      });
      expectStatus(res, [200, 201]);
      const check = await editor.get(`/api/apps/${appId}`);
      expectStatus(check, 200);
      expectField(check, "body.permission", (v) => v === "editor", "editor 权限");
      viewer.setTenant(t.state.tenantId);
      const viewerCheck = await viewer.get(`/api/apps/${appId}`);
      expectField(viewerCheck, "body.permission", (v) => v === "viewer", "viewer 权限");
    },
  },
  {
    name: "table: 创建订单表（多类型字段）",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.post(`/api/apps/${appId}/collections`, {
        name: "订单",
        slug: "orders",
        fields: [
          { name: "customer", label: "客户", type: "text", required: true },
          { name: "amount", label: "金额", type: "number" },
          { name: "status", label: "状态", type: "select", options: ["new", "in_progress", "done"] },
          { name: "due", label: "到期日", type: "date" },
          { name: "note", label: "备注", type: "text" },
          { name: "attachment", label: "附件", type: "file" },
        ],
      });
      expectStatus(res, 201);
      t.state.ordersSlug = "orders";
    },
  },
  {
    name: "table: 创建客户表并给订单表加关联字段",
    run: async (t) => {
      const { appId, owner } = t.state;
      const customers = await owner.post(`/api/apps/${appId}/collections`, {
        name: "客户",
        slug: "customers",
        fields: [
          { name: "company", label: "公司", type: "text", required: true },
          { name: "city", label: "城市", type: "select", options: ["上海", "北京", "深圳"] },
          { name: "website", label: "网站", type: "url" },
        ],
      });
      expectStatus(customers, 201);
      t.state.customersSlug = "customers";
      const tables = await owner.get(`/api/apps/${appId}/collections`);
      expectStatus(tables, 200);
      const orders = tables.body.find((tbl) => tbl.slug === "orders");
      const updated = await owner.patch(`/api/apps/${appId}/collections/orders`, {
        name: "订单",
        fields: [...orders.fields, { name: "customer_ref", label: "客户档案", type: "relation", target: "customers" }],
      });
      expectStatus(updated, 200);
      expectField(updated, "body.fields", (v) => v.some((f) => f.name === "customer_ref" && f.type === "relation"), "关联字段");
    },
  },
  {
    name: "table: 重复 slug 与非法字段类型被拒绝",
    run: async (t) => {
      const { appId, owner } = t.state;
      const dup = await owner.post(`/api/apps/${appId}/collections`, {
        name: "订单again",
        slug: "orders",
        fields: [{ name: "x", type: "text" }],
      });
      expectStatus(dup, [400, 409], "重复表 slug");
      const badType = await owner.post(`/api/apps/${appId}/collections`, {
        name: "坏表",
        slug: "badtable",
        fields: [{ name: "x", type: "richtext" }],
      });
      expectStatus(badType, 400, "不支持的字段类型");
    },
  },
  {
    name: "table: viewer 不能创建数据表",
    run: async (t) => {
      const { appId, viewer } = t.state;
      viewer.setTenant(t.state.tenantId);
      const res = await viewer.post(`/api/apps/${appId}/collections`, {
        name: "越权表",
        fields: [{ name: "x", type: "text" }],
      });
      expectStatus(res, 403, "viewer 建表");
    },
  },
];
