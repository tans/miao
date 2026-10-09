# miao-test 本地 API 测试套件

面向本地 miao 服务（`npm run server:start`）的端到端 API 测试案例，用真实账号走完
注册 → 工作区 → 应用 → 数据表 → 记录 → 批量/导入 → 状态流/业务动作 → 文件/任务 →
版本校验 → 平台管理 的全链路。仅用于本地演练，不指向生产环境。

## 运行

```sh
node miao-test/run.mjs              # 使用本地默认端口 41874
node miao-test/run.mjs --base http://127.0.0.1:41874
node miao-test/run.mjs --list       # 只列出案例
node miao-test/run.mjs --keep       # 保留测试应用与数据（默认运行结束会删除测试应用）
```

也可通过 `npm run test:miao` 运行。

## 账号

| 角色 | 邮箱 | 用途 |
| --- | --- | --- |
| owner | miao-test-owner@example.test | 工作区所有者、应用创建与授权 |
| editor | miao-test-editor@example.test | 应用 editor（无批量权限） |
| viewer | miao-test-viewer@example.test | 应用 viewer，只读权限边界 |
| outsider | miao-test-outsider@example.test | 无关用户，跨租户边界与停用演练 |
| admin | miao-test-admin@example.test | 平台管理员（需在 miao.env 的 MIAO_ADMIN_EMAILS 中配置） |

首次运行自动注册并建立数据；再次运行复用同一批账号（幂等）。

前提：本地服务需可匿名注册（开放注册）；平台管理案例（10-admin）要求
miao-test-admin@example.test 在平台管理员名单中（安装时由 MIAO_ADMIN_EMAILS
导入数据库，之后在平台管理控制台或 miao.env + 重启中维护）。

## 案例分组（cases/）

- `01-public` 健康检查、公开 schema、未认证边界
- `02-identity` 注册/登录、token 轮换、登出失效
- `03-workspace` 多工作区、邀请接受、成员角色、审计
- `04-app-data` 应用创建与授权、数据表与字段约束、关联字段
- `05-records-query` 记录 CRUD、乐观锁冲突、查询条件与分页
- `06-batch-import` 批量修改计划、导入计划、越权拒绝
- `07-workflow-action` 状态流启用/转换/幂等、业务动作执行
- `08-files-tasks` 文件上传与附件绑定、后台任务定义校验
- `09-versions-public` 界面定义预览校验、发布状态、访问计数
- `10-admin` 平台管理接口与账号停用/恢复

新增案例：在 `cases/` 里按序号添加 `.mjs`，导出
`[{ name, run: async (t) => { ... } }]`；`t` 提供 `t.base`、`t.accounts`、
`t.state`（跨案例共享：owner/editor/viewer/outsider/admin 客户端、appId 等）、
`t.expectStatus(res, 200, 说明)` 与 `t.expectField(res, "body.x", (v) => ..., 说明)`。
