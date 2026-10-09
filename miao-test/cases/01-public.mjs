// Public endpoints reachable without authentication.
export default [
  {
    name: "public: /api/health 返回服务健康状态",
    run: async (t) => {
      const res = await t.anon.get("/api/health");
      t.expectStatus(res, 200);
      t.expectField(res, "body.service", (v) => v === "miao", "health service");
      t.expectField(res, "body.ok", (v) => v === true, "health ok");
    },
  },
  {
    name: "public: 生成的 OpenAPI / schema / 配置 schema 可匿名访问",
    run: async (t) => {
      const openapi = await t.anon.get("/api/openapi.json");
      t.expectStatus(openapi, 200);
      t.expectField(openapi, "body.openapi", (v) => String(v).startsWith("3"), "openapi version");
      const schema = await t.anon.get("/api/schema.json");
      t.expectStatus(schema, 200);
      const config = await t.anon.get("/api/config-schema.json");
      t.expectStatus(config, 200);
    },
  },
  {
    name: "public: 未知 API 路由返回 404",
    run: async (t) => {
      const res = await t.anon.get("/api/definitely-not-a-route");
      t.expectStatus(res, 404);
    },
  },
  {
    name: "public: 未登录访问受保护接口返回 401",
    run: async (t) => {
      const res = await t.anon.get("/api/me");
      t.expectStatus(res, 401);
    },
  },
  {
    name: "public: 匿名访问不存在的公开站点返回 404",
    run: async (t) => {
      const runtime = await t.anon.get("/api/public/miao-test-no-such-slug/runtime");
      t.expectStatus(runtime, 404);
      const page = await t.anon.get("/s/miao-test-no-such-slug");
      t.expectStatus(page, 404);
    },
  },
];
