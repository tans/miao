# 创意喵平台 V1 API 文档

**版本**: V1.0  
**Base URL**: `http://localhost:8888/api/v1`  
**认证方式**: JWT Bearer Token

## 响应格式

所有 API 响应使用统一格式：

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
| 40002 | 业务逻辑错误 |
| 40003 | 数据冲突 |
| 40101 | 未登录 |
| 40102 | Token 无效或过期 |
| 40301 | 无权限 |
| 40302 | 无权访问资源 |
| 50001 | 服务器错误 |

## 认证相关

### 注册

**POST** `/auth/register`

注册新用户（商家或创作者）。

**请求体**:
```json
{
  "username": "test_user",
  "password": "test123456",
  "phone": "13800138000",
  "role": 2
}
```

**参数说明**:
- `username`: 用户名，3-50 字符
- `password`: 密码，6-50 字符
- `phone`: 手机号
- `role`: 角色，2=商家，3=创作者

**响应**:
```json
{
  "code": 0,
  "message": "注册成功",
  "data": {
    "id": 1,
    "username": "test_user",
    "role": "business,creator",
    "level": 2,
    "level_name": "白银",
    "total_score": 100
  }
}
```

### 登录

**POST** `/auth/login`

用户登录获取 JWT token。

**请求体**:
```json
{
  "username": "test_user",
  "password": "test123456"
}
```

**响应**:
```json
{
  "code": 0,
  "message": "登录成功",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": 1,
      "username": "test_user",
      "role": "business,creator"
    }
  }
}
```

### 获取当前用户信息

**GET** `/users/me`

**Headers**: `Authorization: Bearer <token>`

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "username": "test_user",
    "phone": "13800138000",
    "role": "business,creator",
    "status": 1
  }
}
```

## 商家端 API

所有商家端 API 需要 `Authorization: Bearer <token>` 且用户角色包含 `business`。

### 充值

**POST** `/business/recharge`

商家账户充值（V1 为模拟充值）。

**请求体**:
```json
{
  "amount": 1000
}
```

**响应**:
```json
{
  "code": 0,
  "message": "充值成功",
  "data": {
    "balance": 1000
  }
}
```

### 创建任务

**POST** `/business/tasks`

发布新的视频任务。

**请求体**:
```json
{
  "title": "产品介绍视频",
  "description": "需要制作一个3分钟的产品介绍视频",
  "unit_price": 100.0,
  "total_count": 5
}
```

**参数说明**:
- `title`: 任务标题
- `description`: 任务描述
- `unit_price`: 单价（元）
- `total_count`: 需要的作品数量

**响应**:
```json
{
  "code": 0,
  "message": "任务发布成功，等待审核",
  "data": {
    "task_id": 1
  }
}
```

**注意**: 
- 需要账户余额 >= unit_price * total_count
- 任务创建后状态为"待审核"，需要管理员审核后才能上架

### 查看我的任务

**GET** `/business/tasks`

查看商家发布的所有任务。

**查询参数**:
- `status`: 任务状态（可选）
- `page`: 页码，默认 1
- `page_size`: 每页数量，默认 20

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "business_id": 1,
      "title": "产品介绍视频",
      "description": "需要制作一个3分钟的产品介绍视频",
      "category": 3,
      "unit_price": 100,
      "total_count": 5,
      "remaining_count": 5,
      "status": 1,
      "total_budget": 500,
      "frozen_amount": 500,
      "paid_amount": 0,
      "created_at": "2026-04-09T00:00:00+08:00",
      "updated_at": "2026-04-09T00:00:00+08:00"
    }
  ]
}
```

**任务状态**:
- 1: 待审核
- 2: 已上架
- 3: 进行中
- 4: 已结束
- 5: 已取消

### 查看任务的投稿

**GET** `/business/tasks/:id/claims`

查看指定任务的所有投稿。

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "task_id": 1,
      "creator_id": 2,
      "status": 2,
      "content": "https://example.com/video.mp4",
      "submit_at": "2026-04-09T00:00:00+08:00",
      "created_at": "2026-04-09T00:00:00+08:00"
    }
  ]
}
```

### 查看投稿详情

**GET** `/business/claim/:id`

查看单个投稿的详细信息。

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "task_id": 1,
    "creator_id": 2,
    "status": 2,
    "content": "https://example.com/video.mp4",
    "submit_at": "2026-04-09T00:00:00+08:00",
    "expires_at": "2026-04-10T00:00:00+08:00",
    "creator_reward": 0,
    "platform_fee": 0
  }
}
```

### 审核投稿

**PUT** `/business/claim/:id/review`

审核创作者提交的投稿。

**请求体**:
```json
{
  "result": 1,
  "comment": "作品质量很好，通过审核"
}
```

**参数说明**:
- `result`: 审核结果，1=通过，2=退回
- `comment`: 审核意见

**响应**:
```json
{
  "code": 0,
  "message": "验收成功",
  "data": null
}
```

### 查看余额

**GET** `/business/balance`

查看商家账户余额。

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "balance": 1000,
    "frozen_amount": 500
  }
}
```

## 创作者端 API

所有创作者端 API 需要 `Authorization: Bearer <token>` 且用户角色包含 `creator`。

### 浏览任务大厅

**GET** `/tasks`

查看所有可认领的任务（已上架且有剩余数量）。

**查询参数**:
- `category`: 任务分类（可选）
- `keyword`: 关键词搜索（可选）
- `page`: 页码，默认 1
- `page_size`: 每页数量，默认 20

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "data": [
      {
        "id": 1,
        "business_id": 1,
        "title": "产品介绍视频",
        "description": "需要制作一个3分钟的产品介绍视频",
        "category": 3,
        "unit_price": 100,
        "total_count": 5,
        "remaining_count": 5,
        "status": 2,
        "created_at": "2026-04-09T00:00:00+08:00"
      }
    ],
    "page": 1,
    "limit": 20,
    "total": 1
  }
}
```

