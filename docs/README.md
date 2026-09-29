# MIAO 产品与运维手册

更新日期：2026-09-29。**本文件是 `docs/` 唯一维护的文档**，同时记录当前实现、产品目标、接口、部署和恢复。`PRODUCT.md` 是仓库根目录的产品元数据，不作为第二份操作手册。

> 状态说明：平台后台和基础工作台已实现；Agent-first 首页与数据检查入口完成首轮调整；**可用的生成式业务界面、版本草稿、预览、发布、回退和会话持久化尚未实现**。代码中有入口不代表已在生产环境完成真实账号、邮件、Gateway 或备份恢复演练。下文以「当前」和「目标」明确区分，不能把规划当成已上线功能。

## 1. 产品定位与使用边界

MIAO 面向需要自行建立内部业务工具的企业员工。PocketBase 保存身份和应用数据，MIAO API 逐请求验证工作区成员与应用权限；浏览器中的 fx 通过 MIAO 提供的工具协作，AI 密钥留在服务端。浏览器版 fx 不继承 CLI 的文件系统、shell、keychain 或 MCP 配置，并要求浏览器支持 WebAssembly JSPI。

产品目标分为三件事：

1. **管理应用**：用户向 fx 描述目标，由 fx 创建、修改、预览和发布应用；不再增加拖拽编辑器、字段向导或与 fx 并列的手工搭建主流程。
2. **使用应用**：成员在发布后的业务界面中查看、录入、搜索和更新数据，无需每个日常动作都通过对话。
3. **检查数据**：从应用的次级「··· → 查看数据表」入口检查结构、修正记录；底层数据页不作为应用默认界面。

平台级后台与用户工作台严格分开。平台管理员只看账号、工作区、应用元数据、非敏感配置状态和汇总用量，不读取业务记录、附件内容或 fx 对话正文。需要排查业务数据时，由用户自行导出并分享。

## 2. 当前功能与目标体验

### 当前已实现

- 注册/登录、可选邮箱验证、密码重置、注册模式和邮箱域限制，用户自行停用或删除账号。
- 多工作区、成员邀请和角色；工作区 owner/admin/member，以及 owner 配置的应用级 viewer/editor 权限。移除成员只撤销当前工作区关系；全局账号停用由平台管理员处理。
- 应用创建、改名/描述、归档/恢复及确认后永久删除；数据表改名、调整字段标签/必填/选项、增删字段及确认后删除数据表。
- 记录新增、编辑、删除、分页、文本搜索、排序和单字段筛选。字段类型为文本、数字、日期、布尔、邮箱、网址、选项、同应用关联和附件。
- 工作台首页的 fx 多行输入，Enter 发送、Shift+Enter 换行；原有独立 fx 导航仍存在。首页应用列表只显示真实名称、用途和可用更新时间，不显示伪造统计、运行状态或业务预览。
- 打开应用先进入业务界面区域，但目前只能看到「运行时未接入」提示；旧数据表/记录视图已移入「··· → 查看数据表」，可返回应用并进入 fx 讨论。
- 工作区成员/邀请、应用访问、AI 用量/预算、JSON 导出和分页操作日志；独立的平台总览、用户、工作区、应用目录、AI 用量、审计和 AI 服务后台。
- PocketBase 备份/恢复脚本、AI Gateway 服务端代理、持久化请求与 token 用量、平台 AI 密钥轮换/回退。

### 目标页面

| 区域 | 目标 | 当前差距 |
| --- | --- | --- |
| 工作区首页 | 一个 fx 输入与可访问应用卡片；应用多时搜索、排序；空状态只有一个主行动 | 输入与简化列表已完成；真实版本状态、最近使用和安全缩略图待做 |
| 应用页 | 默认渲染已发布业务界面；应用内 fx 可持续修改；窄屏切换对话面板；专注使用模式 | 当前只有运行时未接入提示，fx 仍在独立对话视图 |
| 数据检查 | 次级入口查看表/记录，可授权修正，结构设计引导 fx | 入口已降级；现有手工结构编辑能力仍保留作兼容 |
| 平台后台 | 独立页面、元数据和汇总用量；全局操作审计 | 基础 API/页面已实现，目标环境验收待完成 |

原型中的客户名称、数字、缩略图、运行状态和最近使用时间是示例，不得直接写入真实页面。没有可信的服务端数据时隐藏相关信息。应用卡片缩略图只能使用脱敏结构预览或安全快照。

