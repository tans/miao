// 通用状态流与业务动作：创建、启用、执行、幂等、非法转换。
import { expectStatus, expectField } from "../lib.mjs";

const WORKFLOW_DEFINITION = {
  table: "orders",
  state_field: "status",
  states: [
    { id: "new", label: "新订单" },
    { id: "in_progress", label: "进行中" },
    { id: "done", label: "已完成" },
  ],
  transitions: [
    { id: "start", label: "开工", from: "new", to: "in_progress" },
    { id: "finish", label: "完成", from: "in_progress", to: "done" },
  ],
};

export default [
  {
    name: "workflow: 创建订单状态流（draft）",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.post(`/api/apps/${appId}/workflows`, {
        name: "订单状态流",
        description: "miao-test 自动生成的状态流",
        definition: WORKFLOW_DEFINITION,
      });
      expectStatus(res, 201);
      expectField(res, "body.status", (v) => v === "draft");
      expectField(res, "body.revision", (v) => v === 1);
      t.state.workflowId = res.body.id;
      t.state.workflowRevision = res.body.revision;
    },
  },
  {
    name: "workflow: 状态不在字段选项中时被拒绝",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.post(`/api/apps/${appId}/workflows`, {
        name: "坏状态流",
        definition: {
          ...WORKFLOW_DEFINITION,
          states: [
            ...WORKFLOW_DEFINITION.states,
            { id: "cancelled", label: "已取消" },
          ],
        },
      });
      expectStatus(res, 400, "select 字段中不存在的状态");
    },
  },
  {
    name: "workflow: 启用状态流",
    run: async (t) => {
      const { appId, owner, workflowId, workflowRevision } = t.state;
      const res = await owner.post(`/api/apps/${appId}/workflows/${workflowId}/enable`, {
        confirm: true,
        expected_revision: workflowRevision,
      });
      expectStatus(res, 200);
      expectField(res, "body.status", (v) => v === "enabled");
    },
  },
  {
    name: "workflow: 合法转换 new -> in_progress 成功并写入记录",
    run: async (t) => {
      const { appId, owner, workflowId, ordersSlug } = t.state;
      const pick = await owner.post(`/api/apps/${appId}/query`, {
        table: ordersSlug,
        conditions: [{ field: "status", op: "eq", value: "new" }],
      });
      expectStatus(pick, 200);
      const record = pick.body.items?.[0];
      if (!record) throw new Error("没有 status=new 的订单可用于状态流转");
      const res = await owner.post(`/api/apps/${appId}/workflows/${workflowId}/transition`, {
        transition_id: "start",
        record_id: record.id,
        expected_updated_at: record.updated_at,
        idempotency_key: `miao-test-${Date.now()}-start`,
      });
      expectStatus(res, 200);
      expectField(res, "body.status", (v) => v === "completed");
      expectField(res, "body.record.data.status", (v) => v === "in_progress", "记录状态已流转");
      t.state.flownRecord = record.id;
    },
  },
  {
    name: "workflow: 相同幂等键重放返回原结果",
    run: async (t) => {
      const { appId, owner, workflowId, ordersSlug } = t.state;
      const key = `miao-test-${Date.now()}-replay`;
      const pick = await owner.post(`/api/apps/${appId}/query`, {
        table: ordersSlug,
        conditions: [{ field: "status", op: "eq", value: "new" }],
      });
      const record = pick.body.items?.find((r) => r.id !== t.state.flownRecord);
      if (!record) return; // 数据不足时跳过
      const first = await owner.post(`/api/apps/${appId}/workflows/${workflowId}/transition`, {
        transition_id: "start",
        record_id: record.id,
        expected_updated_at: record.updated_at,
        idempotency_key: key,
      });
      expectStatus(first, 200);
      const replay = await owner.post(`/api/apps/${appId}/workflows/${workflowId}/transition`, {
        transition_id: "start",
        record_id: record.id,
        expected_updated_at: record.updated_at,
        idempotency_key: key,
      });
      expectStatus(replay, 200);
      expectField(replay, "body.record.data.status", (v) => v === "in_progress", "重放不重复执行");
    },
  },
  {
    name: "workflow: 状态不匹配的转换被拒绝",
    run: async (t) => {
      const { appId, owner, workflowId, ordersSlug } = t.state;
      const pick = await owner.post(`/api/apps/${appId}/query`, {
        table: ordersSlug,
        conditions: [{ field: "status", op: "eq", value: "in_progress" }],
      });
      const record = pick.body.items?.find((r) => r.id !== t.state.flownRecord);
      if (!record) return;
      // finish 只允许 in_progress -> done；对同一记录再次 start（new -> in_progress）应冲突。
      const res = await owner.post(`/api/apps/${appId}/workflows/${workflowId}/transition`, {
        transition_id: "start",
        record_id: record.id,
        expected_updated_at: record.updated_at,
        idempotency_key: `miao-test-${Date.now()}-badstate`,
      });
      expectStatus(res, 409, "记录当前状态不允许此转换");
    },
  },
  {
    name: "action: 创建并启用业务动作（标记完成）",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.post(`/api/apps/${appId}/actions`, {
        name: "标记订单完成",
        description: "将指定订单状态置为 done",
        definition: {
          inputs: [
            { name: "record_id", type: "text", required: true },
            { name: "updated_at", type: "text", required: true },
          ],
          conditions: [],
          steps: [
            {
              id: "s1",
              operation: "update",
              table: "orders",
              record_id: "$record_id",
              expected_updated_at: "$updated_at",
              data: { status: "done" },
            },
          ],
        },
      });
      expectStatus(res, 201);
      t.state.actionId = res.body.id;
      const enable = await owner.post(`/api/apps/${appId}/actions/${res.body.id}/enable`, {
        confirm: true,
        expected_revision: res.body.revision,
      });
      expectStatus(enable, 200);
      expectField(enable, "body.status", (v) => v === "enabled");
    },
  },
  {
    name: "action: 执行业务动作并落库，幂等键防重",
    run: async (t) => {
      const { appId, owner, actionId, ordersSlug } = t.state;
      const pick = await owner.post(`/api/apps/${appId}/query`, {
        table: ordersSlug,
        conditions: [{ field: "status", op: "eq", value: "in_progress" }],
      });
      const record = pick.body.items?.[0];
      if (!record) throw new Error("没有 in_progress 订单可用于业务动作执行");
      const key = `miao-test-action-${Date.now()}`;
      const input = { record_id: record.id, updated_at: record.updated_at };
      const run = await owner.post(`/api/apps/${appId}/actions/${actionId}/execute`, {
        input,
        idempotency_key: key,
      });
      expectStatus(run, 200);
      expectField(run, "body.status", (v) => v === "completed");
      const verify = await owner.get(`/api/apps/${appId}/collections/${ordersSlug}/records/${record.id}`);
      expectField(verify, "body.data.status", (v) => v === "done", "动作执行后记录状态");
      const replay = await owner.post(`/api/apps/${appId}/actions/${actionId}/execute`, {
        input,
        idempotency_key: key,
      });
      expectStatus(replay, 200, "相同幂等键重放");
      expectField(replay, "body.status", (v) => v === "completed");
    },
  },
  {
    name: "action: 缺少必填输入被拒绝",
    run: async (t) => {
      const { appId, owner, actionId } = t.state;
      const res = await owner.post(`/api/apps/${appId}/actions/${actionId}/execute`, {
        input: {},
        idempotency_key: `miao-test-action-${Date.now()}-missing`,
      });
      expectStatus(res, 400, "缺少声明的输入");
    },
  },
];
