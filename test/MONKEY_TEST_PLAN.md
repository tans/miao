# 创意喵平台 E2E 测试计划

**版本**: V2.0
**更新日期**: 2026-04-11
**基于**: `docs/page.md` 页面清单

---

## 一、测试范围

### 1.1 页面覆盖率目标

| 模块 | 页面数 | 目标覆盖 |
|------|--------|----------|
| 公开页面 | 4 | 100% |
| 创作者端 | 13 | 核心流程 |
| 商家端 | 11 | 核心流程 |
| 管理端 | 9 | 核心功能 |
| 移动端 | 12 | 核心页面 |
| 公共页面 | 5 | 100% |

### 1.2 核心测试场景

1. **认证流程**: 注册 → 登录 → 角色切换 → 登出
2. **创作者流程**: 登录 → 浏览任务 → 认领 → 提交交付 → 查看收益
3. **商家流程**: 登录 → 充值 → 发布任务 → 审核验收 → 查看支出
4. **管理流程**: 登录 → 审核任务 → 用户管理

---

## 二、测试环境

| 项目 | 值 |
|------|---|
| 服务器地址 | `http://localhost:8888` |
| 测试工具 | Playwright (Go/JavaScript) |
| 浏览器 | Chromium (headless) |

---

## 三、公开页面测试

### TC-PUBLIC-01: 首页
- **路径**: `/`
- **验证点**:
  - 页面加载成功 (200 OK)
  - 包含任务分类导航
  - 包含热门任务展示
  - 包含入驻入口

### TC-PUBLIC-02: 公开任务大厅
- **路径**: `/tasks`
- **验证点**:
  - 页面加载成功
  - 任务列表展示
  - 分类筛选器存在
  - 搜索框存在

### TC-PUBLIC-03: 用户登录页
- **路径**: `/auth/login.html`
- **验证点**:
  - 页面加载成功
  - 用户名输入框存在
  - 密码输入框存在
  - 登录按钮存在
  - 角色切换存在
  - 微信登录入口存在

### TC-PUBLIC-04: 用户注册页
- **路径**: `/auth/register.html`
- **验证点**:
  - 页面加载成功
  - 用户名输入框存在
  - 密码输入框存在
  - 手机号输入框存在
  - 角色选择存在
  - 注册按钮存在

---

## 四、创作者端测试

### TC-CREATOR-01: 创作者工作台
- **路径**: `/creator/dashboard`
- **前置**: 创作者已登录
- **验证点**:
  - 今日收益统计显示
  - 待处理任务提醒
  - 我的认领概览
  - 收益图表存在

### TC-CREATOR-02: 创作者任务大厅
- **路径**: `/creator/task-hall`
- **前置**: 创作者已登录
- **验证点**:
  - 可认领任务列表加载
  - 分类筛选器工作
  - 关键词搜索工作
  - 价格排序工作
  - 任务卡片显示正确

### TC-CREATOR-03: 任务详情页
- **路径**: `/creator/task/:id`
- **前置**: 创作者已登录，存在可认领任务
- **验证点**:
  - 任务完整信息显示
  - 认领按钮存在（白银及以上等级）
  - 等级不足提示（青铜用户）

### TC-CREATOR-04: 我的认领列表
- **路径**: `/creator/claims`
- **前置**: 创作者已登录，有认领记录
- **验证点**:
  - 认领列表加载
  - 状态筛选器工作
  - 快捷提交入口存在

### TC-CREATOR-05: 交付提交页
- **路径**: `/creator/delivery`
- **前置**: 创作者已登录，有待交付认领
- **验证点**:
  - 作品链接/内容输入框存在
  - 附件上传功能存在
  - 提交按钮存在

### TC-CREATOR-06: 我的投稿
- **路径**: `/creator/submissions`
- **前置**: 创作者已登录，有提交记录
- **验证点**:
  - 投稿列表加载
  - 审核状态显示正确

### TC-CREATOR-07: 创作者钱包
- **路径**: `/creator/wallet`
- **前置**: 创作者已登录
- **验证点**:
  - 账户余额显示
  - 冻结保证金显示
  - 等级信息显示
  - 总积分/行为积分/交易积分显示

