# 秒造内置 Agent 集成

秒造负责应用、Ontology、文件、Action、租户和审计；DeepSeek Harness（DSH）只负责 Agent Loop、Session、Tool Calling 和 Web UI。本仓库不修改 DSH 源码。

## 内置 Agent API

登录后对应用调用：

- `POST /api/apps/:id/agent/sessions`，`mode` 为 `user` 或 `builder`，创建一个可持续的 Agent 会话。
- `GET /api/apps/:id/agent/sessions`，查看当前应用会话。
- `GET /api/apps/:id/agent/profile?mode=user`，取得 DSH Profile、MCP 地址和能力清单。

创建会话会签发一天有效的 App Capability Token，并在响应中返回只绑定当前 App 的 MCP URL/header。除非显式配置了同时包含 `{app_id}` 和 `{session_id}` 的 `DSH_LAUNCH_URL_TEMPLATE`，`launch_url` 为 `null`；秒造不会再伪造 DSH 不支持的 `?session_id=` 深链接。仅配置 `DSH_PUBLIC_URL` 时，页面可打开 DSH Web 根地址，但不能声称已选中指定会话。

## Capability / MCP

`POST /api/mcp/user` 和 `/api/mcp/builder` 使用 JSON-RPC。除兼容的 `object.*`、`file.*`、`action.*` 工具外，User MCP 提供稳定命名空间：

`miaozao.files.*`、`miaozao.ontology.*`、`miaozao.action.execute`、`miaozao.code.execute`。

业务文件始终由秒造 File Service 持有；DSH Workspace 只用于临时脚本和中间结果。`file.save` 的内容进入秒造文件服务并带有 Agent provenance。

## Code Runtime 安全边界

秒造不会在 API 进程或宿主机执行 Agent 代码。只有配置 `CODE_EXECUTOR_URL` 后，`code.execute` 才会把 Python、Node 或 Shell 请求转发给隔离执行器；执行器应使用 non-root、`no-new-privileges`、drop capabilities、CPU/内存/时间限制，不挂载宿主目录和 `docker.sock`。

## DSH 配置

`dsh-config/profiles/web/package.json` 使用官方 `dsh.profile.bundles` 声明 Web profile，`cordis.patch.yml` 使用官方 `@deepseek-ai/dsh-mcp-client` 配置注册秒造 MCP。容器启动命令是 `dsh --profile web --no-open`；`DSH_CONFIG`、自定义 `agent-profile.yml` 和非官方顶层 YAML 均已移除。

Docker Compose 自己构建 `Dockerfile.dsh`，将官方 DSH 版本固定为 `DSH_VERSION`，并将 `DSH_HOME` 持久化到 `dsh_data`；workspace 通过 `dsh_workspaces` 持久化。 `MIAOZAO_MCP_TOKEN` 必须显式传入当前应用级 Token，避免 DSH 以空凭据启动。单个 Compose DSH 进程绑定一个 MCP Token；要切换 App，应创建对应的外部 MCP 配置或按该 Session 重新启动一个 DSH 进程，不能把一个 App Token 宣称成租户级多 App Token。

用 `MIAOZAO_MCP_TOKEN=mzt_user_... scripts/dsh-verify-config.sh` 会在容器内执行官方 `dsh --profile web --dump-config`，并在最终组合树中强制检查 `mcp-miaozao`。这一步失败时不应启动 DSH Web。

## Code Runtime

`miaozao.code.execute` 仍只转发到 `CODE_EXECUTOR_URL`。Compose 不会伪造执行器；未配置隔离执行器时工具明确返回“Code Runtime 未配置”，不会在 API 或 DSH 容器内执行代码。
