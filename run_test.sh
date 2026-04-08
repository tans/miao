#!/bin/bash

# 创意喵平台端到端测试运行脚本

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
TEST_DIR="$SCRIPT_DIR/test"

echo "=========================================="
echo "创意喵平台端到端测试"
echo "=========================================="

# 检查服务器是否运行
echo ""
echo "检查服务器状态..."
if ! curl -s http://localhost:8080 > /dev/null 2>&1; then
    echo "❌ 服务器未运行！"
    echo "请先启动服务器: ./miao-server"
    exit 1
fi
echo "✓ 服务器正在运行"

# 检查 Python
echo ""
echo "检查 Python 环境..."
if ! command -v python3 &> /dev/null; then
    echo "❌ 未找到 Python3"
    exit 1
fi
echo "✓ Python3: $(python3 --version)"

# 检查依赖
echo ""
echo "检查依赖..."
if ! python3 -c "import agent_browser" 2>/dev/null; then
    echo "⚠ agent-browser 未安装，正在安装..."
    pip3 install -r "$TEST_DIR/requirements.txt"
else
    echo "✓ agent-browser 已安装"
fi

# 创建截图目录
mkdir -p "$TEST_DIR/screenshots"

# 运行测试
echo ""
echo "=========================================="
echo "开始运行测试..."
echo "=========================================="
echo ""

cd "$TEST_DIR"
python3 browser_monkey_test.py

# 显示结果
echo ""
echo "=========================================="
echo "测试完成！"
echo "=========================================="
echo ""
echo "查看结果:"
echo "  - 截图目录: $TEST_DIR/screenshots/"
echo "  - 测试报告: $TEST_DIR/test_report_*.txt"
echo ""
