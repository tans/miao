"""
商家业务流程测试
"""
import time
import random
from test.ui.browser import Browser
import pytest


class TestBusinessRecharge:
    """商家充值测试"""

@pytest.fixture
    def business_user(self, browser):
        """创建并登录商家用户"""
        username = f"business_{int(time.time())}"
        password = "test123456"
        phone = f"138{random.randint(10000000, 99999999)}"

        # 注册
        browser.open("/register")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.fill("确认密码", password)
        browser.fill("手机号", phone)
        browser.click("注册")

        # 登录
        browser.open("/login")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.select("登录身份", "商家")
        browser.click("登录")

        return {"username": username, "password": password}

    def test_recharge_success(self, browser, business_user):
        """测试充值成功"""
        # 进入钱包页面
        browser.click("钱包")

        # 记录充值前余额
        balance_before = browser.get_text("可用余额")

        # 点击充值
        browser.click("充值")

        # 填写充值金额
        browser.fill("充值金额", "1000")

        # 选择支付方式
        browser.select("支付方式", "支付宝")

        # 确认充值
        browser.click("确认充值")

        # 验证显示支付页面或二维码
        assert browser.snapshot("支付宝") or browser.is_visible("二维码")

        # 模拟支付成功（在测试环境中自动完成）
        browser.wait(2)

        # 验证充值成功
        assert browser.snapshot("充值成功")

        # 验证余额增加
        balance_after = browser.get_text("可用余额")
        assert float(balance_after) == float(balance_before) + 1000

    def test_recharge_validation_invalid_amount(self, browser, business_user):
        """测试充值金额验证"""
        browser.click("钱包")
        browser.click("充值")

        # 测试负数
        browser.fill("充值金额", "-100")
        browser.click("确认充值")
        assert browser.snapshot("充值金额必须大于0")

        # 测试过小金额
        browser.fill("充值金额", "0.5")
        browser.click("确认充值")
        assert browser.snapshot("最低充值金额为1元")

    def test_recharge_transaction_record(self, browser, business_user):
        """测试充值交易记录"""
        # 充值
        browser.click("钱包")
        browser.click("充值")
        browser.fill("充值金额", "500")
        browser.select("支付方式", "支付宝")
        browser.click("确认充值")
        browser.wait(2)

        # 查看交易记录
        browser.click("交易记录")

        # 验证最新一条记录
        assert browser.snapshot("充值")
        assert browser.snapshot("500.00")
        assert browser.snapshot("支付宝")


class TestBusinessPublishTask:
    """商家发布任务测试"""

