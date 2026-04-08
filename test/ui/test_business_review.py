"""
商家审核作品测试
"""
import time
import random
from test.ui.browser import Browser
import pytest


class TestBusinessReviewSubmission:
    """商家审核作品测试"""

@pytest.fixture
    def business_with_submission(self, browser):
        """创建有待审核作品的商家"""
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

        # 创建创作者，认领并提交作品
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

        browser.click("我的认领")
        browser.click(task_title)
        browser.click("提交作品")
        browser.fill("作品说明", "这是我的创意作品")
        browser.upload("作品文件", "./test/fixtures/work.jpg")
        browser.click("提交")

        browser.click("退出登录")

        # 商家重新登录
        browser.open("/login")
        browser.fill("用户名", business_username)
        browser.fill("密码", business_password)
        browser.select("登录身份", "商家")
        browser.click("登录")

        return {
            "business_username": business_username,
            "business_password": business_password,
            "creator_username": creator_username,
            "creator_password": creator_password,
            "task_title": task_title
        }

    def test_view_pending_submissions(self, browser, business_with_submission):
        """测试查看待审核列表"""
        # 验证待审核红点提示
        assert browser.is_visible("待审核", badge="1")

        # 点击待审核
        browser.click("待审核")

        # 验证待审核列表
        assert browser.snapshot(business_with_submission["task_title"])
        assert browser.snapshot("待审核")

    def test_view_submission_detail(self, browser, business_with_submission):
        """测试查看作品详情"""
        browser.click("待审核")
        browser.click(business_with_submission["task_title"])

        # 验证作品详情
        assert browser.snapshot("作品详情")
        assert browser.snapshot("这是我的创意作品")
        assert browser.is_visible("作品文件")
        assert browser.is_visible("通过")
        assert browser.is_visible("拒绝")

    def test_approve_submission_success(self, browser, business_with_submission):
        """测试通过作品审核"""
        browser.click("待审核")
        browser.click(business_with_submission["task_title"])

        # 点击通过
        browser.click("通过")

        # 填写评价
        browser.fill("评价", "作品质量不错，符合要求")
        browser.select("评分", "5星")

        # 确认
        browser.click("确认")

        # 验证审核成功
        assert browser.snapshot("审核成功")

    def test_approve_submission_balance_changes(self, browser, business_with_submission):
        """测试通过审核后余额变化"""
        # 记录审核前的余额
        browser.click("钱包")
        frozen_before = float(browser.get_text("冻结金额"))

        # 审核通过
        browser.click("待审核")
        browser.click(business_with_submission["task_title"])
        browser.click("通过")
        browser.fill("评价", "很好")
        browser.select("评分", "5星")
        browser.click("确认")

        # 验证冻结金额减少
        browser.click("钱包")
        frozen_after = float(browser.get_text("冻结金额"))
        assert frozen_after == frozen_before - 100

    def test_reject_submission_success(self, browser, business_with_submission):
        """测试拒绝作品审核"""
        browser.click("待审核")
        browser.click(business_with_submission["task_title"])

        # 点击拒绝
        browser.click("拒绝")

        # 填写拒绝原因
        browser.fill("拒绝原因", "作品不符合要求，请重新提交")

        # 确认
        browser.click("确认")

        # 验证审核成功
        assert browser.snapshot("审核成功")

    def test_reject_submission_creator_can_resubmit(self, browser, business_with_submission):
        """测试拒绝后创作者可以重新提交"""
        # 商家拒绝作品
        browser.click("待审核")
        browser.click(business_with_submission["task_title"])
        browser.click("拒绝")
        browser.fill("拒绝原因", "请修改")
        browser.click("确认")

        # 登出商家
        browser.click("退出登录")

        # 创作者登录
        browser.open("/login")
        browser.fill("用户名", business_with_submission["creator_username"])
        browser.fill("密码", business_with_submission["creator_password"])
        browser.select("登录身份", "创作者")
        browser.click("登录")

        # 查看我的认领
        browser.click("我的认领")
        browser.click(business_with_submission["task_title"])

        # 验证可以重新提交
        assert browser.snapshot("已拒绝")
        assert browser.is_visible("重新提交")

    def test_submission_disappears_after_approval(self, browser, business_with_submission):
        """测试审核通过后作品从待审核列表移除"""
        browser.click("待审核")
        browser.click(business_with_submission["task_title"])
        browser.click("通过")
        browser.fill("评价", "很好")
        browser.select("评分", "5星")
        browser.click("确认")

        # 返回待审核列表
        browser.click("待审核")

        # 验证作品已移除
        assert not browser.snapshot(business_with_submission["task_title"])

    def test_view_approved_submissions(self, browser, business_with_submission):
        """测试查看已通过的作品"""
        # 审核通过
        browser.click("待审核")
        browser.click(business_with_submission["task_title"])
        browser.click("通过")
        browser.fill("评价", "很好")
        browser.select("评分", "5星")
        browser.click("确认")

        # 查看我的任务
        browser.click("我的任务")
        browser.click(business_with_submission["task_title"])

        # 点击已完成标签
        browser.click("已完成")

        # 验证作品出现在已完成列表
        assert browser.snapshot("这是我的创意作品")
        assert browser.snapshot("已通过")


class TestBusinessTaskStatistics:
    """商家任务统计测试"""

@pytest.fixture
    def business_user(self, browser):
        """创建并登录商家用户"""
        username = f"business_{int(time.time())}"
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
        browser.select("登录身份", "商家")
        browser.click("登录")

        return {"username": username, "password": password}

    def test_view_dashboard_stats(self, browser, business_user):
        """测试查看工作台统计"""
        # 验证工作台统计数据
        assert browser.snapshot("商家工作台")
        assert browser.snapshot("发布任务数")
        assert browser.snapshot("进行中任务")
        assert browser.snapshot("待审核数")
        assert browser.snapshot("总支出")

    def test_view_transaction_records(self, browser, business_user):
        """测试查看资金流水"""
        browser.click("钱包")
        browser.click("资金流水")

        # 验证资金流水页面
        assert browser.snapshot("资金流水")
        assert browser.is_visible("交易记录列表")

    def test_filter_transactions_by_type(self, browser, business_user):
        """测试按类型筛选资金流水"""
        browser.click("钱包")
        browser.click("资金流水")

        # 筛选支出记录
        browser.select("交易类型", "支出")

        # 验证只显示支出记录
        assert browser.snapshot("任务支付") or browser.snapshot("暂无记录")

    def test_view_task_progress(self, browser, business_user):
        """测试查看任务进度"""
        # 充值并发布任务
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

        # 查看任务详情
        browser.click("我的任务")
        browser.click(task_title)

        # 验证任务进度信息
        assert browser.snapshot("任务进度")
        assert browser.snapshot("已认领：0")
        assert browser.snapshot("已完成：0")
        assert browser.snapshot("剩余：5")
