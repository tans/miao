# 创意喵平台 Monkey Test 规划

## 测试目标

通过自动化测试脚本模拟真实用户操作，验证平台的完整业务流程，发现潜在问题并推动版本迭代。

## 测试策略

### 1. 测试范围
- **页面可访问性测试**：验证所有页面能正常加载
- **API 功能测试**：验证核心业务接口的正确性
- **业务流程测试**：模拟完整的用户操作链路
- **边界条件测试**：测试异常输入和边界情况
- **并发测试**：模拟多用户同时操作

### 2. 测试环境
- 使用临时数据库（每次测试前清空）
- 使用随机生成的测试账号
- 支持通过参数指定测试服务器地址
- 测试完成后自动清理测试数据

### 3. 测试工具
- **curl**：HTTP 请求测试
- **jq**：JSON 数据解析
- **grep/sed**：HTML 页面元素验证
- **bash**：测试脚本编排

---

## 测试用例设计

### Phase 1: 基础功能测试（已实现）

#### 1.1 页面可访问性测试
- [ ] 首页加载
- [ ] 登录页面加载
- [ ] 注册页面加载
- [ ] 帮助中心页面加载

#### 1.2 用户注册登录
- [x] 创作者注册
- [x] 商家注册
- [x] 用户登录
- [ ] 登录失败（错误密码）
- [ ] 重复注册检测

---

### Phase 2: 创作者完整流程测试

#### 2.1 创作者工作台
**测试场景**：创作者登录后查看工作台数据

**API 调用**：
```bash
GET /api/v1/creator/stats
Authorization: Bearer {token}
```

**验证点**：
- 返回状态码 200
- 数据包含：total_earnings, available_balance, frozen_amount, total_tasks, completed_tasks
- 数值类型正确（数字）

#### 2.2 任务大厅
**测试场景**：浏览可认领的任务列表

**API 调用**：
```bash
GET /api/v1/creator/tasks?page=1&page_size=10&status=published
```

**验证点**：
- 返回任务列表
- 每个任务包含：id, title, description, reward, remaining_count
- 只显示 status=published 的任务

#### 2.3 认领任务
**测试场景**：创作者认领一个任务

**前置条件**：
- 任务状态为 published
- 任务 remaining_count > 0
- 创作者未达到每日认领上限

**API 调用**：
```bash
POST /api/v1/creator/tasks/{task_id}/claim
Authorization: Bearer {token}
```

**验证点**：
- 返回 claim_id
- 任务 remaining_count 减 1
- 创作者 daily_claim_count 加 1
- 数据库 task_claims 表新增记录

#### 2.4 提交作品
**测试场景**：创作者提交任务作品

**前置条件**：
- 已认领任务（status=claimed）

**API 调用**：
```bash
POST /api/v1/creator/claims/{claim_id}/submit
Content-Type: application/json

{
  "content": "作品内容或链接",
  "attachments": ["https://example.com/file1.jpg"]
}
```

**验证点**：
- claim status 变为 submitted
- submission 记录创建成功
- 商家收到待审核通知

#### 2.5 查看我的认领
**测试场景**：查看所有认领的任务

**API 调用**：
```bash
GET /api/v1/creator/claims?status=all&page=1
```

**验证点**：
- 返回认领列表
- 包含各种状态：claimed, submitted, approved, rejected

#### 2.6 查看钱包余额
**测试场景**：查看账户余额和冻结金额

**API 调用**：
```bash
GET /api/v1/user/balance
```

**验证点**：
- 返回 balance, frozen_amount
- 数值准确

#### 2.7 提现
**测试场景**：创作者申请提现

**前置条件**：
- balance >= 提现金额
- 提现金额 >= 最低提现额度

**API 调用**：
```bash
POST /api/v1/creator/withdraw
{
  "amount": 100.00,
  "method": "alipay",
  "account": "test@example.com"
}
```

**验证点**：
- 提现记录创建
- balance 减少
- 交易记录生成

---

### Phase 3: 商家完整流程测试

