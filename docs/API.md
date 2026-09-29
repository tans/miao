# MIAO API 参考

API 根路径为 `/api`。JSON 请求应设置 `Content-Type: application/json`。登录后传入 `Authorization: Bearer <token>`；多工作区账号可用 `X-Miao-Tenant-Id` 选择当前工作区。MIAO 每次请求都会重新验证工作区成员与应用权限。

平台管理接口只允许 `MIAO_ADMIN_EMAILS` 中的已登录账号调用。后台只提供用户、工作区和应用元数据、汇总 AI 用量与非敏感运行配置状态，不提供业务记录或附件读取。用户工作台的管理入口也在分阶段建设，详见[产品方案](ADMIN_AND_WORKBENCH_PLAN.md)。

## 身份与工作区

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| POST | `/auth/register` | 注册；可接收 `email`、`password`、`name`、`invite_token` |
| POST | `/auth/login` | 登录，返回会话 token 和默认工作区 |
| POST | `/auth/verify-email` | 使用邮件链接中的 `token` 完成邮箱验证 |
| POST | `/auth/password-reset/request` | 请求重置邮件，响应不会泄露邮箱是否已注册 |
| POST | `/auth/password-reset/confirm` | 以 `token` 和新 `password` 完成重置 |
| GET | `/me` | 当前用户、工作区、可访问应用及 AI 状态 |
| DELETE | `/me` | 删除账号；要求当前密码和 `confirm` 邮箱。拥有的工作区与业务数据会一起删除 |
| POST | `/me/deactivate` | 以当前密码停用账号；数据保留 |
| GET | `/workspace/members` | 列出当前工作区成员 |
| POST | `/workspace/invites` | owner/admin 邀请邮箱 |
| GET | `/workspace/ai-usage` | 查看当前工作区 AI 用量和每日请求预算 |
| PATCH | `/workspace/ai-budget` | owner 设置 `daily_limit`；0 表示不限制 |
| GET | `/workspace/audit` | owner 查看分页操作日志 |
| GET | `/workspace/export` | 下载当前用户可访问的应用、表结构和记录 JSON 导出；附件仅列出 PocketBase 文件名，不包含二进制内容 |

## 应用与应用级权限

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET/POST | `/apps` | 列出可访问应用 / 创建应用 |
| GET/PATCH/DELETE | `/apps/:id` | 查看、修改、归档或永久删除应用；删除必须提交 `{ "confirm": true }` |
| GET/POST | `/apps/:id/collections` | 列出 / 新建数据表 |
| PATCH/DELETE | `/apps/:id/collections/:slug` | 修改名称/字段或删除数据表；删除须确认 |
| GET | `/apps/:id/access` | owner 查看应用权限名单 |
| PUT | `/apps/:id/access` | owner 设置 `{ "restricted": true, "permissions": [{"user_id":"…","role":"viewer"}] }`；`editor` 具有编辑权限 |

受限应用中的未授权成员会得到 404。只读成员可以查询记录，但新增、修改、删除记录和表结构会被拒绝。

## 记录

`GET /apps/:id/collections/:slug/records` 支持以下查询参数：

- `page`：从 1 开始，默认 1。
- `perPage`：每页 1–100 条，默认 25。
- `search`：对文本、邮箱和网址字段进行包含搜索。
- `sort`：字段名正序或 `-字段名` 倒序；默认 `-created`。
- `filterField`、`filterValue`：按一个字段做相等筛选。

响应为 `{ items, page, perPage, totalItems, totalPages }`。记录写入格式为 `{ "data": { "field_name": "value" } }`。选项/关联字段用单个字符串值；布尔字段用 JSON 布尔值。

附件写入格式为 `{ "data": {}, "files": { "attachment": { "name": "file.pdf", "type": "application/pdf", "base64": "data:application/pdf;base64,…" } } }`。附件上限 5 MB，支持 PNG/JPEG/GIF/WebP、PDF 和纯文本。可通过 `/apps/:id/collections/:slug/records/:recordId/files/:fieldName` 经 MIAO 权限校验下载。

## AI 管理

- `GET /admin/overview`：平台账号、工作区、应用数量和今日 AI 汇总。
- `GET /admin/runtime`：注册策略、邮件配置状态、AI provider/model 等非敏感运行状态；不返回服务器密钥。
- `GET /admin/users?page=&perPage=&q=&status=active|disabled`：分页搜索账号元数据。
- `PATCH /admin/users/:id/status`：设置 `{ "disabled": true|false, "reason": "…" }`；原因 5–500 字符，不能停用当前或最后一个可用的平台管理员账号。
- `GET /admin/workspaces?page=&perPage=&q=`：分页列出工作区、所有者和成员/应用数量。
- `GET /admin/apps?page=&perPage=&q=&archived=`：分页列出应用元数据，不返回描述或业务记录。
- `GET /admin/usage?from=&to=&page=&perPage=`：按时间范围汇总全局及工作区请求、状态和 token 用量，查询范围最多 31 天。token 统计不代表账单金额。
- `GET /admin/audit?page=&perPage=&targetType=&targetId=`：分页查看平台操作审计。
- `GET /admin/ai`、`PUT /admin/ai`、`DELETE /admin/ai`：仅 `MIAO_ADMIN_EMAILS` 中的账号可管理 Gateway 密钥。PUT 接收 `{ "api_key": "…" }`；DELETE 删除管理界面密钥并回退至服务器环境变量。
- `PATCH /workspace/members/:id` 不接受 `disabled`；工作区成员管理不能停用全局账号。移出工作区使用 `DELETE /workspace/members/:id`，由 owner 执行。最后一个有效平台管理员不能自行停用或删除账号。
- `/fx/gateway` 是 fx 使用的服务端代理，不应从应用代码直接调用。代理只允许预设 Gateway 路径，并记录请求状态和上游返回的 token 用量。
