"""
视觉回归测试和页面可访问性测试
"""
import time
from test.ui.browser import Browser
import pytest
from PIL import Image
import imagehash


class TestVisualRegression:
    """视觉回归测试"""

    @pytest.fixture
    def browser(self):
        """浏览器fixture"""
        browser = Browser(headless=True)
        yield browser
        browser.close()

    def compare_screenshots(self, current_path, baseline_path, threshold=5):
        """比较两张截图的相似度"""
        try:
            current_img = Image.open(current_path)
            baseline_img = Image.open(baseline_path)

            current_hash = imagehash.average_hash(current_img)
            baseline_hash = imagehash.average_hash(baseline_img)

            diff = current_hash - baseline_hash
            return diff <= threshold
        except FileNotFoundError:
            # 基准图不存在，保存当前图作为基准
            current_img = Image.open(current_path)
            current_img.save(baseline_path)
            return True

    def test_homepage_visual(self, browser):
        """测试首页视觉"""
        browser.open("/")
        screenshot = browser.screenshot("./test/screenshots/current_homepage.png")

        assert self.compare_screenshots(
            "./test/screenshots/current_homepage.png",
            "./test/screenshots/baseline_homepage.png"
        ), "首页视觉发生变化"

    def test_login_page_visual(self, browser):
        """测试登录页面视觉"""
        browser.open("/login")
        screenshot = browser.screenshot("./test/screenshots/current_login.png")

        assert self.compare_screenshots(
            "./test/screenshots/current_login.png",
            "./test/screenshots/baseline_login.png"
        ), "登录页面视觉发生变化"

    def test_register_page_visual(self, browser):
        """测试注册页面视觉"""
        browser.open("/register")
        screenshot = browser.screenshot("./test/screenshots/current_register.png")

        assert self.compare_screenshots(
            "./test/screenshots/current_register.png",
            "./test/screenshots/baseline_register.png"
        ), "注册页面视觉发生变化"

    def test_business_dashboard_visual(self, browser):
        """测试商家工作台视觉"""
        # 登录商家账号
        browser.open("/login")
        browser.fill("用户名", "test_business")
        browser.fill("密码", "test123456")
        browser.select("登录身份", "商家")
        browser.click("登录")

        screenshot = browser.screenshot("./test/screenshots/current_business_dashboard.png")

        assert self.compare_screenshots(
            "./test/screenshots/current_business_dashboard.png",
            "./test/screenshots/baseline_business_dashboard.png"
        ), "商家工作台视觉发生变化"

    def test_creator_dashboard_visual(self, browser):
        """测试创作者工作台视觉"""
        # 登录创作者账号
        browser.open("/login")
        browser.fill("用户名", "test_creator")
        browser.fill("密码", "test123456")
        browser.select("登录身份", "创作者")
        browser.click("登录")

        screenshot = browser.screenshot("./test/screenshots/current_creator_dashboard.png")

        assert self.compare_screenshots(
            "./test/screenshots/current_creator_dashboard.png",
            "./test/screenshots/baseline_creator_dashboard.png"
        ), "创作者工作台视觉发生变化"


class TestPageAccessibility:
    """页面可访问性测试"""

def test_homepage_loads(self, browser):
        """测试首页加载"""
        assert browser.current_url() == "http://localhost:8888/"
        assert browser.snapshot("创意喵")

    def test_login_page_loads(self, browser):
        """测试登录页面加载"""
        browser.open("/login")
        assert browser.snapshot("登录")
        assert browser.is_visible("用户名")
        assert browser.is_visible("密码")

    def test_register_page_loads(self, browser):
        """测试注册页面加载"""
        browser.open("/register")
        assert browser.snapshot("注册")
        assert browser.is_visible("用户名")
        assert browser.is_visible("密码")
        assert browser.is_visible("确认密码")

    def test_404_page_loads(self, browser):
        """测试404页面"""
        browser.open("/nonexistent")
        assert browser.snapshot("404") or browser.snapshot("页面不存在")

    def test_all_navigation_links(self, browser):
        """测试所有导航链接可访问"""
        # 登录
        browser.open("/login")
        browser.fill("用户名", "test_business")
        browser.fill("密码", "test123456")
        browser.select("登录身份", "商家")
        browser.click("登录")

        # 测试商家导航链接
        navigation_links = [
            "工作台",
            "发布任务",
            "我的任务",
            "钱包",
            "个人中心"
        ]

        for link in navigation_links:
            browser.click(link)
            assert browser.current_url() != "http://localhost:8888/404"

    def test_responsive_design_mobile(self, browser):
        """测试移动端响应式设计"""
        browser.set_viewport(375, 667)  # iPhone SE
        browser.open("/")

        # 验证移动端布局
        assert browser.is_visible("导航菜单")

    def test_responsive_design_tablet(self, browser):
        """测试平板端响应式设计"""
        browser.set_viewport(768, 1024)  # iPad
        browser.open("/")

        # 验证平板端布局
        assert browser.is_visible("导航栏")

    def test_responsive_design_desktop(self, browser):
        """测试桌面端响应式设计"""
        browser.set_viewport(1920, 1080)  # Desktop
        browser.open("/")

        # 验证桌面端布局
        assert browser.is_visible("导航栏")


