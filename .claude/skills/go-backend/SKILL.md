---
name: go-backend
description: "Go/Gin REST API 开发技能。实现 /api/v1/ 端点、处理器、服务、仓库、SQLite 联动。构建创意喵平台后端时必须使用。"
---

# Go/Gin 后端开发技能

## 概述

构建创意喵平台的 Go/Gin 后端。使用现有 `go.mod` 依赖，遵循 `internal/` 目录结构。

## 项目结构

```
miao/
├── cmd/server/main.go           # 服务器入口
├── internal/
│   ├── config/config.go         # 配置（DB、JWT secret、Port）
│   ├── model/                   # 数据模型（user.go、task.go、submission.go 等）
│   ├── handler/                 # HTTP 处理器（auth.go、task.go、submission.go 等）
│   ├── service/                 # 业务逻辑
│   ├── repository/              # 数据库访问
│   ├── middleware/              # JWT 认证、CORS、日志
│   └── router/                  # Gin 路由设置
├── web/
│   ├── static/                  # CSS、JS、images
│   └── templates/                # HTML 模板
├── migrations/                  # 数据库迁移
└── go.mod
```

## Phase 实现顺序

### Phase 1: 项目初始化（Day 1-2）
1. `cmd/server/main.go` — Gin 基础配置、SQLite 连接、中间件注册
2. `internal/config/config.go` — DB 路径、JWT Secret、Server Port
3. `internal/router/router.go` — 基础路由设置、CORS 中间件
4. 迁移：`migrations/` 下的 schema.sql

### Phase 2: 用户系统（Day 3-5）
1. `internal/model/user.go` — User 结构体（id、username、password_hash、role、email、created_at）
2. `internal/repository/user.go` — CreateUser、GetUserByUsername、GetUserByID
3. `internal/service/auth.go` — Register、Login（生成 JWT）
4. `internal/handler/auth.go` — POST /api/v1/auth/register、POST /api/v1/auth/login
5. `internal/middleware/auth.go` — JWT 验证中间件

### Phase 3: 商家端-任务管理（Day 6-10）
1. `internal/model/task.go` — Task 结构体（id、business_id、title、description、reward、status、created_at）
2. `internal/repository/task.go` — CRUD 操作
3. `internal/handler/task.go` — POST /api/v1/tasks、GET /api/v1/tasks/:id、PUT /api/v1/tasks/:id、DELETE /api/v1/tasks/:id、GET /api/v1/tasks/my
4. 商家端认证必需 — JWT 中验证 role=business

### Phase 4: 创作者端-投稿（Day 11-14）
1. `internal/model/submission.go` — Submission 结构体（id、task_id、creator_id、content、status、created_at）
2. `internal/repository/submission.go` — CRUD 操作
3. `internal/handler/submission.go` — POST /api/v1/submissions、GET /api/v1/submissions?task_id=X、PUT /api/v1/submissions/:id/review

### Phase 5: 审核与评选（Day 15-18）
1. PUT /api/v1/submissions/:id/review — 状态变更（pending → reviewed）
2. PUT /api/v1/submissions/:id/award — 获奖处理、奖金转移
3. 审核日志

### Phase 6: 资金系统（Day 19-22）
1. `internal/model/account.go` — Account、Transaction 结构体
2. POST /api/v1/account/recharge — 模拟充值
3. POST /api/v1/account/prepay — 任务悬赏金预付
4. GET /api/v1/account/balance — 余额查询
5. GET /api/v1/account/transactions — 流水记录

### Phase 7-9: 信用系统、申诉、管理端（Day 23-35）
1. 信用分计算及查询
2. 申诉 APIs
3. 管理端 APIs（GET /api/v1/admin/dashboard，仅管理员访问）

## API 响应格式

所有响应：
```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

错误：
```json
{
  "code": 40001,
  "message": "错误描述",
  "data": null
}
```

## JWT 认证

- Header：`Authorization: Bearer <token>`
- Payload：`{"user_id": 1, "username": "xxx", "role": "business|creator|admin"}`
- 中间件：`internal/middleware/auth.go` — 验证 token，注入 user_id

## 数据库

使用 SQLite。schema.sql：
```sql
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL CHECK(role IN ('business', 'creator', 'admin')),
  email TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tasks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  business_id INTEGER NOT NULL,
  title TEXT NOT NULL,
  description TEXT,
  reward DECIMAL(10,2) NOT NULL,
  status TEXT DEFAULT 'open' CHECK(status IN ('open', 'in_review', 'closed')),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (business_id) REFERENCES users(id)
);

CREATE TABLE submissions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id INTEGER NOT NULL,
  creator_id INTEGER NOT NULL,
  content TEXT NOT NULL,
  status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'reviewed', 'rejected', 'awarded')),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (task_id) REFERENCES tasks(id),
  FOREIGN KEY (creator_id) REFERENCES users(id)
);
```

## 处理器实现模式

```go
func Register(c *gin.Context) {
  var req RegisterRequest
  if err := c.ShouldBindJSON(&req); err != nil {
    c.JSON(400, gin.H{"code": 40001, "message": "参数错误", "data": nil})
    return
  }
  // 业务逻辑
  c.JSON(200, gin.H{"code": 0, "message": "success", "data": result})
}
```

## 测试

每个模块完成后：
1. `curl` 手动测试
2. 确认响应格式一致
3. 需要认证的 API 包含 JWT token 测试