#### 3.1 商家工作台
**测试场景**：商家登录后查看工作台数据

**API 调用**：
```bash
GET /api/v1/business/stats
```

**验证点**：
- 返回统计数据：total_tasks, active_tasks, total_spent, pending_reviews

#### 3.2 账户充值
**测试场景**：商家充值账户余额

**API 调用**：
```bash
POST /api/v1/business/recharge
{
  "amount": 1000.00,
  "method": "alipay"
}
```

**验证点**：
- 充值订单创建
- 返回支付链接（模拟环境直接到账）
- balance 增加

#### 3.3 发布任务
**测试场景**：商家发布新任务

**前置条件**：
- 账户余额充足（>= 任务总预算）

**API 调用**：
```bash
POST /api/v1/business/tasks
{
  "title": "测试任务",
  "description": "任务描述",
  "requirements": "任务要求",
  "reward": 10.00,
  "total_count": 100,
  "category": "writing",
  "payment_type": "prepay"
}
```

**验证点**：
- 任务创建成功
- 预付模式：冻结金额 = reward * total_count
- 后付模式：不冻结金额
- 任务状态为 pending（待审核）

#### 3.4 查看任务列表
**测试场景**：查看自己发布的所有任务

**API 调用**：
```bash
GET /api/v1/business/tasks?page=1&status=all
```

**验证点**：
- 返回任务列表
- 包含各种状态的任务

#### 3.5 审核认领
**测试场景**：商家审核创作者的认领申请

**前置条件**：
- 任务设置了需要审核认领（require_claim_review=true）
- 有待审核的认领（claim status=pending）

**API 调用**：
```bash
POST /api/v1/business/claims/{claim_id}/review
{
  "action": "approve",
  "reason": ""
}
```

**验证点**：
- claim status 变为 claimed 或 rejected
- 创作者收到通知

#### 3.6 验收作品
**测试场景**：商家验收创作者提交的作品

**前置条件**：
- claim status=submitted

**API 调用**：
```bash
POST /api/v1/business/submissions/{submission_id}/review
{
  "action": "approve",
  "rating": 5,
  "comment": "很好"
}
```

**验证点**：
- submission status 变为 approved 或 rejected
- 通过：创作者获得奖励，冻结金额解冻并扣除
- 拒绝：创作者可重新提交或申诉

#### 3.7 查看待审核列表
**测试场景**：查看所有待审核的认领和提交

**API 调用**：
```bash
GET /api/v1/business/claims?status=pending
GET /api/v1/business/submissions?status=submitted
```

**验证点**：
- 返回待审核列表
- 数据完整

---

### Phase 4: 管理员功能测试

#### 4.1 管理员登录
**测试场景**：使用管理员账号登录

**前置条件**：
- 数据库中存在 role=admin 的用户

#### 4.2 用户管理
**测试场景**：查看和管理用户

**API 调用**：
```bash
GET /api/v1/admin/users?page=1&keyword=test
POST /api/v1/admin/users/{user_id}/status
{
  "status": 0  // 禁用用户
}
```

**验证点**：
- 返回用户列表
- 可以搜索用户
- 可以禁用/启用用户

#### 4.3 任务审核
**测试场景**：审核商家发布的任务

**API 调用**：
```bash
GET /api/v1/admin/tasks?status=pending
POST /api/v1/admin/tasks/{task_id}/review
{
  "action": "approve",
  "reason": ""
}
```

**验证点**：
- 返回待审核任务
- 审核通过后任务状态变为 published
- 审核拒绝后任务状态变为 rejected

#### 4.4 申诉处理
**测试场景**：处理用户申诉

**API 调用**：
```bash
GET /api/v1/admin/appeals?status=pending
POST /api/v1/admin/appeals/{appeal_id}/handle
{
  "action": "approve",
  "result": "处理结果"
}
```

**验证点**：
- 返回待处理申诉
- 处理后申诉状态更新

---

### Phase 5: 边界条件和异常测试

#### 5.1 余额不足测试
- [ ] 商家余额不足时发布任务
- [ ] 创作者余额不足时提现

