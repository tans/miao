#!/bin/bash

# 重启创意喵服务

set -e

GREEN='\033[0;32m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_info "重启服务..."

# 停止服务
./stop.sh

# 等待1秒
sleep 1

# 设置数据库路径（使用绝对路径避免从不同目录启动时数据丢失）
export DB_PATH="$(cd "$(dirname "$0")" && pwd)/data/miao.db"

# 启动服务
log_info "启动服务..."
nohup setsid ./miao-server > logs/server.log 2>&1 < /dev/null &
SERVER_PID=$!
disown $SERVER_PID 2>/dev/null || true
echo $SERVER_PID > miao.pid

log_info "服务已启动，PID: $SERVER_PID"

# 健康检查
sleep 3
if curl -s http://localhost:8080/health > /dev/null; then
    log_info "服务运行正常"
else
    log_info "健康检查失败，请查看日志: tail -f logs/server.log"
    exit 1
fi
