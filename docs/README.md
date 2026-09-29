# MIAO 产品与运维手册

更新日期：2026-09-29。**本文件是 `docs/` 唯一维护的文档**，同时记录当前实现、产品目标、接口、部署和恢复。`PRODUCT.md` 是仓库根目录的产品元数据，不作为第二份操作手册。

> 状态说明：仓库已加入有限业务界面运行时、草稿/预览/发布、应用线程、细粒度应用权限、受控查询/批量修改和站内提醒规则。**已通过代码检查、独立 PocketBase 迁移和路由测试，尚未完成目标环境真实账号、Gateway、权限和提醒的端到端联调**。以下区分代码能力、限制和仍待验收项目；不能把代码入口等同于生产验收。

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
- 多工作区、成员邀请和角色；工作区 owner/admin/member，以及 owner 配置的应用级 viewer/editor/manager/publisher 与单独的批量修改授权。移除成员只撤销当前工作区关系；全局账号停用由平台管理员处理。
- 应用创建、改名/描述、归档/恢复及确认后永久删除；数据表改名、调整字段标签/必填/选项、增删字段及确认后删除数据表。
- 记录新增、编辑、删除、分页、文本搜索、排序和单字段筛选。字段类型为文本、数字、日期、布尔、邮箱、网址、选项、同应用关联和附件。
- 工作台首页的 fx 多行输入，Enter 发送、Shift+Enter 换行；原有独立 fx 导航仍存在。首页应用列表只显示真实名称、用途和可用更新时间，不显示伪造统计、运行状态或业务预览。
- 打开有已发布版本的应用进入受限定义渲染的列表、看板或表单页面；支持记录读写、分页与搜索，未发布应用显示真实空状态。草稿可预览并由有发布权的用户确认发布；数据检查仍是次级入口。
- 工作区成员/邀请、应用访问、AI 用量/预算、JSON 导出和分页操作日志；独立的平台总览、用户、工作区、应用目录、AI 用量、审计和 AI 服务后台。
- PocketBase 备份/恢复脚本、AI Gateway 服务端代理、持久化请求与 token 用量、平台 AI 密钥轮换/回退。
- 工作区与应用私有 Agent 线程、最近一段消息恢复；结构化查询与最多 100 条记录的批量修改计划/确认；到期、状态变化、记录创建的站内提醒规则。

### 目标页面

| 区域 | 目标 | 当前差距 |
| --- | --- | --- |
| 工作区首页 | 一个 fx 输入与可访问应用卡片；应用多时搜索、排序；空状态只有一个主行动 | 输入与简化列表已完成；真实版本状态、最近使用和安全缩略图待做 |
| 应用页 | 已发布业务界面、草稿预览、应用内 Agent 和成员直达链接 | 有限列表/看板/表单及可切换 Agent 面板已写入代码；更多组件、复杂动作和目标环境验收待完成 |
| 数据检查 | 次级入口查看表/记录，可授权修正，结构设计引导 fx | 入口已降级；现有手工结构编辑能力仍保留作兼容 |
| 平台后台 | 独立页面、元数据和汇总用量；全局操作审计 | 基础 API/页面已实现，目标环境验收待完成 |

原型中的客户名称、数字、缩略图、运行状态和最近使用时间是示例，不得直接写入真实页面。没有可信的服务端数据时隐藏相关信息。应用卡片缩略图只能使用脱敏结构预览或安全快照。

### 当前用户操作路径

1. 注册并登录；如启用邮箱验证，先完成验证。登录后进入当前工作区首页，可从选择器切换自己拥有或加入的空间。
2. 在首页输入要完成的工作，fx 对话可以通过已有工具创建应用和数据表。可使用 `propose_ui` 工具生成草稿；发布仍由有权用户在预览中确认。
3. 打开已发布应用可操作业务页面；无已发布版本时显示空状态。需要处理底层记录时，打开「··· → 查看数据表」，在有权限的表中查看、搜索、筛选和修改记录。
4. AI Gateway 未配置或浏览器缺少 JSPI 时，fx 不可用；现有表/记录手工维护路径仍可使用。已发布业务界面不依赖 fx 的浏览器运行时；这一故障路径仍需目标环境验证。
5. 工作区 owner/admin 可邀请成员；owner 可调整角色、移出成员和设置应用访问名单。工作区用量按请求统计，不等于费用。

## 3. Agent-first 实现与验证

### 当前代码路径

1. 应用管理者通过 fx 检查表结构、提出 `pages` 界面草稿；服务端限制页面类型、字段绑定、筛选和页面数。草稿与正式版分开，用户在应用页预览并确认发布；发布重验基版本、表/字段与权限。纯界面旧版可在结构兼容时恢复。
2. 已发布应用可渲染列表、看板、表单和记录详情，读取授权记录并提交普通记录。列表/看板当前每页 25 条，表单沿用原记录校验。草稿预览显示页面差异且只读，不写正式记录；应用深链接含工作区参数，仍需登录与授权。
3. 应用内 Agent 面板和私有线程按当前成员、空间与应用作用域保存；重新登录恢复可见消息。浏览器 fx 会话重建时把近期文本作为上下文，不重放旧工具调用。
4. owner 可给成员授予 viewer、editor、manager、publisher；批量修改另行授权。自然语言查询由 Agent 提交结构化条件，服务端返回全量计数与分页记录。批量更改先保存最多 100 条目标快照，再由用户确认；服务端报告更新、冲突与失败。
5. Agent 可以提出到期、状态变化、记录新增三种触发规则；默认动作为站内提醒。记录新增也可设置同一记录的一个字段为固定值。规则默认停用，用户确认后启用。到期扫描由 MIAO 服务进程每分钟执行，重复事件以规则/事件键去重。