### 当前用户操作路径

1. 注册并登录；如启用邮箱验证，先完成验证。登录后进入当前工作区首页，可从选择器切换自己拥有或加入的空间。
2. 在首页输入要完成的工作，fx 对话可以通过已有工具创建应用和数据表。**目前不会生成可使用的业务界面，也没有草稿/发布步骤。**
3. 打开应用会看到运行时尚未接入的提示。需要处理现有记录时，打开「··· → 查看数据表」，在有权限的表中查看、搜索、筛选和修改记录。
4. AI Gateway 未配置或浏览器缺少 JSPI 时，fx 不可用；现有表/记录手工维护路径仍可使用。未来已发布业务界面也必须在 AI 不可用时继续工作。
5. 工作区 owner/admin 可邀请成员；owner 可调整角色、移出成员和设置应用访问名单。工作区用量按请求统计，不等于费用。

## 3. Agent-first 交付计划

### 已完成的基线

- 平台管理 API、分页目录、AI 配置、账号停用/恢复、独立审计和后台页面。
- 工作区导航、应用列表、成员与权限、用量、导出、审计及表/记录入口。修复了工作区移除成员误停用全局账号、跨工作区沿用旧上下文等问题。
- 首页以 fx 输入为主，移除统计卡、三步教学和并行的手工创建主流程；应用数据页移为次级入口。当前业务界面区域仍是未接入状态。

### 下一步：版本化应用运行时（进行中）

- 采用**仅由 fx 编写的有限界面定义**，支持页面、指标、列表/表格、详情、表单、筛选、任务/看板和动作按钮；用户不直接编辑组件定义。模型生成的任意 JavaScript 不在 MIAO 主站同源执行。
- 应用区分未发布、草稿、已发布、发布失败、已归档。主路由只呈现已发布版本；没有版本时显示准确的空状态。旧应用保留原表和记录，迁移时不覆盖业务数据。
- 组件只绑定当前应用获授权的数据源；列表分页，表单校验，写入经服务端验证；加载、空数据、错误和权限变化均有明确反馈。
- 数据检查页仍可浏览和授权修正记录；结构改动逐步改由 fx 引导。已有手工结构编辑暂作兼容，不扩展成低代码编辑器。

**验收：** 已发布页面刷新后仍可用；录入记录后重新打开仍存在；无版本应用不标为运行中；AI 故障不妨碍已发布应用使用。

### 然后：fx 草稿、预览与发布

1. 用户描述目标，fx 澄清必要约束并提出应用名称、页面、动作和数据方案。
2. fx 生成独立草稿和变更摘要；预览使用隔离示例数据或只读的授权数据，不隐式写正式记录。
3. 用户确认发布后，服务端校验界面定义、表/字段引用、权限及起草版本；成功时原子切换正式版本，失败保留草稿和旧版本。
4. 应用内继续对话修改。并发修改必须检测版本冲突，不静默覆盖他人的发布。
5. 纯界面版本可在兼容时切回旧版；涉及删除字段或数据迁移的变更不能承诺一键恢复已删除数据，应另行确认与备份。

需要补齐 fx 的应用检查、界面提案、预览、发布、版本列表及兼容回退工具；现有 `list_apps`、`activate_app`、建表与记录工具继续复用。所有工具执行结果必须与回复一致。删除、数据丢失、批量改写和扩大成员可见范围要求明确对象、影响及用户确认，服务端再次校验。

**验收：** 新应用能从自然语言走到预览和显式发布；发布失败不影响现行版本；viewer 无法通过 UI、fx 或直接 API 越权写入。

### 最后：持续使用与交付

- 按工作区创建线程、应用线程持久化对话；私人正文默认不向其他成员公开，共享的应用变更只保留必要摘要。刷新、重新登录后可在授权范围内继续。
- 最近使用、搜索/排序、变更历史、安全缩略图和专注使用模式按真实数据完成；空间切换、成员移出、权限改变时清除或重验旧上下文。
- 在真实账号及目标环境验收登录/邮件、Gateway、平台管理员边界、跨工作区隔离、附件、反向代理及备份恢复。当前仓库代码和历史 API 结果不代替这些演练。

首版不做拖拽编辑器、通用工作流画布、组件市场、任意同源脚本执行、平台管理员代入用户身份或编造的健康/费用数据。