### TC-CREATOR-08: 创作者交易记录
- **路径**: `/creator/transactions`
- **前置**: 创作者已登录
- **验证点**:
  - 收支明细列表加载
  - 交易类型筛选工作

### TC-CREATOR-09: 创作者申诉
- **路径**: `/creator/appeal`
- **前置**: 创作者已登录，有可申诉的认领
- **验证点**:
  - 申诉表单存在
  - 证据上传功能存在
  - 提交按钮存在

### TC-CREATOR-10: 创作者申诉列表
- **路径**: `/creator/appeals`
- **前置**: 创作者已登录，有申诉记录
- **验证点**:
  - 申诉记录列表加载
  - 状态追踪显示正确

### TC-CREATOR-11: 创作者通知
- **路径**: `/creator/notifications`
- **前置**: 创作者已登录
- **验证点**:
  - 通知列表加载
  - 标记已读功能存在
  - 未读数量显示正确

### TC-CREATOR-12: 个人资料
- **路径**: `/user/profile`
- **前置**: 用户已登录
- **验证点**:
  - 个人资料显示
  - 修改入口存在

### TC-CREATOR-13: 修改密码
- **路径**: `/user/password`
- **前置**: 用户已登录
- **验证点**:
  - 旧密码输入框存在
  - 新密码输入框存在
  - 确认密码输入框存在
  - 提交按钮存在

---

## 五、商家端测试

### TC-BUSINESS-01: 商家工作台
- **路径**: `/business/dashboard`
- **前置**: 商家已登录
- **验证点**:
  - 账户余额概览显示
  - 任务发布统计显示
  - 支出图表存在
  - 待处理事项提醒存在

### TC-BUSINESS-02: 发布任务
- **路径**: `/business/task/create`
- **前置**: 商家已登录，账户有余额
- **验证点**:
  - 任务表单加载
  - 标题输入框存在
  - 描述输入框存在
  - 分类选择存在
  - 单价输入框存在
  - 总数量输入框存在
  - 截止时间选择存在
  - 预付金额计算正确

### TC-BUSINESS-03: 我的任务列表
- **路径**: `/business/tasks`
- **前置**: 商家已登录，有发布任务
- **验证点**:
  - 任务列表加载
  - 状态筛选器工作
  - 任务数据统计显示

### TC-BUSINESS-04: 商家任务详情
- **路径**: `/business/task/:id`
- **前置**: 商家已登录，是任务所有者
- **验证点**:
  - 任务详情显示
  - 取消任务按钮存在（条件允许时）

### TC-BUSINESS-05: 认领审核
- **路径**: `/business/task/:id/claims`
- **前置**: 商家已登录，任务有认领
- **验证点**:
  - 认领列表加载
  - 作品内容显示
  - 通过/退回按钮存在

### TC-BUSINESS-06: 稿件审核
- **路径**: `/business/submissions`
- **前置**: 商家已登录，有待审核投稿
- **验证点**:
  - 投稿列表加载
  - 批量审核功能存在

### TC-BUSINESS-07: 商家充值
- **路径**: `/business/recharge`
- **前置**: 商家已登录
- **验证点**:
  - 充值表单加载
  - 金额输入框存在
  - 支付方式选择存在

### TC-BUSINESS-08: 商家交易记录
- **路径**: `/business/transactions`
- **前置**: 商家已登录
- **验证点**:
  - 资金流水列表加载
  - 交易类型筛选工作

### TC-BUSINESS-09: 商家申诉列表
- **路径**: `/business/appeals`
- **前置**: 商家已登录，有申诉记录
- **验证点**:
  - 申诉列表加载
  - 处理入口存在

### TC-BUSINESS-10: 商家申诉处理
- **路径**: `/business/appeal/:id`
- **前置**: 商家已登录，是相关申诉的处理方
- **验证点**:
  - 申诉详情显示
  - 处理操作存在

### TC-BUSINESS-11: 商家通知
- **路径**: `/business/notifications`
- **前置**: 商家已登录
- **验证点**:
  - 通知列表加载
  - 标记已读功能存在

---

## 六、管理端测试

