# 钱包 API 统一 + 测试代码清理设计

## 背景

创意喵平台设计中，每个用户**同时具备**商家和创作者身份（而非二选一）。数据库层已完成改造（`users` 表同时包含 creator 和 business 字段），但存在以下问题：

1. **Wallet API 数据不一致** - `/creator/wallet` 和 `/business/balance` 返回不同数据结构
2. **测试代码遗留** - `monkey_test.go` 中仍有 `Role` 字段

## 目标

1. 统一钱包 API，两端返回一致数据
2. 清理测试代码中的角色遗留
3. 两项目（miao 后端 + miao-mini 小程序）同步修改

---

## 改动 1: 统一 Wallet API

### 问题

| 端点 | 返回字段 |
|------|---------|
| `GET /creator/wallet` | balance, margin_frozen, total_score, level, behavior_score, trade_score, level_name |
| `GET /business/balance` | balance, frozen_amount |

同一用户，两套数据结构，前端调用混乱。

### 解决方案

**删除** `/business/balance` 端点，**统一**使用 `/api/v1/wallet`：

```json
GET /api/v1/wallet
{
  "balance": 100.00,
  "frozen_amount": 50.00,
  "margin_frozen": 10.00,
  "total_score": 250,
  "behavior_score": 100,
  "trade_score": 150.0,
  "level": 2,
  "level_name": "白银"
}
```

### 修改文件

**miao/internal/router/router.go**
- 删除 `businessGroup.GET("/balance", handler.GetBalance)`
- 新增 `protected.GET("/wallet", handler.GetWallet)`

**miao/internal/handler/creator.go**
- `GetWallet` 保持不变（已返回完整钱包数据）

**miao/internal/handler/business.go**
- 删除 `GetBalance` 函数

---

## 改动 2: 清理测试代码

### 问题

`miao/test/monkey_test.go` 中存在 `Role` 字段，是旧的角色分离设计遗留：

```go
type TestUser struct {
    Role string  // 应删除
    ...
}
```

### 修改文件

**miao/test/monkey_test.go**
- 删除 `Role string` 字段
- 删除所有 `Role: "creator"` 和 `Role: "business"` 赋值
- 删除所有 `if user.Role == "business"` 判断逻辑

---

## 改动 3: 小程序适配

### 问题

`miao-mini/utils/api.js` 中：
- `getBalance()` 调用 `/business/balance`（将被删除）
- `getWallet()` 调用 `/creator/wallet`（保留）

### 修改文件

**miao-mini/utils/api.js**
- `getBalance()` 改为调用 `/wallet`

---

## API 路径说明

保留 `/business/*` 和 `/creator/*` 路径不变，原因：
- 已有小程序线上调用，改动路径风险大
- 后端已移除角色校验，逻辑上所有用户可访问两组 API
- 路径命名仅是语义问题，不影响功能

---

## 数据流验证

```
用户钱包数据 (同一份)
├── balance       ← 账户余额
├── frozen_amount ← 冻结金额（任务预付）
└── margin_frozen ← 保证金（青铜创作者）

创作者数据 (同一份)
├── level, level_name
├── behavior_score
├── trade_score
└── total_score
```

所有字段都存储在 `users` 表，无角色分离。

---

## 测试验证

修改完成后验证：
1. `GET /wallet` 返回完整钱包数据
2. `GET /business/balance` 返回 404
3. 小程序"我的"页面正常显示余额