## 4. 权限与隐私

| 操作 | 平台管理员 | 工作区 owner | 工作区 admin | member |
| --- | --- | --- | --- | --- |
| 查看全局用户/工作区/应用元数据及汇总用量 | 是 | 否 | 否 | 否 |
| 全局停用/恢复账号、管理 AI 密钥 | 是 | 否 | 否 | 否 |
| 查看本空间成员与当日 AI 用量 | 否 | 是 | 是 | 是 |
| 管理邀请 | 否 | 是 | 是 | 否 |
| 移除成员、调整角色、设置应用访问名单 | 否 | 是 | 否 | 否 |
| 设置每日请求预算、查看工作区操作日志 | 否 | 是 | 否 | 否 |
| 浏览其他工作区业务记录 | 否 | 否 | 否 | 否 |

应用级 viewer 可读获授权记录，不可新增、修改、删除记录或表结构；editor 可在服务端规则内写入。未授权成员对受限应用收到 404。未来「谁可改界面、谁可发布」还需单独授予并在服务端校验，首版建议仅 owner 或明确授权的应用管理者发布；不能仅凭隐藏按钮授权。

平台管理员只能处理元数据和配置，不能从页面、fx、附件或深链接读取业务内容。平台 `/api/admin/*` 操作与工作区操作日志分开；平台审计记录 actor、动作、目标、时间、结果和原因，不存密钥、业务字段、附件或对话正文。工作区日志只记录成功写请求的路由、操作者、状态及目标标识，不存字段值。

全局停用账号仅由平台管理员执行，带原因并立即阻断后续会话；最后一个有效平台管理员不能自行停用或删除。用户自助停用后需联系平台管理员恢复。移出某空间不影响账号和其他空间。

## 5. API 参考（当前已实现）

根路径为 `/api`；JSON 请求设置 `Content-Type: application/json`。登录后使用 `Authorization: Bearer <token>`，多工作区账号以 `X-Miao-Tenant-Id` 选择空间。每个业务请求重新验证成员与应用权限。

### 身份与工作区

| 方法 | `/api` 后的路径 | 用途 |
| --- | --- | --- |
| POST | `/auth/register`、`/auth/login` | 注册（`email/password/name/invite_token`）、登录并返回 token 与默认空间 |
| POST | `/auth/verify-email`、`/auth/password-reset/request`、`/auth/password-reset/confirm` | 邮箱验证及密码重置；重置请求不泄露账号是否存在 |
| GET / DELETE | `/me` | 当前用户/空间/应用/AI 状态；删除账号要求当前密码与确认邮箱，拥有的空间及业务数据一并删除 |
| POST | `/me/deactivate` | 当前密码确认后停用账号，数据保留 |
| GET | `/workspace/members`、`/workspace/invites` | 成员列表、待接受邀请（后者仅 owner/admin） |
| POST / DELETE | `/workspace/invites`、`/workspace/invites/:id` | owner/admin 邀请与撤销邀请 |
| PATCH / DELETE | `/workspace/members/:id` | owner 调整成员角色或移出空间；PATCH 不接受 `disabled` |
| GET / PATCH | `/workspace/ai-usage`、`/workspace/ai-budget` | 当前用量；owner 设置 `daily_limit`，0 表示不限制 |
| GET | `/workspace/audit`、`/workspace/export` | owner 分页日志；导出当前用户获授权应用、表结构与记录 JSON，附件只含文件名 |

### 应用、表和记录

| 方法 | `/api` 后的路径 | 用途 |
| --- | --- | --- |
| GET / POST | `/apps` | 列表 / 创建应用 |
| GET / PATCH / DELETE | `/apps/:id` | 详情、修改、归档或永久删除；永久删除提交 `{ "confirm": true }` |
| GET / POST | `/apps/:id/collections` | 列出 / 新建数据表 |
| PATCH / DELETE | `/apps/:id/collections/:slug` | 修改表名/字段或确认后删除 |
| GET / PUT | `/apps/:id/access` | owner 查看/设置限制名单；`permissions` 包含 `user_id` 与 `viewer/editor` 角色 |
| GET / POST | `/apps/:id/collections/:slug/records` | 分页查询 / 新增记录 |
| PATCH / DELETE | `/apps/:id/collections/:slug/records/:recordId` | 编辑 / 删除记录 |
| GET | `/apps/:id/collections/:slug/records/:recordId/files/:fieldName` | 经授权下载附件 |

