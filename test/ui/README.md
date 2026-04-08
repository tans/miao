# UI 自动化测试

基于 agent-browser 的 UI 自动化测试套件。

## 安装依赖

```bash
# 安装 Python 依赖
pip install agent-browser pytest pytest-html pillow imagehash

# 或使用 requirements.txt
pip install -r requirements.txt
```

## 测试文件结构

```
test/ui/
├── conftest.py                          # pytest 配置
├── test_user_registration.py           # 用户注册测试
├── test_user_login.py                   # 用户登录和角色切换测试
├── test_business_flow.py                # 商家业务流程测试
├── test_creator_flow.py                 # 创作者业务流程测试
├── test_business_review.py              # 商家审核作品测试
├── test_full_workflow.py                # 完整业务流程端到端测试
└── test_visual_and_accessibility.py     # 视觉回归和可访问性测试
```

## 运行测试

### 运行所有测试
```bash
cd test/ui
pytest -v
```

### 运行特定测试文件
```bash
pytest test_user_registration.py -v
pytest test_full_workflow.py -v
```

### 运行特定测试用例
```bash
pytest test_user_registration.py::TestUserRegistration::test_register_as_business -v
```

### 按标记运行测试
```bash
# 只运行端到端测试
pytest -m e2e -v

# 只运行视觉回归测试
pytest -m visual -v

# 排除视觉回归测试
pytest -m "not visual" -v
```

### 生成 HTML 报告
```bash
pytest --html=report.html --self-contained-html
```

### 并行运行测试
```bash
pip install pytest-xdist
pytest -n 4  # 使用4个进程并行运行
```

## 测试覆盖

### 1. 用户注册测试 (test_user_registration.py)
- ✅ 注册商家账号
- ✅ 注册创作者账号
- ✅ 表单验证：空字段
- ✅ 表单验证：密码不一致
- ✅ 表单验证：用户名重复
- ✅ 表单验证：手机号格式错误

### 2. 用户登录测试 (test_user_login.py)
- ✅ 以商家身份登录
- ✅ 以创作者身份登录
- ✅ 表单验证：空字段
- ✅ 表单验证：密码错误
- ✅ 表单验证：用户不存在
- ✅ 从商家切换到创作者
- ✅ 从创作者切换到商家
- ✅ 角色切换保持会话
- ✅ 角色切换更新统计数据

### 3. 商家业务流程测试 (test_business_flow.py)
- ✅ 充值成功
- ✅ 充值金额验证
- ✅ 充值交易记录
- ✅ 发布任务成功
- ✅ 发布任务并上传文件
- ✅ 发布任务表单验证
- ✅ 余额不足时发布任务
- ✅ 发布任务后冻结金额
- ✅ 查看任务列表
- ✅ 查看任务详情
- ✅ 按状态筛选任务
- ✅ 搜索任务

### 4. 创作者业务流程测试 (test_creator_flow.py)
- ✅ 查看任务大厅
- ✅ 按类型筛选任务
- ✅ 按价格排序任务
- ✅ 搜索任务
- ✅ 查看任务详情
- ✅ 认领任务成功
- ✅ 认领的任务出现在我的认领中
- ✅ 不能重复认领同一任务
- ✅ 认领后剩余数量减少
- ✅ 提交作品成功
- ✅ 提交作品后状态变化
- ✅ 提交作品验证：空内容
- ✅ 不能重复提交作品
- ✅ 查看钱包
- ✅ 查看交易记录
- ✅ 按类型筛选交易记录

### 5. 商家审核作品测试 (test_business_review.py)
- ✅ 查看待审核列表
- ✅ 查看作品详情
- ✅ 通过作品审核
- ✅ 通过审核后余额变化
- ✅ 拒绝作品审核
- ✅ 拒绝后创作者可以重新提交
- ✅ 审核通过后作品从待审核列表移除
- ✅ 查看已通过的作品
- ✅ 查看工作台统计
- ✅ 查看资金流水
- ✅ 按类型筛选资金流水
- ✅ 查看任务进度

### 6. 完整业务流程测试 (test_full_workflow.py)
- ✅ 完整的商家-创作者业务流程（17步）
- ✅ 多个创作者认领同一任务
- ✅ 用户同时拥有商家和创作者角色
- ✅ 任务完整生命周期

### 7. 视觉回归和可访问性测试 (test_visual_and_accessibility.py)
- ✅ 首页视觉回归
- ✅ 登录页面视觉回归
- ✅ 注册页面视觉回归
- ✅ 商家工作台视觉回归
- ✅ 创作者工作台视觉回归
- ✅ 首页加载
- ✅ 登录页面加载
- ✅ 注册页面加载
- ✅ 404页面
- ✅ 所有导航链接可访问
- ✅ 移动端响应式设计
- ✅ 平板端响应式设计
- ✅ 桌面端响应式设计
- ✅ 登录表单验证
- ✅ 注册表单验证
- ✅ 任务表单验证
- ✅ 网络错误处理
- ✅ 会话超时处理
- ✅ 未授权访问
- ✅ 错误角色访问

## 测试数据准备

### 创建测试fixtures
```bash
mkdir -p test/fixtures
# 准备测试用的图片文件
cp /path/to/test/image.jpg test/fixtures/work.jpg
cp /path/to/test/reference.jpg test/fixtures/reference.jpg
```

### 创建测试账号
测试会自动创建临时账号，也可以手动创建固定测试账号：

```sql
-- 商家测试账号
INSERT INTO users (username, password_hash, role, phone) 
VALUES ('test_business', '$2a$10$...', 'business,creator', '13800000001');

-- 创作者测试账号
INSERT INTO users (username, password_hash, role, phone) 
VALUES ('test_creator', '$2a$10$...', 'business,creator', '13900000001');
```

## CI/CD 集成

### GitHub Actions 示例
```yaml
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
          pip install -r test/requirements.txt
      
      - name: Start server
        run: |
          ./deploy.sh dev &
          sleep 5
      
      - name: Run UI tests
        run: |
          cd test/ui
          pytest -v --html=report.html --self-contained-html
      
      - name: Upload test report
        if: always()
        uses: actions/upload-artifact@v2
        with:
          name: test-report
          path: test/ui/report.html
      
      - name: Upload screenshots
        if: failure()
        uses: actions/upload-artifact@v2
        with:
          name: failure-screenshots
          path: test/screenshots/failures/
```

## 故障排查

### 浏览器启动失败
```bash
# 安装浏览器驱动
agent-browser install
```

### 测试超时
```bash
# 增加超时时间
pytest --timeout=300
```

### 截图对比失败
```bash
# 更新基准截图
rm -rf test/screenshots/baseline/*
pytest -m visual  # 重新生成基准截图
```

## 最佳实践

1. **测试隔离**：每个测试用例使用独立的测试数据
2. **等待策略**：使用智能等待而不是固定sleep
3. **错误处理**：测试失败时自动截图
4. **并行执行**：独立测试用例可以并行运行
5. **持续集成**：集成到CI/CD流程中自动运行

## 维护指南

### 更新基准截图
当UI有意图的改动时，需要更新基准截图：
```bash
rm -rf test/screenshots/baseline/*
pytest -m visual
```

### 添加新测试用例
1. 在对应的测试文件中添加测试方法
2. 使用描述性的测试名称
3. 添加必要的断言
4. 运行测试验证

### 调试测试
```bash
# 显示浏览器窗口
pytest --headed

# 单步调试
pytest --pdb

# 详细输出
pytest -vv -s
```
