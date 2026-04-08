"""
创作者业务流程测试
"""
import time
import random
from browser import Browser
import pytest


class TestCreatorBrowseTasks:
    """创作者浏览任务测试"""

    @pytest.fixture
    def browser(self):
        """浏览器fixture"""
        browser = Browser()
        browser.open("/")
        yield browser
        browser.close()

    @pytest.fixture
    def creator_user(self, browser):
        """创建并登录创作者用户"""
        username = f"creator_{int(time.time())}"
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
        browser.select("登录身份", "创作者")
        browser.click("登录")

        return {"username": username, "password": password}

    def test_view_task_hall(self, browser, creator_user):
        """测试查看任务大厅"""
        browser.click("任务大厅")

        # 验证任务列表显示
        assert browser.snapshot("任务大厅")
        assert browser.is_visible("任务列表")

    def test_filter_tasks_by_type(self, browser, creator_user):
        """测试按类型筛选任务"""
        browser.click("任务大厅")

        # 筛选设计类任务
        browser.select("任务类型", "设计")

        # 验证只显示设计类任务
        assert browser.snapshot("设计")

    def test_sort_tasks_by_price(self, browser, creator_user):
        """测试按价格排序任务"""
        browser.click("任务大厅")

        # 按价格从高到低排序
        browser.select("排序方式", "价格从高到低")

        # 验证排序正确（第一个任务价格最高）
        prices = browser.get_all_text("任务价格")
        assert prices[0] >= prices[1] if len(prices) > 1 else True

    def test_search_tasks(self, browser, creator_user):
        """测试搜索任务"""
        browser.click("任务大厅")

        # 搜索关键词
        browser.fill("搜索", "设计")
        browser.click("搜索按钮")

        # 验证搜索结果
        assert browser.snapshot("设计")

    def test_view_task_detail(self, browser, creator_user):
        """测试查看任务详情"""
        browser.click("任务大厅")

        # 点击第一个任务
        browser.click_first("任务卡片")

        # 验证任务详情页
        assert browser.snapshot("任务详情")
        assert browser.snapshot("任务要求")
        assert browser.snapshot("单价")
        assert browser.snapshot("剩余数量")
        assert browser.is_visible("认领任务")


