"""
用户注册流程测试
"""
import time
import random
from agent_browser import Browser
import pytest


class TestUserRegistration:
    """用户注册测试"""

    @pytest.fixture
    def browser(self):
        """浏览器fixture"""
        browser = Browser(headless=False)
        browser.goto("http://localhost:8888")
        yield browser
        browser.close()

    def test_register_as_business(self, browser):
        """测试注册商家账号"""
        username = f"business_{int(time.time())}"
        phone = f"138{random.randint(10000000, 99999999)}"

        # 进入注册页面
        browser.click("注册")

        # 填写注册表单
        browser.fill("用户名", username)
        browser.fill("密码", "test123456")
        browser.fill("确认密码", "test123456")
        browser.fill("手机号", phone)
        browser.select("登录身份", "商家")

        # 提交注册
        browser.click("注册")

        # 验证
        assert browser.has_text("注册成功")
        assert browser.current_url().endswith("/login")

    def test_register_as_creator(self, browser):
        """测试注册创作者账号"""
        username = f"creator_{int(time.time())}"
        phone = f"139{random.randint(10000000, 99999999)}"

        # 进入注册页面
        browser.click("注册")

        # 填写注册表单
        browser.fill("用户名", username)
        browser.fill("密码", "test123456")
        browser.fill("确认密码", "test123456")
        browser.fill("手机号", phone)
        browser.select("登录身份", "创作者")

        # 提交注册
        browser.click("注册")

        # 验证
        assert browser.has_text("注册成功")
        assert browser.current_url().endswith("/login")

    def test_register_validation_empty_fields(self, browser):
        """测试注册表单验证：空字段"""
        browser.click("注册")
        browser.click("注册")

        # 验证错误提示
        assert browser.has_text("请填写用户名")

    def test_register_validation_password_mismatch(self, browser):
        """测试注册表单验证：密码不一致"""
        browser.click("注册")

        browser.fill("用户名", f"test_{int(time.time())}")
        browser.fill("密码", "test123456")
        browser.fill("确认密码", "different123")
        browser.fill("手机号", f"138{random.randint(10000000, 99999999)}")

        browser.click("注册")

        # 验证错误提示
        assert browser.has_text("两次密码不一致")

    def test_register_validation_duplicate_username(self, browser):
        """测试注册表单验证：用户名重复"""
        username = f"duplicate_{int(time.time())}"
        phone = f"138{random.randint(10000000, 99999999)}"

        # 第一次注册
        browser.click("注册")
        browser.fill("用户名", username)
        browser.fill("密码", "test123456")
        browser.fill("确认密码", "test123456")
        browser.fill("手机号", phone)
        browser.click("注册")

        # 第二次注册相同用户名
        browser.goto("http://localhost:8888/register")
        browser.fill("用户名", username)
        browser.fill("密码", "test123456")
        browser.fill("确认密码", "test123456")
        browser.fill("手机号", f"139{random.randint(10000000, 99999999)}")
        browser.click("注册")

        # 验证错误提示
        assert browser.has_text("用户名已存在")

    def test_register_validation_invalid_phone(self, browser):
        """测试注册表单验证：手机号格式错误"""
        browser.click("注册")

        browser.fill("用户名", f"test_{int(time.time())}")
        browser.fill("密码", "test123456")
        browser.fill("确认密码", "test123456")
        browser.fill("手机号", "12345")  # 无效手机号

        browser.click("注册")

        # 验证错误提示
        assert browser.has_text("手机号格式不正确")
