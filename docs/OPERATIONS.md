# MIAO 产品与运维手册

本文档是 MIAO 唯一的详细产品、接口与运维手册。根目录 [README.md](../README.md) 提供项目概览和快速开始；仓库维护指引见 [AGENTS.md](../AGENTS.md)。

## 1. 产品概览

MIAO 帮助企业员工通过内置 fx Agent 创建和使用内部业务工具。PocketBase 管理身份与业务数据；MIAO API 按请求检查工作区和应用权限。企业 AI 密钥保存在服务端，浏览器 Agent 通过已认证的 MIAO 代理访问 AI 服务。

MIAO 的工作流程是：描述业务目标、由 Agent 规划数据结构和业务界面、预览并确认发布，然后在持久化的业务界面里处理日常工作。数据表和记录管理作为检查与维护入口保留。

Agent 在浏览器中通过 WebAssembly SDK 运行，使用 MIAO 明确提供的工具。它不继承 fx CLI 的文件系统、Shell、Keychain 或 MCP 配置；当前浏览器运行需要支持 WebAssembly JSPI。

## 2. 当前功能

- 账号注册、登录、邮箱验证、密码重置；支持注册策略和邮箱域限制。
- 工作区、成员邀请、工作区角色，以及应用访问名单。
- 应用、数据表、字段和记录的管理；字段支持文本、数字、布尔、日期、邮箱、网址、选项、关联和附件。
- 受限 schema v1 业务界面：单个数据表的列表、真实数据预览、搜索、分页，以及字段条件满足时的新增和编辑表单。
- 应用界面版本、草稿修订、历史界面前向恢复和显式发布；发布前检查数据表、字段和当前发布版本。
- fx 对话在同一浏览器内按账号和工作区保存检查点与可见消息；权限变化后清除旧会话并重新授权。服务端也有私有线程 API，但浏览器会话当前未接入该 API，因此不能跨设备续接。
- Agent 支持结构化查询、批量修改预览和确认执行、到期/状态变化/新增记录提醒，以及新增记录后的固定字段动作。
- 工作区审计、数据 JSON 导出、平台管理、AI 用量统计与服务端密钥配置。

## 3. 权限模型

工作区 owner 管理空间和应用访问权限。应用权限分为 viewer（只读）、editor（修改普通记录）、manager（管理应用与草稿）、publisher（管理并发布）。批量修改需要单独授予 `can_batch`；viewer 不能写入。未受限应用默认对工作区成员开放普通记录编辑，发布、应用管理和结构变更仍检查对应角色。

平台管理员只能管理账号、工作区和应用元数据、AI 配置与汇总用量，不能浏览业务记录、附件内容或 fx 对话正文。每个业务 API 都会在服务端检查当前工作区与应用权限；隐藏前端按钮不构成授权。

## 4. 已知范围与安全边界

- 当前业务界面仅支持一张现有数据表的受限列表定义，不运行模型生成的任意 JavaScript；没有通用工作流画布或拖拽式编辑器。
- 运行时的新增和编辑表单仅在界面覆盖全部必填且受支持字段时提供；附件和关联字段仍需从数据检查页维护。
- 批量更新限同一张表、统一字段赋值、最多 100 条记录。先预览目标记录，再由用户确认；过期计划、权限变化或记录被其他操作修改时会阻止相应写入。整批操作不保证事务性回滚。
- 自动化首版提供站内通知，以及新增记录时对同一记录设置一个固定字段值；不发送邮件或企微消息。
- 当前没有文件上传导入或 CSV/Excel 批量导入功能。Agent 不能被描述为已经支持导入现有业务文件。
- 浏览器 fx 会话当前保存在本机 IndexedDB，且会在权限变更或退出时清理；跨设备续接待完成。AI Gateway 或 JSPI 不可用时，已发布业务界面仍通过 MIAO API 工作。
- 静态检查和自动化测试不能替代目标环境中的真实账号、邮件、AI Gateway、反向代理、附件和备份恢复验收。

