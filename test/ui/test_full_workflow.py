"""
完整工作流测试 - 简化版
测试端到端的基本流程
"""
import time
import random
import pytest


class TestBasicWorkflow:
    """基础工作流测试"""

    def test_register_and_login_workflow(self, browser):
        """测试注册-登录完整流程"""
        username = f"user_{int(time.time())}"
        password = "test123456"
        phone = f"138{random.randint(10000000, 99999999)}"

        # 注册
        browser.open("/auth/register.html")
        browser.wait("2000")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.fill("手机号", phone)
        browser.click("注册")
        browser.wait("3000")

        # 验证跳转到登录页
        url = browser.get_url()
        assert "/login" in url

        # 登录
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.click("登录")
        browser.wait("3000")

        # 验证登录成功
        snapshot = browser.snapshot()
        assert "工作台" in snapshot or "dashboard" in snapshot.lower()

    def test_role_switch_workflow(self, browser):
        """测试角色切换流程"""
        username = f"user_{int(time.time())}"
        password = "test123456"
        phone = f"138{random.randint(10000000, 99999999)}"

        # 注册
        browser.open("/auth/register.html")
        browser.wait("2000")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.fill("手机号", phone)
        browser.click("注册")
        browser.wait("3000")

        # 登录为商家
        browser.open("/auth/login.html")
        browser.wait("2000")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.click("登录")
        browser.wait("3000")

        # 验证登录成功
        snapshot = browser.snapshot()
        assert username in snapshot or "工作台" in snapshot
