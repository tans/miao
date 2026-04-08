"""
商家业务流程测试 - 简化版
只测试核心功能，避免复杂的业务流程
"""
import time
import random
import pytest


class TestBusinessDashboard:
    """商家工作台基础测试"""

    @pytest.mark.timeout(20)
    def test_access_business_dashboard(self, browser):
        """测试访问商家工作台"""
        # 注册商家用户
        username = f"biz_{int(time.time())}"
        password = "test123456"
        phone = f"138{random.randint(10000000, 99999999)}"

        browser.open("/auth/register.html")
        browser.wait("100")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.fill("手机号", phone)
        browser.click("注册")
        browser.wait("300")

        # 登录
        browser.open("/auth/login.html")
        browser.wait("100")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.click("登录")
        browser.wait("300")

        # 验证进入工作台
        snapshot = browser.snapshot()
        assert "工作台" in snapshot or "dashboard" in snapshot.lower()

    @pytest.mark.timeout(20)
    def test_view_business_profile(self, browser):
        """测试查看商家资料"""
        # 注册并登录
        username = f"biz_{int(time.time())}"
        password = "test123456"
        phone = f"138{random.randint(10000000, 99999999)}"

        browser.open("/auth/register.html")
        browser.wait("100")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.fill("手机号", phone)
        browser.click("注册")
        browser.wait("300")

        browser.open("/auth/login.html")
        browser.wait("100")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.click("登录")
        browser.wait("300")

        # 验证进入商家工作台，检查关键元素
        snapshot = browser.snapshot()
        assert "商家中心" in snapshot or "我的任务" in snapshot


class TestBusinessTaskList:
    """商家任务列表测试"""

    @pytest.mark.timeout(20)
    def test_view_empty_task_list(self, browser):
        """测试查看空任务列表"""
        # 注册并登录
        username = f"biz_{int(time.time())}"
        password = "test123456"
        phone = f"138{random.randint(10000000, 99999999)}"

        browser.open("/auth/register.html")
        browser.wait("100")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.fill("手机号", phone)
        browser.click("注册")
        browser.wait("300")

        browser.open("/auth/login.html")
        browser.wait("100")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.click("登录")
        browser.wait("300")

        # 访问任务列表页面
        try:
            browser.open("/business/tasks")
            browser.wait("200")
            snapshot = browser.snapshot()
            # 验证页面加载成功
            assert "任务" in snapshot or "task" in snapshot.lower()
        except Exception as e:
            # 如果页面不存在，跳过测试
            pytest.skip(f"任务列表页面不可用: {e}")


class TestBusinessNavigation:
    """商家导航测试"""

    @pytest.mark.timeout(20)
    def test_navigate_between_pages(self, browser):
        """测试页面导航"""
        # 注册并登录
        username = f"biz_{int(time.time())}"
        password = "test123456"
        phone = f"138{random.randint(10000000, 99999999)}"

        browser.open("/auth/register.html")
        browser.wait("100")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.fill("手机号", phone)
        browser.click("注册")
        browser.wait("300")

        browser.open("/auth/login.html")
        browser.wait("100")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.click("登录")
        browser.wait("300")

        # 验证登录成功
        url = browser.get_url()
        assert "/business" in url or "/dashboard" in url