#### 5.2 权限测试
- [ ] 创作者访问商家接口（应返回 403）
- [ ] 商家访问创作者接口（应返回 403）
- [ ] 未登录访问需要认证的接口（应返回 401）

#### 5.3 重复操作测试
- [ ] 重复认领同一任务
- [ ] 重复提交同一认领
- [ ] 重复审核同一提交

#### 5.4 并发测试
- [ ] 多个创作者同时认领任务（测试 remaining_count 原子性）
- [ ] 商家同时审核多个提交

#### 5.5 数据验证测试
- [ ] 提交空数据
- [ ] 提交超长字符串
- [ ] 提交负数金额
- [ ] 提交非法 JSON

---

### Phase 6: 多角色切换测试

#### 6.1 角色切换
**测试场景**：用户在创作者和商家身份间切换

**前置条件**：
- 用户 roles 包含 "creator,business"

**操作步骤**：
1. 以创作者身份登录
2. 切换到商家身份
3. 验证导航栏和权限变化
4. 切换回创作者身份

**验证点**：
- localStorage 中 current_role 正确更新
- 页面导航栏显示正确
- API 调用使用正确的角色

---

## 测试脚本结构

### 目录结构
```
test/
├── MONKEY_TEST_PLAN.md          # 本文档
├── monkey_test.sh               # 主测试脚本
├── lib/
│   ├── api.sh                   # API 调用封装
│   ├── assert.sh                # 断言函数
│   ├── data.sh                  # 测试数据生成
│   └── utils.sh                 # 工具函数
├── cases/
│   ├── 01_basic.sh              # 基础功能测试
│   ├── 02_creator.sh            # 创作者流程测试
│   ├── 03_business.sh           # 商家流程测试
│   ├── 04_admin.sh              # 管理员功能测试
│   ├── 05_edge.sh               # 边界条件测试
│   └── 06_concurrent.sh         # 并发测试
└── reports/
    └── test_report_YYYYMMDD_HHMMSS.log
```

### 脚本功能模块

#### 1. API 调用封装（lib/api.sh）
```bash
# 通用 API 调用
api_call() {
  local method=$1
  local endpoint=$2
  local data=$3
  local token=$4
  
  curl -s -X "$method" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $token" \
    -d "$data" \
    "$BASE_URL$endpoint"
}

# 注册用户
api_register() {
  local username=$1
  local password=$2
  local phone=$3
  local role=$4
  
  api_call POST "/api/v1/auth/register" \
    "{\"username\":\"$username\",\"password\":\"$password\",\"phone\":\"$phone\",\"role\":\"$role\"}"
}

# 登录
api_login() {
  local username=$1
  local password=$2
  
  api_call POST "/api/v1/auth/login" \
    "{\"username\":\"$username\",\"password\":\"$password\"}"
}

# 创作者认领任务
api_claim_task() {
  local task_id=$1
  local token=$2
  
  api_call POST "/api/v1/creator/tasks/$task_id/claim" "" "$token"
}

# 商家发布任务
api_create_task() {
  local token=$1
  local title=$2
  local reward=$3
  local count=$4
  
  api_call POST "/api/v1/business/tasks" \
    "{\"title\":\"$title\",\"description\":\"测试任务\",\"requirements\":\"测试要求\",\"reward\":$reward,\"total_count\":$count,\"category\":\"writing\",\"payment_type\":\"prepay\"}" \
    "$token"
}
```