### 实际限制与待验证

- 首版页面组件限列表、看板和表单；没有任意脚本、通用工作流画布、复杂跨表操作。关联选择器首版只列出目标表的前 100 条记录，大表仍需可搜索的选择器。
- 批量操作限同一表的统一字段赋值，最多 100 条；进程中断后的旧作业需重新提交，已经改变版本的记录按冲突处理，不承诺事务性整批回滚。
- 通知目前只在站内显示，不发邮件或企微；表单后动作限同记录固定字段赋值。接收人失去访问权时规则暂停。到期扫描单条规则最多遍历 10,000 条候选记录；大规模表需要另建有界调度。
- 文件导入仍属于 Agent 数据工具的后续能力，目前没有文件上传和批量导入接口；不要把它展示为已上线功能。
- 目标环境必须实际验收：真实账号与角色、Gateway/JSPI、列表和附件、应用链接、并发发布、批量重试、提醒去重、权限撤销、反向代理和备份恢复。静态检查与迁移成功不能替代有状态验收。

首版仍不做拖拽编辑器、通用工作流画布、组件市场或任意模型生成 JS 的主站同源执行。

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

应用级 viewer 只读；editor 可改普通记录；manager 可改草稿及结构；publisher 可管理并发布；owner 拥有完整权限。批量修改是单独的 `can_batch` 授权，不随 editor 自动获得。非受限应用默认供工作区成员编辑普通记录，管理与发布仍须显式授权。未授权成员访问受限应用返回 404；每条 API 和 Agent 工具都由服务端重新校验。

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
| GET / PUT | `/apps/:id/access` | owner 查看/设置限制名单；`permissions` 包含 `user_id`、`viewer/editor/manager/publisher` 角色与可选 `can_batch` |
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

### 应用运行时、Agent、查询与提醒

| 方法 | 路径（省略 `/api`） | 用途 |
| --- | --- | --- |
| GET | `/apps/:id/runtime`、`/apps/:id/runtime/pages/:pageId/records` | 已发布界面与授权分页记录；后者可用 `preview=true` 只读草稿数据 |
| GET / POST | `/apps/:id/versions` | 有权者查看版本、提交有界草稿（`definition.pages`、`summary`、`base_version_id`） |
| GET | `/apps/:id/versions/:versionId/preview` | 当前草稿定义 |
| POST | `/apps/:id/versions/:versionId/publish`、`.../restore` | 发布须提交 `confirm:true,version_id`；恢复须 `confirm:true` |
| GET / POST | `/agent/threads`、`/agent/threads/:threadId/messages` | 当前用户私有线程与分页消息 |
| POST | `/apps/:id/query`、`/apps/:id/batch-plans` | 有界查询；生成最多 100 条目标的批量计划 |
| GET / POST | `/apps/:id/batch-plans/:jobId`、`.../commit` | 查看计划；确认提交 `confirm:true,plan_id` |
| GET / POST | `/apps/:id/automations`、`/apps/:id/automations/:ruleId/enable` | 查看/提出停用规则；明确确认启停 |
| GET | `/notifications` | 当前工作区当前用户的站内提醒 |

查询条件为 `[{field,op,value}]`，最多 8 项，`op` 支持 `eq/contains/before/after/empty`。批量计划 `change` 为 `{field,value}`，默认不得由 Agent 自动提交。运行时页面定义当前限 `list/board/form`；筛选还可用 `this_week` 并指定 IANA 时区。所有路径仍需登录及工作区上下文。

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

本次新增能力已完成迁移、代码检查和路由测试，仍未完成目标环境的有状态页面和多角色真实账号联调。发布前至少验收：注册/邮件、平台管理员与普通用户隔离、owner/admin/member 和应用 viewer/editor、跨空间切换、真实 fx Gateway、附件授权、数据导出、HTTPS 代理、备份及隔离恢复。`miao-test` 的历史 API 自动化覆盖记录不能替代这些页面与运维检查。

Agent-first 端到端验收再增加：新用户描述客户跟进应用 → 草稿预览 → 明确发布 → 成员在业务界面新增记录；owner 要求首页显示待跟进客户 → 预览差异 → 发布；viewer 在界面、fx、数据检查页及直接 API 均不能越权写入；发布失败和并发冲突不覆盖正式版；AI 不可用时已发布应用继续运行。

**维护约定：** 此后修改产品、权限、接口、部署或操作流程，只更新本文件相应章节。功能交付时同时把「目标」迁到「当前」，补充实际 API 和验收结果；不要再新增并行的方案、评审、快速开始或 API 文档。仓库根目录的 `PRODUCT.md` 仅保留机器可读的产品元数据；若其产品原则变化，与本文件同步修改。