class TestCreatorClaimTask:
    """创作者认领任务测试"""

    @pytest.fixture
    def browser(self):
        """浏览器fixture"""
        browser = Browser()
        browser.open("/")
        yield browser
        browser.close()

    @pytest.fixture
    def creator_with_available_task(self, browser):
        """创建创作者和可认领的任务"""
        # 创建商家并发布任务
        business_username = f"business_{int(time.time())}"
        business_password = "test123456"
        business_phone = f"138{random.randint(10000000, 99999999)}"

        browser.open("/register")
        browser.fill("用户名", business_username)
        browser.fill("密码", business_password)
        browser.fill("确认密码", business_password)
        browser.fill("手机号", business_phone)
        browser.click("注册")

        browser.open("/login")
        browser.fill("用户名", business_username)
        browser.fill("密码", business_password)
        browser.select("登录身份", "商家")
        browser.click("登录")

        browser.click("钱包")
        browser.click("充值")
        browser.fill("充值金额", "10000")
        browser.click("确认充值")
        browser.wait(2)

        task_title = f"测试任务_{int(time.time())}"
        browser.click("发布任务")
        browser.fill("任务标题", task_title)
        browser.fill("任务描述", "测试")
        browser.select("任务类型", "设计")
        browser.fill("单价", "100")
        browser.fill("需求数量", "5")
        browser.click("发布")

        # 登出商家
        browser.click("退出登录")

        # 创建创作者
        creator_username = f"creator_{int(time.time())}"
        creator_password = "test123456"
        creator_phone = f"139{random.randint(10000000, 99999999)}"

        browser.open("/register")
        browser.fill("用户名", creator_username)
        browser.fill("密码", creator_password)
        browser.fill("确认密码", creator_password)
        browser.fill("手机号", creator_phone)
        browser.click("注册")

        browser.open("/login")
        browser.fill("用户名", creator_username)
        browser.fill("密码", creator_password)
        browser.select("登录身份", "创作者")
        browser.click("登录")

        return {
            "creator_username": creator_username,
            "creator_password": creator_password,
            "task_title": task_title
        }

    def test_claim_task_success(self, browser, creator_with_available_task):
        """测试认领任务成功"""
        browser.click("任务大厅")

        # 找到并点击任务
        browser.click(creator_with_available_task["task_title"])

        # 点击认领按钮
        browser.click("认领任务")

        # 确认认领
        browser.click("确认")

        # 验证认领成功
        assert browser.snapshot("认领成功")
        assert browser.is_visible("按钮", text="已认领")

    def test_claim_task_appears_in_my_claims(self, browser, creator_with_available_task):
        """测试认领的任务出现在我的认领中"""
        browser.click("任务大厅")
        browser.click(creator_with_available_task["task_title"])
        browser.click("认领任务")
        browser.click("确认")

        # 进入我的认领
        browser.click("我的认领")

        # 验证任务出现在列表中
        assert browser.snapshot(creator_with_available_task["task_title"])

    def test_cannot_claim_same_task_twice(self, browser, creator_with_available_task):
        """测试不能重复认领同一任务"""
        browser.click("任务大厅")
        browser.click(creator_with_available_task["task_title"])

        # 第一次认领
        browser.click("认领任务")
        browser.click("确认")

        # 尝试第二次认领
        browser.refresh()
        assert browser.is_visible("按钮", text="已认领")
        assert not browser.is_visible("按钮", text="认领任务")

    def test_claim_task_remaining_count_decreases(self, browser, creator_with_available_task):
        """测试认领任务后剩余数量减少"""
        browser.click("任务大厅")
        browser.click(creator_with_available_task["task_title"])

        # 记录认领前的剩余数量
        remaining_before = int(browser.get_text("剩余数量"))

        # 认领任务
        browser.click("认领任务")
        browser.click("确认")

        # 刷新页面
        browser.refresh()

        # 验证剩余数量减少
        remaining_after = int(browser.get_text("剩余数量"))
        assert remaining_after == remaining_before - 1


