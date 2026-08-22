# 秒造内置 Agent 集成

秒造负责应用、Ontology、文件、Action、租户和审计；DeepSeek Harness（DSH）只负责 Agent Loop、Session、Tool Calling 和 Web UI。本仓库不修改 DSH 源码。

## 内置 Agent API

登录后对应用调用：

- `POST /api/apps/:id/agent/sessions`，`mode` 为 `user` 或 `builder`，创建一个可持续的 Agent 会话。
- `GET /api/apps/:id/agent/sessions`，查看当前应用会话。
- `GET /api/apps/:id/agent/profile?mode=user`，取得 DSH Profile、MCP 地址和能力清单。

如果设置 `DSH_PUBLIC_URL`（或 `DSH_URL`），创建会话会返回可直接打开的 `launch_url`；未配置时仍会创建会话并可供外部 DSH/MCP 客户端使用。

## Capability / MCP

`POST /api/mcp/user` 和 `/api/mcp/builder` 使用 JSON-RPC。除兼容的 `object.*`、`file.*`、`action.*` 工具外，User MCP 提供稳定命名空间：

`miaozao.files.*`、`miaozao.ontology.*`、`miaozao.action.execute`、`miaozao.code.execute`。

业务文件始终由秒造 File Service 持有；DSH Workspace 只用于临时脚本和中间结果。`file.save` 的内容进入秒造文件服务并带有 Agent provenance。

## Code Runtime 安全边界

秒造不会在 API 进程或宿主机执行 Agent 代码。只有配置 `CODE_EXECUTOR_URL` 后，`code.execute` 才会把 Python、Node 或 Shell 请求转发给隔离执行器；执行器应使用 non-root、`no-new-privileges`、drop capabilities、CPU/内存/时间限制，不挂载宿主目录和 `docker.sock`。

## DSH 配置

`dsh-config/` 提供 profile、MCP 配置、Cordis overlay 和启动校验脚本。将官方 DSH 镜像挂载或复制这些配置即可；不要把企业文件挂载进 DSH 容器。

Docker Compose 要求通过 `DSH_IMAGE` 指定固定 tag 或 digest，并将 `DSH_HOME` 持久化到 `dsh_data`；workspace 通过 `dsh_workspaces` 持久化。`MIAOZAO_MCP_TOKEN` 必须显式传入应用级 Token，避免 DSH 以空凭据启动。由于不同 DSH 版本的 `settings.yaml` schema 可能变化，本仓库不生成未知版本的凭据文件，也不修改 DSH 源码。
