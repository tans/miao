# 创意喵平台 V1 API 文档

**版本**: V1.0
**Base URL**: `http://localhost:8888/api/v1`
**认证方式**: JWT Bearer Token

## 概述

创意喵平台仅支持视频类悬赏任务，聚焦商家核心需求，操作便捷、无冗余选项。

**核心流程**:
1. 商家发布任务（托管赏金）
2. 创作者浏览并认领任务
3. 创作者提交作品
4. 商家审核并选中获胜者
5. 平台自动发放奖励

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

## 枚举值说明

### 任务状态

| 值 | 说明 |
|----|------|
| 1 | 待审核 |
| 2 | 已上架 |
| 3 | 进行中 |
| 4 | 已结束 |
| 5 | 已取消 |

### 认领状态

| 值 | 说明 |
|----|------|
| 1 | 已认领（待提交） |
| 2 | 已提交（待审核） |
| 3 | 审核通过 |
| 4 | 审核退回 |
| 5 | 已过期 |

### 审核结果

| 值 | 说明 |
|----|------|
| 1 | 通过 |
| 2 | 退回 |

### 行业选项（多选）

| 值 | 说明 |
|----|------|
| 本地餐饮 | 本地餐饮 |
| 美妆护肤 | 美妆护肤 |
| 家居家电 | 家居家电 |
| 教育培训 | 教育培训 |
| 本地生活服务 | 本地生活服务 |
| 服饰鞋帽 | 服饰鞋帽 |
| 母婴用品 | 母婴用品 |
| 数码3C | 数码3C |
| 运动健身 | 运动健身 |
| 宠物用品 | 宠物用品 |
| 汽车服务 | 汽车服务 |
| 文旅娱乐 | 文旅娱乐 |
| 企业宣传 | 企业宣传 |
| 电商带货 | 电商带货 |
| 便民服务 | 便民服务 |

### 视频时长

| 值 | 说明 |
|----|------|
| 15秒内 | 15秒内 |
| 30秒 | 30秒 |
| 60秒 | 60秒 |
| 1-3分钟 | 1-3分钟 |
| 不限制 | 不限制 |

### 视频尺寸

| 值 | 说明 |
|----|------|
| 9:16 | 抖音/小红书竖屏 |
| 16:9 | 横屏 |
| 1:1 | 正方形 |

### 分辨率

| 值 | 说明 |
|----|------|
| 720P | 720P（高清） |
| 1080P | 1080P（超清） |

### 创作风格

