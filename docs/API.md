# 创意喵平台 API 接口文档

## 基础信息

- **Base URL**: `http://localhost:8080/api/v1`
- **认证方式**: JWT Bearer Token
- **请求格式**: `application/json`
- **响应格式**: `application/json`

## 通用响应格式

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

### 状态码说明

| Code | 说明 |
|------|------|
| 0 | 成功 |
| 40001 | 参数错误 |
| 40101 | 未登录/认证失败 |
| 40301 | 权限不足 |
| 40401 | 资源不存在 |
| 50001 | 服务器内部错误 |

---

## 1. 认证模块

### 1.1 用户注册

**接口**: `POST /auth/register`

**权限**: 公开

**请求参数**:
```json
{
  "username": "testuser",
  "password": "password123",
  "phone": "13800138000",
  "role": "creator"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 是 | 用户名，3-20字符 |
| password | string | 是 | 密码，6-50字符 |
| phone | string | 是 | 手机号 |
| role | string | 是 | 角色：creator(创作者) / business(商家) |

**响应示例**:
```json
{
  "code": 0,
  "message": "注册成功",
  "data": {
    "user_id": 1
  }
}
```

---

### 1.2 用户登录

**接口**: `POST /auth/login`

**权限**: 公开

**请求参数**:
```json
{
  "username": "testuser",
  "password": "password123"
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "登录成功",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": 1,
      "username": "testuser",
      "role": "creator",
      "level": 1,
      "level_name": "新手创作者",
      "balance": 0,
      "total_score": 100
    }
  }
}
```

---

### 1.3 微信小程序登录

**接口**: `POST /auth/wechat-mini-login`

**权限**: 公开

**描述**: 通过微信小程序 `wx.login` 返回的 code 进行登录，自动创建新用户或返回已存在用户。

**请求参数**:
```json
{
  "code": "xxxxxxxxxxxxxxx"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| code | string | 是 | wx.login 返回的 code |

**响应示例**:
```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": 1,
      "username": "wechat_xxxx",
      "role": "creator",
      "level": 1,
      "level_name": "新手创作者",
      "balance": 0,
      "total_score": 0
    },
    "is_new": true
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| token | string | JWT 认证令牌 |
| user | object | 用户信息 |
| is_new | boolean | 是否为新创建的用户 |

**说明**:
- 如果用户已存在，直接返回用户信息和 token
- 如果用户不存在，自动创建新用户（角色默认为创作者）
- 新用户用户名格式：`wechat_xxxx`（基于 openid）

---

## 2. 用户模块

### 2.1 获取当前用户信息

**接口**: `GET /users/me`

**权限**: 需要登录

**请求头**:
```
Authorization: Bearer {token}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "username": "testuser",
    "phone": "13800138000",
    "role": "creator",
    "level": 1,
    "balance": 100.50,
    "total_score": 150
  }
}
```

---

### 2.2 更新用户资料

**接口**: `PUT /users/me`

**权限**: 需要登录

**请求参数**:
```json
{
  "phone": "13900139000",
  "email": "user@example.com"
}
```

---

### 2.3 修改密码

**接口**: `PUT /user/password`

**权限**: 需要登录

**请求参数**:
```json
{
  "old_password": "oldpass123",
  "new_password": "newpass456"
}
```

---

## 3. 创作者模块

### 3.1 获取任务列表

**接口**: `GET /creator/tasks`

**权限**: 创作者角色

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认1 |
| limit | int | 否 | 每页数量，默认20，最大100 |
| category | int | 否 | 分类ID，0为全部 |
| keyword | string | 否 | 搜索关键词 |
| sort | string | 否 | 排序方式：created_at(默认), price_desc, price_asc |

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 50,
    "page": 1,
    "limit": 20,
    "data": [
      {
        "id": 1,
        "title": "小红书种草文案",
        "description": "需要撰写产品种草文案",
        "category": 1,
        "unit_price": 50.00,
        "total_count": 10,
        "remaining_count": 8,
        "status": 1,
        "created_at": "2024-01-01T10:00:00Z"
      }
    ]
  }
}
```

