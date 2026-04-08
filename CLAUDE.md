# 创意喵 (Creative Meow) 平台

创意喵是一个连接商家和创作者的任务撮合平台，支持任务发布、投稿、审核、资金托管功能。

## 技术栈

- **后端**: Go + Gin
- **数据库**: SQLite
- **前端**: HTML + Bootstrap 5 (Gin 模板)
- **认证**: JWT
- **API**: RESTful /api/v1/

## 项目结构

```
miao/
├── cmd/server/main.go
├── internal/
│   ├── config/
│   ├── handler/
│   ├── middleware/
│   ├── model/
│   ├── repository/
│   ├── service/
│   └── router/
├── web/
│   ├── static/
│   └── templates/
├── migrations/
├── docs/
├── go.mod
└── CLAUDE.md
```

## 开发阶段

Phase 1: 项目初始化（Day 1-2）
Phase 2: 用户系统（Day 3-5）
Phase 3: 商家端-任务管理（Day 6-10）
Phase 4: 创作者端-投稿（Day 11-14）
Phase 5: 审核与评选（Day 15-18）
Phase 6: 资金系统（Day 19-22）
Phase 7: 信用系统（Day 23-25）
Phase 8: 申诉与风控（Day 26-28）
Phase 9: 管理端后台（Day 29-35）

---

## Harness: 创意喵平台开发

### 1. V1 版本交付（MVP 优先）

**目标:** 快速交付可用的第一版本，核心功能优先。

**触发词:** 「交付 V1」「发布第一版」「MVP 开发」「快速上线」

**技能:** `v1-delivery`

**Agent 团队:**

| Agent | 角色 | 模型 |
|-------|------|------|
| v1-coordinator | V1 交付协调者 | opus |
| backend-dev | Go/Gin 后端开发 | opus |
| frontend-dev | HTML/Bootstrap 5 前端开发 | opus |
| qa | QA 验证 | opus |

**V1 范围（Phase 2-5）:**
- 用户注册/登录（商家、创作者）
- 商家发布任务
- 创作者浏览任务、提交投稿
- 商家审核投稿、选中获胜者
- 简化资金状态（标记，不实际支付）

**不包含（Phase 6-9）:**
- 复杂资金托管、信用系统、申诉风控、管理端后台

**工作目录:** `_workspace_v1/`

**时间预估:** 8-12 小时

---

### 2. 完整平台开发（Phase 1-9）

**目标:** 通过 agent 团队协调开发创意喵平台的完整功能。

**触发词:** 「构建创意喵」「开发创意喵」「开始创意喵项目」

**技能:** `miao-orchestrator`

**Agent 团队:**

| Agent | 角色 |
|-------|------|
| backend-dev | Go/Gin 后端开发，实现 API、处理器、服务、仓库 |
| frontend-dev | HTML/Bootstrap 5 前端开发，实现商家端、创作者端、管理端页面 |
| qa | QA 验证，API-前端集成一致性、状态转换、数据流验证 |

**技能:**

| 技能 | 用途 | 使用 Agent |
|------|------|------------|
| go-backend | Go/Gin REST API 开发，Phase 1-9 后端实现指南 | backend-dev |
| html-frontend | HTML/Bootstrap 5 前端开发，Phase 2-9 前端实现指南 | frontend-dev |
| qa-check | QA 验证清单，边界面一致性检查 | qa |
| miao-orchestrator | 编排器，协调团队工作流程 | - |

**执行规则:**
- 创意喵相关开发请求，使用 `miao-orchestrator` 技能通过 agent 团队处理
- 简单问题/确认可直接响应，无需 agent 团队
- 所有 agent 使用 `model: "opus"`
- 中间产出：`_workspace/` 目录

---

**目录结构:**

```
.claude/
├── agents/
│   ├── v1-coordinator.md
│   ├── backend-dev.md
│   ├── frontend-dev.md
│   └── qa.md
└── skills/
    ├── v1-delivery/
    │   └── SKILL.md
    ├── go-backend/
    │   └── SKILL.md
    ├── html-frontend/
    │   └── SKILL.md
    ├── qa-check/
    │   └── SKILL.md
    └── miao-orchestrator/
        └── SKILL.md
```

**变更历史:**

| 日期 | 变更内容 | 对象 | 事由 |
|------|---------|------|------|
| 2026-04-09 | 新增 V1 交付 harness | v1-delivery, v1-coordinator | MVP 快速交付需求 |
| 2026-04-07 | 初始配置 | 整体 | - |