| 值 | 说明 |
|----|------|
| 口语化 | 口语化 |
| 商务正式 | 商务正式 |
| 种草安利 | 种草安利 |
| 搞笑轻松 | 搞笑轻松 |
| 温情故事 | 温情故事 |
| 科普专业 | 科普专业 |
| 其他 | 其他 |

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
    "level_name": "活跃创作者",
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
  "title": "抖音餐饮宣传视频",
  "description": "突出产品特色，风格口语化",
  "unit_price": 5.0,
  "total_count": 20,
  "deadline": "2026-04-15T23:59:59+08:00",
  "industries": ["本地餐饮", "美妆护肤"],
  "video_duration": "60秒",
  "video_aspect": "9:16",
  "video_resolution": "1080P",
  "creative_style": "口语化",
  "award_price": 8.0,
  "award_count": 3
}
```

**参数说明**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| title | string | 是 | 任务标题，≤128字符，需包含视频场景+核心需求 |
| description | string | 是 | 任务描述，仅需1条核心需求 |
| unit_price | float64 | 是 | 基础奖励，单价（元），≥2元 |
| total_count | int | 是 | 报名人数上限，≥10 |
| deadline | string | 是 | 报名截止时间，RFC3339格式 |
| industries | []string | 否 | 行业选项（多选），不选则不限 |
| video_duration | string | 否 | 视频时长：15秒内/30秒/60秒/1-3分钟/不限制 |
| video_aspect | string | 否 | 视频尺寸：9:16/16:9/1:1 |
| video_resolution | string | 否 | 分辨率：720P/1080P |
| creative_style | string | 否 | 创作风格：口语化/商务正式/种草安利/搞笑轻松/温情故事/科普专业/其他 |
| award_price | float64 | 否 | 入围奖励（元），≥8元，入围即中标 |
| award_count | int | 否 | 入围数量，≥1 |

**预算计算公式**:
```
基础奖励总额 = unit_price × total_count
入围奖励总额 = award_price × award_count
商家预支总金额 = 基础奖励总额 + 入围奖励总额
```

**响应**:
```json
{
  "code": 0,
  "message": "任务发布成功",
  "data": {
    "task_id": 1
  }
}
```

**注意**:
- 需要账户余额 >= 预支总金额
- 任务创建后状态为"待审核"，需管理员审核后上架

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
  "data": {
    "data": [
      {
        "id": 1,
        "business_id": 1,
        "title": "抖音餐饮宣传视频",
        "description": "突出产品特色，风格口语化",
        "category": 3,
        "unit_price": 5.0,
        "total_count": 20,
        "remaining_count": 18,
        "status": 2,
        "total_budget": 124.0,
        "frozen_amount": 100.0,
        "paid_amount": 10.0,
        "created_at": "2026-04-09T00:00:00+08:00",
        "updated_at": "2026-04-09T00:00:00+08:00",
        "end_at": "2026-04-15T23:59:59+08:00",
        "industries": "本地餐饮,美妆护肤",
        "video_duration": "60秒",
        "video_aspect": "9:16",
        "video_resolution": "1080P",
        "creative_style": "口语化",
        "award_price": 8.0,
        "award_count": 3
      }
    ],
    "page": 1,
    "limit": 20,
    "total": 1
  }
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
      "expires_at": "2026-04-10T00:00:00+08:00",
      "review_at": null,
      "review_result": null,
      "review_comment": "",
      "creator_reward": 0,
      "platform_fee": 0,
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
- `sort`: 排序（可选）：price_asc/price_desc/deadline_asc
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
        "title": "抖音餐饮宣传视频",
        "description": "突出产品特色，风格口语化",
        "category": 3,
        "unit_price": 5.0,
        "total_count": 20,
        "remaining_count": 18,
        "status": 2,
        "total_budget": 124.0,
        "end_at": "2026-04-15T23:59:59+08:00",
        "created_at": "2026-04-09T00:00:00+08:00",
        "industries": "本地餐饮,美妆护肤",
        "video_duration": "60秒",
        "video_aspect": "9:16",
        "video_resolution": "1080P",
        "creative_style": "口语化",
        "award_price": 8.0,
        "award_count": 3
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
- Lv0起即可认领任务
- 每日认领数量根据等级限制

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
  "level": 2,                  // 创作者等级 Lv0-Lv5
  "level_name": "活跃创作者",         // 等级名称
  "total_score": 150,          // 总积分
  "behavior_score": 100,       // 行为积分
  "trade_score": 50,           // 交易积分
  "status": 1,                 // 状态：1=正常，0=禁用
  "created_at": "2026-04-09T00:00:00+08:00"
}
```

**创作者等级说明**:

| 等级 | 名称 | 条件 |
|------|------|------|
| Lv0 | 试用创作者 | 0 采纳 |
| Lv1 | 新手创作者 | ≥1 采纳 |
| Lv2 | 活跃创作者 | ≥5 采纳 |
| Lv3 | 优质创作者 | ≥20 采纳 |
| Lv4 | 金牌创作者 | ≥50 采纳 |
| Lv5 | 特约创作者 | ≥100 采纳 |

### Transaction（交易记录）

```go
{
  "id": 1,
  "user_id": 1,
  "type": 1,                 // 类型：1=充值, 2=消费, 3=退款, 4=提现
  "amount": 100.0,           // 金额
  "balance_before": 900.0,   // 变动前余额
  "balance_after": 1000.0,   // 变动后余额
  "remark": "账户充值",        // 备注
  "related_id": 1,           // 关联ID（任务ID/认领ID等）
  "created_at": "2026-04-09T00:00:00+08:00"
}
```

### Task（任务）

