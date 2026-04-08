"""
完整业务流程端到端测试
"""
import time
import random
from agent_browser import Browser
import pytest


class TestFullWorkflow:
    """完整业务流程测试"""

    @pytest.fixture
    def browser(self):
        """浏览器fixture"""
        browser = Browser(headless=False)
        browser.goto("http://localhost:8888")
        yield browser
        browser.close()

    def test_complete_business_flow(self, browser):
        """测试完整的商家-创作者业务流程"""
        # ========== 第一步：商家注册 ==========
        business_username = f"business_{int(time.time())}"
        business_password = "test123456"
        business_phone = f"138{random.randint(10000000, 99999999)}"

        browser.click("注册")
        browser.fill("用户名", business_username)
        browser.fill("密码", business_password)
        browser.fill("确认密码", business_password)
        browser.fill("手机号", business_phone)
        browser.select("登录身份", "商家")
        browser.click("注册")

        assert browser.has_text("注册成功")

        # ========== 第二步：商家登录 ==========
        browser.fill("用户名", business_username)
        browser.fill("密码", business_password)
        browser.select("登录身份", "商家")
        browser.click("登录")

        assert browser.has_text("商家工作台")

        # ========== 第三步：商家充值 ==========
        browser.click("钱包")
        browser.click("充值")
        browser.fill("充值金额", "10000")
        browser.select("支付方式", "支付宝")
        browser.click("确认充值")
        browser.wait(2)

        assert browser.has_text("充值成功")

        # ========== 第四步：商家发布任务 ==========
        task_title = f"测试任务_{int(time.time())}"
        browser.click("发布任务")
        browser.fill("任务标题", task_title)
        browser.fill("任务描述", "这是一个完整流程测试任务")
        browser.select("任务类型", "设计")
        browser.fill("单价", "100")
        browser.fill("需求数量", "5")
        browser.fill("截止日期", "2026-05-01")
        browser.fill("任务要求", "1. 原创作品\n2. 高清图片")
        browser.click("发布")

        assert browser.has_text("发布成功")

        # 验证冻结金额
        browser.click("钱包")
        frozen_amount = float(browser.get_text("冻结金额"))
        assert frozen_amount == 500.0

        # ========== 第五步：商家登出 ==========
        browser.click("退出登录")

        # ========== 第六步：创作者注册 ==========
        creator_username = f"creator_{int(time.time())}"
        creator_password = "test123456"
        creator_phone = f"139{random.randint(10000000, 99999999)}"

        browser.click("注册")
        browser.fill("用户名", creator_username)
        browser.fill("密码", creator_password)
        browser.fill("确认密码", creator_password)
        browser.fill("手机号", creator_phone)
        browser.select("登录身份", "创作者")
        browser.click("注册")

        assert browser.has_text("注册成功")

        # ========== 第七步：创作者登录 ==========
        browser.fill("用户名", creator_username)
        browser.fill("密码", creator_password)
        browser.select("登录身份", "创作者")
        browser.click("登录")

        assert browser.has_text("创作者工作台")

        # ========== 第八步：创作者浏览任务大厅 ==========
        browser.click("任务大厅")
        assert browser.has_text(task_title)

        # ========== 第九步：创作者认领任务 ==========
        browser.click(task_title)
        browser.click("认领任务")
        browser.click("确认")

        assert browser.has_text("认领成功")

        # ========== 第十步：创作者提交作品 ==========
        browser.click("我的认领")
        browser.click(task_title)
        browser.click("提交作品")
        browser.fill("作品说明", "这是我精心创作的作品，符合所有要求")
        browser.upload("作品文件", "./test/fixtures/work.jpg")
        browser.click("提交")

        assert browser.has_text("提交成功")

        # ========== 第十一步：创作者登出 ==========
        browser.click("退出登录")

        # ========== 第十二步：商家重新登录 ==========
        browser.goto("http://localhost:8888/login")
        browser.fill("用户名", business_username)
        browser.fill("密码", business_password)
        browser.select("登录身份", "商家")
        browser.click("登录")

        # ========== 第十三步：商家查看待审核 ==========
        assert browser.has_element("待审核", badge="1")
        browser.click("待审核")
        assert browser.has_text(task_title)

        # ========== 第十四步：商家审核通过 ==========
        browser.click(task_title)
        browser.click("通过")
        browser.fill("评价", "作品质量很好，完全符合要求")
        browser.select("评分", "5星")
        browser.click("确认")

        assert browser.has_text("审核成功")

        # 验证冻结金额减少
        browser.click("钱包")
        frozen_amount_after = float(browser.get_text("冻结金额"))
        assert frozen_amount_after == 400.0

        # ========== 第十五步：商家登出 ==========
        browser.click("退出登录")

        # ========== 第十六步：创作者重新登录 ==========
        browser.goto("http://localhost:8888/login")
        browser.fill("用户名", creator_username)
        browser.fill("密码", creator_password)
        browser.select("登录身份", "创作者")
        browser.click("登录")

        # ========== 第十七步：创作者查看收益 ==========
        browser.click("钱包")
        balance = float(browser.get_text("可用余额"))
        assert balance == 100.0

        # 查看收益明细
        browser.click("收益明细")
        assert browser.has_text("任务收益")
        assert browser.has_text("100.00")

        # ========== 测试完成 ==========
        print(f"✅ 完整业务流程测试通过！")
        print(f"商家: {business_username}")
        print(f"创作者: {creator_username}")
        print(f"任务: {task_title}")

    def test_multiple_creators_claim_same_task(self, browser):
        """测试多个创作者认领同一任务"""
        # 创建商家并发布任务
        business_username = f"business_{int(time.time())}"
        business_password = "test123456"
        business_phone = f"138{random.randint(10000000, 99999999)}"

        browser.click("注册")
        browser.fill("用户名", business_username)
        browser.fill("密码", business_password)
        browser.fill("确认密码", business_password)
        browser.fill("手机号", business_phone)
        browser.click("注册")

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
        browser.fill("需求数量", "3")
        browser.click("发布")

        browser.click("退出登录")

        # 创建三个创作者并认领任务
        creators = []
        for i in range(3):
            creator_username = f"creator_{int(time.time())}_{i}"
            creator_password = "test123456"
            creator_phone = f"139{random.randint(10000000, 99999999)}"

            browser.click("注册")
            browser.fill("用户名", creator_username)
            browser.fill("密码", creator_password)
            browser.fill("确认密码", creator_password)
            browser.fill("手机号", creator_phone)
            browser.click("注册")

            browser.fill("用户名", creator_username)
            browser.fill("密码", creator_password)
            browser.select("登录身份", "创作者")
            browser.click("登录")

            browser.click("任务大厅")
            browser.click(task_title)
            browser.click("认领任务")
            browser.click("确认")

            assert browser.has_text("认领成功")

            creators.append(creator_username)
            browser.click("退出登录")

        # 验证任务已满
        browser.goto("http://localhost:8888/login")
        browser.fill("用户名", creators[0])
        browser.fill("密码", "test123456")
        browser.select("登录身份", "创作者")
        browser.click("登录")

        browser.click("任务大厅")
        browser.click(task_title)

        # 验证剩余数量为0
        assert browser.has_text("剩余：0")

    def test_creator_with_dual_roles(self, browser):
        """测试用户同时拥有商家和创作者角色"""
        # 注册用户（默认拥有双角色）
        username = f"dual_user_{int(time.time())}"
        password = "test123456"
        phone = f"138{random.randint(10000000, 99999999)}"

        browser.click("注册")
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.fill("确认密码", password)
        browser.fill("手机号", phone)
        browser.click("注册")

        # 以商家身份登录
        browser.fill("用户名", username)
        browser.fill("密码", password)
        browser.select("登录身份", "商家")
        browser.click("登录")

        assert browser.has_text("商家工作台")

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

        # 切换到创作者身份
        browser.click("商家")
        browser.click("创作者")

        assert browser.has_text("创作者工作台")

        # 尝试认领自己发布的任务（应该被阻止）
        browser.click("任务大厅")
        browser.click(task_title)

        # 验证不能认领自己的任务
        assert browser.has_text("不能认领自己发布的任务") or not browser.has_element("认领任务")

    def test_task_lifecycle(self, browser):
        """测试任务完整生命周期"""
        # 创建商家
        business_username = f"business_{int(time.time())}"
        business_password = "test123456"
        business_phone = f"138{random.randint(10000000, 99999999)}"

        browser.click("注册")
        browser.fill("用户名", business_username)
        browser.fill("密码", business_password)
        browser.fill("确认密码", business_password)
        browser.fill("手机号", business_phone)
        browser.click("注册")

        browser.fill("用户名", business_username)
        browser.fill("密码", business_password)
        browser.select("登录身份", "商家")
        browser.click("登录")

        browser.click("钱包")
        browser.click("充值")
        browser.fill("充值金额", "10000")
        browser.click("确认充值")
        browser.wait(2)

        # 发布任务（状态：待审核）
        task_title = f"测试任务_{int(time.time())}"
        browser.click("发布任务")
        browser.fill("任务标题", task_title)
        browser.fill("任务描述", "测试")
        browser.select("任务类型", "设计")
        browser.fill("单价", "100")
        browser.fill("需求数量", "1")
        browser.click("发布")

        # 验证任务状态
        browser.click("我的任务")
        browser.click(task_title)
        assert browser.has_text("待审核") or browser.has_text("进行中")

        browser.click("退出登录")

        # 创建创作者并认领任务
        creator_username = f"creator_{int(time.time())}"
        creator_password = "test123456"
        creator_phone = f"139{random.randint(10000000, 99999999)}"

        browser.click("注册")
        browser.fill("用户名", creator_username)
        browser.fill("密码", creator_password)
        browser.fill("确认密码", creator_password)
        browser.fill("手机号", creator_phone)
        browser.click("注册")

        browser.fill("用户名", creator_username)
        browser.fill("密码", creator_password)
        browser.select("登录身份", "创作者")
        browser.click("登录")

        browser.click("任务大厅")
        browser.click(task_title)
        browser.click("认领任务")
        browser.click("确认")

        # 提交作品
        browser.click("我的认领")
        browser.click(task_title)
        browser.click("提交作品")
        browser.fill("作品说明", "测试作品")
        browser.upload("作品文件", "./test/fixtures/work.jpg")
        browser.click("提交")

        browser.click("退出登录")

        # 商家审核通过
        browser.goto("http://localhost:8888/login")
        browser.fill("用户名", business_username)
        browser.fill("密码", business_password)
        browser.select("登录身份", "商家")
        browser.click("登录")

        browser.click("待审核")
        browser.click(task_title)
        browser.click("通过")
        browser.fill("评价", "很好")
        browser.select("评分", "5星")
        browser.click("确认")

        # 验证任务状态变为已完成
        browser.click("我的任务")
        browser.click(task_title)
        assert browser.has_text("已完成") or browser.has_text("完成：1")