### 认领任务

**POST** `/creator/claim`

认领一个任务。

**请求体**:
```json
{
  "task_id": 1,
  "quantity": 1
}
```

**参数说明**:
- `task_id`: 任务 ID
- `quantity`: 认领数量

**响应**:
```json
{
  "code": 0,
  "message": "认领成功",
  "data": {
    "claim_id": 1,
    "expires_at": "2026-04-10T00:00:00+08:00"
  }
}
```

**注意**:
- 白银及以上等级可直接认领
- 青铜等级需要支付保证金
- 每日认领数量有限制

### 查看我的认领

**GET** `/creator/claims`

查看创作者的所有认领记录。

**查询参数**:
- `status`: 认领状态（可选）
- `page`: 页码，默认 1
- `page_size`: 每页数量，默认 20

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "task_id": 1,
      "creator_id": 2,
      "status": 1,
      "content": "",
      "expires_at": "2026-04-10T00:00:00+08:00",
      "created_at": "2026-04-09T00:00:00+08:00"
    }
  ]
}
```

**认领状态**:
- 1: 已认领（待提交）
- 2: 已提交（待审核）
- 3: 审核通过
- 4: 审核退回
- 5: 已过期

### 提交投稿

**PUT** `/creator/claim/:id/submit`

提交作品内容。

**请求体**:
```json
{
  "content": "https://example.com/my-video.mp4",
  "description": "已完成视频制作，请审核"
}
```

**参数说明**:
- `content`: 作品链接（V1 使用 URL）
- `description`: 作品说明（可选）

**响应**:
```json
{
  "code": 0,
  "message": "提交成功",
  "data": null
}
```

### 查看钱包

**GET** `/creator/wallet`

查看创作者钱包信息。

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "balance": 500,
    "margin_frozen": 0,
    "total_score": 150,
    "behavior_score": 100,
    "trade_score": 50,
    "level": 2,
    "level_name": "白银"
  }
}
```

## 用户端 API

### 更新个人资料

**PUT** `/users/me`

更新用户个人信息。

**请求体**:
```json
{
  "nickname": "我的昵称",
  "avatar": "https://example.com/avatar.jpg"
}
```

**响应**:
```json
{
  "code": 0,
  "message": "更新成功",
  "data": null
}
```

## 错误响应示例

### 参数错误

```json
{
  "code": 40001,
  "message": "参数错误: Key: 'TaskCreate.Title' Error:Field validation for 'Title' failed on the 'required' tag",
  "data": null
}
```

### 未登录

```json
{
  "code": 40101,
  "message": "未登录",
  "data": null
}
```

### Token 无效

```json
{
  "code": 40102,
  "message": "Invalid or expired token",
  "data": null
}
```

### 余额不足

```json
{
  "code": 40002,
  "message": "余额不足，需要预付总金额",
  "data": {
    "available": 0,
    "required": 500
  }
}
```

### 无权限

```json
{
  "code": 40301,
  "message": "无权查看此任务的认领",
  "data": null
}
```

## 数据模型

### User（用户）

```go
{
  "id": 1,
  "username": "test_user",
  "role": "business,creator",  // 角色：business, creator, admin
  "phone": "13800138000",
  "nickname": "昵称",
  "avatar": "头像URL",
  "balance": 1000.0,           // 账户余额
  "frozen_amount": 500.0,      // 冻结金额
  "level": 2,                  // 创作者等级 1-4
  "total_score": 150,          // 总积分
  "status": 1,                 // 状态：1=正常，0=禁用
  "created_at": "2026-04-09T00:00:00+08:00"
}
```

### Task（任务）

```go
{
  "id": 1,
  "business_id": 1,
  "title": "任务标题",
  "description": "任务描述",
  "category": 3,               // 分类：3=视频
  "unit_price": 100.0,         // 单价
  "total_count": 5,            // 总数量
  "remaining_count": 5,        // 剩余数量
  "status": 2,                 // 状态：1=待审核，2=已上架，3=进行中，4=已结束，5=已取消
  "total_budget": 500.0,       // 总预算
  "frozen_amount": 500.0,      // 已冻结
  "paid_amount": 0.0,          // 已支付
  "created_at": "2026-04-09T00:00:00+08:00",
  "updated_at": "2026-04-09T00:00:00+08:00"
}
```

### Claim（认领/投稿）

```go
{
  "id": 1,
  "task_id": 1,
  "creator_id": 2,
  "status": 2,                 // 状态：1=已认领，2=已提交，3=审核通过，4=审核退回，5=已过期
  "content": "作品链接",
  "submit_at": "2026-04-09T00:00:00+08:00",
  "expires_at": "2026-04-10T00:00:00+08:00",
  "review_comment": "审核意见",
  "creator_reward": 80.0,      // 创作者收益
  "platform_fee": 20.0,        // 平台抽成
  "created_at": "2026-04-09T00:00:00+08:00",
  "updated_at": "2026-04-09T00:00:00+08:00"
}
```

## 使用示例

完整的使用示例请参考 [README.md](README.md) 中的"完整流程示例"部分。
