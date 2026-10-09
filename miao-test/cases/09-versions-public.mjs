// 界面版本预览校验、发布状态、访问统计。
import { expectStatus, expectField } from "../lib.mjs";

const VALID_DEFINITION = {
  schema_version: 3,
  title: "miao-test 台账界面",
  pages: [
    {
      id: "home",
      title: "订单概览",
      data_sources: [
        { id: "orders", collection: "orders", fields: ["customer", "amount", "status", "due"] },
      ],
      spec: {
        root: "page",
        elements: {
          page: { type: "Page", props: { title: "订单概览" }, children: ["table"] },
          table: { type: "RecordTable", props: { source: "orders" } },
        },
      },
    },
  ],
};

export default [
  {
    name: "version: 应用运行态接口可读",
    run: async (t) => {
      const res = await t.state.owner.get(`/api/apps/${t.state.appId}/runtime`);
      expectStatus(res, 200);
    },
  },
  {
    name: "version: 合法界面定义可通过预览校验",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.post(`/api/apps/${appId}/versions/preview`, { definition: VALID_DEFINITION });
      expectStatus(res, 200, "合法界面预览");
      expectField(res, "body.status", (v) => v === "preview");
    },
  },
  {
    name: "version: 界面 schema_version 不匹配被拒绝",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.post(`/api/apps/${appId}/versions/preview`, {
        definition: { ...VALID_DEFINITION, schema_version: 2 },
      });
      expectStatus(res, 400, "schema_version 校验");
    },
  },
  {
    name: "version: 数据源引用不存在的表被拒绝",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.post(`/api/apps/${appId}/versions/preview`, {
        definition: {
          ...VALID_DEFINITION,
          pages: [
            {
              ...VALID_DEFINITION.pages[0],
              data_sources: [{ id: "ghost", collection: "ghost_table", fields: ["customer"] }],
              spec: {
                root: "page",
                elements: {
                  page: { type: "Page", props: { title: "x" }, children: ["table"] },
                  table: { type: "RecordTable", props: { source: "ghost" } },
                },
              },
            },
          ],
        },
      });
      expectStatus(res, 400, "未知数据表");
    },
  },
  {
    name: "version: 未声明的数据源不能在界面中使用",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.post(`/api/apps/${appId}/versions/preview`, {
        definition: {
          ...VALID_DEFINITION,
          pages: [
            {
              ...VALID_DEFINITION.pages[0],
              spec: {
                root: "page",
                elements: {
                  page: { type: "Page", props: { title: "x" }, children: ["table"] },
                  table: { type: "RecordTable", props: { source: "missing_source" } },
                },
              },
            },
          ],
        },
      });
      expectStatus(res, 400, "界面引用未声明数据源");
    },
  },
  {
    name: "version: 版本列表初始为空或合法分页",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.get(`/api/apps/${appId}/versions`);
      expectStatus(res, 200);
      const rows = res.body.items ?? res.body.versions ?? res.body;
      expectField({ body: { rows } }, "body.rows", (v) => Array.isArray(v), "版本列表为数组");
    },
  },
  {
    name: "publication: 未发布时公开配置为关闭",
    run: async (t) => {
      const { appId, owner } = t.state;
      const res = await owner.get(`/api/apps/${appId}/publication`);
      expectStatus(res, 200);
      expectField(res, "body.enabled", (v) => v === false, "未发布应用 enabled=false");
    },
  },
  {
    name: "visit: 访问计数递增",
    run: async (t) => {
      const { appId, owner } = t.state;
      const before = await owner.get(`/api/apps/${appId}`);
      const visit = await owner.post(`/api/apps/${appId}/visit`, {});
      expectStatus(visit, 200);
      const after = await owner.get(`/api/apps/${appId}`);
      expectField(
        { body: { diff: after.body.view_count - before.body.view_count } },
        "body.diff",
        (v) => v >= 1,
        "view_count 递增",
      );
    },
  },
];
