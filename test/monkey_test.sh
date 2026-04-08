#!/bin/bash

# 创意喵平台 Monkey Test
# 通过 curl 模拟完整的业务流程，验证 API 和页面功能

BASE_URL="${1:-http://localhost:8080}"
TIMESTAMP=$(date +%s)

# 使用主数据库（不使用临时数据库，因为服务器已经启动）
# export DB_PATH="./test/test_miao.db"
# rm -f "$DB_PATH"

echo "========================================"
echo "创意喵平台 Monkey Test"
echo "测试地址: $BASE_URL"
echo "开始时间: $(date '+%Y-%m-%d %H:%M:%S')"
echo "========================================"

# 生成测试数据
CREATOR_USERNAME="creator_${TIMESTAMP}"
CREATOR_PHONE="139${RANDOM}${RANDOM}"
CREATOR_PASSWORD="Test123456"

MERCHANT_USERNAME="merchant_${TIMESTAMP}"
MERCHANT_PHONE="138${RANDOM}${RANDOM}"
MERCHANT_PASSWORD="Test123456"

# 测试结果统计
PASS_COUNT=0
FAIL_COUNT=0

# 测试函数
test_api() {
    local name="$1"
    local method="$2"
    local url="$3"
    local data="$4"
    local token="$5"
    local expected_status="${6:-200}"

    echo ""
    echo "测试: $name"

    if [ -n "$token" ]; then
        response=$(curl -s -w "\n%{http_code}" -X "$method" "$BASE_URL$url" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $token" \
            -d "$data" 2>&1)
    else
        response=$(curl -s -w "\n%{http_code}" -X "$method" "$BASE_URL$url" \
            -H "Content-Type: application/json" \
            -d "$data" 2>&1)
    fi

    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    if [ "$http_code" = "$expected_status" ]; then
        echo "✓ PASS - HTTP $http_code"
        PASS_COUNT=$((PASS_COUNT + 1))
        echo "$body"
        return 0
    else
        echo "✗ FAIL - Expected $expected_status, got $http_code"
        echo "$body"
        FAIL_COUNT=$((FAIL_COUNT + 1))
        return 1
    fi
}

test_page() {
    local name="$1"
    local url="$2"
    local expected_text="$3"

    echo ""
    echo "测试页面: $name"

    response=$(curl -s "$BASE_URL$url")

    if echo "$response" | grep -q "$expected_text"; then
        echo "✓ PASS - 页面包含预期内容: $expected_text"
        PASS_COUNT=$((PASS_COUNT + 1))
        return 0
    else
        echo "✗ FAIL - 页面不包含预期内容: $expected_text"
        FAIL_COUNT=$((FAIL_COUNT + 1))
        return 1
    fi
}

echo ""
echo "========================================"
echo "第一阶段: 页面可访问性测试"
echo "========================================"

test_page "首页" "/" "创意任务平台"
test_page "登录页" "/auth/login.html" "登录"
test_page "注册页" "/auth/register.html" "注册"

echo ""
echo "========================================"
echo "第二阶段: 创作者注册和登录"
echo "========================================"

# 注册创作者
creator_register_data="{\"username\":\"$CREATOR_USERNAME\",\"phone\":\"$CREATOR_PHONE\",\"password\":\"$CREATOR_PASSWORD\"}"
if test_api "创作者注册" "POST" "/api/v1/auth/register" "$creator_register_data" "" "200"; then
    # 登录创作者
    creator_login_data="{\"username\":\"$CREATOR_USERNAME\",\"password\":\"$CREATOR_PASSWORD\"}"
    if test_api "创作者登录" "POST" "/api/v1/auth/login" "$creator_login_data" "" "200"; then
        CREATOR_TOKEN=$(echo "$body" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
        echo "创作者 Token: ${CREATOR_TOKEN:0:20}..."
    fi
fi

echo ""
echo "========================================"
echo "第三阶段: 商家注册和登录"
echo "========================================"

# 注册商家
merchant_register_data="{\"username\":\"$MERCHANT_USERNAME\",\"phone\":\"$MERCHANT_PHONE\",\"password\":\"$MERCHANT_PASSWORD\"}"
if test_api "商家注册" "POST" "/api/v1/auth/register" "$merchant_register_data" "" "200"; then
    # 登录商家
    merchant_login_data="{\"username\":\"$MERCHANT_USERNAME\",\"password\":\"$MERCHANT_PASSWORD\"}"
    if test_api "商家登录" "POST" "/api/v1/auth/login" "$merchant_login_data" "" "200"; then
        MERCHANT_TOKEN=$(echo "$body" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
        echo "商家 Token: ${MERCHANT_TOKEN:0:20}..."
    fi
fi

if [ -z "$CREATOR_TOKEN" ] || [ -z "$MERCHANT_TOKEN" ]; then
    echo ""
    echo "✗ 认证失败，无法继续测试"
    exit 1
fi

echo ""
echo "========================================"
echo "第四阶段: 创作者功能测试"
echo "========================================"

test_api "创作者工作台统计" "GET" "/api/v1/creator/stats" "" "$CREATOR_TOKEN" "200"
test_api "创作者钱包" "GET" "/api/v1/creator/wallet" "" "$CREATOR_TOKEN" "200"
test_api "任务大厅" "GET" "/api/v1/creator/tasks?page=1&limit=10" "" "$CREATOR_TOKEN" "200"

echo ""
echo "========================================"
echo "第五阶段: 商家功能测试"
echo "========================================"

test_api "商家工作台统计" "GET" "/api/v1/business/stats" "" "$MERCHANT_TOKEN" "200"
test_api "商家余额" "GET" "/api/v1/business/balance" "" "$MERCHANT_TOKEN" "200"

# 商家充值（模拟）
recharge_data="{\"amount\":1000,\"payment_method\":\"alipay\"}"
if test_api "商家充值" "POST" "/api/v1/business/recharge" "$recharge_data" "$MERCHANT_TOKEN" "200"; then
    echo "充值成功，等待2秒..."
    sleep 2
fi

# 商家发布任务
task_data="{\"title\":\"测试任务_${TIMESTAMP}\",\"description\":\"这是一个自动化测试任务\",\"category\":2,\"unit_price\":100,\"total_count\":5,\"deadline\":\"2026-12-31T23:59:59+08:00\"}"
if test_api "发布任务" "POST" "/api/v1/business/tasks" "$task_data" "$MERCHANT_TOKEN" "200"; then
    TASK_ID=$(echo "$body" | grep -o '"id":[0-9]*' | head -1 | cut -d':' -f2)
    echo "任务 ID: $TASK_ID"
fi

test_api "商家任务列表" "GET" "/api/v1/business/tasks?page=1&limit=10" "" "$MERCHANT_TOKEN" "200"

echo ""
echo "========================================"
echo "第六阶段: 创作者认领和提交"
echo "========================================"

if [ -n "$TASK_ID" ]; then
    # 创作者认领任务
    if test_api "认领任务" "POST" "/api/v1/creator/tasks/$TASK_ID/claim" "{}" "$CREATOR_TOKEN" "200"; then
        CLAIM_ID=$(echo "$body" | grep -o '"id":[0-9]*' | head -1 | cut -d':' -f2)
        echo "认领 ID: $CLAIM_ID"

        # 创作者提交作品
        submission_data="{\"content\":\"这是我的测试作品内容\",\"attachment_url\":\"https://example.com/work.jpg\"}"
        test_api "提交作品" "POST" "/api/v1/creator/claims/$CLAIM_ID/submit" "$submission_data" "$CREATOR_TOKEN" "200"
    fi

    test_api "我的认领列表" "GET" "/api/v1/creator/claims?page=1&limit=10" "" "$CREATOR_TOKEN" "200"
fi

echo ""
echo "========================================"
echo "第七阶段: 商家审核"
echo "========================================"

if [ -n "$CLAIM_ID" ]; then
    # 商家查看待审核认领
    test_api "待审核认领" "GET" "/api/v1/business/claims/pending?page=1&limit=10" "" "$MERCHANT_TOKEN" "200"

    # 商家审核认领通过
    review_data="{\"status\":\"approved\"}"
    test_api "审核认领通过" "POST" "/api/v1/business/claims/$CLAIM_ID/review" "$review_data" "$MERCHANT_TOKEN" "200"

    # 商家查看待验收作品
    test_api "待验收作品" "GET" "/api/v1/business/submissions/pending?page=1&limit=10" "" "$MERCHANT_TOKEN" "200"
fi

echo ""
echo "========================================"
echo "第八阶段: 交易记录查询"
echo "========================================"

test_api "创作者交易记录" "GET" "/api/v1/creator/transactions?page=1&limit=10" "" "$CREATOR_TOKEN" "200"
test_api "商家交易记录" "GET" "/api/v1/business/transactions?page=1&limit=10" "" "$MERCHANT_TOKEN" "200"

echo ""
echo "========================================"
echo "测试完成"
echo "========================================"
echo "通过: $PASS_COUNT"
echo "失败: $FAIL_COUNT"
echo "总计: $((PASS_COUNT + FAIL_COUNT))"
echo "成功率: $(awk "BEGIN {printf \"%.1f%%\", ($PASS_COUNT/($PASS_COUNT+$FAIL_COUNT))*100}")"
echo "结束时间: $(date '+%Y-%m-%d %H:%M:%S')"
echo "========================================"

if [ $FAIL_COUNT -gt 0 ]; then
    exit 1
fi
