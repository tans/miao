---
name: v1-delivery
description: "创意喵平台 V1 版本交付编排器。MVP 优先，核心功能快速迭代。触发词：「交付 V1」「发布第一版」「MVP 开发」「快速上线」。"
---

# 创意喵平台 V1 交付编排器

专注于交付可用的第一版本，优先核心功能，快速迭代。

## V1 核心范围（MVP）

**必须有：**
- Phase 2: 用户注册/登录（商家、创作者）
- Phase 3: 商家发布任务（基础字段）
- Phase 4: 创作者浏览任务、提交投稿
- Phase 5: 商家审核投稿、选中获胜者
- 简化的资金流程（标记状态，不实际支付）

**暂不包含：**
- Phase 6: 复杂资金托管
- Phase 7: 信用系统
- Phase 8: 申诉与风控
- Phase 9: 管理端后台

## 执行模式：Agent Team

使用 Agent 工具并行调用 backend-dev 和 frontend-dev，通过任务协调。

## Agent 配置

| Agent | 类型 | 职责 | 模型 |
|-------|------|------|------|
| backend-dev | backend-dev | Go/Gin API 实现 | opus |
| frontend-dev | frontend-dev | HTML/Bootstrap 5 页面 | opus |
| qa | qa | 集成验证 | opus |

## 工作流程

### Phase 0: 范围确认

1. 向用户确认 V1 范围：
   - 用户类型：商家、创作者
   - 核心流程：发布任务 → 投稿 → 审核选中
   - 资金处理：状态标记（不实际支付）
   
2. 检查现有代码：
   ```bash
   ls -la internal/
   ls -la web/templates/
   ```

3. 确定开发模式：
   - **从零开始** → 完整实现
   - **已有基础** → 增量开发

### Phase 1: 准备工作

1. 创建工作目录：
   ```
   _workspace_v1/
   ├── 00_plan/
   │   ├── mvp_scope.md
   │   ├── api_spec.md
   │   └── page_list.md
   ├── 01_backend/
   ├── 02_frontend/
   └── 03_qa/
   ```

2. 生成 MVP 规格文档：
   - API 端点清单（最小集）
   - 页面清单（核心流程）
   - 数据模型（简化版）

### Phase 2: 并行开发

**任务分配：**

```
TaskCreate([
  {
    subject: "实现用户认证 API",
    description: "POST /api/v1/auth/register, /login, /me",
    owner: "backend-dev",
    activeForm: "实现用户认证 API"
  },
  {
    subject: "实现任务管理 API",
    description: "POST /api/v1/tasks, GET /api/v1/tasks, GET /api/v1/tasks/:id",
    owner: "backend-dev",
    activeForm: "实现任务管理 API"
  },
  {
    subject: "实现投稿 API",
    description: "POST /api/v1/submissions, GET /api/v1/submissions",
    owner: "backend-dev",
    activeForm: "实现投稿 API"
  },
  {
    subject: "实现登录注册页面",
    description: "web/templates/auth/login.html, register.html",
    owner: "frontend-dev",
    activeForm: "实现登录注册页面"
  },
  {
    subject: "实现商家任务页面",
    description: "web/templates/business/tasks.html, task_create.html",
    owner: "frontend-dev",
    activeForm: "实现商家任务页面"
  },
  {
    subject: "实现创作者页面",
    description: "web/templates/creator/tasks.html, submit.html",
    owner: "frontend-dev",
    activeForm: "实现创作者页面"
  }
])
```

**并行执行：**

使用单个消息并行调用两个 Agent：

```
Agent(
  subagent_type: "backend-dev",
  description: "实现 V1 核心 API",
  prompt: "实现创意喵 V1 核心 API...",
  run_in_background: true
)

Agent(
  subagent_type: "frontend-dev", 
  description: "实现 V1 核心页面",
  prompt: "实现创意喵 V1 核心页面...",
  run_in_background: true
)
```

### Phase 3: 集成验证

等待两个 agent 完成后：

1. 检查产出：
   - `internal/` 下的 Go 文件
   - `web/templates/` 下的 HTML 文件

2. 执行 QA 验证：
   ```
   Agent(
     subagent_type: "qa",
     description: "V1 集成验证",
     prompt: "验证创意喵 V1 核心流程..."
   )
   ```

