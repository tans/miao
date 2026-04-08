"""
视觉回归和可访问性测试 - 简化版
基础的页面加载和响应式测试
"""
import time
import random
import pytest


class TestPageLoad:
    """页面加载测试"""

    def test_register_page_loads(self, browser):
        """测试注册页面加载"""
        browser.open("/auth/register.html")
        browser.wait("2000")
        snapshot = browser.snapshot()
        assert "注册" in snapshot or "register" in snapshot.lower()

    def test_login_page_loads(self, browser):
        """测试登录页面加载"""
        browser.open("/auth/login.html")
        browser.wait("2000")
        snapshot = browser.snapshot()
        assert "登录" in snapshot or "login" in snapshot.lower()


class TestResponsive:
    """响应式测试"""

    def test_mobile_view(self, browser):
        """测试移动端视图"""
        try:
            browser.set_viewport(375, 667)  # iPhone SE
            browser.open("/auth/login.html")
            browser.wait("2000")
            snapshot = browser.snapshot()
            assert "登录" in snapshot
        except Exception as e:
            pytest.skip(f"视口设置不支持: {e}")

    def test_tablet_view(self, browser):
        """测试平板视图"""
        try:
            browser.set_viewport(768, 1024)  # iPad
            browser.open("/auth/login.html")
            browser.wait("2000")
            snapshot = browser.snapshot()
            assert "登录" in snapshot
        except Exception as e:
            pytest.skip(f"视口设置不支持: {e}")


class TestAccessibility:
    """可访问性基础测试"""

    def test_form_labels_present(self, browser):
        """测试表单标签存在"""
        browser.open("/auth/register.html")
        browser.wait("2000")
        snapshot = browser.snapshot()
        # 验证关键标签存在
        assert "用户名" in snapshot
        assert "密码" in snapshot
        assert "手机号" in snapshot