#### 2. 断言函数（lib/assert.sh）
```bash
# 断言 HTTP 状态码
assert_status() {
  local response=$1
  local expected=$2
  local actual=$(echo "$response" | jq -r '.code')
  
  if [ "$actual" != "$expected" ]; then
    echo "❌ 断言失败: 期望状态码 $expected, 实际 $actual"
    echo "响应: $response"
    return 1
  fi
  echo "✅ 状态码正确: $expected"
}

# 断言字段存在
assert_field_exists() {
  local response=$1
  local field=$2
  
  if ! echo "$response" | jq -e ".$field" > /dev/null 2>&1; then
    echo "❌ 断言失败: 字段 $field 不存在"
    return 1
  fi
  echo "✅ 字段存在: $field"
}

# 断言字段值
assert_field_value() {
  local response=$1
  local field=$2
  local expected=$3
  local actual=$(echo "$response" | jq -r ".$field")
  
  if [ "$actual" != "$expected" ]; then
    echo "❌ 断言失败: $field 期望 $expected, 实际 $actual"
    return 1
  fi
  echo "✅ 字段值正确: $field = $expected"
}

# 断言数值大于
assert_greater_than() {
  local response=$1
  local field=$2
  local threshold=$3
  local actual=$(echo "$response" | jq -r ".$field")
  
  if (( $(echo "$actual <= $threshold" | bc -l) )); then
    echo "❌ 断言失败: $field ($actual) 应大于 $threshold"
    return 1
  fi
  echo "✅ 数值正确: $field ($actual) > $threshold"
}
```

#### 3. 测试数据生成（lib/data.sh）
```bash
# 生成随机用户名
generate_username() {
  echo "user_$(date +%s)_$RANDOM"
}

# 生成随机手机号
generate_phone() {
  echo "138$(printf "%08d" $RANDOM)"
}

# 生成测试任务数据
generate_task_data() {
  local title="测试任务_$(date +%s)"
  local reward=$(( RANDOM % 50 + 10 ))
  local count=$(( RANDOM % 100 + 10 ))
  
  echo "{\"title\":\"$title\",\"reward\":$reward,\"count\":$count}"
}
```

#### 4. 工具函数（lib/utils.sh）
```bash
# 日志输出
log_info() {
  echo "[INFO] $(date '+%Y-%m-%d %H:%M:%S') $*"
}

log_error() {
  echo "[ERROR] $(date '+%Y-%m-%d %H:%M:%S') $*" >&2
}

log_success() {
  echo "[SUCCESS] $(date '+%Y-%m-%d %H:%M:%S') $*"
}

# 测试计数
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

test_start() {
  TOTAL_TESTS=$((TOTAL_TESTS + 1))
  log_info "开始测试: $1"
}

test_pass() {
  PASSED_TESTS=$((PASSED_TESTS + 1))
  log_success "测试通过: $1"
}

test_fail() {
  FAILED_TESTS=$((FAILED_TESTS + 1))
  log_error "测试失败: $1"
}

# 测试报告
generate_report() {
  echo ""
  echo "========================================="
  echo "测试报告"
  echo "========================================="
  echo "总测试数: $TOTAL_TESTS"
  echo "通过: $PASSED_TESTS"
  echo "失败: $FAILED_TESTS"
  echo "通过率: $(echo "scale=2; $PASSED_TESTS * 100 / $TOTAL_TESTS" | bc)%"
  echo "========================================="
}

# 清理测试数据
cleanup() {
  log_info "清理测试数据..."
  # 删除测试数据库或清空测试表
}
```

---

## 测试执行计划

### 第一阶段：基础测试（1-2天）
- 完善现有的 monkey_test.sh
- 实现 lib/ 下的工具函数
- 完成 Phase 1 和 Phase 2 的测试用例

### 第二阶段：业务流程测试（2-3天）
- 实现 Phase 3 商家流程测试
- 实现 Phase 4 管理员功能测试
- 完善测试报告生成

### 第三阶段：边界和并发测试（2-3天）
- 实现 Phase 5 边界条件测试
- 实现并发测试
- 性能测试和压力测试

### 第四阶段：持续集成（1天）
- 集成到 CI/CD 流程
- 定时执行测试
- 测试结果通知

---

## 问题跟踪和版本迭代

### 问题分类
1. **P0 - 阻塞性问题**：影响核心功能，必须立即修复
2. **P1 - 严重问题**：影响重要功能，需要尽快修复
3. **P2 - 一般问题**：影响用户体验，可以排期修复
4. **P3 - 优化建议**：不影响功能，可以考虑优化

