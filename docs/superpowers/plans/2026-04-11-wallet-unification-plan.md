# Wallet API 统一 + 测试清理实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 统一钱包 API 数据结构，清理测试代码中的角色遗留字段，两项目同步修改无 bug。

**Architecture:** 删除 `/business/balance` 端点，统一使用 `/wallet` 返回完整钱包数据。清理 `monkey_test.go` 中的 `Role` 字段。更新小程序 `api.js` 调用。

**Tech Stack:** Go/Gin (miao),微信小程序 (miao-mini)

---

## 文件修改映射

| 文件 | 修改内容 |
|------|---------|
| `miao/internal/router/router.go` | 删除 `businessGroup.GET("/balance")`, 新增 `protected.GET("/wallet")` |
| `miao/internal/handler/business.go` | 删除 `GetBalance` 函数 |
| `miao/test/monkey_test.go` | 删除 `Role` 字段及所有引用 |
| `miao-mini/utils/api.js` | `getBalance()` 改为调用 `/wallet` |

---

## Task 1: 删除 business/balance 端点，统一到 /wallet

**Files:**
- Modify: `miao/internal/router/router.go:285`
- Modify: `miao/internal/handler/business.go:572-602`

- [ ] **Step 1: 确认 GetWallet 返回完整数据**

查看 `miao/internal/handler/creator.go:381-418`，确认 `GetWallet` 返回：
```go
wallet := model.UserWallet{
    Balance:       user.Balance,
    MarginFrozen:  user.MarginFrozen,
    TotalScore:    user.CalcTotalScore(),
    BehaviorScore: user.BehaviorScore,
    TradeScore:    user.TradeScore,
    Level:         int(user.Level),
    LevelName:     user.GetLevelName(),
}
```
`UserWallet` 还缺少 `frozen_amount` 字段，需要补充。

- [ ] **Step 2: 更新 UserWallet 模型添加 frozen_amount**

修改 `miao/internal/model/user.go:123-132`，在 `UserWallet` 结构体中添加：
```go
// UserWallet 创作者钱包信息
type UserWallet struct {
    Balance       float64 `json:"balance"`        // 账户余额
    FrozenAmount  float64 `json:"frozen_amount"` // 冻结金额
    MarginFrozen  float64 `json:"margin_frozen"` // 冻结保证金
    TotalScore    int     `json:"total_score"`    // 总积分
    BehaviorScore int     `json:"behavior_score"` // 行为分
    TradeScore    float64 `json:"trade_score"`    // 交易分
    Level         int     `json:"level"`          // 等级
    LevelName     string  `json:"level_name"`     // 等级名称
}
```

- [ ] **Step 3: 更新 GetWallet 返回完整数据**

修改 `miao/internal/handler/creator.go:404-412`，更新返回：
```go
wallet := model.UserWallet{
    Balance:       user.Balance,
    FrozenAmount:  user.FrozenAmount,  // 新增
    MarginFrozen:  user.MarginFrozen,
    TotalScore:    user.CalcTotalScore(),
    BehaviorScore: user.BehaviorScore,
    TradeScore:    user.TradeScore,
    Level:         int(user.Level),
    LevelName:     user.GetLevelName(),
}
```

- [ ] **Step 4: 删除 business/balance 路由**

修改 `miao/internal/router/router.go`，删除：
```go
businessGroup.GET("/balance", handler.GetBalance)
```
新增到 protected 组：
```go
protected.GET("/wallet", handler.GetWallet)
```

- [ ] **Step 5: 删除 GetBalance 函数**

删除 `miao/internal/handler/business.go:572-602` 中的 `GetBalance` 函数。

- [ ] **Step 6: 验证修改**

编译检查：
```bash
cd /Users/ke/code/miao-repo/miao && go build ./...
```

---

## Task 2: 清理 monkey_test.go 中的 Role 字段

**Files:**
- Modify: `miao/test/monkey_test.go`

- [ ] **Step 1: 查看 TestUser 结构体**

确认 `Role string` 字段在第几行（大约 29 行附近）。

- [ ] **Step 2: 删除 Role 字段**

删除 `miao/test/monkey_test.go` 第 29 行：
```go
Role     string
```

- [ ] **Step 3: 删除 Role 赋值**

删除第 113 行和 119 行：
```go
Role:     "creator",
Role:     "business",
```

- [ ] **Step 4: 删除 Role 相关判断，改为固定路径**

第 391-393 行：删除 `if user.Role == "business"` 判断，改为固定使用 `/business/transactions`：
```go
// 修改前：
endpoint := "/creator/transactions"
if user.Role == "business" {
    endpoint = "/business/transactions"
}

// 修改后：
endpoint := "/business/transactions"
```

第 416-418 行：删除 `if user.Role == "business"` 判断，改为固定使用 `/business/stats`：
```go
// 修改前：
endpoint := "/creator/stats"
if user.Role == "business" {
    endpoint = "/business/stats"
}

// 修改后：
endpoint := "/business/stats"
```

- [ ] **Step 5: 验证编译**

```bash
cd /Users/ke/code/miao-repo/miao && go build ./...
```

---

## Task 3: 更新小程序 api.js

**Files:**
- Modify: `miao-mini/utils/api.js`

- [ ] **Step 1: 修改 getBalance 函数**

修改 `miao-mini/utils/api.js:184-186`，将：
```js
getBalance() {
    return this.request('GET', '/business/balance');
},
```
改为：
```js
getBalance() {
    return this.request('GET', '/wallet');
},
```

- [ ] **Step 2: 检查所有调用处**

搜索 `miao-mini` 中所有调用 `getBalance` 的地方，确认它们现在会收到完整钱包数据。

- [ ] **Step 3: 验证**

确认 miao-mini 没有其他地方直接依赖 `/business/balance` 端点。

---

## Task 4: 回归测试

- [ ] **Step 1: 启动后端服务**

```bash
cd /Users/ke/code/miao-repo/miao && go run ./cmd/server/main.go &
```

- [ ] **Step 2: 测试 /wallet 端点**

```bash
curl -H "Authorization: Bearer <token>" http://localhost:8080/api/v1/wallet
```
确认返回包含 `balance`, `frozen_amount`, `margin_frozen`, `total_score`, `level` 等完整字段。

- [ ] **Step 3: 测试 /business/balance 端点已删除**

```bash
curl -H "Authorization: Bearer <token>" http://localhost:8080/api/v1/business/balance
```
确认返回 404。

---

## 验证清单

- [ ] `/wallet` 返回完整钱包数据（包含 frozen_amount）
- [ ] `/business/balance` 返回 404
- [ ] `monkey_test.go` 中无 `Role` 字段
- [ ] 小程序 `getBalance()` 调用 `/wallet`
- [ ] 后端编译通过