## 5. API 参考

API 根路径为 `/api`。登录后发送 `Authorization: Bearer <token>`。多工作区账号使用 `X-Miao-Tenant-Id` 指定工作区。JSON 请求设置 `Content-Type: application/json`。

### 身份与工作区

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| POST | `/auth/register`、`/auth/login` | 注册与登录 |
| POST | `/auth/verify-email`、`/auth/password-reset/request`、`/auth/password-reset/confirm` | 邮箱验证与密码重置 |
| GET / DELETE | `/me` | 获取当前用户上下文；删除账号需密码和确认邮箱 |
| POST | `/me/deactivate` | 停用当前账号，保留数据 |
| GET | `/workspace/members`、`/workspace/invites` | 查询成员与邀请 |
| POST / DELETE | `/workspace/invites`、`/workspace/invites/:id` | 创建或撤销邀请 |
| PATCH / DELETE | `/workspace/members/:id` | 调整成员角色或移出工作区 |
| GET / PATCH | `/workspace/ai-usage`、`/workspace/ai-budget` | 查询用量或调整请求预算 |
| GET | `/workspace/audit`、`/workspace/export` | 查看审计日志或导出获授权数据 |

### 应用和业务数据

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET / POST | `/apps` | 列表或创建应用 |
| GET / PATCH / DELETE | `/apps/:id` | 查看、修改、归档或确认后删除应用 |
| GET / POST | `/apps/:id/collections` | 列表或创建数据表 |
| PATCH / DELETE | `/apps/:id/collections/:slug` | 修改或确认后删除数据表 |
| GET | `/apps/:id/runtime` | 已发布 schema v1 界面、记录和表单能力；支持 `page`、`perPage`、`search` |
| GET | `/apps/:id/versions`、`/apps/:id/versions/:versionId` | 查询版本列表或版本定义 |
| GET | `/apps/:id/versions/:versionId/preview` | 只读预览当前有权访问的真实记录，最多 5 条 |
| POST | `/apps/:id/versions` | 创建界面草稿；支持 `based_on_version_id` 保存草稿修订 |
| POST | `/apps/:id/versions/:versionId/restore` | 从兼容的已发布历史版本创建前向恢复草稿 |
| POST | `/apps/:id/versions/:versionId/publish` | 确认发布；必须传 `expected_published_version_id`，首次发布传 `null` |
| GET / PUT | `/apps/:id/access` | owner 查看或设置成员应用角色与 `can_batch` |
| GET / POST | `/apps/:id/collections/:slug/records` | 分页查询或新增记录 |
| PATCH / DELETE | `/apps/:id/collections/:slug/records/:recordId` | 编辑或删除记录 |
| GET | `/apps/:id/collections/:slug/records/:recordId/files/:fieldName` | 下载获授权的附件 |

记录分页使用 `page`、`perPage`，支持文本搜索、排序和单字段筛选。记录写入格式为 `{"data":{"field_name":"value"}}`。附件写入额外提供 `files` 映射；每个文件最多 5 MB，支持 PNG、JPEG、GIF、WebP、PDF 和纯文本。

### Agent 操作、批量修改与提醒

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| POST | `/apps/:id/query` | 结构化条件查询；最多 8 个条件，支持 `eq`、`contains`、`before`、`after`、`empty` |
| POST | `/apps/:id/batch-plans` | 预览 1–100 条记录的统一字段赋值，不写入记录 |
| GET / POST | `/apps/:id/batch-plans/:jobId`、`/apps/:id/batch-plans/:jobId/commit` | 查询计划或提交确认执行 |
| GET / POST | `/agent/threads`、`/agent/threads/:threadId/messages` | 服务端私有对话线程和消息 API；浏览器 fx 当前尚未接入 |
| GET / POST | `/apps/:id/automations`、`/apps/:id/automations/:ruleId/enable` | 查询或创建默认停用的规则，再确认启用 |
| GET | `/notifications` | 当前用户可访问应用的站内提醒 |