---

### 3.2 认领任务

**接口**: `POST /creator/claim`

**权限**: 创作者角色（Lv0起即可认领）

**请求参数**:
```json
{
  "task_id": 1
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "认领成功",
  "data": {
    "claim_id": 1,
    "expires_at": "2024-01-02T10:00:00Z"
  }
}
```

**错误码**:
- `40303`: 今日认领数已达上限

---

### 3.3 获取我的认领列表

**接口**: `GET /creator/claims`

**权限**: 创作者角色

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "task_id": 1,
      "task_title": "小红书种草文案",
      "creator_id": 1,
      "status": 0,
      "content": "",
      "expires_at": "2024-01-02T10:00:00Z",
      "created_at": "2024-01-01T10:00:00Z"
    }
  ]
}
```

**认领状态**:
- `0`: 待交付
- `1`: 待验收
- `2`: 已完成
- `3`: 已过期
- `4`: 已取消
- `5`: 已退回

---

### 3.4 提交交付

**接口**: `PUT /creator/claim/:id/submit`

**权限**: 创作者角色

**请求参数**:
```json
{
  "content": "https://example.com/work.pdf",
  "materials": [
    {
      "file_name": "video.mp4",
      "file_path": "/uploads/2024/01/video.mp4",
      "file_size": 10240000,
      "file_type": "video/mp4",
      "thumbnail_path": "/uploads/2024/01/thumb.jpg"
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| content | string | 是 | 作品链接或内容描述 |
| materials | array | 否 | 媒体文件列表（先通过 `/upload` 接口上传） |
| materials[].file_name | string | 是 | 文件名 |
| materials[].file_path | string | 是 | 文件路径（上传后返回的路径） |
| materials[].file_size | int | 否 | 文件大小（字节） |
| materials[].file_type | string | 是 | 文件类型（如 video/mp4, image/jpeg） |
| materials[].thumbnail_path | string | 否 | 缩略图路径 |

**响应示例**:
```json
{
  "code": 0,
  "message": "提交成功",
  "data": null
}
```

---

### 3.5 获取钱包信息

**接口**: `GET /creator/wallet`

**权限**: 创作者角色

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "balance": 100.50,
    "margin_frozen": 10.00,
    "total_score": 150,
    "behavior_score": 100,
    "trade_score": 50.0,
    "level": 1,
    "level_name": "青铜"
  }
}
```

**等级说明**:
- `Lv0`: 试用创作者 - 每日3单，10%抽成
- `Lv1`: 新手创作者 - 每日8单，10%抽成
- `Lv2`: 活跃创作者 - 每日15单，10%抽成
- `Lv3`: 优质创作者 - 每日30单，10%抽成
- `Lv4`: 金牌创作者 - 每日50单，5%抽成
- `Lv5`: 特约创作者 - 每日999单，3%抽成

---

### 3.6 获取交易记录

**接口**: `GET /creator/transactions`

**权限**: 创作者角色

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| limit | int | 否 | 返回数量，默认20 |

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "transactions": [
      {
        "id": 1,
        "user_id": 1,
        "type": 3,
        "amount": 40.00,
        "balance_before": 60.50,
        "balance_after": 100.50,
        "remark": "任务交付收入: 小红书种草文案",
        "created_at": "2024-01-01T10:00:00Z"
      }
    ]
  }
}
```

**交易类型**:
- `1`: 充值
- `2`: 提现
- `3`: 任务收入
- `4`: 冻结
- `5`: 解冻

---

### 3.7 获取创作者统计

**接口**: `GET /creator/stats`

**权限**: 创作者角色

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total_claims": 50,
    "completed_claims": 45,
    "pending_claims": 3,
    "total_earnings": 2250.00,
    "today_claims": 2
  }
}
```

---

## 4. 商家模块

### 4.1 发布任务

**接口**: `POST /business/tasks`

**权限**: 商家角色（需企业认证）

**请求参数**:
```json
{
  "title": "小红书种草文案",
  "description": "需要撰写产品种草文案，要求...",
  "category": 1,
  "unit_price": 50.00,
  "total_count": 10,
  "deadline": "2024-12-31T23:59:59Z"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| title | string | 是 | 任务标题 |
| description | string | 是 | 任务描述 |
| category | int | 是 | 分类ID |
| unit_price | float | 是 | 单价 |
| total_count | int | 是 | 总数量 |
| deadline | string | 否 | 截止时间（RFC3339格式） |

**响应示例**:
```json
{
  "code": 0,
  "message": "任务发布成功，等待审核",
  "data": {
    "task_id": 1
  }
}
```

**说明**:
- 需要100%预付总金额（unit_price × total_count）
- 金额会被冻结，验收通过后支付给创作者
- 任务需要管理员审核后才能上线

---

### 4.2 获取我的任务列表

**接口**: `GET /business/tasks`

**权限**: 商家角色

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "business_id": 2,
      "title": "小红书种草文案",
      "category": 1,
      "unit_price": 50.00,
      "total_count": 10,
      "remaining_count": 8,
      "status": 1,
      "total_budget": 500.00,
      "frozen_amount": 400.00,
      "paid_amount": 100.00,
      "created_at": "2024-01-01T10:00:00Z"
    }
  ]
}
```

**任务状态**:
- `0`: 待审核
- `1`: 已上线
- `2`: 进行中
- `3`: 已完成
- `4`: 已取消

---

### 4.3 获取任务的认领列表

**接口**: `GET /business/task/:id/claims`

**权限**: 商家角色（任务所有者）

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "task_id": 1,
      "creator_id": 3,
      "creator_name": "creator1",
      "status": 1,
      "content": "https://example.com/work.pdf",
      "submitted_at": "2024-01-01T15:00:00Z",
      "created_at": "2024-01-01T10:00:00Z"
    }
  ]
}
```