记录查询支持 `page`（从 1 起）、`perPage`（1–100，默认 25）、`search`、`sort`（默认 `-created`）、`filterField/filterValue`。响应为 `{ items, page, perPage, totalItems, totalPages }`；写入为 `{ "data": { "field_name": "value" } }`。搜索覆盖文本、邮箱和网址；单字段等值过滤不支持复杂多值关系、选项和文件过滤。字段类型和关联目标不能原地变更。

附件上传：`{ "data": {}, "files": { "attachment": { "name": "file.pdf", "type": "application/pdf", "base64": "data:application/pdf;base64,…" } } }`。单文件上限 5 MB，支持 PNG/JPEG/GIF/WebP、PDF、纯文本；导出 JSON 不含附件二进制。

### 平台与 fx

- `GET /admin/overview`、`/admin/runtime`：全局数量/今日 AI 汇总与非敏感配置状态。
- `GET /admin/users?page=&perPage=&q=&status=active|disabled`；`PATCH /admin/users/:id/status` 提交 `disabled` 与 5–500 字的 `reason`。不可停用当前或最后一个有效平台管理员。
- `GET /admin/workspaces?page=&perPage=&q=`、`/admin/apps?page=&perPage=&q=&archived=`：分页元数据目录，不含业务记录；应用目录不返回可能包含业务内容的描述。
- `GET /admin/usage?from=&to=&page=&perPage=`：最多 31 天的请求、状态与 token 汇总，不代表账单金额。
- `GET /admin/audit?page=&perPage=&targetType=&targetId=`：分页平台审计。
- `GET/PUT/DELETE /admin/ai`：管理员读取掩码状态、保存加密密钥或回退环境密钥，不返回完整密钥。
- `/fx/gateway` 是认证后的固定路径服务端代理，仅供 fx 使用，应用运行时代码不能直接调用。记录请求状态和上游可识别的 token 用量。

版本化界面、草稿、预览、发布及对话持久化的接口**尚未存在**。实现时在本节新增真实路径与请求/响应，不提前把建议路径写成已上线 API。

## 6. 部署与配置

运行环境需要 Bun、系统 PM2、curl、unzip、openssl；安装脚本支持 Linux/macOS x64 和 ARM64，下载固定版本 PocketBase 0.40.4，按 `bun.lock` 安装依赖并构建 fx 浏览器资源。PM2 托管 PocketBase 与 Bun 服务；不额外启动常驻开发服务。

```sh
bun run server:install
bun run server:start
bun run server:status
bun run server:logs
bun run server:stop
```

`server:logs` 可接收 `miao-platform`/`miao-pocketbase` 与行数；也可用 `./scripts/logs.sh miao-platform`。`start.sh` 保存 PM2 进程清单；异常退出自动重启，但当前不安装系统级开机启动项。安装时生成权限为 `600` 的 PocketBase superuser 本地配置文件，安装输出给出位置。

| 配置 | 用途 |
| --- | --- |
| `MIAO_DATA_DIR` | PocketBase 数据、文件、日志及运行期 migration；Linux 默认 `$XDG_DATA_HOME/miao` 或 `~/.local/share/miao`，macOS 默认 `~/Library/Application Support/Miao/data` |
| `POCKETBASE_PORT`、`MIAO_PORT`、`HOST` | 默认 PocketBase `127.0.0.1:8090`、MIAO `0.0.0.0:41874`；生产用 HTTPS 反向代理 |
| `POCKETBASE_SUPERUSER_EMAIL/PASSWORD` | 服务端与备份脚本使用的管理员凭据；重启确保密码与配置一致 |
| `MIAO_AI_PROVIDER` | `vercel`（默认）或 `capi`；CAPI 使用 `MIAO_AI_BASE_URL`、`AI_GATEWAY_API_KEY`、`MIAO_AI_MODEL`，服务端负责流格式转换 |
| `MIAO_PUBLIC_URL`、`RESEND_API_KEY`、`MIAO_MAIL_FROM` | 验证、重置和邀请的公网链接及邮件投递 |
| `MIAO_REQUIRE_EMAIL_VERIFICATION`、`MIAO_REGISTRATION_MODE`、`MIAO_ALLOWED_EMAIL_DOMAINS` | 可选验证；注册模式 `open`（默认）/`invite`/`closed`；逗号分隔域名限制 |
| `MIAO_ADMIN_EMAILS`、`MIAO_SETTINGS_ENCRYPTION_KEY` | 平台管理员邮箱和至少 32 字符、需长期保管的设置加密密钥 |
| `MIAO_BACKUP_DIR`、`MIAO_BACKUP_RETENTION_DAYS` | 备份位置及保留期；默认数据目录下 `backups`、30 天 |