class TestFormValidation:
    """表单验证测试"""

def test_login_form_validation(self, browser):
        """测试登录表单验证"""
        browser.open("/login")

        # 测试空表单提交
        browser.click("登录")
        assert browser.snapshot("请填写用户名") or browser.is_visible("input:invalid")

        # 测试只填写用户名
        browser.fill("用户名", "test")
        browser.click("登录")
        assert browser.snapshot("请填写密码") or browser.is_visible("input:invalid")

    def test_register_form_validation(self, browser):
        """测试注册表单验证"""
        browser.open("/register")

        # 测试密码不一致
        browser.fill("用户名", "test")
        browser.fill("密码", "password1")
        browser.fill("确认密码", "password2")
        browser.click("注册")
        assert browser.snapshot("两次密码不一致")

        # 测试手机号格式
        browser.fill("确认密码", "password1")
        browser.fill("手机号", "123")
        browser.click("注册")
        assert browser.snapshot("手机号格式不正确")

    def test_task_form_validation(self, browser):
        """测试任务发布表单验证"""
        # 登录商家
        browser.open("/login")
        browser.fill("用户名", "test_business")
        browser.fill("密码", "test123456")
        browser.select("登录身份", "商家")
        browser.click("登录")

        # 进入发布任务页面
        browser.click("发布任务")

        # 测试空表单
        browser.click("发布")
        assert browser.snapshot("请填写任务标题")

        # 测试价格验证
        browser.fill("任务标题", "测试")
        browser.fill("单价", "-10")
        browser.click("发布")
        assert browser.snapshot("单价必须大于0")

        # 测试数量验证
        browser.fill("单价", "100")
        browser.fill("需求数量", "0")
        browser.click("发布")
        assert browser.snapshot("需求数量必须大于0")


class TestErrorHandling:
    """错误处理测试"""

def test_network_error_handling(self, browser):
        """测试网络错误处理"""
        # 模拟网络断开
        browser.set_offline(True)

        browser.open("/login")
        browser.fill("用户名", "test")
        browser.fill("密码", "test123456")
        browser.click("登录")

        # 验证错误提示
        assert browser.snapshot("网络错误") or browser.snapshot("请检查网络连接")

        # 恢复网络
        browser.set_offline(False)

    def test_session_timeout_handling(self, browser):
        """测试会话超时处理"""
        # 登录
        browser.open("/login")
        browser.fill("用户名", "test_business")
        browser.fill("密码", "test123456")
        browser.click("登录")

        # 清除token模拟会话过期
        browser.execute_script("localStorage.removeItem('token')")

        # 尝试访问需要认证的页面
        browser.open("/business/tasks")

        # 验证跳转到登录页
        assert browser.current_url().endswith("/login")
        assert browser.snapshot("请先登录")

    def test_unauthorized_access(self, browser):
        """测试未授权访问"""
        # 未登录访问商家页面
        browser.open("/business/dashboard")

        # 验证跳转到登录页
        assert browser.current_url().endswith("/login")

    def test_wrong_role_access(self, browser):
        """测试错误角色访问"""
        # 以创作者身份登录
        browser.open("/login")
        browser.fill("用户名", "test_creator")
        browser.fill("密码", "test123456")
        browser.select("登录身份", "创作者")
        browser.click("登录")

        # 尝试访问商家页面
        browser.open("/business/tasks")

        # 验证错误提示或跳转
        assert browser.snapshot("权限不足") or browser.current_url().endswith("/creator/dashboard")