---

### 4.4 获取所有认领列表

**接口**: `GET /business/claims`

**权限**: 商家角色

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| status | int | 否 | 认领状态筛选 |

**响应示例**: 同上

---

### 4.5 获取认领详情

**接口**: `GET /business/claim/:id`

**权限**: 商家角色（任务所有者）

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "task_id": 1,
    "task_title": "小红书种草文案",
    "creator_id": 3,
    "creator_name": "creator1",
    "status": 1,
    "content": "https://example.com/work.pdf",
    "submitted_at": "2024-01-01T15:00:00Z",
    "created_at": "2024-01-01T10:00:00Z"
  }
}
```

---

### 4.6 验收认领

**接口**: `PUT /business/claim/:id/review`

**权限**: 商家角色（任务所有者）

**请求参数**:
```json
{
  "result": 1,
  "comment": "质量很好，通过验收"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| result | int | 是 | 验收结果：1-通过，0-退回 |
| comment | string | 否 | 验收评语 |

**响应示例**:
```json
{
  "code": 0,
  "message": "验收成功",
  "data": null
}
```

**说明**:
- 通过验收：支付创作者报酬，扣除平台抽成，解冻保证金
- 退回：创作者可重新提交，不影响保证金

---

### 4.7 取消任务

**接口**: `DELETE /business/task/:id`

**权限**: 商家角色（任务所有者）

**响应示例**:
```json
{
  "code": 0,
  "message": "任务已取消",
  "data": {
    "refunded": 400.00
  }
}
```

**说明**:
- 只能取消"已上线"或"进行中"的任务
- 取消所有待交付的认领，退还创作者保证金
- 退还商家未支付的冻结金额

---

### 4.8 获取账户余额

**接口**: `GET /business/balance`

**权限**: 商家角色

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "balance": 1000.00,
    "frozen_amount": 500.00
  }
}
```

---

### 4.9 充值

**接口**: `POST /business/recharge`

**权限**: 商家角色

**请求参数**:
```json
{
  "amount": 1000.00,
  "payment_method": "alipay"
}
```

---

### 4.10 获取交易记录

**接口**: `GET /business/transactions`

**权限**: 商家角色

**响应示例**: 同创作者交易记录

---

### 4.11 获取商家统计

**接口**: `GET /business/stats`

**权限**: 商家角色

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total_tasks": 20,
    "online_tasks": 5,
    "completed_tasks": 15,
    "total_expense": 10000.00,
    "frozen_amount": 2500.00
  }
}
```