### 版本迭代流程
1. **测试发现问题** → 记录到 issues
2. **问题分类和优先级排序**
3. **修复问题** → 提交代码
4. **回归测试** → 验证修复
5. **发布新版本**

### 测试报告模板
```
# Monkey Test 报告 - YYYY-MM-DD

## 测试环境
- 服务器地址: http://localhost:8888
- 测试时间: YYYY-MM-DD HH:MM:SS
- 测试版本: v1.0.0

## 测试结果
- 总测试数: 50
- 通过: 45
- 失败: 5
- 通过率: 90%

## 失败用例
1. [P0] 创作者认领任务失败 - remaining_count 未正确减少
2. [P1] 商家验收作品时金额计算错误
3. [P2] 用户资料页面加载缓慢
4. [P3] 任务列表排序不符合预期
5. [P3] 错误提示文案不友好

## 性能数据
- 平均响应时间: 150ms
- 最慢接口: POST /api/v1/business/tasks (500ms)
- 并发测试: 10 用户同时操作，无错误

## 建议
1. 优化任务认领的并发控制
2. 添加金额计算的单元测试
3. 优化数据库查询性能
```

---

## Phase 7: 基于 Agent-Browser 的 UI 自动化测试

### 7.1 测试工具选择

**Agent-Browser** 是一个基于 AI 的浏览器自动化工具，相比传统的 Selenium/Playwright，具有以下优势：
- 使用自然语言描述测试步骤，无需编写复杂的选择器
- AI 自动识别页面元素，适应 UI 变化
- 更接近真实用户操作，能发现更多 UX 问题
- 支持视觉验证和截图对比

### 7.2 测试环境配置

```bash
# 安装 agent-browser
npm install -g agent-browser

# 或使用 Python 版本
pip install agent-browser

# 配置测试环境
export TEST_BASE_URL="http://localhost:8888"
export BROWSER_HEADLESS=false  # 开发时可视化，CI 时设为 true
export SCREENSHOT_DIR="./test/screenshots"
```

### 7.3 UI 测试用例设计

#### 7.3.1 用户注册流程
**测试场景**：新用户完整注册流程

**测试步骤**（自然语言）：
```
1. 打开首页
2. 点击"注册"按钮
3. 填写用户名：{random_username}
4. 填写密码：test123456
5. 填写确认密码：test123456
6. 填写手机号：{random_phone}
7. 选择角色：商家
8. 点击"注册"按钮
9. 验证：页面跳转到登录页
10. 验证：显示"注册成功"提示
```

**验证点**：
- 表单验证正确（必填项、格式校验）
- 注册成功后正确跳转
- 错误提示友好（用户名重复、密码不一致等）

#### 7.3.2 登录并选择身份
**测试场景**：用户登录时选择进入的身份

**测试步骤**：
```
1. 打开登录页面
2. 填写用户名：{username}
3. 填写密码：{password}
4. 选择登录身份：商家
5. 点击"登录"按钮
6. 验证：跳转到商家工作台
7. 验证：导航栏显示"商家"标签
8. 验证：页面显示商家相关统计数据
```

**验证点**：
- 身份选择下拉框正常工作
- 登录后进入正确的工作台
- 角色标识显示正确

#### 7.3.3 角色切换
**测试场景**：用户在商家和创作者身份间切换

**测试步骤**：
```
1. 以商家身份登录
2. 点击右上角"商家"标签
3. 在下拉菜单中选择"创作者"
4. 验证：页面跳转到创作者工作台
5. 验证：导航栏变为创作者菜单
6. 验证：统计数据变为创作者数据
7. 点击"创作者"标签
8. 切换回"商家"
9. 验证：回到商家工作台
```

**验证点**：
- 角色切换器正常工作
- 切换后页面内容正确更新
- localStorage 中 current_role 正确保存

#### 7.3.4 商家充值流程
**测试场景**：商家账户充值

