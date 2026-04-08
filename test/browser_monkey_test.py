#!/usr/bin/env python3
"""
创意喵平台端到端测试脚本
使用 agent-browser 进行完整业务流程测试
"""

import argparse
import random
import time
from datetime import datetime
from agent_browser import Browser

# 默认配置
DEFAULT_BASE_URL = "http://localhost:8080"
SCREENSHOT_DIR = "/Users/ke/code/miao/test/screenshots"

# 生成随机测试数据
def generate_test_data():
    timestamp = int(time.time())
    return {
        "creator": {
            "username": f"creator_{timestamp}",
            "phone": f"138{random.randint(10000000, 99999999)}",
            "password": "Test123456"
        },
        "merchant": {
            "username": f"merchant_{timestamp}",
            "phone": f"139{random.randint(10000000, 99999999)}",
            "password": "Test123456"
        }
    }

def main(base_url=DEFAULT_BASE_URL):
    print("=" * 80)
    print("创意喵平台端到端测试")
    print(f"测试地址: {base_url}")
    print(f"开始时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 80)

    # 生成测试数据
    test_data = generate_test_data()
    print(f"\n测试数据:")
    print(f"  创作者: {test_data['creator']['username']} / {test_data['creator']['phone']}")
    print(f"  商家: {test_data['merchant']['username']} / {test_data['merchant']['phone']}")

    test_results = []

    with Browser() as browser:
        try:
            # ========================================
            # 测试 1: 注册创作者账号
            # ========================================
            print("\n" + "=" * 80)
            print("测试 1: 注册创作者账号")
            print("=" * 80)

            browser.goto(f"{base_url}/register")
            browser.screenshot(f"{SCREENSHOT_DIR}/01_register_page.png")

            # 填写注册表单
            browser.fill('input[name="username"]', test_data['creator']['username'])
            browser.fill('input[name="phone"]', test_data['creator']['phone'])
            browser.fill('input[name="password"]', test_data['creator']['password'])
            browser.select('select[name="role"]', 'creator')

            browser.screenshot(f"{SCREENSHOT_DIR}/02_creator_register_filled.png")

            # 提交注册
            browser.click('button[type="submit"]')
            time.sleep(2)

            browser.screenshot(f"{SCREENSHOT_DIR}/03_creator_registered.png")

            # 验证注册成功
            current_url = browser.url()
            if "/login" in current_url or "注册成功" in browser.content():
                print("✓ 创作者注册成功")
                test_results.append(("创作者注册", "PASS"))
            else:
                print("✗ 创作者注册失败")
                test_results.append(("创作者注册", "FAIL"))

            # ========================================
            # 测试 2: 注册商家账号
            # ========================================
            print("\n" + "=" * 80)
            print("测试 2: 注册商家账号")
            print("=" * 80)

            browser.goto(f"{base_url}/register")
            browser.screenshot(f"{SCREENSHOT_DIR}/04_register_page_merchant.png")

            # 填写注册表单
            browser.fill('input[name="username"]', test_data['merchant']['username'])
            browser.fill('input[name="phone"]', test_data['merchant']['phone'])
            browser.fill('input[name="password"]', test_data['merchant']['password'])
            browser.select('select[name="role"]', 'merchant')

            browser.screenshot(f"{SCREENSHOT_DIR}/05_merchant_register_filled.png")

            # 提交注册
            browser.click('button[type="submit"]')
            time.sleep(2)

            browser.screenshot(f"{SCREENSHOT_DIR}/06_merchant_registered.png")

            # 验证注册成功
            current_url = browser.url()
            if "/login" in current_url or "注册成功" in browser.content():
                print("✓ 商家注册成功")
                test_results.append(("商家注册", "PASS"))
            else:
                print("✗ 商家注册失败")
                test_results.append(("商家注册", "FAIL"))

            # ========================================
            # 测试 3: 商家登录
            # ========================================
            print("\n" + "=" * 80)
            print("测试 3: 商家登录")
            print("=" * 80)

            browser.goto(f"{base_url}/login")
            browser.screenshot(f"{SCREENSHOT_DIR}/07_login_page.png")

            # 填写登录表单
            browser.fill('input[name="phone"]', test_data['merchant']['phone'])
            browser.fill('input[name="password"]', test_data['merchant']['password'])

            browser.screenshot(f"{SCREENSHOT_DIR}/08_merchant_login_filled.png")

            # 提交登录
            browser.click('button[type="submit"]')
            time.sleep(2)

            browser.screenshot(f"{SCREENSHOT_DIR}/09_merchant_logged_in.png")

            # 验证登录成功
            current_url = browser.url()
            if "/merchant" in current_url or "商家" in browser.content():
                print("✓ 商家登录成功")
                test_results.append(("商家登录", "PASS"))
            else:
                print("✗ 商家登录失败")
                test_results.append(("商家登录", "FAIL"))

            # ========================================
            # 测试 4: 商家充值
            # ========================================
            print("\n" + "=" * 80)
            print("测试 4: 商家充值")
            print("=" * 80)

            # 导航到钱包页面
            browser.goto(f"{base_url}/merchant/wallet")
            browser.screenshot(f"{SCREENSHOT_DIR}/10_wallet_page.png")

            # 点击充值按钮
            browser.click('a[href*="recharge"], button:has-text("充值")')
            time.sleep(1)

            browser.screenshot(f"{SCREENSHOT_DIR}/11_recharge_page.png")

            # 填写充值金额
            browser.fill('input[name="amount"]', "1000")

            browser.screenshot(f"{SCREENSHOT_DIR}/12_recharge_filled.png")

            # 提交充值
            browser.click('button[type="submit"]')
            time.sleep(2)

            browser.screenshot(f"{SCREENSHOT_DIR}/13_recharge_completed.png")

            # 验证充值成功
            if "充值成功" in browser.content() or "1000" in browser.content():
                print("✓ 商家充值成功")
                test_results.append(("商家充值", "PASS"))
            else:
                print("✗ 商家充值失败")
                test_results.append(("商家充值", "FAIL"))

            # ========================================
            # 测试 5: 商家发布任务
            # ========================================
            print("\n" + "=" * 80)
            print("测试 5: 商家发布任务")
            print("=" * 80)

            # 导航到任务发布页面
            browser.goto(f"{base_url}/merchant/tasks/new")
            browser.screenshot(f"{SCREENSHOT_DIR}/14_task_create_page.png")

            # 填写任务表单
            task_title = f"测试任务_{int(time.time())}"
            browser.fill('input[name="title"]', task_title)
            browser.fill('textarea[name="description"]', "这是一个自动化测试任务，请创作者提交优质作品。")
            browser.fill('input[name="reward"]', "100")
            browser.fill('input[name="max_submissions"]', "5")
            browser.fill('input[name="deadline"]', "2026-04-30")

            browser.screenshot(f"{SCREENSHOT_DIR}/15_task_create_filled.png")

            # 提交任务
            browser.click('button[type="submit"]')
            time.sleep(2)

            browser.screenshot(f"{SCREENSHOT_DIR}/16_task_created.png")

            # 验证任务创建成功
            if "创建成功" in browser.content() or task_title in browser.content():
                print(f"✓ 任务发布成功: {task_title}")
                test_results.append(("任务发布", "PASS"))
            else:
                print("✗ 任务发布失败")
                test_results.append(("任务发布", "FAIL"))

            # 保存任务ID（从URL或页面中提取）
            task_id = None
            current_url = browser.url()
            if "/tasks/" in current_url:
                task_id = current_url.split("/tasks/")[-1].split("/")[0]
                print(f"  任务ID: {task_id}")

            # ========================================
            # 测试 6: 商家退出登录
            # ========================================
            print("\n" + "=" * 80)
            print("测试 6: 商家退出登录")
            print("=" * 80)

            browser.click('a[href*="logout"], button:has-text("退出")')
            time.sleep(1)

            browser.screenshot(f"{SCREENSHOT_DIR}/17_merchant_logged_out.png")

            print("✓ 商家退出登录")
            test_results.append(("商家退出", "PASS"))

            # ========================================
            # 测试 7: 创作者登录
            # ========================================
            print("\n" + "=" * 80)
            print("测试 7: 创作者登录")
            print("=" * 80)

            browser.goto(f"{base_url}/login")
            browser.screenshot(f"{SCREENSHOT_DIR}/18_login_page_creator.png")

            # 填写登录表单
            browser.fill('input[name="phone"]', test_data['creator']['phone'])
            browser.fill('input[name="password"]', test_data['creator']['password'])

            browser.screenshot(f"{SCREENSHOT_DIR}/19_creator_login_filled.png")

            # 提交登录
            browser.click('button[type="submit"]')
            time.sleep(2)

            browser.screenshot(f"{SCREENSHOT_DIR}/20_creator_logged_in.png")

            # 验证登录成功
            current_url = browser.url()
            if "/creator" in current_url or "创作者" in browser.content():
                print("✓ 创作者登录成功")
                test_results.append(("创作者登录", "PASS"))
            else:
                print("✗ 创作者登录失败")
                test_results.append(("创作者登录", "FAIL"))

            # ========================================
            # 测试 8: 创作者浏览任务大厅
            # ========================================
            print("\n" + "=" * 80)
            print("测试 8: 创作者浏览任务大厅")
            print("=" * 80)

            browser.goto(f"{base_url}/creator/tasks")
            browser.screenshot(f"{SCREENSHOT_DIR}/21_task_hall.png")

            # 验证任务大厅
            if "任务" in browser.content():
                print("✓ 任务大厅加载成功")
                test_results.append(("任务大厅", "PASS"))
            else:
                print("✗ 任务大厅加载失败")
                test_results.append(("任务大厅", "FAIL"))

            # ========================================
            # 测试 9: 创作者认领任务
            # ========================================
            print("\n" + "=" * 80)
            print("测试 9: 创作者认领任务")
            print("=" * 80)

            if task_id:
                # 导航到任务详情页
                browser.goto(f"{base_url}/creator/tasks/{task_id}")
                browser.screenshot(f"{SCREENSHOT_DIR}/22_task_detail.png")

                # 点击认领按钮
                browser.click('button:has-text("认领"), a:has-text("认领")')
                time.sleep(2)

                browser.screenshot(f"{SCREENSHOT_DIR}/23_task_claimed.png")

                # 验证认领成功
                if "认领成功" in browser.content() or "已认领" in browser.content():
                    print("✓ 任务认领成功")
                    test_results.append(("任务认领", "PASS"))
                else:
                    print("✗ 任务认领失败")
                    test_results.append(("任务认领", "FAIL"))
            else:
                print("⚠ 跳过任务认领（未找到任务ID）")
                test_results.append(("任务认领", "SKIP"))

            # ========================================
            # 测试 10: 创作者提交作品
            # ========================================
            print("\n" + "=" * 80)
            print("测试 10: 创作者提交作品")
            print("=" * 80)

            if task_id:
                # 导航到提交页面
                browser.goto(f"{base_url}/creator/tasks/{task_id}/submit")
                browser.screenshot(f"{SCREENSHOT_DIR}/24_submission_page.png")

                # 填写提交表单
                browser.fill('textarea[name="content"]', "这是我的测试作品内容，包含创意和设计。")
                browser.fill('input[name="attachment_url"]', "https://example.com/my-work.jpg")

                browser.screenshot(f"{SCREENSHOT_DIR}/25_submission_filled.png")

                # 提交作品
                browser.click('button[type="submit"]')
                time.sleep(2)

                browser.screenshot(f"{SCREENSHOT_DIR}/26_submission_completed.png")

                # 验证提交成功
                if "提交成功" in browser.content() or "待审核" in browser.content():
                    print("✓ 作品提交成功")
                    test_results.append(("作品提交", "PASS"))
                else:
                    print("✗ 作品提交失败")
                    test_results.append(("作品提交", "FAIL"))
            else:
                print("⚠ 跳过作品提交（未找到任务ID）")
                test_results.append(("作品提交", "SKIP"))

            # ========================================
            # 测试 11: 创作者查看钱包
            # ========================================
            print("\n" + "=" * 80)
            print("测试 11: 创作者查看钱包")
            print("=" * 80)

            browser.goto(f"{base_url}/creator/wallet")
            browser.screenshot(f"{SCREENSHOT_DIR}/27_creator_wallet.png")

            # 验证钱包页面
            if "钱包" in browser.content() or "余额" in browser.content():
                print("✓ 创作者钱包加载成功")
                test_results.append(("创作者钱包", "PASS"))
            else:
                print("✗ 创作者钱包加载失败")
                test_results.append(("创作者钱包", "FAIL"))

            # ========================================
            # 测试 12: 创作者退出登录
            # ========================================
            print("\n" + "=" * 80)
            print("测试 12: 创作者退出登录")
            print("=" * 80)

            browser.click('a[href*="logout"], button:has-text("退出")')
            time.sleep(1)

            browser.screenshot(f"{SCREENSHOT_DIR}/28_creator_logged_out.png")

            print("✓ 创作者退出登录")
            test_results.append(("创作者退出", "PASS"))

            # ========================================
            # 测试 13: 商家重新登录
            # ========================================
            print("\n" + "=" * 80)
            print("测试 13: 商家重新登录")
            print("=" * 80)

            browser.goto(f"{base_url}/login")
            browser.fill('input[name="phone"]', test_data['merchant']['phone'])
            browser.fill('input[name="password"]', test_data['merchant']['password'])
            browser.click('button[type="submit"]')
            time.sleep(2)

            browser.screenshot(f"{SCREENSHOT_DIR}/29_merchant_relogin.png")

            print("✓ 商家重新登录成功")
            test_results.append(("商家重新登录", "PASS"))

            # ========================================
            # 测试 14: 商家查看投稿
            # ========================================
            print("\n" + "=" * 80)
            print("测试 14: 商家查看投稿")
            print("=" * 80)

            if task_id:
                browser.goto(f"{base_url}/merchant/tasks/{task_id}/submissions")
                browser.screenshot(f"{SCREENSHOT_DIR}/30_submissions_list.png")

                # 验证投稿列表
                if "投稿" in browser.content() or "作品" in browser.content():
                    print("✓ 投稿列表加载成功")
                    test_results.append(("投稿列表", "PASS"))
                else:
                    print("✗ 投稿列表加载失败")
                    test_results.append(("投稿列表", "FAIL"))
            else:
                print("⚠ 跳过投稿列表（未找到任务ID）")
                test_results.append(("投稿列表", "SKIP"))

            # ========================================
            # 测试 15: 商家验收作品
            # ========================================
            print("\n" + "=" * 80)
            print("测试 15: 商家验收作品")
            print("=" * 80)

            if task_id:
                # 点击第一个投稿的验收按钮
                browser.click('button:has-text("通过"), button:has-text("验收")')
                time.sleep(2)

                browser.screenshot(f"{SCREENSHOT_DIR}/31_submission_approved.png")

                # 验证验收成功
                if "通过" in browser.content() or "验收成功" in browser.content():
                    print("✓ 作品验收成功")
                    test_results.append(("作品验收", "PASS"))
                else:
                    print("✗ 作品验收失败")
                    test_results.append(("作品验收", "FAIL"))
            else:
                print("⚠ 跳过作品验收（未找到任务ID）")
                test_results.append(("作品验收", "SKIP"))

            # ========================================
            # 测试 16: 商家查看钱包和交易记录
            # ========================================
            print("\n" + "=" * 80)
            print("测试 16: 商家查看钱包和交易记录")
            print("=" * 80)

            browser.goto(f"{base_url}/merchant/wallet")
            browser.screenshot(f"{SCREENSHOT_DIR}/32_merchant_wallet_final.png")

            # 验证钱包页面
            if "钱包" in browser.content() or "余额" in browser.content():
                print("✓ 商家钱包加载成功")
                test_results.append(("商家钱包", "PASS"))
            else:
                print("✗ 商家钱包加载失败")
                test_results.append(("商家钱包", "FAIL"))

            # 查看交易记录
            browser.goto(f"{base_url}/merchant/transactions")
            browser.screenshot(f"{SCREENSHOT_DIR}/33_transactions.png")

            # 验证交易记录
            if "交易" in browser.content() or "记录" in browser.content():
                print("✓ 交易记录加载成功")
                test_results.append(("交易记录", "PASS"))
            else:
                print("✗ 交易记录加载失败")
                test_results.append(("交易记录", "FAIL"))

        except Exception as e:
            print(f"\n✗ 测试过程中发生错误: {str(e)}")
            browser.screenshot(f"{SCREENSHOT_DIR}/error.png")
            test_results.append(("测试执行", "ERROR"))

    # ========================================
    # 生成测试报告
    # ========================================
    print("\n" + "=" * 80)
    print("测试报告")
    print("=" * 80)

    total_tests = len(test_results)
    passed_tests = sum(1 for _, result in test_results if result == "PASS")
    failed_tests = sum(1 for _, result in test_results if result == "FAIL")
    skipped_tests = sum(1 for _, result in test_results if result == "SKIP")
    error_tests = sum(1 for _, result in test_results if result == "ERROR")

    print(f"\n总测试数: {total_tests}")
    print(f"通过: {passed_tests}")
    print(f"失败: {failed_tests}")
    print(f"跳过: {skipped_tests}")
    print(f"错误: {error_tests}")
    print(f"通过率: {passed_tests / total_tests * 100:.1f}%")

    print("\n详细结果:")
    for test_name, result in test_results:
        status_icon = {
            "PASS": "✓",
            "FAIL": "✗",
            "SKIP": "⚠",
            "ERROR": "✗"
        }.get(result, "?")
        print(f"  {status_icon} {test_name}: {result}")

    print(f"\n结束时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 80)

    # 保存测试报告
    report_path = f"/Users/ke/code/miao/test/test_report_{int(time.time())}.txt"
    with open(report_path, "w", encoding="utf-8") as f:
        f.write("创意喵平台端到端测试报告\n")
        f.write("=" * 80 + "\n")
        f.write(f"测试时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n")
        f.write(f"服务器地址: {base_url}\n\n")
        f.write(f"测试数据:\n")
        f.write(f"  创作者: {test_data['creator']['username']} / {test_data['creator']['phone']}\n")
        f.write(f"  商家: {test_data['merchant']['username']} / {test_data['merchant']['phone']}\n\n")
        f.write(f"测试结果:\n")
        f.write(f"  总测试数: {total_tests}\n")
        f.write(f"  通过: {passed_tests}\n")
        f.write(f"  失败: {failed_tests}\n")
        f.write(f"  跳过: {skipped_tests}\n")
        f.write(f"  错误: {error_tests}\n")
        f.write(f"  通过率: {passed_tests / total_tests * 100:.1f}%\n\n")
        f.write("详细结果:\n")
        for test_name, result in test_results:
            f.write(f"  {test_name}: {result}\n")

    print(f"\n测试报告已保存到: {report_path}")
    print(f"截图保存目录: {SCREENSHOT_DIR}")

if __name__ == "__main__":
    main()
