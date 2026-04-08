---
name: miao-orchestrator
description: "创意喵平台开发任务的编排器。初始化执行关键词：「构建创意喵」「开发创意喵」「开始创意喵项目」。后续工作：创意喵结果修改、部分重新执行、更新、完善、再次运行、基于之前结果改进等也必须使用本技能。"
---

# 创意喵平台 Orchestrator

协调后端、前端、QA agent 团队构建创意喵平台。

## 执行模式：Agent Team

使用 TeamCreate 构建团队，团队成员通过 SendMessage 直接通信，通过 TaskCreate/TaskUpdate 协调。

## Agent 配置

| 团队成员 | Agent 类型 | 角色 | 技能 | 输出 |
|---------|-----------|------|------|------|
| backend-dev | backend-dev | Go/Gin 后端开发 | go-backend | internal/ 下的 Go 文件 |
| frontend-dev | frontend-dev | HTML/Bootstrap 5 前端开发 | html-frontend | web/templates/ 下的 HTML 文件 |
| qa | qa | QA 验证 | qa-check | 验证报告 |

## 工作流程

### Phase 0: 上下文确认（后续工作支持）

确认现有产出决定执行模式：

1. 检查 `_workspace/` 目录是否存在
2. 执行模式决定：
   - **`_workspace/` 不存在** → 初始执行。进入 Phase 1
   - **`_workspace/` 存在 + 用户部分修改请求** → 部分重新执行。只调用相关 agent，用新产出覆盖现有产出
   - **`_workspace/` 存在 + 用户提供新输入** → 新执行。将现有 `_workspace/` 移动到 `_workspace_{timestamp}/` 后进入 Phase 1
3. 部分重新执行时：在 agent prompt 中包含之前产出路径，指示 agent 读取现有结果并应用反馈

### Phase 1: 准备
1. 分析用户输入 — 确定开发阶段（Phase 1-9）
2. 创建 `_workspace/` 目录（初始执行时）
3. 将输入数据保存到 `_workspace/00_input/`

### Phase 2: 团队配置

1. 团队创建：
   ```
   TeamCreate(
     team_name: "miao-dev-team",
     members: [
       { name: "backend-dev", agent_type: "backend-dev", model: "opus" },
       { name: "frontend-dev", agent_type: "frontend-dev", model: "opus" },
       { name: "qa", agent_type: "qa", model: "opus" }
     ]
   )
   ```

2. 工作注册：
   ```
   TaskCreate(tasks: [
     { title: "后端 API 实现 - [阶段名]", description: "具体 API 列表", assignee: "backend-dev" },
     { title: "前端页面实现 - [阶段名]", description: "具体页面列表", assignee: "frontend-dev" },
     { title: "QA 验证 - [阶段名]", description: "验证清单", assignee: "qa", depends_on: ["后端 API 实现"] }
   ])
   ```

### Phase 3: 并行开发

**执行方式：** backend-dev 和 frontend-dev 并行开发

**团队通信规则：**
- backend-dev 完成 API 时向 frontend-dev 发送 SendMessage 告知 API 列表和响应格式
- frontend-dev 发现 API 不一致时向 backend-dev 发送 SendMessage
- QA 在 backend 和 frontend 各自完成一个模块后立即验证（incremental QA）

**产出保存：**

| 团队成员 | 输出路径 |
|---------|---------|
| backend-dev | `internal/` 下的 Go 文件 |
| frontend-dev | `web/templates/` 下的 HTML 文件 |

**Leader 监控：**
- 团队成员 idle 时自动收到通知
- 特定团队成员遇到问题时通过 SendMessage 提供指导或重新分配工作
- 通过 TaskGet 确认整体进度

### Phase 4: 验证
1. 等待所有团队成员工作完成（通过 TaskGet 确认状态）
2. 收集各团队成员的产出（Read）
3. 执行 QA 验证清单
4. 生成验证报告：`_workspace/QA_report.md`

### Phase 5: 整理
1. 向团队成员发送结束请求（SendMessage）
2. 团队整理（TeamDelete）
3. 保留 `_workspace/` 目录（不删除中间产出 — 用于事后验证和审计追踪）
4. 向用户报告结果摘要

> **团队重构需要时：** 如果不同 Phase 需要不同的专家组合，先将当前团队用 TeamDelete 整理，然后使用 TeamCreate 创建下一 Phase 的团队。上一团队的产出保留在 `_workspace/` 中，新团队可以通过 Read 访问。

## 数据流

```
[Leader] → TeamCreate → [backend-dev] ←SendMessage→ [frontend-dev]
                           ↓                            ↓
                     internal/*.go              web/templates/*.html
                           ↓                            ↓
                           └────────Read────────────────┘
                                      ↓
                             [QA: 验证报告]
                                      ↓
                               最终产出
```

## 错误处理

| 情况 | 策略 |
|------|------|
| 1 个团队成员失败/停止 | Leader 感知 → 通过 SendMessage 确认状态 → 重启或替换团队成员 |
| 半数以上团队成员失败 | 通知用户并确认是否继续 |
| 超时 | 使用目前收集到的部分结果，未完成团队成员终止 |
| 团队成员间数据冲突 | 注明来源后合并，不删除 |
| 工作状态延迟 | Leader 通过 TaskGet 确认后手动 TaskUpdate |

## 测试场景

### 正常流程
1. 用户输入「构建创意喵平台」
2. Phase 1 分析需求，确定 Phase 2-9 开发阶段
3. Phase 2 团队配置（3 名团队成员 + 多个工作）
4. Phase 3 团队成员自行协调开发
5. Phase 4 产出集成和验证
6. Phase 5 团队整理
7. 预期结果：`_workspace/` 包含完整的 `internal/` 和 `web/templates/` 实现

### 错误流程
1. Phase 3 中 backend-dev 因错误停止
2. Leader 收到 idle 通知
3. 通过 SendMessage 确认状态 → 尝试重启
4. 重启失败时将 backend-dev 的工作分配给 frontend-dev
5. 其余结果进入 Phase 4
6. 最终报告注明「backend-dev 区域部分未收集」

## 后续支持

本 Orchestrator 在用户提出以下请求时触发：
- 「重新构建」「再次开发」「更新创意喵」
- 「修改创意喵的某个功能」「部分重新开发」
- 「基于之前结果改进」「完善创意喵」
- 「继续开发创意喵」

每次触发时执行 Phase 0 进行上下文确认。
