#!/bin/bash
# 模拟任务生成脚本
# 用于发布测试任务到创意喵平台

set -e

# 配置
API_HOST="${API_HOST:-http://localhost:8888}"
TASKS_FILE="/tmp/mock_tasks.json"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 检查参数
if [ $# -lt 1 ]; then
    echo "用法: $0 <用户名> [密码]"
    echo "示例: $0 admin admin123"
    exit 1
fi

USERNAME="$1"
PASSWORD="${2:-admin123}"

# 登录获取 token
log_info "正在登录用户: $USERNAME"
LOGIN_RESP=$(curl -s --noproxy localhost -X POST "$API_HOST/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")

TOKEN=$(echo "$LOGIN_RESP" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
    log_error "登录失败，请检查用户名和密码"
    echo "$LOGIN_RESP"
    exit 1
fi

log_info "登录成功!"

# 先充值，确保有足够余额
log_info "正在充值账户..."
RECHARGE_RESP=$(curl -s --noproxy localhost -X POST "$API_HOST/api/v1/business/recharge" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d '{"amount": 10000}')

RECHARGE_CODE=$(echo "$RECHARGE_RESP" | grep -o '"code":[0-9]*' | cut -d':' -f2)
if [ "$RECHARGE_CODE" = "0" ]; then
    log_info "充值成功! 账户余额: 10000元"
else
    log_warn "充值失败，尝试继续..."
fi

# 模拟任务数据
declare -a TITLES=(
    "短视频创作：夏日饮品推荐"
    "小红书笔记：春季穿搭分享"
    "抖音挑战赛：#一周穿搭不重样"
    "产品测评：新款蓝牙耳机"
    "美食探店：本地网红餐厅"
    "旅游攻略：周末周边游推荐"
    "美妆教程：日常通勤妆容"
    "健身打卡：居家运动计划"
    "读书分享：本周书单推荐"
    "数码评测：平板电脑对比"
)

declare -a DESCRIPTIONS=(
    "创作一条15-60秒的短视频，展示夏日饮品制作过程或品尝体验，要求画面清晰，有字幕说明。"
    "撰写一篇图文笔记，介绍春季流行穿搭搭配，包含至少3套搭配方案，图片要求原创或授权。"
    "参与抖音挑战赛，发布7条不同风格的穿搭视频，添加指定话题标签，@官方账号。"
    "对指定蓝牙耳机进行深度测评，包含外观、音质、续航、佩戴舒适度等方面，时长3-5分钟视频。"
    "到指定餐厅进行探店拍摄，产出图文笔记和短视频各一份，分享真实用餐体验。"
    "规划一条2天1夜的周边游路线，包含景点、美食、住宿推荐，需包含详细费用预算。"
    "录制一个日常通勤妆容教程视频，时长5-10分钟，讲解妆容步骤和产品推荐。"
    "制定一份居家运动计划，包含热身、力量训练、拉伸三个部分，需拍摄运动过程视频。"
    "分享本周阅读的书籍，写一篇500字以上的读书笔记，需包含书籍封面图和精彩片段。"
    "对两款平板电脑进行横评对比，产出视频和图文两版内容，包含性能测试、屏幕对比等。"
)

declare -a CATEGORIES=("1" "2" "1" "3" "4" "5" "6" "7" "8" "9")

# 发布任务
TASK_COUNT=0
FAILED_COUNT=0

for i in "${!TITLES[@]}"; do
    idx=$((i + 1))

    # 随机生成任务参数
    BUDGET=$((50 + RANDOM % 450))
    BASE_REWARD=$((5 + RANDOM % 45))
    SUBMISSION_LIMIT=$((10 + RANDOM % 40))
    DEADLINE_DAYS=$((7 + RANDOM % 14))

    # 计算截止日期
    DEADLINE=$(date -v+${DEADLINE_DAYS}d "+%Y-%m-%dT23:59:59Z")

    log_info "发布任务 [$idx/${#TITLES[@]}]: ${TITLES[$i]}"

    AWARD1=$(echo "$BASE_REWARD * 2" | bc)
    AWARD2=$(echo "$BASE_REWARD * 1.5" | bc)
    AWARD3=$(echo "$BASE_REWARD * 1.2" | bc)

    RESP=$(curl -s --noproxy localhost -X POST "$API_HOST/api/v1/business/tasks" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $TOKEN" \
        -d "{
            \"title\": \"${TITLES[$i]}\",
            \"description\": \"${DESCRIPTIONS[$i]}\",
            \"category\": ${CATEGORIES[$i]},
            \"unit_price\": $BASE_REWARD,
            \"total_count\": $SUBMISSION_LIMIT,
            \"remaining_count\": $SUBMISSION_LIMIT,
            \"total_budget\": $BUDGET,
            \"base_reward\": $BASE_REWARD,
            \"base_reward_limit\": $SUBMISSION_LIMIT,
            \"award1_amount\": $AWARD1,
            \"award1_count\": 1,
            \"award2_amount\": $AWARD2,
            \"award2_count\": 2,
            \"award3_amount\": $AWARD3,
            \"award3_count\": 3,
            \"award_good_amount\": $BASE_REWARD,
            \"award_good_count\": 5,
            \"max_per_user\": 1,
            \"is_public\": 1,
            \"allow_duplicate\": 0,
            \"enable_check\": 1,
            \"deadline\": \"$DEADLINE\"
        }")

    CODE=$(echo "$RESP" | grep -o '"code":[0-9]*' | cut -d':' -f2)

    if [ "$CODE" = "0" ]; then
        TASK_ID=$(echo "$RESP" | grep -o '"task_id":[0-9]*' | cut -d':' -f2)
        log_info "  ✓ 任务创建成功 (ID: $TASK_ID, 预算: ${BUDGET}元, 基础奖励: ${BASE_REWARD}元)"

        # 将任务标记为已上线（跳过审核）
        sqlite3 ./data/miao.db "UPDATE tasks SET status = 2 WHERE id = $TASK_ID;"
        log_info "  ✓ 任务已上线"

        ((TASK_COUNT++))
    else
        MSG=$(echo "$RESP" | grep -o '"message":"[^"]*"' | cut -d'"' -f4)
        log_error "  ✗ 创建失败: $MSG"
        ((FAILED_COUNT++))
    fi
done

echo ""
log_info "========================================"
log_info "任务发布完成!"
log_info "成功: $TASK_COUNT 个"
if [ $FAILED_COUNT -gt 0 ]; then
    log_warn "失败: $FAILED_COUNT 个"
fi
log_info "========================================"

# 显示当前任务列表
echo ""
log_info "当前任务列表:"
curl -s --noproxy localhost -X GET "$API_HOST/api/v1/tasks" \
    -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | "  - [\(.id)] \(.title) | 状态:\(.status)| 预算:\(.total_budget)元"' 2>/dev/null || true
