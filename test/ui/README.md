# UI 自动化测试

## 测试状态

### ✅ 已完成（使用 agent-browser CLI）
- `test_user_registration.py` - 用户注册测试（6个测试，全部通过）
- `test_user_login.py` - 登录和角色切换测试（10个测试）

### 🔄 待重写（需要适配 agent-browser CLI）
以下测试文件已更新导入语句，但需要根据实际页面结构重写测试逻辑：

- `test_business_flow.py` - 商家业务流程测试（12个测试）
- `test_creator_flow.py` - 创作者业务流程测试（15个测试）
- `test_business_review.py` - 商家审核作品测试（12个测试）
- `test_full_workflow.py` - 完整端到端测试（4个测试）
- `test_visual_and_accessibility.py` - 视觉回归和可访问性测试（20个测试）

## 运行测试

```bash
# 运行所有已完成的测试
python3 -m pytest test/ui/test_user_registration.py test/ui/test_user_login.py -v

# 运行单个测试文件
python3 -m pytest test/ui/test_user_registration.py -v

# 运行单个测试用例
python3 -m pytest test/ui/test_user_registration.py::test_register_as_business -v
```

## Browser 类 API

Browser 类封装了 agent-browser CLI，主要方法：

- `open(path)` - 打开页面（相对路径）
- `fill(label, text)` - 填充输入框（支持中文标签）
- `click(selector)` - 点击元素（支持中文按钮文本）
- `wait(ms)` - 等待指定毫秒
- `snapshot()` - 获取页面快照
- `get_url()` - 获取当前URL
- `is_visible(selector)` - 检查元素是否可见
- `screenshot(path)` - 截图
- `close()` - 关闭浏览器

## 注意事项

1. agent-browser 使用自然语言和智能选择器，不需要精确的 CSS 选择器
2. 中文标签（如"用户名"、"密码"）可以直接使用
3. 需要适当的等待时间（wait）确保页面加载和跳转完成
4. 使用 snapshot() 检查页面内容而不是 has_text()
