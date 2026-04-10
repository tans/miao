# 创意喵平台页面与功能清单

## 一、公开页面

### 1.1 首页
- **路径**: `/`
- **模板**: `templates/index.html`
- **功能**:
  - 平台介绍
  - 任务分类导航
  - 热门任务展示
  - 创作者/商家入驻入口

### 1.2 任务大厅（公开）
- **路径**: `/tasks`
- **模板**: `templates/tasks.html`
- **功能**:
  - 浏览所有可认领任务
  - 分类筛选
  - 关键词搜索
  - 价格排序（升序/降序）
  - 任务卡片展示（标题、单价、剩余数量、分类）

### 1.3 用户登录
- **路径**: `/auth/login`
- **模板**: `templates/auth/login.html`, `templates/mobile/login.html`
- **功能**:
  - 用户名/密码登录
  - 角色切换（创作者/商家）
  - 微信小程序登录入口

### 1.4 用户注册
- **路径**: `/auth/register`
- **模板**: `templates/auth/register.html`, `templates/mobile/register.html`
- **功能**:
  - 用户名、密码、手机号注册
  - 角色选择（创作者/商家）

---

## 二、创作者端

### 2.1 工作台
- **路径**: `/creator/dashboard`
- **模板**: `templates/creator/dashboard.html`
- **功能**:
  - 今日收益统计
  - 待处理任务提醒
  - 我的认领概览
  - 收益图表（7天/30天/90天）

### 2.2 任务大厅
- **路径**: `/creator/task-hall`
- **模板**: `templates/creator/task_hall.html`
- **功能**:
  - 浏览可认领任务（需认证）
  - 分类筛选、关键词搜索
  - 按价格排序
  - 任务详情查看

### 2.3 任务详情
- **路径**: `/creator/task/:id`
- **模板**: `templates/creator/task_detail.html`
- **功能**:
  - 查看任务完整信息
  - 认领任务（需满足等级要求）

### 2.4 我的认领
- **路径**: `/creator/claims`
- **模板**: `templates/creator/claim_list.html`
- **功能**:
  - 查看所有认领的任务
  - 状态筛选（待交付/待验收/已完成/已过期/已取消）
  - 快捷提交作品

### 2.5 交付提交
- **路径**: `/creator/delivery`
- **模板**: `templates/creator/delivery.html`
- **功能**:
  - 提交作品链接/内容
  - 上传附件

### 2.6 我的投稿
- **路径**: `/creator/submissions`
- **模板**: `templates/creator/my_submissions.html`
- **功能**:
  - 查看已提交的稿件
  - 查看审核状态

### 2.7 钱包
- **路径**: `/creator/wallet`
- **模板**: `templates/creator/wallet.html`
- **功能**:
  - 账户余额
  - 冻结保证金
  - 等级信息
  - 总积分/行为积分/交易积分

### 2.8 交易记录
- **路径**: `/creator/transactions`
- **模板**: `templates/creator/transactions.html`
- **功能**:
  - 查看收支明细
  - 筛选交易类型（充值/提现/任务收入/冻结/解冻）

### 2.9 申诉
- **路径**: `/creator/appeal`
- **模板**: `templates/creator/appeal.html`
- **功能**:
  - 对验收结果提起申诉
  - 上传证据

### 2.10 申诉列表
- **路径**: `/creator/appeals`
- **模板**: `templates/creator/appeal_list.html`
- **功能**:
  - 查看我的申诉记录
  - 申诉状态追踪

### 2.11 通知消息
- **路径**: `/creator/notifications`
- **模板**: `templates/creator/notifications.html`
- **功能**:
  - 系统通知列表
  - 标记已读
  - 未读数量提醒

### 2.12 个人资料
- **路径**: `/user/profile`
- **模板**: `templates/user/profile.html`
- **功能**:
  - 查看/修改个人资料
  - 手机号、昵称、头像