```go
{
  "id": 1,
  "business_id": 1,
  "title": "抖音餐饮宣传视频",
  "description": "突出产品特色，风格口语化",
  "category": 3,               // 分类：3=视频（平台唯一支持）
  "unit_price": 5.0,          // 基础奖励
  "total_count": 20,           // 报名人数上限
  "remaining_count": 18,       // 剩余数量
  "status": 2,                // 状态：1=待审核，2=已上架，3=进行中，4=已结束，5=已取消
  "total_budget": 124.0,      // 总预算 = unit_price×total_count + award_price×award_count
  "frozen_amount": 100.0,     // 已冻结金额
  "paid_amount": 10.0,        // 已支付金额
  "end_at": "2026-04-15T23:59:59+08:00",
  "created_at": "2026-04-09T00:00:00+08:00",
  "updated_at": "2026-04-09T00:00:00+08:00",
  // v1.md 规范字段
  "industries": "本地餐饮,美妆护肤",    // 行业选项（逗号分隔）
  "video_duration": "60秒",            // 视频时长
  "video_aspect": "9:16",             // 视频尺寸
  "video_resolution": "1080P",        // 分辨率
  "creative_style": "口语化",          // 创作风格
  "award_price": 8.0,                // 入围奖励
  "award_count": 3                    // 入围数量
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
  "review_at": "2026-04-09T12:00:00+08:00",    // 审核时间
  "review_result": 1,          // 审核结果：1=通过，2=退回
  "review_comment": "审核意见",
  "creator_reward": 80.0,      // 创作者收益
  "platform_fee": 20.0,        // 平台抽成
  "margin_returned": 50.0,    // 保证金退还
  "created_at": "2026-04-09T00:00:00+08:00",
  "updated_at": "2026-04-09T00:00:00+08:00"
}
```

## 完整流程示例

### 1. 商家发布任务

```bash
# 登录获取 token
curl -X POST http://localhost:8888/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"business1","password":"test123456"}'

# 创建任务（包含 v1.md 所有字段）
curl -X POST http://localhost:8888/api/v1/business/tasks \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "title": "抖音餐饮宣传视频",
    "description": "突出产品特色，风格口语化",
    "unit_price": 5.0,
    "total_count": 20,
    "deadline": "2026-04-15T23:59:59+08:00",
    "industries": ["本地餐饮", "美妆护肤"],
    "video_duration": "60秒",
    "video_aspect": "9:16",
    "video_resolution": "1080P",
    "creative_style": "口语化",
    "award_price": 8.0,
    "award_count": 3
  }'
```

### 2. 创作者认领任务

```bash
# 创作者登录
curl -X POST http://localhost:8888/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"creator1","password":"test123456"}'

# 浏览任务大厅
curl http://localhost:8888/api/v1/tasks \
  -H "Authorization: Bearer <token>"

# 认领任务
curl -X POST http://localhost:8888/api/v1/creator/claim \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"task_id": 1, "quantity": 1}'
```

### 3. 创作者提交作品

```bash
# 提交投稿
curl -X PUT http://localhost:8888/api/v1/creator/claim/1/submit \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"content": "https://example.com/my-video.mp4"}'
```

### 4. 商家审核投稿

```bash
# 商家查看投稿
curl http://localhost:8888/api/v1/business/tasks/1/claims \
  -H "Authorization: Bearer <token>"

# 审核通过
curl -X PUT http://localhost:8888/api/v1/business/claim/1/review \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"result": 1, "comment": "作品质量很好，通过审核"}'
```

## 字段约束汇总

| 字段 | 类型 | 最小值 | 最大值 | 说明 |
|------|------|--------|--------|------|
| unit_price | float64 | 2 | 1000000 | 基础奖励（元） |
| total_count | int | 10 | 10000 | 报名人数上限 |
| award_price | float64 | 0 | 1000000 | 入围奖励（元），0表示无入围奖 |
| award_count | int | 0 | 1000 | 入围数量，0表示无入围奖 |
| title | string | 1 | 128 | 任务标题 |
| deadline | string | - | - | RFC3339 格式的截止时间 |