**测试步骤**：
```
1. 以商家身份登录
2. 点击导航栏"钱包"
3. 点击"充值"按钮
4. 输入充值金额：1000
5. 选择支付方式：支付宝
6. 点击"确认充值"
7. 验证：显示支付二维码或跳转支付页面
8. 模拟支付成功回调
9. 验证：余额增加 1000 元
10. 验证：交易记录中显示充值记录
```

**验证点**：
- 充值表单验证（金额范围、格式）
- 支付流程正常
- 余额更新正确
- 交易记录生成

#### 7.3.5 商家发布任务
**测试场景**：商家发布新任务

**测试步骤**：
```
1. 以商家身份登录
2. 点击"发布任务"
3. 填写任务标题：测试任务_{timestamp}
4. 填写任务描述：这是一个测试任务
5. 选择任务类型：设计
6. 填写单价：100
7. 填写需求数量：5
8. 填写截止日期：{7天后}
9. 填写任务要求：原创作品，高清图片
10. 上传参考文件（可选）
11. 点击"发布"按钮
12. 验证：显示"发布成功"提示
13. 验证：任务列表中显示新任务
14. 验证：账户冻结金额增加 500 元
```

**验证点**：
- 表单验证完整（必填项、格式、金额计算）
- 文件上传功能正常
- 任务创建成功
- 金额冻结正确

#### 7.3.6 创作者浏览任务大厅
**测试场景**：创作者浏览和筛选任务

**测试步骤**：
```
1. 以创作者身份登录
2. 点击"任务大厅"
3. 验证：显示可认领的任务列表
4. 选择任务类型筛选：设计
5. 验证：只显示设计类任务
6. 选择价格排序：从高到低
7. 验证：任务按价格降序排列
8. 点击某个任务查看详情
9. 验证：显示任务详细信息
10. 验证：显示"认领"按钮
```

**验证点**：
- 任务列表正确显示
- 筛选和排序功能正常
- 任务详情页完整
- 按钮状态正确（已认领/已满/可认领）

#### 7.3.7 创作者认领任务
**测试场景**：创作者认领任务

**测试步骤**：
```
1. 在任务详情页
2. 点击"认领任务"按钮
3. 验证：显示确认对话框
4. 点击"确认"
5. 验证：显示"认领成功"提示
6. 验证：按钮变为"已认领"状态
7. 点击"我的认领"
8. 验证：列表中显示刚认领的任务
```

**验证点**：
- 认领确认流程
- 认领成功反馈
- 按钮状态更新
- 我的认领列表更新

#### 7.3.8 创作者提交作品
**测试场景**：创作者提交任务作品

**测试步骤**：
```
1. 进入"我的认领"
2. 找到待提交的任务
3. 点击"提交作品"
4. 填写作品说明：这是我的创意作品
5. 上传作品文件
6. 点击"提交"
7. 验证：显示"提交成功"提示
8. 验证：任务状态变为"待审核"
9. 验证：不能再次提交
```

**验证点**：
- 作品提交表单
- 文件上传功能
- 提交成功反馈
- 状态更新正确

#### 7.3.9 商家审核作品
**测试场景**：商家验收创作者提交的作品

**测试步骤**：
```
1. 以商家身份登录
2. 点击"待审核"（显示红点提示）
3. 验证：显示待审核的作品列表
4. 点击某个作品查看详情
5. 验证：显示作品内容和文件
6. 点击"通过"按钮
7. 填写评价：作品质量不错
8. 选择评分：5星
9. 点击"确认"
10. 验证：显示"审核成功"提示
11. 验证：作品从待审核列表移除
12. 验证：账户冻结金额减少
```

**验证点**：
- 待审核列表正确
- 作品详情完整
- 审核流程顺畅
- 金额结算正确

#### 7.3.10 创作者查看收益
**测试场景**：创作者查看钱包和收益明细

**测试步骤**：
```
1. 以创作者身份登录
2. 点击"钱包"
3. 验证：显示可用余额、冻结金额、总收益
4. 点击"收益明细"
5. 验证：显示交易记录列表
6. 验证：最新一条为刚才的任务收益
7. 点击某条记录查看详情
8. 验证：显示交易详细信息
```