配置文件为 Bun dotenv 格式；修改后重新运行 `start.sh` 载入。运行账户对安装、配置和数据目录需有相应权限；限制配置文件访问，绝不提交真实密钥。PM2 使用当前系统用户的 daemon 与进程清单，`pm2 list` 同时显示该用户其他服务。

邀请链接 72 小时有效，仅可使用一次，受邀者必须用指定邮箱登录或注册。owner/admin 可邀请和撤销；配置 Resend 时尝试发送邮件，失败或未配置时仍返回可复制链接。每个业务 API 与 AI 代理请求按当前空间重新授权。

每用户/空间/平台的分钟请求限制和空间每日预算按**请求次数**计数，不是费用上限；流式 token 统计依赖上游 usage 格式，须与供应商账单对账。平台管理员可保存加密的 AI 密钥或回退服务端环境密钥。轮换 `MIAO_SETTINGS_ENCRYPTION_KEY` 前应迁移或重新保存设置。

邮件发件域、公网回调链接、Gateway、反向代理和真实账号权限仍需在目标环境验证。

## 7. 备份与恢复

`bun run backup` 调用 PocketBase superuser 备份接口，生成包含数据库、文件和集合结构的完整归档，下载到 `$MIAO_BACKUP_DIR`（默认 `$MIAO_DATA_DIR/backups`），归档权限仅限当前系统用户。默认保留 30 天；脚本**不会**自行建立计划任务或异地副本。备份包含用户与附件数据，按生产数据保护。

至少每日运行一次并复制到另一主机或对象存储。Linux cron 示例（替换安装路径）：

```cron
20 2 * * * cd /path/to/miao && bun run backup >> /var/log/miao-backup.log 2>&1
```

恢复会**替换现有 PocketBase 数据并重启服务**。停止写入流量，确认归档与版本兼容，在隔离环境演练，并在恢复前另存当前数据归档：

```sh
bun run restore -- /path/to/miao_backup_20260929022000z.zip --confirm
```

之后检查 PocketBase `/api/health`、登录、空间切换、应用/表/附件读取、记录新增编辑和 migration，再恢复流量。失败时保留原数据与日志，不直接删旧目录。每季度在隔离实例恢复近期归档并记录校验结果；持续检查异地复制和附件覆盖。恢复加密的管理设置还需要原来的 `MIAO_SETTINGS_ENCRYPTION_KEY`。

删除账号、应用、数据表和字段属于在线永久删除；此前备份按保留期暂时保有快照，到期才清除。恢复脚本使用 `POCKETBASE_SUPERUSER_EMAIL/PASSWORD`。

## 8. 发布验收与维护规则

当前未完成目标环境的有状态页面和多角色真实账号联调。发布前至少验收：注册/邮件、平台管理员与普通用户隔离、owner/admin/member 和应用 viewer/editor、跨空间切换、真实 fx Gateway、附件授权、数据导出、HTTPS 代理、备份及隔离恢复。`miao-test` 的历史 API 自动化覆盖记录不能替代这些页面与运维检查。

Agent-first 端到端验收再增加：新用户描述客户跟进应用 → 草稿预览 → 明确发布 → 成员在业务界面新增记录；owner 要求首页显示待跟进客户 → 预览差异 → 发布；viewer 在界面、fx、数据检查页及直接 API 均不能越权写入；发布失败和并发冲突不覆盖正式版；AI 不可用时已发布应用继续运行。

**维护约定：** 此后修改产品、权限、接口、部署或操作流程，只更新本文件相应章节。功能交付时同时把「目标」迁到「当前」，补充实际 API 和验收结果；不要再新增并行的方案、评审、快速开始或 API 文档。仓库根目录的 `PRODUCT.md` 仅保留机器可读的产品元数据；若其产品原则变化，与本文件同步修改。