@pytest.fixture
    def business_user_with_balance(self, browser):
        """创建有余额的商家用户"""
        username = f"business_{int(time.time())}"
        password = "test123456"
        phone = f"138{random.randint(10000000, 99999999)}"

        # 注册并登录
        browser.open("/register")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.fill("确认密码", password)
        browser.fill("手机号", phone)
        browser.click("注册")

        browser.open("/login")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.select("登录身份", "商家")
        browser.click("登录")

        # 充值
        browser.click("钱包")
        browser.click("充值")
        browser.fill("充值金额", "10000")
        browser.click("确认充值")
        browser.wait(2)

        return {"username": username, "password": password}

    def test_publish_task_success(self, browser, business_user_with_balance):
        """测试发布任务成功"""
        task_title = f"测试任务_{int(time.time())}"

        # 点击发布任务
        browser.click("发布任务")

        # 填写任务信息
        browser.fill("任务标题", task_title)
        browser.fill("任务描述", "这是一个测试任务，请按要求完成创意作品。")
        browser.select("任务类型", "设计")
        browser.fill("单价", "100")
        browser.fill("需求数量", "5")
        browser.fill("截止日期", "2026-05-01")
        browser.fill("任务要求", "1. 原创作品\n2. 高清图片\n3. 符合主题")

        # 提交发布
        browser.click("发布")

        # 验证发布成功
        assert browser.snapshot("发布成功")

        # 验证跳转到任务列表
        assert browser.current_url().endswith("/business/tasks")

        # 验证任务出现在列表中
        assert browser.snapshot(task_title)

    def test_publish_task_with_file_upload(self, browser, business_user_with_balance):
        """测试发布任务并上传参考文件"""
        task_title = f"测试任务_{int(time.time())}"

        browser.click("发布任务")
        browser.fill("任务标题", task_title)
        browser.fill("任务描述", "需要参考文件的任务")
        browser.select("任务类型", "设计")
        browser.fill("单价", "100")
        browser.fill("需求数量", "5")

        # 上传参考文件
        browser.upload("参考文件", "./test/fixtures/reference.jpg")

        browser.click("发布")

        # 验证发布成功
        assert browser.snapshot("发布成功")

    def test_publish_task_validation_empty_fields(self, browser, business_user_with_balance):
        """测试发布任务表单验证：空字段"""
        browser.click("发布任务")
        browser.click("发布")

        # 验证错误提示
        assert browser.snapshot("请填写任务标题")

    def test_publish_task_validation_insufficient_balance(self, browser):
        """测试发布任务：余额不足"""
        # 创建无余额的商家用户
        username = f"business_{int(time.time())}"
        password = "test123456"
        phone = f"138{random.randint(10000000, 99999999)}"

        browser.open("/register")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.fill("确认密码", password)
        browser.fill("手机号", phone)
        browser.click("注册")

        browser.open("/login")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.select("登录身份", "商家")
        browser.click("登录")

        # 尝试发布任务
        browser.click("发布任务")
        browser.fill("任务标题", "测试任务")
        browser.fill("任务描述", "测试")
        browser.select("任务类型", "设计")
        browser.fill("单价", "100")
        browser.fill("需求数量", "5")
        browser.click("发布")

        # 验证错误提示
        assert browser.snapshot("余额不足")

    def test_publish_task_frozen_amount(self, browser, business_user_with_balance):
        """测试发布任务后冻结金额"""
        # 记录发布前的余额和冻结金额
        browser.click("钱包")
        balance_before = float(browser.get_text("可用余额"))
        frozen_before = float(browser.get_text("冻结金额"))

        # 发布任务
        browser.click("发布任务")
        browser.fill("任务标题", f"测试任务_{int(time.time())}")
        browser.fill("任务描述", "测试")
        browser.select("任务类型", "设计")
        browser.fill("单价", "100")
        browser.fill("需求数量", "5")
        browser.click("发布")

        # 验证冻结金额增加
        browser.click("钱包")
        balance_after = float(browser.get_text("可用余额"))
        frozen_after = float(browser.get_text("冻结金额"))

        assert balance_after == balance_before - 500
        assert frozen_after == frozen_before + 500


class TestBusinessTaskManagement:
    """商家任务管理测试"""

@pytest.fixture
    def business_with_task(self, browser):
        """创建有任务的商家用户"""
        username = f"business_{int(time.time())}"
        password = "test123456"
        phone = f"138{random.randint(10000000, 99999999)}"

        # 注册、登录、充值
        browser.open("/register")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.fill("确认密码", password)
        browser.fill("手机号", phone)
        browser.click("注册")

        browser.open("/login")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.select("登录身份", "商家")
        browser.click("登录")

        browser.click("钱包")
        browser.click("充值")
        browser.fill("充值金额", "10000")
        browser.click("确认充值")
        browser.wait(2)

        # 发布任务
        task_title = f"测试任务_{int(time.time())}"
        browser.click("发布任务")
        browser.fill("任务标题", task_title)
        browser.fill("任务描述", "测试")
        browser.select("任务类型", "设计")
        browser.fill("单价", "100")
        browser.fill("需求数量", "5")
        browser.click("发布")

        return {"username": username, "password": password, "task_title": task_title}

    def test_view_task_list(self, browser, business_with_task):
        """测试查看任务列表"""
        browser.click("我的任务")

        # 验证任务列表显示
        assert browser.snapshot(business_with_task["task_title"])
        assert browser.snapshot("设计")
        assert browser.snapshot("100.00")

    def test_view_task_detail(self, browser, business_with_task):
        """测试查看任务详情"""
        browser.click("我的任务")
        browser.click(business_with_task["task_title"])

        # 验证任务详情
        assert browser.snapshot("任务详情")
        assert browser.snapshot(business_with_task["task_title"])
        assert browser.snapshot("单价：100.00")
        assert browser.snapshot("需求数量：5")

    def test_filter_tasks_by_status(self, browser, business_with_task):
        """测试按状态筛选任务"""
        browser.click("我的任务")

        # 筛选进行中的任务
        browser.select("任务状态", "进行中")
        assert browser.snapshot(business_with_task["task_title"])

        # 筛选已完成的任务
        browser.select("任务状态", "已完成")
        assert not browser.snapshot(business_with_task["task_title"])

    def test_search_tasks(self, browser, business_with_task):
        """测试搜索任务"""
        browser.click("我的任务")

        # 搜索任务
        browser.fill("搜索", business_with_task["task_title"])
        browser.click("搜索按钮")

        # 验证搜索结果
        assert browser.snapshot(business_with_task["task_title"])