3. 生成验证报告：`_workspace_v1/03_qa/report.md`

### Phase 4: 快速修复

如果 QA 发现问题：

1. 分类问题：
   - **阻塞性** → 立即修复
   - **非阻塞性** → 记录到 backlog

2. 针对性修复：
   - 后端问题 → 调用 backend-dev
   - 前端问题 → 调用 frontend-dev
   - 集成问题 → 两者协调

3. 重新验证

### Phase 5: 交付准备

1. 生成启动脚本：
   ```bash
   # scripts/start_v1.sh
   go run cmd/server/main.go
   ```

2. 生成 V1 文档：
   ```
   docs/v1/
   ├── README.md          # 快速开始
   ├── API.md             # API 文档
   ├── USER_GUIDE.md      # 用户指南
   └── KNOWN_ISSUES.md    # 已知问题
   ```

3. 向用户报告：
   - 已实现功能清单
   - 启动方式
   - 测试账号
   - 已知限制

## 开发原则

### 1. MVP 优先
- 只实现核心流程
- 避免过度设计
- 功能够用即可

### 2. 快速迭代
- 每个模块独立可测
- 先跑通流程，再优化
- 问题分优先级

### 3. 技术债务可控
- 标记 TODO 注释
- 记录已知问题
- 规划 V2 改进点

### 4. 用户体验基本
- 表单验证
- 错误提示
- 加载状态

## 简化策略

### 数据模型简化
```go
// V1: 最小字段
type Task struct {
    ID          uint
    Title       string
    Description string
    Budget      float64
    Status      string
    BusinessID  uint
    CreatedAt   time.Time
}

// V2 再加：
// - Tags []string
// - Requirements string
// - Deadline time.Time
```

### API 简化
```
V1 端点（15 个）：
- POST   /api/v1/auth/register
- POST   /api/v1/auth/login
- GET    /api/v1/auth/me
- POST   /api/v1/tasks
- GET    /api/v1/tasks
- GET    /api/v1/tasks/:id
- PUT    /api/v1/tasks/:id
- POST   /api/v1/submissions
- GET    /api/v1/submissions
- GET    /api/v1/submissions/:id
- PUT    /api/v1/submissions/:id/approve
- PUT    /api/v1/submissions/:id/reject
- GET    /api/v1/users/profile
- PUT    /api/v1/users/profile
- GET    /api/v1/users/balance

V2 再加：
- 文件上传
- 消息通知
- 搜索过滤
```

### 页面简化
```
V1 页面（8 个）：
- auth/login.html
- auth/register.html
- business/tasks.html
- business/task_create.html
- business/submissions.html
- creator/tasks.html
- creator/submit.html
- creator/my_submissions.html

V2 再加：
- 个人中心
- 消息中心
- 数据统计
```

## 错误处理

| 情况 | 策略 |
|------|------|
| Agent 失败 | 重试 1 次，失败则人工介入 |
| API 不一致 | 以 backend 为准，frontend 适配 |
| 功能缺失 | 记录到 backlog，不阻塞交付 |
| 严重 bug | 立即修复，延迟交付 |

## 成功标准

V1 交付成功的标准：

1. **功能完整性**
   - [ ] 用户可以注册/登录
   - [ ] 商家可以发布任务
   - [ ] 创作者可以浏览任务
   - [ ] 创作者可以提交投稿
   - [ ] 商家可以审核投稿
   - [ ] 商家可以选中获胜者

2. **技术可用性**
   - [ ] 服务可以启动
   - [ ] API 返回正确响应
   - [ ] 页面可以正常访问
   - [ ] 基本错误处理

3. **文档完整性**
   - [ ] README 说明启动方式
   - [ ] API 文档列出所有端点
   - [ ] 已知问题清单

## 时间预估

- Phase 0-1: 1 小时（规划）
- Phase 2: 4-6 小时（并行开发）
- Phase 3: 1-2 小时（验证）
- Phase 4: 1-2 小时（修复）
- Phase 5: 1 小时（交付准备）

**总计：8-12 小时**

## 后续触发

本编排器在以下情况触发：
- 「交付 V1」「发布第一版」
- 「MVP 开发」「快速上线」
- 「先做个能用的版本」
- 「V1 迭代」「V1 修复」
