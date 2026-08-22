# 秒造 · Agent Ontology Runtime

用 `APP.md` 定义业务对象、关系和动作，让 Agent 通过统一 Runtime 使用应用。MongoDB 保存对象、文件、历史和执行轨迹，Web 页面只作为轻量控制台。

## 本地启动

需要 Bun 1.3.6+ 和 MongoDB 7+：

```bash
bun install --frozen-lockfile
MONGODB_URI=mongodb://127.0.0.1:27017 MONGODB_DB=agent_native_runtime bun run start
```

打开 <http://localhost:41874>。默认端口固定为 `41874`，可通过 `PORT` 覆盖。

开发时使用 `bun run dev` 启用热重载。启动命令会读取当前 shell 的 `PORT`、`HOST`、`MONGODB_URI` 和 `MONGODB_DB`。

## Docker 部署

```bash
docker compose up -d --build
```

服务不会降级到 SQLite；MongoDB 连接失败时进程退出并记录错误。设置 `MONGODB_URI`、`MONGODB_DB` 可连接托管 MongoDB。

启用内置 DSH 时由本仓库构建固定版本的官方 npm 包，并提供当前应用级 MCP Token：

```bash
DSH_VERSION=0.1.1-rc.2 \
MIAOZAO_MCP_TOKEN=mzt_user_... \
docker compose --profile agent up -d --build
```

DSH 镜像只安装 `@deepseek-ai/dsh@0.1.1-rc.2`，不复制或修改 DSH 源码。官方 profile 位于 `dsh-config/profiles/web`，MCP 通过官方 `@deepseek-ai/dsh-mcp-client` Cordis patch 注册；Compose 会在容器内用本地回环启动 DSH，再由同容器代理暴露 41875。

运行时文件默认绑定到宿主机 `./data/files`（可用 `MIAOZAO_DATA_DIR` 改变根目录），DSH Home 和 workspace 使用持久化卷。完整备份使用 `scripts/miaozao-backup.sh`，恢复使用 `scripts/miaozao-restore.sh BACKUP_DIR`；两个脚本只要求 Docker Compose，且会同时保存原始文件和 extracted 缓存。

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
