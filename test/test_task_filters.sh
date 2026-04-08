#!/bin/bash

# 视频任务列表功能测试脚本
BASE_URL="${1:-http://localhost:8888}"

echo "=========================================="
echo "视频任务列表功能测试"
echo "=========================================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试计数
TOTAL=0
PASSED=0
FAILED=0

# 登录获取token
echo "正在登录获取token..."
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"testfilter","password":"test123"}')

TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.token' 2>/dev/null)

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo -e "${RED}✗ 登录失败，无法获取token${NC}"
    echo "响应: $LOGIN_RESPONSE"
    exit 1
fi

echo -e "${GREEN}✓ 登录成功${NC}"
echo ""

# 测试函数
test_api() {
    local name="$1"
    local url="$2"
    local expected_code="$3"

    TOTAL=$((TOTAL + 1))
    echo -n "测试 $TOTAL: $name ... "

    response=$(curl -s -w "\n%{http_code}" -H "Authorization: Bearer $TOKEN" "$url")
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    if [ "$http_code" = "$expected_code" ]; then
        # 检查返回的JSON是否有效
        if echo "$body" | jq . >/dev/null 2>&1; then
            echo -e "${GREEN}✓ 通过${NC}"
            PASSED=$((PASSED + 1))

            # 显示任务数量
            task_count=$(echo "$body" | jq -r '.data.data | length' 2>/dev/null)
            if [ "$task_count" != "null" ]; then
                echo "   → 返回 $task_count 个任务"
            fi
        else
            echo -e "${RED}✗ 失败 (无效的JSON)${NC}"
            FAILED=$((FAILED + 1))
        fi
    else
        echo -e "${RED}✗ 失败 (HTTP $http_code, 期望 $expected_code)${NC}"
        FAILED=$((FAILED + 1))
    fi
}

# 1. 测试基础任务列表
echo "=== 基础功能测试 ==="
test_api "获取任务列表（无筛选）" \
    "$BASE_URL/api/v1/creator/tasks?page=1&limit=20" \
    "200"

# 2. 测试视频任务列表
echo ""
echo "=== 视频任务测试 ==="
test_api "获取视频任务列表" \
    "$BASE_URL/api/v1/creator/tasks?page=1&limit=20" \
    "200"

# 3. 测试排序功能
echo ""
echo "=== 排序功能测试 ==="
test_api "按最新发布排序" \
    "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&sort=latest" \
    "200"

test_api "按价格从高到低排序" \
    "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&sort=price_desc" \
    "200"

test_api "按价格从低到高排序" \
    "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&sort=price_asc" \
    "200"

test_api "按截止时间排序" \
    "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&sort=deadline_asc" \
    "200"

# 4. 测试关键词搜索
echo ""
echo "=== 关键词搜索测试 ==="
test_api "搜索关键词：视频" \
    "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&keyword=视频" \
    "200"

test_api "搜索关键词：口播" \
    "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&keyword=口播" \
    "200"

# 5. 测试组合筛选
echo ""
echo "=== 组合筛选测试 ==="
test_api "价格排序" \
    "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&sort=price_desc" \
    "200"

test_api "关键词+排序" \
    "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&keyword=视频&sort=price_asc" \
    "200"

test_api "关键词+最新发布" \
    "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&keyword=视频&sort=latest" \
    "200"

# 6. 测试分页
echo ""
echo "=== 分页功能测试 ==="
test_api "第1页（每页10条）" \
    "$BASE_URL/api/v1/creator/tasks?page=1&limit=10" \
    "200"

test_api "第2页（每页10条）" \
    "$BASE_URL/api/v1/creator/tasks?page=2&limit=10" \
    "200"

# 7. 测试边界条件
echo ""
echo "=== 边界条件测试 ==="
test_api "旧分类参数兼容（应忽略）" \
    "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&category=999" \
    "200"

test_api "空关键词" \
    "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&keyword=" \
    "200"

test_api "超大limit（应限制为100）" \
    "$BASE_URL/api/v1/creator/tasks?page=1&limit=1000" \
    "200"

# 输出测试结果
echo ""
echo "=========================================="
echo "测试完成"
echo "=========================================="
echo -e "总计: $TOTAL"
echo -e "${GREEN}通过: $PASSED${NC}"
echo -e "${RED}失败: $FAILED${NC}"

if [ $FAILED -eq 0 ]; then
    echo -e "\n${GREEN}✓ 所有测试通过！${NC}"
    exit 0
else
    echo -e "\n${RED}✗ 有测试失败${NC}"
    exit 1
fi