### 平台与 AI Gateway

- `/admin/overview`、`/admin/runtime` 提供平台汇总数据与非敏感运行状态。
- `/admin/users`、`/admin/workspaces`、`/admin/apps`、`/admin/usage`、`/admin/audit` 提供分页平台管理能力。
- `/admin/ai` 管理服务端 AI 密钥；完整密钥不会返回给浏览器。
- `/fx/gateway` 是经过认证的固定代理，仅供 Agent 调用。应用运行时不直接连接 AI 服务。

## 6. 自托管部署

部署脚本支持 Linux 和 macOS 的 x64 与 ARM64。需要 Bun 1.3.6、PM2、`curl`、`unzip` 和 `openssl`。安装脚本下载 PocketBase 0.40.4，安装依赖并生成服务配置。

```sh
bun run server:install
# 按安装输出打开并修改生成的配置，至少设置管理员密码与 AI 服务密钥
bun run server:start
bun run server:status
bun run server:logs
```

默认 MIAO 监听 `0.0.0.0:41874`，PocketBase 仅监听 `127.0.0.1:8090`。对公网提供服务时，应配置 HTTPS 反向代理并限制 MIAO 服务端口的访问。

### 主要配置项

| 配置 | 用途 |
| --- | --- |
| `MIAO_DATA_DIR` | PocketBase 数据、附件、日志和运行配置所在目录 |
| `MIAO_PORT`、`POCKETBASE_PORT`、`HOST` | 服务端口和监听地址 |
| `POCKETBASE_SUPERUSER_EMAIL`、`POCKETBASE_SUPERUSER_PASSWORD` | PocketBase 服务端管理员账号 |
| `MIAO_AI_PROVIDER`、`AI_GATEWAY_API_KEY` | AI 服务提供方与服务端密钥 |
| `MIAO_AI_BASE_URL`、`MIAO_AI_MODEL` | AI Gateway 地址和模型名称 |
| `MIAO_PUBLIC_URL`、`RESEND_API_KEY`、`MIAO_MAIL_FROM` | 邮件验证、密码重置和邀请邮件 |
| `MIAO_REGISTRATION_MODE`、`MIAO_ALLOWED_EMAIL_DOMAINS` | 注册策略和允许注册的邮箱域 |
| `MIAO_ADMIN_EMAILS`、`MIAO_SETTINGS_ENCRYPTION_KEY` | 平台管理员与加密保存的服务设置 |
| `MIAO_BACKUP_DIR`、`MIAO_BACKUP_RETENTION_DAYS` | 备份目录和保留时间 |

所有密钥和管理员凭据都应保存在受限访问的服务端配置中，不能提交到版本库。

## 7. 备份与恢复

```sh
bun run backup
bun run restore -- /path/to/miao_backup.zip --confirm
```

备份归档包含 PocketBase 数据库、附件和集合结构。默认保留期为 30 天；备份脚本不会自动创建定时任务或异地副本，应由部署者配置计划任务并复制到独立存储。备份文件包含用户和业务数据，应按生产数据保护。

恢复会替换当前 PocketBase 数据。执行前停止写入、检查归档并另存当前数据；先在隔离环境演练，再恢复线上服务。恢复后检查健康接口、登录、工作区切换、业务界面、附件和记录读写。

## 8. 验证与维护

运行自动化测试：

```sh
npm test
```

在安装依赖后，前端资源构建命令为 `bun run prepare:fx`。数据库迁移使用版本化文件；已应用的迁移不得修改，应通过新迁移调整结构。

交付前还需在目标环境验证真实账号权限、邮件、AI Gateway、JSPI 浏览器、HTTPS 代理、附件授权和备份恢复。修改产品、API 或部署行为时更新本手册对应章节；保持根目录 README 聚焦项目概览和快速开始，不新建重复的产品或 API 说明文件。