**验证点**：
- 钱包数据准确
- 交易记录完整
- 金额计算正确

### 7.4 测试脚本示例

#### 使用 Agent-Browser (Python)
```python
from agent_browser import Browser
import pytest

class TestUserFlow:
    @pytest.fixture
    def browser(self):
        browser = Browser(headless=False)
        browser.goto("http://localhost:8888")
        yield browser
        browser.close()
    
    def test_register_and_login(self, browser):
        """测试注册和登录流程"""
        # 注册
        browser.click("注册")
        browser.fill("用户名", f"test_user_{int(time.time())}")
        browser.fill("密码", "test123456")
        browser.fill("确认密码", "test123456")
        browser.fill("手机号", f"138{random.randint(10000000, 99999999)}")
        browser.select("登录身份", "商家")
        browser.click("注册")
        
        # 验证
        assert browser.has_text("注册成功")
        assert browser.current_url().endswith("/login")
        
        # 登录
        browser.fill("用户名", username)
        browser.fill("密码", "test123456")
        browser.select("登录身份", "商家")
        browser.click("登录")
        
        # 验证
        assert browser.has_text("商家工作台")
        assert browser.has_element("导航栏", text="商家")
    
    def test_role_switch(self, browser):
        """测试角色切换"""
        # 登录为双角色用户
        self.login(browser, "dual_role_user", "test123456")
        
        # 切换到创作者
        browser.click("商家")  # 点击角色标签
        browser.click("创作者")  # 选择创作者
        
        # 验证
        assert browser.has_text("创作者工作台")
        assert browser.has_element("导航栏", text="创作者")
        
        # 切换回商家
        browser.click("创作者")
        browser.click("商家")
        
        # 验证
        assert browser.has_text("商家工作台")
    
    def test_business_publish_task(self, browser):
        """测试商家发布任务"""
        self.login_as_business(browser)
        
        # 发布任务
        browser.click("发布任务")
        browser.fill("任务标题", f"测试任务_{int(time.time())}")
        browser.fill("任务描述", "这是一个测试任务")
        browser.select("任务类型", "设计")
        browser.fill("单价", "100")
        browser.fill("需求数量", "5")
        browser.fill("任务要求", "原创作品，高清图片")
        browser.click("发布")
        
        # 验证
        assert browser.has_text("发布成功")
        assert browser.has_element("任务列表", text="测试任务")
    
    def test_creator_claim_task(self, browser):
        """测试创作者认领任务"""
        self.login_as_creator(browser)
        
        # 浏览任务
        browser.click("任务大厅")
        browser.click("第一个任务")  # 点击第一个可认领的任务
        
        # 认领
        browser.click("认领任务")
        browser.click("确认")
        
        # 验证
        assert browser.has_text("认领成功")
        assert browser.has_element("按钮", text="已认领")
    
    def test_full_workflow(self, browser):
        """测试完整业务流程"""
        # 1. 商家充值
        self.login_as_business(browser)
        browser.click("钱包")
        browser.click("充值")
        browser.fill("金额", "1000")
        browser.click("确认充值")
        assert browser.has_text("充值成功")
        
        # 2. 商家发布任务
        task_title = f"测试任务_{int(time.time())}"
        browser.click("发布任务")
        browser.fill("任务标题", task_title)
        browser.fill("单价", "100")
        browser.fill("需求数量", "5")
        browser.click("发布")
        assert browser.has_text("发布成功")
        
        # 3. 创作者认领任务
        browser.logout()
        self.login_as_creator(browser)
        browser.click("任务大厅")
        browser.click(task_title)
        browser.click("认领任务")
        browser.click("确认")
        assert browser.has_text("认领成功")
        
        # 4. 创作者提交作品
        browser.click("我的认领")
        browser.click(task_title)
        browser.click("提交作品")
        browser.fill("作品说明", "这是我的创意作品")
        browser.upload("作品文件", "./test_file.jpg")
        browser.click("提交")
        assert browser.has_text("提交成功")
        
        # 5. 商家审核作品
        browser.logout()
        self.login_as_business(browser)
        browser.click("待审核")
        browser.click(task_title)
        browser.click("通过")
        browser.fill("评价", "作品质量不错")
        browser.select("评分", "5星")
        browser.click("确认")
        assert browser.has_text("审核成功")
        
        # 6. 创作者查看收益
        browser.logout()
        self.login_as_creator(browser)
        browser.click("钱包")
        assert browser.has_text("100.00")  # 验证收益到账
```

