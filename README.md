# 秒造 · Agent Native App Runtime

按 PRD 实现的 MongoDB 持久化 MVP：业务人员注册后，用 onboarding 描述业务，生成并发布 `APP.md`；页面、动态数据、文件、历史、Builder/User REST 与 MCP 共用同一套 Runtime。

## 本地启动

需要 Node.js 22+ 和 MongoDB 7+：

```bash
npm install
MONGODB_URI=mongodb://127.0.0.1:27017 MONGODB_DB=agent_native_runtime npm start
```

打开 <http://localhost:41873>。默认端口固定为 `41873`，可通过 `PORT` 覆盖。

## Docker 部署

```bash
docker compose up -d --build
```

服务不会降级到 SQLite；MongoDB 连接失败时进程退出并记录错误。设置 `MONGODB_URI`、`MONGODB_DB` 可连接托管 MongoDB。

## API 面

- `POST /api/auth/register`、`POST /api/auth/login`：轻量账号和工作区入口
- `POST /api/onboard`：生成首个应用、APP.md、Manifest 和 v1
- `/api/apps/:id/source`、`compile`、`publish`、`rollback`、`versions`：Builder 生命周期
- `/api/apps/:id/records`：动态数据查询、写入、更新、软删除、聚合
- `/api/apps/:id/files`：上传、预览、导入、导出
- `/api/apps/:id/history`、`/traces`：事实记忆和调试轨迹
- `POST /api/mcp/builder`、`POST /api/mcp/user`：统一 JSON-RPC 风格 MCP 适配层

所有数据文档都带 `tenant_id`/`app_id` 作用域，文件保留原始路径，业务记录保留 `provenance_json`，版本发布可回滚。