### 2.13 修改密码
- **路径**: `/user/password`
- **模板**: `templates/user/password.html`
- **功能**:
  - 修改登录密码

---

## 三、商家端

### 3.1 工作台
- **路径**: `/business/dashboard`
- **模板**: `templates/business/dashboard.html`
- **功能**:
  - 账户余额概览
  - 任务发布统计
  - 支出图表
  - 待处理事项提醒

### 3.2 发布任务
- **路径**: `/business/task/create`
- **模板**: `templates/business/task_create.html`
- **功能**:
  - 创建新任务
  - 设置标题、描述、分类
  - 设置单价、总数量
  - 设置截止时间
  - 预付金额计算

### 3.3 我的任务
- **路径**: `/business/tasks`
- **模板**: `templates/business/task_list.html`
- **功能**:
  - 查看已发布的任务列表
  - 任务状态筛选
  - 任务数据统计（认领数、完成数）

### 3.4 任务详情
- **路径**: `/business/task/:id`
- **模板**: `templates/business/task_detail.html`
- **功能**:
  - 查看任务详情
  - 管理任务（取消任务）

### 3.5 认领审核
- **路径**: `/business/task/:id/claims`
- **模板**: `templates/business/claim_review.html`
- **功能**:
  - 查看任务的所有认领
  - 审核创作者提交的作品
  - 通过/退回操作

### 3.6 稿件审核
- **路径**: `/business/submissions`
- **模板**: `templates/business/submission_review.html`
- **功能**:
  - 查看所有待审核的投稿
  - 批量审核操作

### 3.7 充值
- **路径**: `/business/recharge`
- **模板**: `templates/business/recharge.html`
- **功能**:
  - 账户充值
  - 选择支付方式

### 3.8 交易记录
- **路径**: `/business/transactions`
- **模板**: `templates/business/transactions.html`
- **功能**:
  - 查看资金流水
  - 筛选交易类型

### 3.9 申诉管理
- **路径**: `/business/appeals`
- **模板**: `templates/business/appeal_list.html`
- **功能**:
  - 查看买家提出的申诉
  - 处理申诉

### 3.10 申诉处理
- **路径**: `/business/appeal/:id`
- **模板**: `templates/business/appeal.html`
- **功能**:
  - 查看申诉详情
  - 处理申诉（重新验收等）

### 3.11 通知消息
- **路径**: `/business/notifications`
- **模板**: `templates/business/notifications.html`
- **功能**:
  - 系统通知列表
  - 标记已读

---

## 四、管理端

### 4.1 管理员登录
- **路径**: `/admin/login`
- **模板**: `templates/admin/login.html`
- **功能**:
  - 管理员身份验证

### 4.2 管理后台首页
- **路径**: `/admin/dashboard`
- **模板**: `templates/admin/layout.html`, `templates/admin/dashboard.html`
- **功能**:
  - 平台整体数据概览
  - 用户数/任务数/认领数统计
  - 平台收入统计

### 4.3 用户管理
- **路径**: `/admin/users`
- **模板**: `templates/admin/users.html`
- **功能**:
  - 查看所有用户列表
  - 按角色/关键词筛选
  - 禁用/启用用户
  - 调整用户信用分

### 4.4 任务管理
- **路径**: `/admin/tasks`
- **模板**: `templates/admin/tasks.html`
- **功能**:
  - 查看所有任务
  - 审核任务（通过/拒绝）

### 4.5 任务审核
- **路径**: `/admin/task/:id/review`
- **模板**: `templates/admin/task_review.html`
- **功能**:
  - 查看任务详情
  - 审核操作

### 4.6 认领管理
- **路径**: `/admin/claims`
- **模板**: `templates/admin/task_list.html`
- **功能**:
  - 查看所有认领记录
  - 状态管理

### 4.7 申诉管理
- **路径**: `/admin/appeals`
- **模板**: `templates/admin/appeals.html`
- **功能**:
  - 查看所有申诉
  - 处理申诉