---

## 5. 通知系统（新版）

### 5.1 获取通知列表

**接口**: `GET /notifications`

**权限**: 需要登录

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认1 |
| limit | int | 否 | 每页数量，默认20 |

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 10,
    "page": 1,
    "limit": 20,
    "data": [
      {
        "id": 1,
        "user_id": 1,
        "type": "task_status",
        "title": "任务已上线",
        "content": "您的任务《小红书种草文案》已审核通过",
        "is_read": false,
        "created_at": "2024-01-01T10:00:00Z"
      }
    ]
  }
}
```

**通知类型**:
- `task_status`: 任务状态变更
- `new_submission`: 新投稿
- `claim_approved`: 认领通过
- `income_received`: 收益到账

---

### 5.2 标记通知已读

**接口**: `PUT /notifications/:id/read`

**权限**: 需要登录

---

### 5.3 获取未读通知数

**接口**: `GET /notifications/unread-count`

**权限**: 需要登录

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "count": 5
  }
}
```

---

### 5.4 标记全部通知已读

**接口**: `PUT /notifications/read-all`

**权限**: 需要登录

---

## 6. 申诉模块

### 7.1 创建申诉

**接口**: `POST /appeals`

**权限**: 需要登录

**请求参数**:
```json
{
  "claim_id": 1,
  "reason": "商家验收不公平",
  "description": "详细说明..."
}
```

---

### 7.2 获取申诉列表

**接口**: `GET /appeals`

**权限**: 需要登录

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "claim_id": 1,
      "user_id": 1,
      "reason": "商家验收不公平",
      "status": 0,
      "created_at": "2024-01-01T10:00:00Z"
    }
  ]
}
```

**申诉状态**:
- `0`: 待处理
- `1`: 已处理
- `2`: 已驳回

---

### 7.3 获取申诉详情

**接口**: `GET /appeals/:id`

**权限**: 需要登录

---

### 7.4 商家查看申诉列表

**接口**: `GET /business/appeals`

**权限**: 商家角色

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "claim_id": 1,
      "creator_name": "creator1",
      "task_title": "小红书种草文案",
      "reason": "验收不公平",
      "evidence": "https://example.com/evidence.png",
      "status": 0,
      "result": "",
      "created_at": "2024-01-01T10:00:00Z"
    }
  ]
}
```

**申诉状态**:
- `0`: 待处理
- `1`: 处理中
- `2`: 已完成

---

### 7.5 商家处理申诉

**接口**: `PUT /business/appeals/:id/handle`

**权限**: 商家角色

**请求参数**:
```json
{
  "result": "已重新验收并通过"
}
```

---

## 7. 管理员模块

### 8.1 获取仪表盘数据

**接口**: `GET /admin/dashboard`

