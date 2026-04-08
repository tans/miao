"""
pytest配置文件
"""
import pytest
import os
import shutil
from datetime import datetime
from browser import Browser


def pytest_configure(config):
    """pytest配置"""
    # 创建截图目录
    os.makedirs("./test/screenshots", exist_ok=True)
    os.makedirs("./test/screenshots/baseline", exist_ok=True)
    os.makedirs("./test/screenshots/current", exist_ok=True)

    # 创建测试报告目录
    os.makedirs("./test/reports", exist_ok=True)

    # 创建测试fixtures目录
    os.makedirs("./test/fixtures", exist_ok=True)


def pytest_collection_modifyitems(items):
    """修改测试用例收集"""
    for item in items:
        # 为所有UI测试添加标记和超时
        if "test/ui/" in str(item.fspath):
            item.add_marker(pytest.mark.ui)
            # 为每个测试添加30秒超时
            item.add_marker(pytest.mark.timeout(30))

        # 为视觉回归测试添加标记
        if "visual" in item.nodeid.lower():
            item.add_marker(pytest.mark.visual)

        # 为完整流程测试添加标记
        if "full_workflow" in item.nodeid.lower():
            item.add_marker(pytest.mark.e2e)
            # 完整流程测试允许更长时间
            item.add_marker(pytest.mark.timeout(60))


@pytest.fixture(scope="session")
def test_server():
    """启动测试服务器"""
    import subprocess
    import time

    # 启动服务器
    server_process = subprocess.Popen(
        ["./miao-server"],
        env={**os.environ, "GIN_MODE": "test", "SERVER_PORT": "8888"},
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE
    )

    # 等待服务器启动
    time.sleep(3)

    yield server_process

    # 关闭服务器
    server_process.terminate()
    server_process.wait()


@pytest.fixture(scope="function")
def browser():
    """浏览器实例"""
    # 不指定session，让Browser类自动生成唯一session
    b = Browser(base_url="http://localhost:8888")
    yield b
    try:
        b.close()
    except:
        pass


@pytest.fixture(scope="function")
def clean_database():
    """清理测试数据库"""
    import sqlite3

    # 测试前备份数据库
    if os.path.exists("./data/miao.db"):
        shutil.copy("./data/miao.db", "./data/miao.db.backup")

    yield

    # 测试后恢复数据库
    if os.path.exists("./data/miao.db.backup"):
        shutil.copy("./data/miao.db.backup", "./data/miao.db")
        os.remove("./data/miao.db.backup")


@pytest.fixture(scope="function")
def screenshot_on_failure(request):
    """测试失败时自动截图"""
    yield

    if request.node.rep_call.failed:
        # 获取浏览器实例
        browser = request.getfixturevalue("browser")

        # 生成截图文件名
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        test_name = request.node.name
        screenshot_path = f"./test/screenshots/failures/{test_name}_{timestamp}.png"

        # 保存截图
        os.makedirs("./test/screenshots/failures", exist_ok=True)
        browser.screenshot(screenshot_path)

        print(f"\n截图已保存: {screenshot_path}")


@pytest.hookimpl(tryfirst=True, hookwrapper=True)
def pytest_runtest_makereport(item, call):
    """生成测试报告"""
    outcome = yield
    rep = outcome.get_result()
    setattr(item, f"rep_{rep.when}", rep)


def pytest_html_report_title(report):
    """自定义HTML报告标题"""
    report.title = "创意喵平台 UI 自动化测试报告"


def pytest_html_results_summary(prefix, summary, postfix):
    """自定义HTML报告摘要"""
    prefix.extend([
        "<h2>测试环境</h2>",
        "<p>服务器地址: http://localhost:8888</p>",
        f"<p>测试时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}</p>",
    ])