### TC-ADMIN-01: 管理员登录
- **路径**: `/admin/login`
- **验证点**:
  - 登录表单加载
  - 管理员认证工作

### TC-ADMIN-02: 管理后台首页
- **路径**: `/admin/dashboard`
- **前置**: 管理员已登录
- **验证点**:
  - 平台数据概览显示
  - 用户数/任务数/认领数统计显示
  - 平台收入统计显示

### TC-ADMIN-03: 用户管理
- **路径**: `/admin/users`
- **前置**: 管理员已登录
- **验证点**:
  - 用户列表加载
  - 角色筛选工作
  - 关键词搜索工作
  - 禁用/启用功能存在

### TC-ADMIN-04: 任务管理
- **路径**: `/admin/tasks`
- **前置**: 管理员已登录
- **验证点**:
  - 任务列表加载
  - 审核操作存在

### TC-ADMIN-05: 任务审核页
- **路径**: `/admin/task/:id/review`
- **前置**: 管理员已登录，有待审核任务
- **验证点**:
  - 任务详情显示
  - 通过/拒绝按钮存在

### TC-ADMIN-06: 认领管理
- **路径**: `/admin/claims`
- **前置**: 管理员已登录
- **验证点**:
  - 认领记录列表加载
  - 状态管理功能存在

### TC-ADMIN-07: 申诉管理
- **路径**: `/admin/appeals`
- **前置**: 管理员已登录
- **验证点**:
  - 申诉列表加载
  - 处理功能存在

### TC-ADMIN-08: 财务管理
- **路径**: `/admin/finance`
- **前置**: 管理员已登录
- **验证点**:
  - 财务统计数据显示
  - 充值/提现记录显示

---

## 七、用户流程测试

### FLOW-01: 创作者完整流程
```
1. 访问 /auth/register.html
2. 注册新账号（选择创作者角色）
3. 登录成功，自动跳转工作台
4. 访问 /creator/task-hall 浏览任务
5. 点击任务进入详情页
6. 点击认领按钮
7. 跳转我的认领列表
8. 点击提交作品
9. 填写作品链接并提交
10. 访问 /creator/wallet 查看余额变化
11. 访问 /creator/transactions 查看交易记录
12. 登出
```

### FLOW-02: 商家完整流程
```
1. 访问 /auth/register.html
2. 注册新账号（选择商家角色）
3. 登录成功，自动跳转工作台
4. 访问 /business/recharge 充值
5. 跳转 /business/task/create 发布任务
6. 跳转 /business/tasks 查看任务列表
7. 等待管理员审核（需手动或API触发）
8. 访问任务认领审核页
9. 查看创作者提交的作品
10. 点击通过验收
11. 访问 /business/transactions 查看资金流水
12. 登出
```

### FLOW-03: 管理员审核流程
```
1. 访问 /admin/login
2. 使用管理员账号登录
3. 跳转 /admin/dashboard 查看概览
4. 访问 /admin/tasks 查看待审核任务
5. 点击任务进入审核页
6. 点击通过审核
7. 访问 /admin/users 管理用户
8. 禁用某用户
9. 登出
```

---

## 八、边界条件测试

### EDGE-01: 未登录访问需认证页面
- 访问 `/creator/dashboard`（未登录）
- **期望**: 重定向到登录页

### EDGE-02: 创作者访问商家页面
- 创作者登录后访问 `/business/task/create`
- **期望**: 403 禁止访问或重定向

### EDGE-03: 商家访问创作者页面
- 商家登录后访问 `/creator/task-hall`
- **期望**: 正常访问（商家也有创作者权限）

### EDGE-04: 余额不足发布任务
- 商家余额为 0 时发布任务
- **期望**: 提示余额不足

### EDGE-05: 青铜用户认领任务
- 青铜用户（level=1）尝试认领
- **期望**: 提示等级不足，需白银及以上

### EDGE-06: 重复认领
- 创作者已认领某任务，再次认领
- **期望**: 提示已认领

### EDGE-07: 访问不存在的任务
- 访问 `/creator/task/99999`
- **期望**: 404 或任务不存在提示

### EDGE-08: 商家访问他人任务
- 商家 B 访问商家 A 的任务详情
- **期望**: 403 禁止访问

