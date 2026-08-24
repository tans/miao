# 秒造 · Agent Business Runtime

秒造不负责 LLM 推理、Prompt、Memory 或 Agent 编排；它提供 Agent 可以理解和操作企业业务应用的 Runtime。MongoDB 保存对象、文件、历史和执行轨迹，Web 页面只作为轻量控制台。

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

Runtime 镜像基于自建 `miaozao/bun-base`，不再依赖 Docker Hub 的 `oven/bun`。`Dockerfile.bun-base` 以 ECR Public 的 Debian 为基础，从 Bun 镜像源（默认 `cdn.npmmirror.com`，可用 `BUN_BASE_URL` 改为 GitHub Release）下载官方 Bun 1.3.6 二进制，构建产物不包含 Node.js；`Dockerfile` 通过 `BASE_IMAGE` 构造参数引用该基座。若部署机访问 Docker Hub 受限（仅能访问 ECR Public 与镜像源），这是推荐的构建路径。

推送镜像到仓库时使用 `scripts/push-docker-images.sh`，它会先构建并推送 `miaozao/bun-base:${BUN_VERSION}`，再构建并推送 `miaozao/runtime`：

```bash
IMAGE_TAG=release-20260822 /data/miaozao/scripts/push-docker-images.sh
```

只更新基座镜像时可执行 `SERVICES=bun-base scripts/push-docker-images.sh`。

推送脚本默认推给 `REGISTRY=127.0.0.1:5000`（部署机守护进程只把回环地址视为非安全仓库）；镜像以仓库路径 `miaozao/bun-base`、`miaozao/runtime` 存储，消费方通过公网名（如 `8.130.70.64:5000`）拉取。若无法访问公网名，可使用 `REGISTRY=127.0.0.1:5000` 在部署机本地拉取。

服务不会降级到 SQLite；MongoDB 连接失败时进程退出并记录错误。设置 `MONGODB_URI`、`MONGODB_DB` 可连接托管 MongoDB。

启用内置 DSH 时由本仓库构建固定版本的官方 npm 包。Runtime 使用长期的内部服务密钥注册 DSH，DSH 启动时按应用创建一个与当前 DSH 会话绑定的长期 MCP Token：

```bash
DSH_VERSION=0.1.1-rc.2 \
MIAOZAO_INTERNAL_TOKEN=$(openssl rand -hex 32) \
MIAOZAO_SESSION_APP_ID=your-app-id \
docker compose --profile agent up -d --build
```

外部 Agent 通过应用页面签发长期 `MCP Token`，默认不设置过期时间，只能由用户主动撤销；内置 DSH 的 `MIAOZAO_SESSION_TOKEN` 也是长期凭据，但只绑定当前 DSH 会话，可通过内部 Session 接口撤销。`MIAOZAO_MCP_TOKEN` 仍可作为旧部署的内部密钥别名，但不再作为外部 Agent 的 MCP 凭据。

DSH 镜像只安装 `@deepseek-ai/dsh@0.1.1-rc.2`，不复制或修改 DSH 源码。官方 profile 位于 `dsh-config/profiles/web`，MCP 通过官方 `@deepseek-ai/dsh-mcp-client` Cordis patch 注册；Compose 会在容器内用本地回环启动 DSH，再由同容器代理暴露 41875。

运行时文件默认绑定到宿主机 `./data/files`（可用 `MIAOZAO_DATA_DIR` 改变根目录），DSH Home 和 workspace 使用持久化卷。完整备份使用 `scripts/miaozao-backup.sh`，恢复使用 `scripts/miaozao-restore.sh BACKUP_DIR`；两个脚本只要求 Docker Compose，且会同时保存原始文件和 extracted 缓存。

## 应用定义

```text
app.md + app.yaml + ontology.yaml + workflow.yaml + actions.yaml
                              |
                         Manifest Compiler
                              |
                Ontology / Workflow / Actions / Data / File / Audit
                              |
                             MCP
```

`app.md` 只负责业务介绍，供人和 Agent 阅读；YAML 文件是 Runtime 的执行定义。`ontology.yaml` 描述业务对象及业务关系，不等同于数据库表；`workflow.yaml` 描述业务流程，不包含 Agent Chain；`actions.yaml` 只允许声明式规则、`set` 和 `link` 变更，不执行任意脚本。

Runtime 在应用创建、定义更新和发布时编译五个文件，运行期间只消费 Manifest。版本快照保存完整定义包，避免再把 Markdown 与执行配置混在一起。

## Runtime 资源、模板与知识

Runtime 将三类内容严格分开：

- Static Resource 是应用公开展示资源（Logo、CSS、JavaScript、字体等），通过 `/assets/{app_id}/{path}` 公开访问并按路径生成版本。
- File 是业务对象附件，继续使用私有 File Service、应用引用和权限校验；默认不生成公开 URL。
- Knowledge 是供 Agent 检索的参考资料，可由文本或已解析 File 入库，使用 `knowledge.search` 检索，不作为业务对象数据。

模板使用 LiquidJS。模板必须绑定 `object_type`，只渲染 Runtime 显式注入的数据，不加载服务器文件、不执行 JavaScript；Builder MCP 负责 `template.create`、`template.update`，User MCP 使用 `template.render`。

对应 MCP 工具包括 `resource.list`、`resource.upload`、`resource.delete`、`resource.url`，`file.*`，`knowledge.ingest`、`knowledge.search`、`knowledge.delete` 和 `template.*`。HTTP 控制台也提供 `/api/apps/:id/resources`、`/templates`、`/knowledge` 资源管理接口。

User MCP 只暴露核心工具：

- `ontology.describe`
- `object.get`、`object.search`、`object.create`、`object.related`
- `action.list`、`action.describe`、`action.apply`
- `file.list`、`file.read`
- `history.search`、`trace.search`

Builder MCP 负责读取、编译、更新和发布完整应用定义。`POST /api/mcp/:mode?app_id=...` 支持标准 JSON-RPC `initialize`、`tools/list`、`tools/call`。User MCP 按 Manifest 动态暴露业务能力，例如 `customer.search`、`customer.create` 和 `qualify_opportunity`，而不是数据库 CRUD。

旧 REST records 接口暂时保留给 Web 控制台兼容使用；新 Agent 不应直接修改底层记录。
