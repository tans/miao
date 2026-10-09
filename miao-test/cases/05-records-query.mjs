// 记录 CRUD、乐观锁、查询条件与分页。
import { expectStatus, expectField, updatedAt } from "../lib.mjs";

const DUE_BASE = "2026-11-01";

export function seedOrders(t) {
  return [
    { data: { customer: "上海云帆科技", amount: 1200, status: "new", due: DUE_BASE, note: "首单" } },
    { data: { customer: "北京星轨网络", amount: 860, status: "new", due: "2026-11-05" } },
    { data: { customer: "深圳蓝海数据", amount: 3400, status: "in_progress", due: "2026-11-08" } },
    { data: { customer: "杭州白鹭智能", amount: 990, status: "in_progress", due: "2026-11-12", note: "含培训" } },
    { data: { customer: "成都山川能源", amount: 450, status: "new", due: "2026-11-20" } },
  ];
}

export default [
  {
    name: "record: 批量创建 5 条订单记录",
    run: async (t) => {
      const { appId, owner, ordersSlug } = t.state;
      const ids = [];
      for (const row of seedOrders(t)) {
        const res = await owner.post(`/api/apps/${appId}/collections/${ordersSlug}/records`, row);
        expectStatus(res, 201, `创建记录 ${row.data.customer}`);
        expectField(res, "body.id", (v) => typeof v === "string" && v.length > 0);
        ids.push(res.body.id);
      }
      t.state.orderIds = ids;
      const list = await owner.get(`/api/apps/${appId}/collections/${ordersSlug}/records`);
      expectStatus(list, 200);
      expectField(list, "body.totalItems", (v) => v >= 5, "至少 5 条订单");
    },
  },
  {
    name: "record: 缺少必填字段被拒绝",
    run: async (t) => {
      const { appId, owner, ordersSlug } = t.state;
      const res = await owner.post(`/api/apps/${appId}/collections/${ordersSlug}/records`, {
        data: { amount: 1 },
      });
      expectStatus(res, 400, "缺少必填 customer");
    },
  },
  {
    name: "record: 数值字段写入非法值被拒绝",
    run: async (t) => {
      const { appId, owner, ordersSlug } = t.state;
      const res = await owner.post(`/api/apps/${appId}/collections/${ordersSlug}/records`, {
        data: { customer: "坏数据", amount: "不是数字" },
      });
      expectStatus(res, 400, "数值字段类型校验");
    },
  },
  {
    name: "record: 详情读取与乐观锁更新",
    run: async (t) => {
      const { appId, owner, ordersSlug, orderIds } = t.state;
      const id = orderIds[0];
      const before = await owner.get(`/api/apps/${appId}/collections/${ordersSlug}/records/${id}`);
      expectStatus(before, 200);
      const stamp = updatedAt(before.body);
      const ok = await owner.patch(`/api/apps/${appId}/collections/${ordersSlug}/records/${id}`, {
        data: { note: "已联系客户" },
        expected_updated_at: stamp,
      });
      expectStatus(ok, 200, "使用正确 updated_at 更新");
      const stale = await owner.patch(`/api/apps/${appId}/collections/${ordersSlug}/records/${id}`, {
        data: { note: "过期并发写" },
        expected_updated_at: stamp,
      });
      expectStatus(stale, 409, "使用过期 updated_at 应冲突");
    },
  },
  {
    name: "record: viewer 不能写入记录",
    run: async (t) => {
      const { appId, viewer, ordersSlug } = t.state;
      viewer.setTenant(t.state.tenantId);
      const res = await viewer.post(`/api/apps/${appId}/collections/${ordersSlug}/records`, {
        data: { customer: "越权写入" },
      });
      expectStatus(res, 403, "viewer 写记录");
    },
  },
  {
    name: "record: 删除记录后不可再读取",
    run: async (t) => {
      const { appId, owner, ordersSlug } = t.state;
      const res = await owner.post(`/api/apps/${appId}/collections/${ordersSlug}/records`, {
        data: { customer: "待删除", amount: 1 },
      });
      expectStatus(res, 201);
      const id = res.body.id;
      const del = await owner.delete(`/api/apps/${appId}/collections/${ordersSlug}/records/${id}`);
      expectStatus(del, [200, 204]);
      const gone = await owner.get(`/api/apps/${appId}/collections/${ordersSlug}/records/${id}`);
      expectStatus(gone, 404, "已删除记录");
    },
  },
  {
    name: "query: eq / contains / before / after / empty 条件",
    run: async (t) => {
      const { appId, owner, ordersSlug } = t.state;
      const base = `/api/apps/${appId}/query`;
      const eq = await owner.post(base, { table: ordersSlug, conditions: [{ field: "status", op: "eq", value: "in_progress" }] });
      expectStatus(eq, 200);
      expectField(eq, "body.totalItems", (v) => v >= 2, "in_progress 订单数");
      const contains = await owner.post(base, { table: ordersSlug, conditions: [{ field: "customer", op: "contains", value: "蓝海" }] });
      expectField(contains, "body.totalItems", (v) => v >= 1, "contains 客户名");
      const before = await owner.post(base, { table: ordersSlug, conditions: [{ field: "due", op: "before", value: "2026-11-10" }] });
      expectField(before, "body.totalItems", (v) => v >= 3, "到期日早于 11-10");
      const after = await owner.post(base, { table: ordersSlug, conditions: [{ field: "due", op: "after", value: "2026-11-10" }] });
      expectField(after, "body.totalItems", (v) => v >= 1, "到期日晚于 11-10");
      const empty = await owner.post(base, { table: ordersSlug, conditions: [{ field: "note", op: "empty" }] });
      expectField(empty, "body.totalItems", (v) => v >= 2, "备注为空的记录");
    },
  },
  {
    name: "query: 非法操作符、未知字段、日期条件用错字段被拒绝",
    run: async (t) => {
      const { appId, owner, ordersSlug } = t.state;
      const base = `/api/apps/${appId}/query`;
      const badOp = await owner.post(base, { table: ordersSlug, conditions: [{ field: "status", op: "regex", value: "x" }] });
      expectStatus(badOp, 400, "非法操作符");
      const badField = await owner.post(base, { table: ordersSlug, conditions: [{ field: "nope", op: "eq", value: "x" }] });
      expectStatus(badField, 400, "未知字段");
      const badDate = await owner.post(base, { table: ordersSlug, conditions: [{ field: "customer", op: "before", value: "2026-01-01" }] });
      expectStatus(badDate, 400, "日期条件用于文本字段");
    },
  },
  {
    name: "query: 超过 8 个条件被拒绝",
    run: async (t) => {
      const { appId, owner, ordersSlug } = t.state;
      const conditions = Array.from({ length: 9 }, (_, i) => ({ field: "status", op: "eq", value: "new" }));
      const res = await owner.post(`/api/apps/${appId}/query`, { table: ordersSlug, conditions });
      expectStatus(res, 400, "条件数量上限");
    },
  },
  {
    name: "query: 分页返回一致",
    run: async (t) => {
      const { appId, owner, ordersSlug } = t.state;
      const page1 = await owner.get(`/api/apps/${appId}/collections/${ordersSlug}/records?page=1`);
      expectStatus(page1, 200);
      expectField(page1, "body.page", (v) => v === 1, "页码");
      expectField(page1, "body.items", (v) => Array.isArray(v) && v.length >= 1, "首页有数据");
    },
  },
];
