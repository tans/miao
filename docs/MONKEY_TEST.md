# Monkey Test 使用指南

## 简介

Monkey Test 是一个完整的端到端自动化测试脚本，用于测试创意喵平台的所有核心业务流程。

## 测试覆盖范围

### 1. 用户注册与登录
- ✅ 创作者注册
- ✅ 商家注册
- ✅ 创作者登录
- ✅ 商家登录

### 2. 商家流程
- ✅ 账户充值
- ✅ 发布任务
- ✅ 查看投稿
- ✅ 验收作品（通过/拒绝）
- ✅ 查看资金流水
- ✅ 查看工作台统计

### 3. 创作者流程
- ✅ 浏览任务大厅
- ✅ 认领任务
- ✅ 查看我的认领
- ✅ 提交作品
- ✅ 查看钱包余额
- ✅ 查看收益明细
- ✅ 查看工作台统计

### 4. 通用功能
- ✅ 查看个人资料
- ✅ JWT 认证
- ✅ API 错误处理

## 使用方法

### 方法一：使用脚本运行（推荐）

```bash
# 1. 确保服务器正在运行
./miao-server

# 2. 在新终端运行测试
./run_monkey_test.sh
```

### 方法二：直接运行 Go 测试

```bash
# 确保服务器正在运行
./miao-server

# 运行测试
cd /Users/ke/code/miao
go test -v ./test -run TestMonkeyFullFlow -timeout 5m
```

## 测试流程

```
1. 注册创作者账号 (creator_<timestamp>)
   ↓
2. 注册商家账号 (business_<timestamp>)
   ↓
3. 创作者登录 → 获取 JWT Token
   ↓
4. 商家登录 → 获取 JWT Token
   ↓
5. 商家充值 10000 元
   ↓
6. 商家发布任务（单价100元，需求5份）
   ↓
7. 创作者浏览任务大厅
   ↓
8. 创作者认领任务
   ↓
9. 创作者查看我的认领
   ↓
10. 创作者提交作品
    ↓
11. 商家查看投稿
    ↓
12. 商家验收作品（通过）
    ↓
13. 创作者查看钱包（余额增加）
    ↓
14. 创作者查看收益明细
    ↓
15. 商家查看资金流水
    ↓
16. 创作者查看个人资料
    ↓
17. 商家查看工作台统计
    ↓
18. 创作者查看工作台统计
    ↓
✅ 测试完成
```

## 测试输出示例

```
=== RUN   TestMonkeyFullFlow
=== RUN   TestMonkeyFullFlow/1._注册创作者账号
✅ 注册成功: creator_1234567890 (creator)
=== RUN   TestMonkeyFullFlow/2._注册商家账号
✅ 注册成功: business_1234567890 (business)
=== RUN   TestMonkeyFullFlow/3._创作者登录
✅ 登录成功: creator_1234567890 (Token: eyJhbGciOiJIUzI1NiIs...)
=== RUN   TestMonkeyFullFlow/4._商家登录
✅ 登录成功: business_1234567890 (Token: eyJhbGciOiJIUzI1NiIs...)
=== RUN   TestMonkeyFullFlow/5._商家充值
✅ 充值成功: 10000.00 元
=== RUN   TestMonkeyFullFlow/6._商家发布任务
✅ 发布任务成功: ID=123
=== RUN   TestMonkeyFullFlow/7._创作者浏览任务大厅
✅ 浏览任务大厅成功: 共 5 个任务
=== RUN   TestMonkeyFullFlow/8._创作者认领任务
✅ 认领任务成功: ClaimID=456
=== RUN   TestMonkeyFullFlow/9._创作者查看我的认领
✅ 查看我的认领成功: 共 1 个认领
=== RUN   TestMonkeyFullFlow/10._创作者提交作品
✅ 提交作品成功: ClaimID=456
=== RUN   TestMonkeyFullFlow/11._商家查看投稿
✅ 查看投稿成功: TaskID=123
=== RUN   TestMonkeyFullFlow/12._商家验收作品
✅ 验收作品成功: ClaimID=456, 结果=true
=== RUN   TestMonkeyFullFlow/13._创作者查看钱包
✅ 查看钱包成功: 余额=90.00
=== RUN   TestMonkeyFullFlow/14._创作者查看收益明细
✅ 查看交易记录成功
=== RUN   TestMonkeyFullFlow/15._商家查看资金流水
✅ 查看交易记录成功
=== RUN   TestMonkeyFullFlow/16._创作者查看个人资料
✅ 查看个人资料成功
=== RUN   TestMonkeyFullFlow/17._商家查看工作台统计
✅ 查看工作台成功
=== RUN   TestMonkeyFullFlow/18._创作者查看工作台统计
✅ 查看工作台成功
✅ 完整业务流程测试通过！
创作者: creator_1234567890 (ID: 10)
商家: business_1234567890 (ID: 11)
任务ID: 123, 认领ID: 456
--- PASS: TestMonkeyFullFlow (5.23s)
PASS
ok      miao/test       5.234s
```

## 注意事项

1. **服务器必须运行**：测试前确保 `miao-server` 正在运行
2. **数据库状态**：每次测试会创建新的测试数据
3. **端口配置**：默认使用 `http://localhost:8080`
4. **超时设置**：测试超时时间为 5 分钟
5. **随机数据**：每次运行使用不同的用户名和手机号

## 自定义配置

如需修改测试配置，编辑 `test/monkey_test.go`：

```go
const (
    BaseURL = "http://localhost:8080"  // 修改服务器地址
    APIURL  = BaseURL + "/api/v1"
)
```

## 故障排查

### 问题：服务器未运行
```
❌ 服务器未运行，请先启动服务器
```
**解决**：运行 `./miao-server` 启动服务器

### 问题：测试超时
```
panic: test timed out after 5m0s
```
**解决**：检查服务器性能，或增加超时时间 `-timeout 10m`

### 问题：API 返回错误
```
注册失败: 用户名已存在
```
**解决**：测试使用时间戳生成唯一用户名，通常不会冲突。如遇到，重新运行测试即可。

## 扩展测试

可以在 `test/monkey_test.go` 中添加更多测试场景：

- 申诉流程测试
- 多次认领测试
- 并发操作测试
- 边界条件测试
- 错误处理测试

## 持续集成

可将此测试集成到 CI/CD 流程：

```yaml
# .github/workflows/test.yml
- name: Run Monkey Test
  run: |
    ./miao-server &
    sleep 5
    ./run_monkey_test.sh
```