### 4.8 申诉列表
- **路径**: `/admin/appeal/list`
- **模板**: `templates/admin/appeal_list.html`
- **功能**:
  - 申诉记录列表

### 4.9 财务管理
- **路径**: `/admin/finance`
- **模板**: `templates/admin/finance.html`
- **功能**:
  - 平台财务统计
  - 充值/提现记录

---

## 五、移动端页面

### 5.1 移动端首页
- **路径**: `/mobile`
- **模板**: `templates/mobile/index.html`
- **功能**:
  - 简化的任务浏览
  - 移动端适配布局

### 5.2 移动端登录/注册
- **路径**: `/mobile/login`, `/mobile/register`
- **模板**: `templates/mobile/login.html`, `templates/mobile/register.html`
- **功能**:
  - 手机号快捷登录

### 5.3 移动端任务大厅
- **路径**: `/mobile/tasks`
- **模板**: `templates/mobile/index.html`
- **功能**:
  - 任务列表
  - 下拉刷新

### 5.4 移动端任务详情
- **路径**: `/mobile/task/:id`
- **模板**: `templates/mobile/task_detail.html`
- **功能**:
  - 移动端任务详情
  - 认领操作

### 5.5 移动端我的任务
- **路径**: `/mobile/my-tasks`
- **模板**: `templates/mobile/my_tasks.html`
- **功能**:
  - 查看我的发布任务

### 5.6 移动端我的认领
- **路径**: `/mobile/my-claims`
- **模板**: `templates/mobile/my_claims.html`
- **功能**:
  - 查看我的认领
  - 提交作品

### 5.7 移动端作品
- **路径**: `/mobile/works`
- **模板**: `templates/mobile/works.html`
- **功能**:
  - 作品列表

### 5.8 移动端作品详情
- **路径**: `/mobile/work/:id`
- **模板**: `templates/mobile/work_detail.html`
- **功能**:
  - 作品详情

### 5.9 移动端钱包
- **路径**: `/mobile/wallet`
- **模板**: `templates/mobile/wallet.html`
- **功能**:
  - 余额查看
  - 交易记录

### 5.10 移动端交易记录
- **路径**: `/mobile/transactions`
- **模板**: `templates/mobile/transactions.html`
- **功能**:
  - 收支明细

### 5.11 移动端个人中心
- **路径**: `/mobile/mine`
- **模板**: `templates/mobile/mine.html`
- **功能**:
  - 个人信息
  - 设置

### 5.12 移动端设置
- **路径**: `/mobile/settings`
- **模板**: `templates/mobile/settings.html`
- **功能**:
  - 账号设置
  - 退出登录

---

## 六、公共页面

### 6.1 消息中心
- **路径**: `/messages`
- **模板**: `templates/messages.html`
- **功能**:
  - 用户消息列表
  - 标记已读/全部已读
  - 删除消息

### 6.2 帮助中心
- **路径**: `/help`
- **模板**: `templates/help/index.html`
- **功能**:
  - 平台帮助文档

### 6.3 新手教程
- **路径**: `/help/tutorial`
- **模板**: `templates/help/tutorial.html`
- **功能**:
  - 创作者/商家入门指南

### 6.4 常见问题
- **路径**: `/help/faq`
- **模板**: `templates/help/faq.html`
- **功能**:
  - FAQ 列表

### 6.5 错误页面
- **路径**: `/error`
- **模板**: `templates/error.html`
- **功能**:
  - 错误信息展示

---

## 七、页面功能对应表

