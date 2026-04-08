"""
用户登录和角色切换测试
"""
import time
import random
from agent_browser import Browser
import pytest


class TestUserLogin:
    """用户登录测试"""

    @pytest.fixture
    def browser(self):
        """浏览器fixture"""
        browser = Browser(headless=False)
        browser.goto("http://localhost:8888")
        yield browser
        browser.close()

    @pytest.fixture
    def test_user(self, browser):
        """创建测试用户"""
        username = f"test_user_{int(time.time())}"
        password = "test123456"
        phone = f"138{random.randint(10000000, 99999999)}"

        # 注册用户（默认拥有商家和创作者双角色）
        browser.goto("http://localhost:8888/register")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.fill("确认密码", password)
        browser.fill("手机号", phone)
        browser.click("注册")

        return {"username": username, "password": password}

    def test_login_as_business(self, browser, test_user):
        """测试以商家身份登录"""
        browser.goto("http://localhost:8888/login")

        # 填写登录表单
        browser.fill("用户名", test_user["username"])
        browser.fill("密码", test_user["password"])
        browser.select("登录身份", "商家")

        # 提交登录
        browser.click("登录")

        # 验证
        assert browser.has_text("商家工作台")
        assert browser.has_element("导航栏", text="商家")
        assert browser.current_url().endswith("/business/dashboard")

    def test_login_as_creator(self, browser, test_user):
        """测试以创作者身份登录"""
        browser.goto("http://localhost:8888/login")

        # 填写登录表单
        browser.fill("用户名", test_user["username"])
        browser.fill("密码", test_user["password"])
        browser.select("登录身份", "创作者")

        # 提交登录
        browser.click("登录")

        # 验证
        assert browser.has_text("创作者工作台")
        assert browser.has_element("导航栏", text="创作者")
        assert browser.current_url().endswith("/creator/dashboard")

    def test_login_validation_empty_fields(self, browser):
        """测试登录表单验证：空字段"""
        browser.goto("http://localhost:8888/login")
        browser.click("登录")

        # 验证错误提示
        assert browser.has_text("请填写用户名")

    def test_login_validation_wrong_password(self, browser, test_user):
        """测试登录表单验证：密码错误"""
        browser.goto("http://localhost:8888/login")

        browser.fill("用户名", test_user["username"])
        browser.fill("密码", "wrongpassword")
        browser.click("登录")

        # 验证错误提示
        assert browser.has_text("用户名或密码错误")

    def test_login_validation_nonexistent_user(self, browser):
        """测试登录表单验证：用户不存在"""
        browser.goto("http://localhost:8888/login")

        browser.fill("用户名", "nonexistent_user_12345")
        browser.fill("密码", "test123456")
        browser.click("登录")

        # 验证错误提示
        assert browser.has_text("用户名或密码错误")


class TestRoleSwitch:
    """角色切换测试"""

    @pytest.fixture
    def browser(self):
        """浏览器fixture"""
        browser = Browser(headless=False)
        browser.goto("http://localhost:8888")
        yield browser
        browser.close()

    @pytest.fixture
    def logged_in_user(self, browser):
        """登录的测试用户"""
        username = f"dual_role_{int(time.time())}"
        password = "test123456"
        phone = f"138{random.randint(10000000, 99999999)}"

        # 注册
        browser.goto("http://localhost:8888/register")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.fill("确认密码", password)
        browser.fill("手机号", phone)
        browser.click("注册")

        # 登录为商家
        browser.goto("http://localhost:8888/login")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.select("登录身份", "商家")
        browser.click("登录")

        return {"username": username, "password": password}

    def test_switch_from_business_to_creator(self, browser, logged_in_user):
        """测试从商家切换到创作者"""
        # 当前在商家工作台
        assert browser.has_text("商家工作台")

        # 点击角色标签
        browser.click("商家")

        # 选择创作者
        browser.click("创作者")

        # 验证切换成功
        assert browser.has_text("创作者工作台")
        assert browser.has_element("导航栏", text="创作者")
        assert browser.current_url().endswith("/creator/dashboard")

        # 验证导航栏变化
        assert browser.has_element("任务大厅")
        assert browser.has_element("我的认领")

    def test_switch_from_creator_to_business(self, browser, logged_in_user):
        """测试从创作者切换到商家"""
        # 先切换到创作者
        browser.click("商家")
        browser.click("创作者")
        assert browser.has_text("创作者工作台")

        # 切换回商家
        browser.click("创作者")
        browser.click("商家")

        # 验证切换成功
        assert browser.has_text("商家工作台")
        assert browser.has_element("导航栏", text="商家")
        assert browser.current_url().endswith("/business/dashboard")

        # 验证导航栏变化
        assert browser.has_element("发布任务")
        assert browser.has_element("我的任务")

    def test_role_switch_preserves_session(self, browser, logged_in_user):
        """测试角色切换保持会话"""
        # 切换到创作者
        browser.click("商家")
        browser.click("创作者")

        # 刷新页面
        browser.refresh()

        # 验证仍然是创作者身份
        assert browser.has_text("创作者工作台")
        assert browser.has_element("导航栏", text="创作者")

    def test_role_switch_updates_stats(self, browser, logged_in_user):
        """测试角色切换更新统计数据"""
        # 商家工作台统计
        business_stats = browser.get_text("统计数据")

        # 切换到创作者
        browser.click("商家")
        browser.click("创作者")

        # 创作者工作台统计
        creator_stats = browser.get_text("统计数据")

        # 验证统计数据不同
        assert business_stats != creator_stats
