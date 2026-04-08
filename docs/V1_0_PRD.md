# 创意喵 V1.0 PRD 实施文档

**版本**: V1.0
**更新日期**: 2026-04-07
**状态**: 已完成核心重构

---

## 一、版本范围

### 1.1 本期包含

- 创作者四级体系（青铜/白银/黄金/钻石）+ 积分系统
- 实时认领撮合（唯一模式）
- 商家极简后台（统一15%抽成，100%预付）
- 资金托管与结算（保证金 + 预付）

### 1.2 本期不做（后续版本）

- 定向邀约（V2.0）
- 商家分级体系（V2.0）
- 拼稿竞选（V3.0）
- 月结账期（暂不）
- AI剪辑工具（暂不）

---

## 二、创作者四级体系

### 等级定义

| 等级 | 保证金 | 日接单上限 | 平台抽成 | 升级条件 | 核心特权 |
|------|--------|----------|---------|---------|---------|
| 青铜 | 10元/条 | 3条 | 20% | 新注册+完成新手培训 | 需导师审核后才能认领任务 |
| 白银 | 10元/条 | 10条 | 15% | 完成10单+通过率≥70% | 直接认领，无需审核 |
| 黄金 | 0元 | 20条 | 12% | 总积分≥800+完成50单 | 提前5分钟预览任务 |
| 钻石 | 0元 | 50条 | 10% | 总积分≥1500+完成200单 | 提前10分钟预览+专属高价任务 |

### User 模型字段

```go
type User struct {
    // 创作者专属
    Level           UserLevel  // 1-4 (青铜/白银/黄金/钻石)
    BehaviorScore   int        // 行为分 -1000~+2000
    TradeScore      float64    // 交易分 0~500
    TotalScore      int        // 总积分 = BehaviorScore + TradeScore
    MarginFrozen    float64    // 冻结保证金
    DailyClaimCount int        // 今日认领数
    DailyClaimReset time.Time  // 重置时间
}
```

### 积分规则

**总积分 = 行为分（主导） + 交易分（辅助）**

- 行为分: -1000 ~ +2000（无上限，决定等级）
- 交易分: 收入1元 = 0.1分（封顶500分）

#### 加分行为

| 行为 | 分值 | 说明 |
|------|------|------|
| 按时交付（24h内） | +30/单 | 核心加分 |
| 获得好评（4-5星） | +20/单 | 商家评价 |
| 连续7天接单 | +50 | 每月限1次 |

#### 扣分行为

| 行为 | 分值 | 后果 |
|------|------|------|
| 超时交付 | -50/单 | 扣保证金+降权 |
| 质量不合格 | -100/单 | - |
| 恶意弃单 | -200/单 | 没收保证金 |
| 发布违规内容 | -300/次 | 封禁7天 |
| 飞单 | -500/次 | 永久封禁 |

---

## 三、商家端规则

### 统一标准

| 项目 | 规则 | 说明 |
|------|------|------|
| 准入门槛 | 企业实名认证 | 个人不支持 |
| 资金模式 | 100%预付 | 余额不足无法发布 |
| 平台抽成 | 15% | 从赏金中扣除 |
| 发布限制 | ≤100条，单价≥10元 | 防刷单 |
| 审核时效 | 2小时内 | 仅审违规 |
| 验收时限 | 48小时内 | 超时自动通过 |

---

## 四、撮合流程

### 核心流程

```
商家发布任务
    ↓
平台审核（2h）
    ↓
任务上架（70%立即 + 30%延迟）
    ↓
创作者认领（锁定） + 冻结保证金
    ↓
24h生产
    ↓
AI初筛 + 人工抽检
    ↓
商家验收（48h）
    ↓
结算（85%创作者 + 15%平台） + 退保证金
```

### Task 模型

```go
type Task struct {
    ID             int64
    BusinessID     int64
    Title          string
    Description    string
    Category       int       // 1=文案, 2=设计, 3=视频, 4=摄影, 5=音乐, 6=开发, 7=其他

    UnitPrice      float64   // 单价
    TotalCount    int       // 总数量
    RemainingCount int      // 剩余数量

    Status         int       // 1=待审核, 2=已上架, 3=进行中, 4=已结束, 5=已取消
    ReviewAt       *time.Time
    PublishAt      *time.Time
    EndAt          *time.Time

    TotalBudget    float64   // = UnitPrice * TotalCount
    FrozenAmount   float64   // 已冻结
    PaidAmount     float64   // 已支付
}
```

### Claim 模型

```go
type Claim struct {
    ID            int64
    TaskID        int64
    CreatorID     int64
    Status        int       // 1=已认领, 2=已提交, 3=已验收, 4=已取消, 5=超时
    Content       string     // 交付内容
    SubmitAt      *time.Time
    ExpiresAt     time.Time // 认领+24h
    ReviewAt      *time.Time
    ReviewResult  int        // 1=通过, 2=退回
    ReviewComment string

    CreatorReward  float64   // 85%
    PlatformFee   float64   // 15%
    MarginReturned float64   // 保证金退还(10元)
}
```

---

## 五、资金流

### 结算计算

```
创作者收益 = 任务单价 × 85%
平台抽成 = 任务单价 × 15%
保证金退还 = 10元（认领时冻结，验收后退还）
```

### 资金流表

