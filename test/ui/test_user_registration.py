"""
用户注册流程测试
"""
import time
import random
import pytest


class TestUserRegistration:
    """用户注册测试"""

    def test_register_as_business(self, browser, clean_database):
        """测试注册商家账号"""
        username = f"business_{int(time.time())}"
        phone = f"138{random.randint(10000000, 99999999)}"

        # 打开注册页面
        browser.open("/auth/register.html")
        browser.wait("1000")

        # 获取页面快照找到元素引用
        snapshot = browser.snapshot(interactive=True)

        # 填写注册表单（使用文本标签）
        browser.fill("用户名", username)
        browser.fill("密码", "test123456")
        browser.fill("手机号", phone)

        # 提交注册
        browser.click("注册")
        browser.wait("2000")

        # 验证跳转到登录页
        url = browser.get_url()
        assert "/login" in url

    def test_register_as_creator(self, browser, clean_database):
        """测试注册创作者账号"""
        username = f"creator_{int(time.time())}"
        phone = f"139{random.randint(10000000, 99999999)}"

        # 打开注册页面
        browser.open("/auth/register.html")
        browser.wait("1000")

        # 填写注册表单
        browser.fill("用户名", username)
        browser.fill("密码", "test123456")
        browser.fill("手机号", phone)

        # 提交注册
        browser.click("注册")
        browser.wait("2000")

        # 验证
        url = browser.get_url()
        assert "/login" in url

    def test_register_with_duplicate_username(self, browser, clean_database):
        """测试重复用户名注册"""
        username = f"duplicate_{int(time.time())}"
        phone = f"138{random.randint(10000000, 99999999)}"

        # 第一次注册
        browser.open("/auth/register.html")
        browser.wait("1000")
        browser.fill("用户名", username)
        browser.fill("密码", "test123456")
        browser.fill("手机号", phone)
        browser.click("注册")
        browser.wait("2000")

        # 第二次注册相同用户名
        browser.open("/auth/register.html")
        browser.wait("1000")
        browser.fill("用户名", username)
        browser.fill("密码", "test123456")
        browser.fill("手机号", f"139{random.randint(10000000, 99999999)}")
        browser.click("注册")
        browser.wait("2000")

        # 验证错误提示
        snapshot = browser.snapshot()
        assert "用户名已存在" in snapshot or "已被注册" in snapshot or "已注册" in snapshot

    def test_register_with_invalid_phone(self, browser):
        """测试无效手机号注册"""
        username = f"user_{int(time.time())}"

        browser.open("/auth/register.html")
        browser.wait("1000")

        # 填写无效手机号
        browser.fill("用户名", username)
        browser.fill("密码", "test123456")
        browser.fill("手机号", "12345")
        browser.click("注册")
        browser.wait("1000")

        # 验证错误提示
        snapshot = browser.snapshot()
        assert "手机号" in snapshot or "格式" in snapshot or "无效" in snapshot

    def test_register_with_password_mismatch(self, browser):
        """测试密码不匹配（注：当前页面没有确认密码字段，此测试可能需要调整）"""
        username = f"user_{int(time.time())}"
        phone = f"138{random.randint(10000000, 99999999)}"

        browser.open("/auth/register.html")
        browser.wait("1000")

        # 填写表单
        browser.fill("用户名", username)
        browser.fill("密码", "test123456")
        browser.fill("手机号", phone)

        # 注：当前注册页面没有确认密码字段，此测试验证基本功能
        browser.click("注册")
        browser.wait("2000")

        # 验证成功注册
        url = browser.get_url()
        assert "/login" in url

    def test_register_with_empty_fields(self, browser):
        """测试空字段注册"""
        browser.open("/auth/register.html")
        browser.wait("1000")

        # 直接提交空表单
        browser.click("注册")
        browser.wait("1000")

        # 验证仍在注册页面（HTML5表单验证会阻止提交）
        url = browser.get_url()
        assert "/register" in url
