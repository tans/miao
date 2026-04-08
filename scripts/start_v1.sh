#!/bin/bash

# 创意喵平台 V1 快速启动脚本

echo "=========================================="
echo "  创意喵平台 V1 - 快速启动"
echo "=========================================="
echo ""

# 检查 Go 环境
if ! command -v go &> /dev/null; then
    echo "❌ 错误: 未找到 Go 环境"
    echo "请先安装 Go 1.21 或更高版本"
    exit 1
fi

echo "✅ Go 版本: $(go version)"
echo ""

# 检查端口占用
if lsof -Pi :8888 -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo "⚠️  警告: 端口 8888 已被占用"
    echo "正在尝试停止现有服务..."
    kill $(lsof -ti:8888) 2>/dev/null
    sleep 2
fi

# 创建数据目录
if [ ! -d "data" ]; then
    echo "📁 创建数据目录..."
    mkdir -p data
fi

# 启动服务
echo "🚀 启动创意喵平台..."
echo ""
go run cmd/server/main.go &

# 等待服务启动
sleep 3

# 健康检查
if curl -s http://localhost:8888/health > /dev/null 2>&1; then
    echo ""
    echo "=========================================="
    echo "✅ 服务启动成功！"
    echo "=========================================="
    echo ""
    echo "访问地址:"
    echo "  - 首页: http://localhost:8888"
    echo "  - 登录: http://localhost:8888/auth/login.html"
    echo "  - 注册: http://localhost:8888/auth/register.html"
    echo ""
    echo "测试账号:"
    echo "  商家: test_biz_1775665314 / test123456"
    echo "  创作者: test_creator_1775665326 / test123456"
    echo ""
    echo "文档:"
    echo "  - 快速开始: docs/v1/README.md"
    echo "  - API 文档: docs/v1/API.md"
    echo "  - 已知问题: docs/v1/KNOWN_ISSUES.md"
    echo ""
    echo "按 Ctrl+C 停止服务"
    echo "=========================================="

    # 保持前台运行
    wait
else
    echo ""
    echo "❌ 服务启动失败"
    echo "请检查日志输出"
    exit 1
fi
