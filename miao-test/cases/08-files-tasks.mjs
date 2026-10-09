// 文件上传/附件、后台任务定义、通知与记录变更。
import { expectStatus, expectField } from "../lib.mjs";

const TXT = "miao-test 附件内容\n第一行\n第二行";

export default [
  {
    name: "files: 上传文本文件并下载回读",
    run: async (t) => {
      const { appId, owner } = t.state;
      const b64 = Buffer.from(TXT, "utf8").toString("base64");
      const res = await owner.post(`/api/apps/${appId}/files`, {
        name: "miao-test-notes.txt",
        base64: b64,
      });
      expectStatus(res, 201);
      const fileId = res.body.id;
      if (!fileId) throw new Error(`文件上传响应缺少 id: ${JSON.stringify(res.body).slice(0, 200)}`);
      t.state.fileId = fileId;
      const list = await owner.get(`/api/apps/${appId}/files`);
      expectStatus(list, 200);
      const rows = list.body.items ?? list.body.files ?? list.body;
      if (!Array.isArray(rows) || !rows.some((f) => f.name?.includes("miao-test-notes"))) {
        throw new Error(`文件列表缺少上传文件: ${JSON.stringify(list.body).slice(0, 300)}`);
      }
      const content = await owner.get(`/api/apps/${appId}/files/${fileId}/content`);
      expectStatus(content, 200);
      const download = await fetch(
        `${t.base}/api/apps/${appId}/files/${fileId}/download`,
        { headers: { Authorization: `Bearer ${owner.token}` } },
      );
      expectStatus({ status: download.status, body: "" }, [200, 302], "附件下载");
    },
  },
  {
    name: "files: 不允许的扩展名被拒绝",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.post(`/api/apps/${appId}/files`, {
        name: "evil.exe",
        base64: Buffer.from("MZ").toString("base64"),
      });
      expectStatus(res, 400, "可执行文件扩展名");
    },
  },
  {
    name: "files: 把附件挂到订单记录的附件字段",
    run: async (t) => {
      const { appId, owner, ordersSlug, fileId } = t.state;
      const pick = await owner.post(`/api/apps/${appId}/query`, {
        table: ordersSlug,
        conditions: [{ field: "status", op: "eq", value: "new" }],
      });
      const record = pick.body.items?.[0];
      if (!record) return; // 全部流转完时跳过
      const res = await owner.post(`/api/apps/${appId}/files/${fileId}/attach`, {
        table: ordersSlug,
        record_id: record.id,
        field: "attachment",
        expected_updated_at: record.updated_at,
      });
      expectStatus(res, 200, "附件绑定");
      const verify = await owner.get(`/api/apps/${appId}/collections/${ordersSlug}/records/${record.id}`);
      expectField(verify, "body.data.attachment", (v) => typeof v === "string" && v.length > 0, "记录附件字段已写入");
    },
  },
  {
    name: "files: 非附件字段不能作为附件目标",
    run: async (t) => {
      const { appId, owner, ordersSlug, fileId } = t.state;
      const pick = await owner.post(`/api/apps/${appId}/query`, {
        table: ordersSlug,
        conditions: [{ field: "customer", op: "contains", value: "上海" }],
      });
      const record = pick.body.items?.[0];
      if (!record) return;
      const res = await owner.post(`/api/apps/${appId}/files/${fileId}/attach`, {
        table: ordersSlug,
        record_id: record.id,
        field: "customer",
        expected_updated_at: record.updated_at,
      });
      expectStatus(res, 400, "目标字段不是附件字段");
    },
  },
  {
    name: "task: 创建每日数据报告任务（draft）",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.post(`/api/apps/${appId}/tasks`, {
        name: "每日订单日报",
        definition: {
          goal: "汇总昨日新增与进行中的订单，输出日报",
          execution: "report",
          trigger: { type: "daily", time: "09:00", timezone: "Asia/Shanghai" },
          scope: {
            tables: [{ table: "orders", read_fields: ["customer", "amount", "status", "due"], write_fields: [] }],
            recipient_ids: [],
          },
          limits: { max_writes: 0, max_requests: 10, timeout_seconds: 600, confirmation_timeout_hours: 24 },
        },
      });
      expectStatus(res, 201);
      expectField(res, "body.status", (v) => v === "draft");
      expectField(res, "body.definition.execution", (v) => v === "report", "执行方式被规范化");
      t.state.taskId = res.body.id;
    },
  },
  {
    name: "task: LLM Agent 执行方式已下架被拒绝",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.post(`/api/apps/${appId}/tasks`, {
        name: "agent 任务",
        definition: {
          goal: "自由发挥",
          execution: "agent",
          trigger: { type: "manual", timezone: "Asia/Shanghai" },
          scope: {
            tables: [{ table: "orders", read_fields: ["customer"], write_fields: [] }],
            recipient_ids: [],
          },
        },
      });
      expectStatus(res, 400, "agent 执行方式");
    },
  },
  {
    name: "task: 非法的每日时间格式被拒绝",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.post(`/api/apps/${appId}/tasks`, {
        name: "坏时间",
        definition: {
          goal: "测试时间格式",
          execution: "report",
          trigger: { type: "daily", time: "9:00", timezone: "Asia/Shanghai" },
          scope: {
            tables: [{ table: "orders", read_fields: ["customer"], write_fields: [] }],
            recipient_ids: [],
          },
        },
      });
      expectStatus(res, 400, "HH:mm 校验");
    },
  },
  {
    name: "task: 任务列表包含刚创建的任务",
    run: async (t) => {
      const { appId, owner, taskId } = t.state;
      const res = await owner.get(`/api/apps/${appId}/tasks`);
      expectStatus(res, 200);
      const rows = res.body.items ?? res.body;
      if (!Array.isArray(rows) || !rows.some((task) => task.id === taskId)) {
        throw new Error(`任务列表缺少新建任务: ${JSON.stringify(res.body).slice(0, 300)}`);
      }
    },
  },
  {
    name: "records: 记录变更审计列表可读",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.get(`/api/apps/${appId}/record-changes`);
      expectStatus(res, 200);
    },
  },
  {
    name: "notifications: 通知列表可读",
    run: async (t) => {
      const res = await t.state.owner.get("/api/notifications");
      expectStatus(res, 200);
    },
  },
];
