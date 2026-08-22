# 秒造 · Agent Ontology Runtime

用 `APP.md` 定义业务对象、关系和动作，让 Agent 通过统一 Runtime 使用应用。MongoDB 保存对象、文件、历史和执行轨迹，Web 页面只作为轻量控制台。

## 本地启动

需要 Node.js 22+ 和 MongoDB 7+：

```bash
npm install
MONGODB_URI=mongodb://127.0.0.1:27017 MONGODB_DB=agent_native_runtime npm start
```

打开 <http://localhost:41874>。默认端口固定为 `41874`，可通过 `PORT` 覆盖。

### PM2 管理

本项目使用 PM2 进程名 `miaozao`，默认监听 `41874`：

```bash
npm run pm2:start
pm2 status
pm2 logs miaozao
```

常用操作：

```bash
npm run pm2:restart
npm run pm2:stop
npm run pm2:delete
```

PM2 配置会读取当前 shell 的 `PORT`、`HOST`、`MONGODB_URI` 和 `MONGODB_DB`；未设置时使用本地 MongoDB 默认值。需要改端口时，例如：

```bash
PORT=41875 npm run pm2:restart
```

## Docker 部署

```bash
docker compose up -d --build
```

服务不会降级到 SQLite；MongoDB 连接失败时进程退出并记录错误。设置 `MONGODB_URI`、`MONGODB_DB` 可连接托管 MongoDB。

## 核心模型

```text
APP.md -> Ontology Manifest -> Objects / Links / Actions / Files -> MCP
```

`APP.md` 使用 `## Objects`、`## Links`、`## Actions`。动作只支持声明式规则和变更，不执行任意脚本。

User MCP 只暴露核心工具：

- `ontology.describe`
- `object.get`、`object.search`、`object.create`、`object.related`
- `action.list`、`action.describe`、`action.apply`
- `file.list`、`file.read`
- `history.search`、`trace.search`

Builder MCP 负责读取、编译、更新和发布 `APP.md`。`POST /api/mcp/:mode?app_id=...` 支持标准 JSON-RPC `initialize`、`tools/list`、`tools/call`。

旧 REST records 接口暂时保留给 Web 控制台兼容使用；新 Agent 不应直接修改底层记录。

## 验证

```bash
npm test
```
