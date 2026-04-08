---
name: html-frontend
description: "HTML/Bootstrap 5 前端开发技能。实现创意喵平台商家端、创作者端、管理端页面。使用 Gin 模板。"
---

# HTML/Bootstrap 5 前端开发技能

## 概述

构建创意喵平台的 HTML/Bootstrap 5 前端。使用 Gin 模板引擎，HTML 文件放在 `web/templates/` 目录。

## 项目结构

```
web/
├── static/
│   ├── css/custom.css      # 自定义样式
│   ├── js/app.js          # API 调用、认证、UI 逻辑
│   └── images/            # 图片资源
└── templates/
    ├── layout.html        # 基础布局（包含 header/footer）
    ├── auth/              # 认证相关页面
    │   ├── login.html
    │   └── register.html
    ├── business/          # 商家端
    │   ├── dashboard.html
    │   ├── task_create.html
    │   ├── task_list.html
    │   ├── task_detail.html
    │   └── submission_review.html
    ├── creator/            # 创作者端
    │   ├── dashboard.html
    │   ├── task_hall.html
    │   ├── task_detail.html
    │   └── my_submissions.html
    └── admin/              # 管理端
        ├── dashboard.html
        ├── user_list.html
        └── task_list.html
```

## 布局模式

### 基础布局（layout.html）

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{% raw %}{% block title %}创意喵{% endblock %}{% endraw %}</title>
  <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
  <link href="/static/css/custom.css" rel="stylesheet">
</head>
<body>
  <nav class="navbar navbar-expand-lg navbar-dark bg-primary">
    <!-- 导航栏 -->
  </nav>
  <main class="container mt-4">
    {% raw %}{% block content %}{% endblock %}{% endraw %}
  </main>
  <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>
  <script src="/static/js/app.js"></script>
  {% raw %}{% block extra_js %}{% endblock %}{% endraw %}
</body>
</html>
```

## API 调用（app.js）

```javascript
const API_BASE = '/api/v1';

function apiRequest(endpoint, method = 'GET', body = null) {
  const token = localStorage.getItem('token');
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  return fetch(`${API_BASE}${endpoint}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : null
  }).then(res => res.json());
}

function handleApiError(err) {
  console.error('API Error:', err);
  alert('请求失败，请稍后重试');
}
```

## 页面列表

### Phase 2: 用户系统

#### /auth/login（login.html）
- username、password 输入
- 登录按钮 → POST /api/v1/auth/login
- 成功后存储 JWT，重定向
- role 别重定向：business→/business/dashboard、creator→/creator/dashboard、admin→/admin/dashboard

#### /auth/register（register.html）
- username、password、email、role（business/creator）选择
- POST /api/v1/auth/register
- 成功后跳转登录页面

### Phase 3: 商家端-任务管理

#### /business/dashboard（business/dashboard.html）
- 商家个人信息
- 快捷链接：创建任务、我的任务、收益明细

#### /business/task/create（business/task_create.html）
- 表单：标题、描述、悬赏金额
- POST /api/v1/tasks
- 认证：JWT role=business

#### /business/tasks（business/task_list.html）
- 我的任务列表（GET /api/v1/tasks/my）
- 任务状态标签（open/in_review/closed）
- 点击 → task_detail

#### /business/task/:id（business/task_detail.html）
- 任务详细信息
- 投稿管理表格（GET /api/v1/submissions?task_id=:id）
- 审核按钮（PUT /api/v1/submissions/:id/review）

### Phase 4: 创作者端-投稿

#### /creator/dashboard（creator/dashboard.html）
- 创作者个人信息
- 快捷链接：任务大厅、我的投稿、收益明细

#### /creator/tasks（creator/task_hall.html）
- 全部任务列表（GET /api/v1/tasks）
- 筛选：状态=open
- 点击 → task_detail

#### /creator/task/:id（creator/task_detail.html）
- 任务详细信息
- 投稿表单：内容输入
- POST /api/v1/submissions

#### /creator/submissions（creator/my_submissions.html）
- 我的投稿列表（GET /api/v1/submissions?creator_id=:user_id）
- 状态标签

### Phase 9: 管理端

#### /admin/dashboard（admin/dashboard.html）
- 统计卡片：用户总数、任务总数、投稿总数、交易总额
- GET /api/v1/admin/dashboard

#### /admin/users（admin/user_list.html）
- 用户列表（GET /api/v1/admin/users）
- 角色标签、信用分显示
- 管理：PUT /api/v1/admin/users/:id/status

#### /admin/tasks（admin/task_list.html）
- 全部任务列表（GET /api/v1/admin/tasks）

## 状态管理

```javascript
// JWT token 存储
localStorage.setItem('token', data.data.token);
localStorage.setItem('user_id', data.data.user_id);
localStorage.setItem('username', data.data.username);
localStorage.setItem('role', data.data.role);

// 登录状态确认
function isLoggedIn() {
  return !!localStorage.getItem('token');
}

function requireRole(role) {
  if (!isLoggedIn()) {
    window.location.href = '/auth/login';
    return false;
  }
  if (localStorage.getItem('role') !== role) {
    alert('无权限访问');
    window.location.href = '/';
    return false;
  }
  return true;
}
```

## 表单验证

```javascript
function validateForm(formId) {
  const form = document.getElementById(formId);
  if (!form.checkValidity()) {
    form.classList.add('was-validated');
    return false;
  }
  return true;
}
```

## Phase 优先级

1. **Phase 2 pages**（login、register）— 可与后端同步开发
2. **Phase 3-4 pages**（business/creator dashboards）— API 完成后
3. **Phase 9 admin pages**— 管理 API 完成后