| 环节 | 付款方 | 接收方 | 金额 | 条件 |
|------|--------|--------|------|------|
| 充值 | 商家 | 平台 | 任意 | 主动充值 |
| 发布冻结 | 平台 | 冻结商家余额 | 赏金总额 | 上架 |
| 认领冻结 | 创作者 | 平台 | 10元 | 认领 |
| 结算 | 平台 | 创作者 | 85% | 验收通过 |
| 结算 | 平台 | 平台收入 | 15% | 验收通过 |
| 退保证金 | 平台 | 创作者 | 10元 | 完成 |
| 提现 | 创作者 | 银行/微信 | 余额 | T+1 |

---

## 六、风控机制

### 创作者熔断

| 条件 | 措施 | 恢复 |
|------|------|------|
| 单日退回≥3 | 停24h | 自动 |
| 连续超时 | 降级 | 完成5单 |
| 积分<0 | 限制接单 | 恢复积分 |
| 行为分<-300 | 冻结 | 申诉 |

### 商家熔断

| 条件 | 措施 |
|------|------|
| 退回率>50% | 下架 |
| 连续投诉 | 暂停48h |
| 违规 | 永封 |

---

## 七、API 端点

### 认证 `/api/v1/auth`

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /register | 注册 |
| POST | /login | 登录 |

### 创作者端 `/api/v1/creator`

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /tasks | 可认领任务列表 |
| POST | /claim | 认领任务 |
| GET | /claims | 我的认领列表 |
| PUT | /claim/:id/submit | 提交交付 |
| GET | /wallet | 我的钱包 |

### 商家端 `/api/v1/business`

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /task | 发布任务 |
| GET | /tasks | 我的任务列表 |
| GET | /task/:id/claims | 任务的认领列表 |
| PUT | /claim/:id/review | 验收认领 |
| GET | /balance | 账户余额 |

### 管理端 `/api/v1/admin`

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /dashboard | 数据看板 |
| GET | /users | 用户列表 |
| PUT | /users/:id/status | 修改用户状态 |
| PUT | /users/:id/credit | 修改信用分 |
| GET | /tasks | 所有任务 |
| PUT | /task/:id/review | 审核任务上架 |
| GET | /claims | 所有认领 |

---

## 八、页面清单

### 商家端

- `/business/dashboard.html` - 首页
- `/business/task_create.html` - 发布任务
- `/business/task_list.html` - 任务列表
- `/business/claim_review.html` - 认领验收
- `/business/balance.html` - 账户余额
- `/business/transactions.html` - 资金流水

### 创作者端

- `/creator/dashboard.html` - 首页
- `/creator/task_hall.html` - 任务大厅
- `/creator/claim_list.html` - 我的认领
- `/creator/delivery.html` - 交付管理
- `/creator/wallet.html` - 我的钱包

### 管理端

- `/admin/dashboard.html` - 数据看板
- `/admin/task_review.html` - 任务审核
- `/admin/user_list.html` - 用户管理
- `/admin/claim_list.html` - 认领管理

---

## 九、数据库表

```sql
-- 用户表
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('business', 'creator', 'admin')),
    phone TEXT,
    level INTEGER DEFAULT 1,           -- 创作者等级 1-4
    behavior_score INTEGER DEFAULT 0,  -- 行为分
    trade_score REAL DEFAULT 0,         -- 交易分
    margin_frozen REAL DEFAULT 0,      -- 冻结保证金
    daily_claim_count INTEGER DEFAULT 0,
    daily_claim_reset DATETIME,
    business_verified INTEGER DEFAULT 0,
    publish_count INTEGER DEFAULT 0,
    balance REAL DEFAULT 0,
    status INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 任务表
CREATE TABLE tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    business_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    category INTEGER NOT NULL,
    unit_price REAL NOT NULL,
    total_count INTEGER NOT NULL,
    remaining_count INTEGER NOT NULL,
    status INTEGER DEFAULT 1,  -- 1=待审核, 2=已上架, 3=进行中, 4=已结束, 5=已取消
    review_at DATETIME,
    publish_at DATETIME,
    end_at DATETIME,
    total_budget REAL NOT NULL,
    frozen_amount REAL DEFAULT 0,
    paid_amount REAL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (business_id) REFERENCES users(id)
);

-- 认领表
CREATE TABLE claims (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    creator_id INTEGER NOT NULL,
    status INTEGER DEFAULT 1,  -- 1=已认领, 2=已提交, 3=已验收, 4=已取消, 5=超时
    content TEXT,
    submit_at DATETIME,
    expires_at DATETIME,
    review_at DATETIME,
    review_result INTEGER,
    review_comment TEXT,
    creator_reward REAL DEFAULT 0,
    platform_fee REAL DEFAULT 0,
    margin_returned REAL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id),
    FOREIGN KEY (creator_id) REFERENCES users(id)
);

-- 交易表
CREATE TABLE transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    type INTEGER NOT NULL,
    amount REAL NOT NULL,
    balance_before REAL NOT NULL,
    balance_after REAL NOT NULL,
    remark TEXT,
    related_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- 积分日志表
CREATE TABLE credit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    type INTEGER NOT NULL,
    change INTEGER NOT NULL,
    reason TEXT,
    related_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
```

---

## 十、待完成事项

### P0 必须
- [x] 注册
- [x] 等级体系
- [x] 保证金系统
- [x] 发布与认领
- [x] 交付与结算
- [ ] 超时定时任务（24h超时检测、48h自动验收）
- [ ] 等级自动升级/降级逻辑

### P1 重要
- [ ] 预览功能（黄金/钻石提前）
- [ ] 商家积分系统
- [ ] 熔断机制实现

### P2 优化
- [ ] 批量上传/验收
- [ ] 申诉流程
- [ ] 数据看板完善
