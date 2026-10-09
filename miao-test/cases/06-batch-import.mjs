// 批量修改计划与导入计划：审阅后提交，越权与非法数据被拒绝。
import { expectStatus, expectField } from "../lib.mjs";

export default [
  {
    name: "batch: 创建批量修改计划（status=new 的订单改金额）",
    run: async (t) => {
      const { appId, owner, ordersSlug } = t.state;
      const res = await owner.post(`/api/apps/${appId}/batch-plans`, {
        table: ordersSlug,
        conditions: [{ field: "status", op: "eq", value: "new" }],
        change: { field: "amount", value: 666 },
      });
      expectStatus(res, 201);
      expectField(res, "body.status", (v) => v === "planned");
      expectField(res, "body.count", (v) => v >= 1, "命中记录数");
      t.state.batchJobId = res.body.plan_id;
    },
  },
  {
    name: "batch: 提交计划后记录确实被修改",
    run: async (t) => {
      const { appId, owner, ordersSlug, batchJobId } = t.state;
      const commit = await owner.post(`/api/apps/${appId}/batch-plans/${batchJobId}/commit`, {
        confirm: true,
        plan_id: batchJobId,
      });
      expectStatus(commit, 200);
      const verify = await owner.post(`/api/apps/${appId}/query`, {
        table: ordersSlug,
        conditions: [
          { field: "status", op: "eq", value: "new" },
          { field: "amount", op: "eq", value: 666 },
        ],
      });
      expectStatus(verify, 200);
      expectField(verify, "body.totalItems", (v) => v >= 1, "提交后金额已改为 666");
    },
  },
  {
    name: "batch: 命中 0 条记录的计划被拒绝",
    run: async (t) => {
      const { appId, owner, ordersSlug } = t.state;
      const res = await owner.post(`/api/apps/${appId}/batch-plans`, {
        table: ordersSlug,
        conditions: [{ field: "customer", op: "eq", value: "不存在的客户名xyz" }],
        change: { field: "amount", value: 1 },
      });
      expectStatus(res, 400, "0 条目标记录");
    },
  },
  {
    name: "batch: 批量修改附件/关联字段被拒绝",
    run: async (t) => {
      const { appId, owner, ordersSlug } = t.state;
      const res = await owner.post(`/api/apps/${appId}/batch-plans`, {
        table: ordersSlug,
        conditions: [{ field: "status", op: "eq", value: "new" }],
        change: { field: "attachment", value: "x.pdf" },
      });
      expectStatus(res, 400, "附件字段不可批量修改");
    },
  },
  {
    name: "batch: 无批量权限的 editor 被拒绝",
    run: async (t) => {
      const { appId, editor, ordersSlug } = t.state;
      editor.setTenant(t.state.tenantId);
      const res = await editor.post(`/api/apps/${appId}/batch-plans`, {
        table: ordersSlug,
        conditions: [{ field: "status", op: "eq", value: "new" }],
        change: { field: "amount", value: 1 },
      });
      expectStatus(res, 403, "editor can_batch=false");
    },
  },
  {
    name: "import: 创建导入计划并提交 3 个客户",
    run: async (t) => {
      const { appId, owner, customersSlug } = t.state;
      const rows = [
        { company: "导入客户 A", city: "上海" },
        { company: "导入客户 B", city: "北京", website: "https://example.com" },
        { company: "导入客户 C", city: "深圳" },
      ];
      const plan = await owner.post(`/api/apps/${appId}/import-plans`, {
        table: customersSlug,
        rows,
      });
      expectStatus(plan, 201);
      expectField(plan, "body.count", (v) => v === 3);
      t.state.importPlanId = plan.body.plan_id;
      const commit = await owner.post(`/api/apps/${appId}/import-plans/${plan.body.plan_id}/commit`, {
        confirm: true,
        plan_id: plan.body.plan_id,
      });
      expectStatus(commit, 200);
      const verify = await owner.post(`/api/apps/${appId}/query`, {
        table: customersSlug,
        conditions: [{ field: "company", op: "contains", value: "导入客户" }],
      });
      expectField(verify, "body.totalItems", (v) => v >= 3, "导入的客户可查询");
    },
  },
  {
    name: "import: 含未知字段的行在计划阶段即失败",
    run: async (t) => {
      const { appId, owner, customersSlug } = t.state;
      const res = await owner.post(`/api/apps/${appId}/import-plans`, {
        table: customersSlug,
        rows: [{ company: "坏行", not_a_field: 1 }],
      });
      expectStatus(res, 400);
      expectField(res, "body.errors", (v) => Array.isArray(v) && v.length >= 1, "逐行错误明细");
    },
  },
  {
    name: "import: 空行数组被拒绝",
    run: async (t) => {
      const { appId, owner, customersSlug } = t.state;
      const res = await owner.post(`/api/apps/${appId}/import-plans`, {
        table: customersSlug,
        rows: [],
      });
      expectStatus(res, 400);
    },
  },
];