class TestCreatorSubmitWork:
    """创作者提交作品测试"""

    @pytest.fixture
    def browser(self):
        """浏览器fixture"""
        browser = Browser()
        browser.open("/")
        yield browser
        browser.close()

    @pytest.fixture
    def creator_with_claimed_task(self, browser):
        """创建已认领任务的创作者"""
        # 创建商家并发布任务
        business_username = f"business_{int(time.time())}"
        business_password = "test123456"
        business_phone = f"138{random.randint(10000000, 99999999)}"

        browser.open("/register")
        browser.fill("用户名", business_username)
        browser.fill("密码", business_password)
        browser.fill("确认密码", business_password)
        browser.fill("手机号", business_phone)
        browser.click("注册")

        browser.open("/login")
        browser.fill("用户名", business_username)
        browser.fill("密码", business_password)
        browser.select("登录身份", "商家")
        browser.click("登录")

        browser.click("钱包")
        browser.click("充值")
        browser.fill("充值金额", "10000")
        browser.click("确认充值")
        browser.wait(2)

        task_title = f"测试任务_{int(time.time())}"
        browser.click("发布任务")
        browser.fill("任务标题", task_title)
        browser.fill("任务描述", "测试")
        browser.select("任务类型", "设计")
        browser.fill("单价", "100")
        browser.fill("需求数量", "5")
        browser.click("发布")

        browser.click("退出登录")

        # 创建创作者并认领任务
        creator_username = f"creator_{int(time.time())}"
        creator_password = "test123456"
        creator_phone = f"139{random.randint(10000000, 99999999)}"

        browser.open("/register")
        browser.fill("用户名", creator_username)
        browser.fill("密码", creator_password)
        browser.fill("确认密码", creator_password)
        browser.fill("手机号", creator_phone)
        browser.click("注册")

        browser.open("/login")
        browser.fill("用户名", creator_username)
        browser.fill("密码", creator_password)
        browser.select("登录身份", "创作者")
        browser.click("登录")

        browser.click("任务大厅")
        browser.click(task_title)
        browser.click("认领任务")
        browser.click("确认")

        return {
            "creator_username": creator_username,
            "creator_password": creator_password,
            "business_username": business_username,
            "business_password": business_password,
            "task_title": task_title
        }

    def test_submit_work_success(self, browser, creator_with_claimed_task):
        """测试提交作品成功"""
        browser.click("我的认领")

        # 找到任务并点击提交
        browser.click(creator_with_claimed_task["task_title"])
        browser.click("提交作品")

        # 填写作品信息
        browser.fill("作品说明", "这是我的创意作品，已按要求完成。")
        browser.upload("作品文件", "./test/fixtures/work.jpg")

        # 提交
        browser.click("提交")

        # 验证提交成功
        assert browser.snapshot("提交成功")

    def test_submit_work_status_changes(self, browser, creator_with_claimed_task):
        """测试提交作品后状态变化"""
        browser.click("我的认领")
        browser.click(creator_with_claimed_task["task_title"])

        # 提交前状态
        assert browser.snapshot("待提交")

        # 提交作品
        browser.click("提交作品")
        browser.fill("作品说明", "测试作品")
        browser.upload("作品文件", "./test/fixtures/work.jpg")
        browser.click("提交")

        # 提交后状态
        browser.click("我的认领")
        browser.click(creator_with_claimed_task["task_title"])
        assert browser.snapshot("待审核")

    def test_submit_work_validation_empty_content(self, browser, creator_with_claimed_task):
        """测试提交作品验证：空内容"""
        browser.click("我的认领")
        browser.click(creator_with_claimed_task["task_title"])
        browser.click("提交作品")

        # 不填写任何内容直接提交
        browser.click("提交")

        # 验证错误提示
        assert browser.snapshot("请填写作品说明或上传作品文件")

    def test_cannot_submit_work_twice(self, browser, creator_with_claimed_task):
        """测试不能重复提交作品"""
        browser.click("我的认领")
        browser.click(creator_with_claimed_task["task_title"])

        # 第一次提交
        browser.click("提交作品")
        browser.fill("作品说明", "测试作品")
        browser.click("提交")

        # 尝试第二次提交
        browser.refresh()
        assert not browser.is_visible("按钮", text="提交作品")
        assert browser.snapshot("待审核")


class TestCreatorWallet:
    """创作者钱包测试"""

    @pytest.fixture
    def browser(self):
        """浏览器fixture"""
        browser = Browser()
        browser.open("/")
        yield browser
        browser.close()

    @pytest.fixture
    def creator_user(self, browser):
        """创建并登录创作者用户"""
        username = f"creator_{int(time.time())}"
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
        browser.select("登录身份", "创作者")
        browser.click("登录")

        return {"username": username, "password": password}

    def test_view_wallet(self, browser, creator_user):
        """测试查看钱包"""
        browser.click("钱包")

        # 验证钱包信息显示
        assert browser.snapshot("可用余额")
        assert browser.snapshot("冻结金额")
        assert browser.snapshot("总收益")

    def test_view_transaction_records(self, browser, creator_user):
        """测试查看交易记录"""
        browser.click("钱包")
        browser.click("收益明细")

        # 验证交易记录页面
        assert browser.snapshot("收益明细")
        assert browser.is_visible("交易记录列表")

    def test_filter_transactions_by_type(self, browser, creator_user):
        """测试按类型筛选交易记录"""
        browser.click("钱包")
        browser.click("收益明细")

        # 筛选收入记录
        browser.select("交易类型", "收入")

        # 验证只显示收入记录
        assert browser.snapshot("任务收益")
