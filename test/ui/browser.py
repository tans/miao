"""
agent-browser CLI 封装类
"""
import subprocess
import json
import os
from typing import Optional, Dict, Any


class Browser:
    """agent-browser CLI 封装"""

    def __init__(self, base_url: str = "http://localhost:8888", session: str = None):
        self.base_url = base_url
        # 使用唯一的 session ID 避免测试间干扰
        if session is None:
            import time
            session = f"test_{int(time.time() * 1000)}"
        self.session = session
        self.agent_browser = "/opt/homebrew/bin/agent-browser"

    def _run(self, *args, json_output: bool = False) -> str:
        """执行 agent-browser 命令"""
        cmd = [self.agent_browser, "--session", self.session]
        if json_output:
            cmd.append("--json")
        cmd.extend(args)

        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=30
        )

        if result.returncode != 0:
            raise Exception(f"Command failed: {' '.join(cmd)}\nError: {result.stderr}")

        return result.stdout.strip()

    def open(self, path: str = "/"):
        """打开页面"""
        url = f"{self.base_url}{path}"
        return self._run("open", url)

    def click(self, selector: str):
        """点击元素（支持文本标签或CSS选择器）"""
        # 如果是简单的中文文本，使用 find role button 方式
        if not selector.startswith(("@", "#", ".", "[")) and any('\u4e00' <= c <= '\u9fff' for c in selector):
            return self._run("find", "role", "button", "click", "--name", selector)
        return self._run("click", selector)

    def fill(self, selector: str, text: str):
        """填充输入框（支持文本标签或CSS选择器）"""
        # 如果是简单的中文文本，使用 find label 方式
        if not selector.startswith(("@", "#", ".", "[")) and any('\u4e00' <= c <= '\u9fff' for c in selector):
            return self._run("find", "label", selector, "fill", text)
        return self._run("fill", selector, text)

    def type(self, selector: str, text: str):
        """输入文本"""
        return self._run("type", selector, text)

    def wait(self, selector_or_ms: str):
        """等待元素或时间"""
        return self._run("wait", selector_or_ms)

    def get_text(self, selector: str) -> str:
        """获取元素文本"""
        return self._run("get", "text", selector)

    def get_value(self, selector: str) -> str:
        """获取输入框值"""
        return self._run("get", "value", selector)

    def get_url(self) -> str:
        """获取当前URL"""
        return self._run("get", "url")

    def is_visible(self, selector: str) -> bool:
        """检查元素是否可见"""
        try:
            result = self._run("is", "visible", selector)
            return "true" in result.lower()
        except:
            return False

    def screenshot(self, path: str):
        """截图"""
        os.makedirs(os.path.dirname(path), exist_ok=True)
        return self._run("screenshot", path)

    def snapshot(self, interactive: bool = True) -> str:
        """获取页面快照"""
        args = ["snapshot"]
        if interactive:
            args.append("-i")
        return self._run(*args)

    def find_and_click(self, role: str, name: str):
        """通过角色和名称查找并点击"""
        return self._run("find", "role", role, "click", "--name", name)

    def select(self, selector: str, value: str):
        """选择下拉框选项"""
        return self._run("select", selector, value)

    def check(self, selector: str):
        """勾选复选框"""
        return self._run("check", selector)

    def uncheck(self, selector: str):
        """取消勾选复选框"""
        return self._run("uncheck", selector)

    def close(self):
        """关闭浏览器"""
        return self._run("close")

    def eval(self, js: str) -> str:
        """执行JavaScript"""
        return self._run("eval", js)