---

## 九、测试执行

### 9.1 Playwright 安装

```bash
# JavaScript/Node.js
npm install -D @playwright/test
npx playwright install chromium

# 或 Go
go get github.com/playwright-community/playwright-go
```

### 9.2 运行测试

```bash
# 运行所有测试
npx playwright test

# 运行特定测试
npx playwright test --grep "TC-CREATOR"

# 生成报告
npx playwright test --reporter=html
```

### 9.3 测试配置

```javascript
// playwright.config.js
module.exports = {
  testDir: './test/e2e',
  timeout: 30000,
  use: {
    baseURL: 'http://localhost:8888',
    headless: true,
    screenshot: 'only-on-failure',
  },
};
```

---

## 十、测试用例状态

| 用例ID | 描述 | 优先级 | 状态 |
|--------|------|--------|------|
| TC-PUBLIC-01 | 首页 | P0 | TODO |
| TC-PUBLIC-02 | 公开任务大厅 | P0 | TODO |
| TC-PUBLIC-03 | 用户登录页 | P0 | TODO |
| TC-PUBLIC-04 | 用户注册页 | P0 | TODO |
| TC-CREATOR-01 | 创作者工作台 | P0 | TODO |
| TC-CREATOR-02 | 创作者任务大厅 | P0 | TODO |
| TC-CREATOR-03 | 任务详情页 | P0 | TODO |
| TC-CREATOR-04 | 我的认领列表 | P0 | TODO |
| TC-CREATOR-05 | 交付提交页 | P0 | TODO |
| TC-CREATOR-07 | 创作者钱包 | P1 | TODO |
| TC-BUSINESS-01 | 商家工作台 | P0 | TODO |
| TC-BUSINESS-02 | 发布任务 | P0 | TODO |
| TC-BUSINESS-05 | 认领审核 | P0 | TODO |
| TC-BUSINESS-07 | 商家充值 | P0 | TODO |
| FLOW-01 | 创作者完整流程 | P0 | TODO |
| FLOW-02 | 商家完整流程 | P0 | TODO |
| EDGE-01 | 未登录访问 | P0 | TODO |

---

## 十一、附录

### A. 页面路径速查

**公开页面**:
- `/` - 首页
- `/tasks` - 公开任务列表
- `/auth/login.html` - 登录
- `/auth/register.html` - 注册

**创作者端**:
- `/creator/dashboard` - 工作台
- `/creator/task-hall` - 任务大厅
- `/creator/task/:id` - 任务详情
- `/creator/claims` - 我的认领
- `/creator/delivery` - 交付提交
- `/creator/submissions` - 我的投稿
- `/creator/wallet` - 钱包
- `/creator/transactions` - 交易记录
- `/creator/appeal` - 申诉
- `/creator/appeals` - 申诉列表
- `/creator/notifications` - 通知

**商家端**:
- `/business/dashboard` - 工作台
- `/business/task/create` - 发布任务
- `/business/tasks` - 我的任务
- `/business/task/:id` - 任务详情
- `/business/task/:id/claims` - 认领审核
- `/business/submissions` - 稿件审核
- `/business/recharge` - 充值
- `/business/transactions` - 交易记录
- `/business/appeals` - 申诉管理
- `/business/appeal/:id` - 申诉处理
- `/business/notifications` - 通知

**管理端**:
- `/admin/login` - 管理员登录
- `/admin/dashboard` - 管理后台首页
- `/admin/users` - 用户管理
- `/admin/tasks` - 任务管理
- `/admin/task/:id/review` - 任务审核
- `/admin/claims` - 认领管理
- `/admin/appeals` - 申诉管理
- `/admin/finance` - 财务管理

### B. 创作者等级说明

| 等级 | Level | 每日限额 | 认领限制 | 保证金 |
|------|-------|---------|---------|-------|
| 青铜 | 1 | 3单 | 需审核 | 10元/单 |
| 白银 | 2 | 10单 | 直接认领 | 无 |
| 黄金 | 3 | 20单 | 直接认领 | 无 |
| 钻石 | 4 | 50单 | 直接认领 | 无 |