| 功能模块 | 页面 | 路径 | 说明 |
|---------|------|------|------|
| **认证** | 登录页 | `/auth/login` | 用户名密码/微信登录 |
| | 注册页 | `/auth/register` | 用户注册 |
| | 管理登录 | `/admin/login` | 管理员登录 |
| **任务** | 公开任务列表 | `/tasks` | 所有人可见 |
| | 创作者任务大厅 | `/creator/task-hall` | 需登录 |
| | 商家任务列表 | `/business/tasks` | 商家发布的管理 |
| | 发布任务 | `/business/task/create` | 创建新任务 |
| **认领** | 我的认领 | `/creator/claims` | 创作者查看 |
| | 认领审核 | `/business/task/:id/claims` | 商家审核 |
| **交付** | 交付提交 | `/creator/delivery` | 创作者提交作品 |
| | 稿件审核 | `/business/submissions` | 商家审核作品 |
| **财务** | 创作者钱包 | `/creator/wallet` | 余额/积分 |
| | 商家充值 | `/business/recharge` | 账户充值 |
| | 交易记录 | `/creator/transactions` | 收支明细 |
| **申诉** | 创作者申诉 | `/creator/appeal` | 提起申诉 |
| | 商家申诉 | `/business/appeals` | 处理申诉 |
| | 管理申诉 | `/admin/appeals` | 平台处理 |
| **通知** | 消息中心 | `/messages` | 用户消息 |
| | 通知列表 | `/notifications` | 系统通知 |
| **管理** | 用户管理 | `/admin/users` | 用户列表/禁用 |
| | 任务审核 | `/admin/tasks` | 审核任务 |
| | 财务管理 | `/admin/finance` | 平台财务 |

---

## 八、路由清单汇总

```
公开路由:
  GET  /                           首页
  GET  /tasks                      公开任务列表
  GET  /auth/login                 登录页
  GET  /auth/register              注册页
  GET  /help                       帮助中心
  GET  /help/tutorial              新手教程
  GET  /help/faq                   常见问题

创作者路由 (需认证):
  GET  /creator/dashboard           工作台
  GET  /creator/task-hall           任务大厅
  GET  /creator/task/:id           任务详情
  GET  /creator/claims             我的认领
  GET  /creator/delivery           交付提交
  GET  /creator/submissions        我的投稿
  GET  /creator/wallet             钱包
  GET  /creator/transactions       交易记录
  GET  /creator/appeal             申诉
  GET  /creator/appeals            申诉列表
  GET  /creator/notifications      通知

商家路由 (需认证):
  GET  /business/dashboard         工作台
  GET  /business/task/create       发布任务
  GET  /business/tasks             我的任务
  GET  /business/task/:id          任务详情
  GET  /business/task/:id/claims   认领审核
  GET  /business/submissions       稿件审核
  GET  /business/recharge          充值
  GET  /business/transactions      交易记录
  GET  /business/appeals           申诉管理
  GET  /business/appeal/:id       申诉处理
  GET  /business/notifications     通知

管理员路由 (需管理员权限):
  GET  /admin/login                管理员登录
  GET  /admin/dashboard            管理后台首页
  GET  /admin/users                用户管理
  GET  /admin/tasks                任务管理
  GET  /admin/task/:id/review      任务审核
  GET  /admin/claims              认领管理
  GET  /admin/appeals             申诉管理
  GET  /admin/finance             财务管理

移动端路由:
  GET  /mobile                     移动端首页
  GET  /mobile/login               移动端登录
  GET  /mobile/register            移动端注册
  GET  /mobile/tasks               移动端任务
  GET  /mobile/task/:id            移动端任务详情
  GET  /mobile/my-tasks            移动端我的任务
  GET  /mobile/my-claims           移动端我的认领
  GET  /mobile/works               移动端作品
  GET  /mobile/work/:id            移动端作品详情
  GET  /mobile/wallet             移动端钱包
  GET  /mobile/transactions        移动端交易记录
  GET  /mobile/mine                移动端个人中心
  GET  /mobile/settings            移动端设置

公共路由:
  GET  /user/profile               个人资料
  PUT  /user/profile               更新资料
  GET  /user/password             修改密码
  PUT  /user/password              更新密码
  GET  /messages                  消息中心
  GET  /notifications             通知列表
  PUT  /notifications/:id/read     标记已读
  GET  /notifications/unread-count 未读数量
  PUT  /notifications/read-all    标记全部已读
```