**权限**: 管理员角色

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total_users": 1000,
    "total_tasks": 500,
    "total_claims": 2000,
    "platform_revenue": 50000.00
  }
}
```

---

### 8.2 获取用户列表

**接口**: `GET /admin/users`

**权限**: 管理员角色

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码 |
| limit | int | 否 | 每页数量 |
| keyword | string | 否 | 搜索关键词 |
| role | string | 否 | 角色筛选 |

---

### 8.3 更新用户状态

**接口**: `PUT /admin/users/:id/status`

**权限**: 管理员角色

**请求参数**:
```json
{
  "status": 0
}
```

**说明**:
- `0`: 禁用
- `1`: 正常

---

### 8.4 更新用户信用分

**接口**: `PUT /admin/users/:id/credit`

**权限**: 管理员角色

**请求参数**:
```json
{
  "behavior_score": 80,
  "reason": "违规操作扣分"
}
```

---

### 8.5 获取任务列表（管理）

**接口**: `GET /admin/tasks`

**权限**: 管理员角色

---

### 8.6 审核任务

**接口**: `PUT /admin/task/:id/review`

**权限**: 管理员角色

**请求参数**:
```json
{
  "result": 1,
  "comment": "审核通过"
}
```

**说明**:
- `result`: 1-通过，0-拒绝

---

### 8.7 获取认领列表（管理）

**接口**: `GET /admin/claims`

**权限**: 管理员角色

---

### 8.8 获取申诉列表（管理）

**接口**: `GET /admin/appeals`

**权限**: 管理员角色

---

### 8.9 处理申诉

**接口**: `PUT /admin/appeals/:id/handle`

**权限**: 管理员角色

**请求参数**:
```json
{
  "result": 1,
  "comment": "申诉成立，已处理"
}
```

---

### 8.10 获取平台统计

**接口**: `GET /admin/stats`

**权限**: 管理员角色

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total_users": 1000,
    "total_creators": 800,
    "total_businesses": 200,
    "total_tasks": 500,
    "online_tasks": 50,
    "total_claims": 2000,
    "completed_claims": 1500,
    "pending_claims": 100,
    "total_revenue": 50000.00,
    "platform_revenue": 5000.00,
    "total_withdrawal": 30000.00
  }
}
```

---

### 8.11 获取创作者收益图表

**接口**: `GET /creator/chart/income`

**权限**: 创作者角色

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| period | string | 否 | 时间段：7d(默认), 30d, 90d |

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "labels": ["周一", "周二", "周三", "周四", "周五", "周六", "周日"],
    "values": [100, 150, 80, 200, 120, 180, 90]
  }
}
```

---

### 8.12 获取商家支出图表

**接口**: `GET /business/chart/expense`

**权限**: 商家角色

**响应示例**: 同上

---

## 8. 文件上传模块

### 9.1 上传文件

**接口**: `POST /upload`

**权限**: 需要登录

**请求格式**: `multipart/form-data`

**请求参数**:
- `file`: 文件（必填）

**响应示例**:
```json
{
  "code": 0,
  "message": "上传成功",
  "data": {
    "url": "https://example.com/uploads/file.pdf",
    "filename": "file.pdf",
    "size": 102400
  }
}
```

---

## 9. 公开接口

### 10.1 健康检查

**接口**: `GET /health`

**权限**: 公开

**响应示例**:
```json
{
  "status": "ok"
}
```

---

### 10.2 获取公开任务列表

**接口**: `GET /tasks`

**权限**: 公开

**说明**: 同创作者任务列表接口，但不需要认证

---

### 10.3 获取过审作品列表

**接口**: `GET /works`

**权限**: 公开

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认1 |
| limit | int | 否 | 每页数量，默认20，最大100 |
| sort | string | 否 | 排序：created_at(默认), likes, views |

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 50,
    "page": 1,
    "limit": 20,
    "data": [
      {
        "id": 1,
        "task_id": 1,
        "task_title": "产品宣传视频",
        "task_category": 3,
        "creator_id": 3,
        "creator_name": "creator1",
        "creator_avatar": "https://example.com/avatar.png",
        "content": "作品描述...",
        "reward": 42.50,
        "submit_at": "2024-01-01T10:00:00Z",
        "review_at": "2024-01-02T10:00:00Z",
        "materials": [
          {
            "id": 1,
            "claim_id": 1,
            "file_name": "video.mp4",
            "file_path": "/uploads/2024/01/video.mp4",
            "file_size": 10240000,
            "file_type": "video/mp4",
            "thumbnail_path": "/uploads/2024/01/thumb.jpg",
            "created_at": "2024-01-01T10:00:00Z"
          }
        ]
      }
    ]
  }
}
```

