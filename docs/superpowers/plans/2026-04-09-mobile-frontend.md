# 移动端独立产品线实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `/mobile/` 路径下实现独立的移动端产品线，包含任务大厅、过审作品、我的三个主 Tab，采用小红书风格的 C 端 app 视觉。

**Architecture:** 新增独立的移动端模板目录和样式体系，复用现有 API，通过前端层重新组织信息架构与视觉风格，不修改后端业务逻辑。

**Tech Stack:** HTML + Gin 模板 + 原生 CSS + 现有 API

---

## Task 1: 移动端基础框架

**Files:**
- Create: `web/templates/mobile/layout.html`
- Create: `web/static/mobile/css/mobile.css`
- Create: `web/static/mobile/js/mobile.js`
- Create: `internal/handler/mobile.go`
- Modify: `internal/router/router.go`

- [ ] **Step 1.1: 创建移动端样式文件**

创建 `web/static/mobile/css/mobile.css`，包含小红书风格的基础样式、底部 Tab、卡片、按钮等组件。

- [ ] **Step 1.2: 创建移动端 JS 文件**

创建 `web/static/mobile/js/mobile.js`，包含 Tab 切换、API 调用、无限滚动等基础功能。

- [ ] **Step 1.3: 创建移动端布局模板**

创建 `web/templates/mobile/layout.html`，定义移动端页面的基础结构：
- `<head>` 引入移动端专用 CSS/JS
- 底部 Tab 导航组件
- 内容区域占位符 `{{template "content" .}}`

- [ ] **Step 1.4: 创建移动端 handler**

创建 `internal/handler/mobile.go`，包含：
- `MobileIndex()` - 任务大厅首页
- `MobileWorks()` - 过审作品页
- `MobileMine()` - 我的页面
- `MobileTaskDetail()` - 任务详情
- `MobileWorkDetail()` - 作品详情

- [ ] **Step 1.5: 注册移动端路由**

修改 `internal/router/router.go`，新增 `/mobile/` 路由组：
```go
mobile := r.Group("/mobile")
{
    mobile.GET("/", handler.MobileIndex)
    mobile.GET("/works", handler.MobileWorks)
    mobile.GET("/mine", handler.MobileMine)
    mobile.GET("/task/:id", handler.MobileTaskDetail)
    mobile.GET("/work/:id", handler.MobileWorkDetail)
}
```

---

## Task 2: 任务大厅页面（首页）

**Files:**
- Create: `web/templates/mobile/index.html`
- Create: `web/templates/mobile/components/task_card.html`

- [ ] **Step 2.1: 创建任务卡片组件**

创建 `web/templates/mobile/components/task_card.html`，小红书风格的任务卡片：
- 大图封面（16:9 或 4:3）
- 任务标题（2 行截断）
- 价格标签（醒目显示）
- 标签组（类型、难度）
- 发布者头像和昵称

- [ ] **Step 2.2: 创建任务大厅页面**

创建 `web/templates/mobile/index.html`：
- 顶部搜索框
- 分类标签横向滚动（全部、图文、视频、直播等）
- 任务卡片流（瀑布流或双列）
- 无限滚动加载更多

- [ ] **Step 2.3: 实现任务列表 API 调用**

在 `mobile.js` 中实现：
- `loadTasks(page, category)` - 调用 `/api/v1/creator/tasks`
- 分页加载
- 下拉刷新
- 上拉加载更多

- [ ] **Step 2.4: 实现 MobileIndex handler**

在 `mobile.go` 中实现 `MobileIndex()`：
- 渲染 `mobile/index.html`
- 传递初始数据（首屏任务列表）
- 设置页面标题和 meta

---

## Task 3: 过审作品页面

**Files:**
- Create: `web/templates/mobile/works.html`
- Create: `web/templates/mobile/components/work_card.html`

- [ ] **Step 3.1: 创建作品卡片组件**

创建 `web/templates/mobile/components/work_card.html`：
- 作品封面图（正方形或 4:3）
- 作品标题
- 创作者信息
- 点赞数、浏览数

- [ ] **Step 3.2: 创建过审作品页面**

创建 `web/templates/mobile/works.html`：
- 顶部筛选（最新、最热、最多点赞）
- 作品卡片流（双列瀑布流）
- 无限滚动

- [ ] **Step 3.3: 实现作品列表 API 调用**

在 `mobile.js` 中实现：
- `loadWorks(page, sort)` - 调用过审作品 API
- 分页加载

- [ ] **Step 3.4: 实现 MobileWorks handler**

在 `mobile.go` 中实现 `MobileWorks()`：
- 渲染 `mobile/works.html`
- 传递初始作品列表

---

## Task 4: 我的页面

**Files:**
- Create: `web/templates/mobile/mine.html`

- [ ] **Step 4.1: 创建我的页面**

创建 `web/templates/mobile/mine.html`：
- 顶部用户信息卡片（头像、昵称、简介）
- 钱包余额卡片（大字号显示余额，跳转钱包详情）
- 功能入口列表：
  - 我发布的任务
  - 我领取的任务
  - 我的作品
  - 我的收益
  - 设置

- [ ] **Step 4.2: 实现 MobileMine handler**

在 `mobile.go` 中实现 `MobileMine()`：
- 获取当前用户信息
- 获取钱包余额
- 渲染 `mobile/mine.html`

---

## Task 5: 任务详情页

**Files:**
- Create: `web/templates/mobile/task_detail.html`

- [ ] **Step 5.1: 创建任务详情页**

创建 `web/templates/mobile/task_detail.html`：
- 顶部大图轮播（任务示例图）
- 任务标题和价格
- 任务描述（富文本）
- 要求和规则
- 发布者信息
- 底部固定按钮（立即领取）

- [ ] **Step 5.2: 实现 MobileTaskDetail handler**

在 `mobile.go` 中实现 `MobileTaskDetail()`：
- 根据 ID 获取任务详情
- 渲染 `mobile/task_detail.html`

---

## Task 6: 作品详情页

**Files:**
- Create: `web/templates/mobile/work_detail.html`

- [ ] **Step 6.1: 创建作品详情页**

创建 `web/templates/mobile/work_detail.html`：
- 作品大图/视频
- 作品标题和描述
- 创作者信息
- 点赞、收藏、分享按钮
- 相关任务信息

- [ ] **Step 6.2: 实现 MobileWorkDetail handler**

在 `mobile.go` 中实现 `MobileWorkDetail()`：
- 根据 ID 获取作品详情
- 渲染 `mobile/work_detail.html`

---

## Task 7: 测试和优化

- [ ] **Step 7.1: 测试移动端页面**

在不同设备和浏览器上测试：
- iPhone Safari
- Android Chrome
- 不同屏幕尺寸（375px, 414px, 768px）

- [ ] **Step 7.2: 性能优化**

- 图片懒加载
- CSS/JS 压缩
- 首屏加载优化

- [ ] **Step 7.3: 样式隔离验证**

确保移动端样式不影响桌面端，桌面端样式不影响移动端。

---

## 实施建议

1. **推荐使用 subagent-driven-development**：每个 Task 派发独立 subagent，任务间有审查点
2. **TDD 流程**：先写测试，再实现功能
3. **增量交付**：完成 Task 1-2 后即可预览任务大厅
4. **复用 API**：最大化复用现有 `/api/v1/creator/` 和 `/api/v1/business/` 接口
5. **独立性**：移动端与桌面端完全隔离，互不影响
