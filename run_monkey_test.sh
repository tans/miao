#!/bin/bash

# Monkey Test 运行脚本
# 用于测试创意喵平台的完整业务流程

set -e

echo "🐵 创意喵平台 Monkey Test"
echo "================================"
echo ""

# 检查服务器是否运行
echo "📡 检查服务器状态..."
if ! curl -s http://localhost:8080/api/v1/health > /dev/null 2>&1; then
    echo "❌ 服务器未运行，请先启动服务器"
    echo "   运行: ./miao-server 或 go run cmd/server/main.go"
    exit 1
fi
echo "✅ 服务器运行正常"
echo ""

# 运行测试
echo "🧪 开始运行 Monkey Test..."
echo "================================"
echo ""

cd "$(dirname "$0")"
go test -v ./test -run TestMonkeyFullFlow -timeout 5m

echo ""
echo "================================"
echo "✅ Monkey Test 完成！"