**说明**: 只返回已过审（status=3，即 approved）的认领作品

---

### 10.4 获取作品详情

**接口**: `GET /works/:id`

**权限**: 公开

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "task_id": 1,
    "task_title": "产品宣传视频",
    "task_category": 3,
    "creator_id": 3,
    "creator_name": "creator1",
    "creator_avatar": "https://example.com/avatar.png",
    "content": "作品描述...",
    "reward": 42.50,
    "submit_at": "2024-01-01T10:00:00Z",
    "review_at": "2024-01-02T10:00:00Z",
    "materials": [
      {
        "id": 1,
        "claim_id": 1,
        "file_name": "video.mp4",
        "file_path": "/uploads/2024/01/video.mp4",
        "file_size": 10240000,
        "file_type": "video/mp4",
        "thumbnail_path": "/uploads/2024/01/thumb.jpg",
        "created_at": "2024-01-01T10:00:00Z"
      }
    ]
  }
}
```

**错误码**:
- `40401`: 作品不存在或未通过审核

---

## 附录

### A. 数据模型

#### 用户等级升级规则

| 等级 | 名称 | 累计采纳数 | 每日限额 | 抽成比例 |
|------|------|-----------|---------|---------|
| Lv0 | 试用创作者 | 0 | 3单 | 10% |
| Lv1 | 新手创作者 | ≥1 | 8单 | 10% |
| Lv2 | 活跃创作者 | ≥5 | 15单 | 10% |
| Lv3 | 优质创作者 | ≥20 | 30单 | 10% |
| Lv4 | 金牌创作者 | ≥50 | 50单 | 5% |
| Lv5 | 特约创作者 | ≥100 | 999单 | 3% |

**升级条件**: 基于累计采纳数自动升级

---

### B. 错误码汇总

| Code | 说明 |
|------|------|
| 0 | 成功 |
| 40001 | 参数错误 |
| 40002 | 业务逻辑错误 |
| 40003 | 余额不足 |
| 40101 | 未登录 |
| 40301 | 权限不足 |
| 40302 | 需要企业认证 |
| 40303 | 超过每日限额 |
| 40401 | 资源不存在 |
| 50001 | 数据库错误 |
| 50002 | 创建失败 |
| 50003 | 更新失败 |

---

### C. 前后端分离开发说明

#### 认证流程

1. 前端调用 `/auth/login` 获取 JWT token
2. 将 token 存储在 `localStorage` 中
3. 后续请求在 Header 中携带: `Authorization: Bearer {token}`
4. Token 过期时返回 401，前端清除 token 并跳转登录页

#### 跨域配置

服务器已配置 CORS，允许所有来源访问：
```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS
Access-Control-Allow-Headers: Origin, Content-Type, Accept, Authorization
```

#### 开发建议

1. **环境变量**: 使用环境变量配置 API Base URL
2. **请求封装**: 统一封装 API 请求函数，处理认证和错误
3. **错误处理**: 统一处理 401/403/500 等错误
4. **加载状态**: 显示 loading 状态提升用户体验
5. **数据缓存**: 合理使用缓存减少请求次数

#### 示例代码（JavaScript）

```javascript
const API_BASE = 'http://localhost:8080/api/v1';

async function apiRequest(endpoint, method = 'GET', body = null) {
  const token = localStorage.getItem('token');
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const response = await fetch(`${API_BASE}${endpoint}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : null
  });

  if (response.status === 401) {
    localStorage.removeItem('token');
    window.location.href = '/auth/login.html';
    throw new Error('Unauthorized');
  }

  return response.json();
}

// 使用示例
const tasks = await apiRequest('/creator/tasks?page=1&limit=20', 'GET');
const claim = await apiRequest('/creator/claim', 'POST', { task_id: 1 });
```

---

## 更新日志

- **v1.0.0** (2024-01-01): 初始版本
  - 完成用户认证、任务管理、认领流程
  - 实现创作者和商家端核心功能
  - 添加消息通知和申诉系统
