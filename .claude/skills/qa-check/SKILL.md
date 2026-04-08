---
name: qa-check
description: "QA 验证技能。验证创意喵平台 API-前端集成一致性、状态转换、数据流。后端/前端实现后必须执行。"
---

# QA 验证技能

## 概述

验证创意喵平台的集成一致性。交叉验证 API ↔ 前端连接、路由、状态转换、数据流。

## 验证对象

| 验证区域 | 生产者（左侧） | 消费者（右侧） |
|---------|--------------|---------------|
| API 响应结构 | internal/handler/*.go | web/templates/**/*.html + web/static/js/app.js |
| 路由 | web/templates/ 文件路径 | HTML 内 href、JS router |
| DB → API | internal/model/*.go | internal/handler/*.go |
| 认证 | internal/middleware/auth.go | 所有 API 调用 |

## 验证清单

### 1. API ↔ 前端连接验证

#### auth APIs
- [ ] POST /api/v1/auth/register 响应：`{code, message, data: {user_id, username}}`
- [ ] POST /api/v1/auth/login 响应：`{code, message, data: {token, user_id, username, role}}`
- [ ] login.html 的 JS 使用上述响应格式确认（存储 token、基于 role 重定向）

#### task APIs
- [ ] GET /api/v1/tasks 响应：`{code, message, data: [{id, title, description, reward, status, business_id}]}`
- [ ] GET /api/v1/tasks/:id 响应：单个 task 对象
- [ ] POST /api/v1/tasks 请求：`{title, description, reward}`，响应：创建的 task
- [ ] task_hall.html 使用上述响应确认（task.id、task.title 等）

#### submission APIs
- [ ] POST /api/v1/submissions 请求：`{task_id, content}`，响应：创建的 submission
- [ ] GET /api/v1/submissions?task_id=X 响应：`[{id, task_id, creator_id, content, status}]`
- [ ] submission_review.html 确认调用状态变更 API

### 2. snake_case ↔ camelCase 一致性

API 响应使用 snake_case（created_at、business_id）
前端 JS 也使用 snake_case 确认：
```javascript
// correct
task.created_at, task.business_id
// wrong
task.createdAt, task.businessId
```

### 3. 路由一致性

| HTML 文件 | 内部链接 |
|----------|----------|
| layout.html | href 值 |
| login.html | "注册" 链接 → /auth/register |
| business/task_create.html | 保存后 → /business/tasks |
| creator/task_detail.html | 提交投稿 → POST /api/v1/submissions |

所有 `href="/xxx"` 与实际 `web/templates/xxx.html` 文件匹配确认

### 4. 状态转换完整性

#### Task Status
```
open → in_review → closed
```
验证：task.go handler 中所有转换代码是否存在

#### Submission Status
```
pending → reviewed → rejected
pending → reviewed → awarded
```
验证：submission.go handler 中所有转换代码是否存在

### 5. 认证/授权验证

- [ ] 无 JWT token 的请求 → 401 响应
- [ ] 需要 business role 的 API（task create）→ 其他 role 请求时 403
- [ ] 需要 admin role 的 API（admin dashboard）→ 其他 role 请求时 403
- [ ] localStorage.token 过期时重定向到登录页面

### 6. 错误响应格式

所有 API 错误响应格式一致确认：
```json
{
  "code": 40001,
  "message": "错误描述",
  "data": null
}
```

## 验证执行

1. 后端实现后：运行 `go run cmd/server/main.go`
2. `curl` 手动 API 测试
3. 前端实现后：浏览器打开页面
4. network 标签确认 API 响应

## Bug 模式

发现时报告：
| Bug | 边界 | 原因 |
|-----|------|------|
| API 404 | route 不匹配 | 前端 `/api/v1/tasks` → 后端 `/api/v1/task` |
| 字段 undefined | snake/camel 不一致 | API `created_at` → JS `data.createdAt` |
| 登录失败 | token 存储缺失 | JS 使用 `data.token` 而非 `data.data.token` |
| 角色权限 | auth middleware 缺失 | middleware 未检查 role |

## 输出格式

验证报告：
```markdown
## QA 验证报告

### 通过项目
- [ ] item

### 失败项目
- [ ] item: 文件:行号 — 修改方法

### 未验证项目
- [ ] item: 原因
```