### 7.5 视觉回归测试

**测试场景**：确保 UI 改动不会破坏现有页面

```python
def test_visual_regression(browser):
    """视觉回归测试"""
    pages = [
        "/",
        "/login",
        "/register",
        "/business/dashboard",
        "/creator/dashboard",
        "/business/tasks",
        "/creator/tasks",
    ]
    
    for page in pages:
        browser.goto(f"http://localhost:8888{page}")
        screenshot = browser.screenshot()
        
        # 与基准截图对比
        baseline = f"./test/screenshots/baseline{page.replace('/', '_')}.png"
        diff = compare_images(screenshot, baseline)
        
        assert diff < 0.05, f"页面 {page} 视觉差异过大: {diff}"
```

### 7.6 测试执行计划

#### 开发阶段
```bash
# 本地运行，可视化模式
BROWSER_HEADLESS=false pytest test/ui/ -v

# 运行特定测试
pytest test/ui/test_user_flow.py::test_register_and_login -v
```

#### CI/CD 集成
```yaml
# .github/workflows/ui-test.yml
name: UI Tests

on: [push, pull_request]

jobs:
  ui-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Setup Python
        uses: actions/setup-python@v2
        with:
          python-version: '3.9'
      
      - name: Install dependencies
        run: |
          pip install agent-browser pytest
          npm install -g agent-browser
      
      - name: Start server
        run: |
          ./deploy.sh dev &
          sleep 5
      
      - name: Run UI tests
        run: |
          BROWSER_HEADLESS=true pytest test/ui/ -v --html=report.html
      
      - name: Upload screenshots
        if: failure()
        uses: actions/upload-artifact@v2
        with:
          name: screenshots
          path: test/screenshots/
```

### 7.7 测试覆盖目标

- **页面覆盖率**：100%（所有页面至少访问一次）
- **功能覆盖率**：90%（核心业务流程全覆盖）
- **用户路径覆盖**：80%（常见用户操作路径）
- **视觉回归**：关键页面 100%

### 7.8 测试维护策略

1. **基准更新**：UI 改动后更新基准截图
2. **选择器维护**：使用语义化描述，减少维护成本
3. **测试数据隔离**：每次测试使用独立的测试数据
4. **失败重试**：网络不稳定时自动重试
5. **并行执行**：独立测试用例并行运行，提高效率

---

## 下一步行动

1. **立即执行**：
   - 修复当前的数据库 CHECK 约束问题
   - 完善 monkey_test.sh 脚本
   - 实现 lib/api.sh 和 lib/assert.sh

2. **本周完成**：
   - 实现 Phase 2 创作者完整流程测试
   - 实现 Phase 3 商家完整流程测试
   - 生成第一份测试报告

3. **下周计划**：
   - 实现边界条件测试
   - 实现并发测试
   - 集成到 CI/CD

---

## 附录：测试数据准备

### 测试账号
```bash
# 创作者账号
creator1: password123
creator2: password123

# 商家账号
business1: password123
business2: password123

# 管理员账号
admin: admin123
```

### 测试任务
```json
{
  "title": "撰写产品评测文章",
  "description": "为我们的新产品撰写一篇评测文章",
  "requirements": "字数不少于500字，包含产品优缺点分析",
  "reward": 20.00,
  "total_count": 50,
  "category": "writing",
  "payment_type": "prepay"
}
```

### 测试作品
```json
{
  "content": "这是一篇测试评测文章...",
  "attachments": [
    "https://example.com/article.pdf"
  ]
}
```
