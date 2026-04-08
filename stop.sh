#!/bin/bash

# 停止创意喵服务

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

BINARY_NAME="miao-server"
PID_FILE="miao.pid"

# 检查是否有运行中的服务
if [ -f "$PID_FILE" ]; then
    PID=$(cat $PID_FILE)
    if ps -p $PID > /dev/null 2>&1; then
        log_info "停止服务 (PID: $PID)..."
        kill -TERM $PID

        # 等待进程结束
        TIMEOUT=10
        while [ $TIMEOUT -gt 0 ] && ps -p $PID > /dev/null 2>&1; do
            sleep 1
            TIMEOUT=$((TIMEOUT-1))
        done

        # 如果还在运行，强制杀掉
        if ps -p $PID > /dev/null 2>&1; then
            log_warn "进程未响应，强制停止..."
            kill -9 $PID
        fi

        rm -f $PID_FILE
        log_info "服务已停止"
    else
        log_warn "PID 文件存在但进程不存在，清理 PID 文件"
        rm -f $PID_FILE
    fi
else
    # 尝试通过进程名查找
    if pgrep -f $BINARY_NAME > /dev/null; then
        log_info "通过进程名停止服务..."
        pkill -TERM -f $BINARY_NAME
        sleep 2
        if pgrep -f $BINARY_NAME > /dev/null; then
            pkill -9 -f $BINARY_NAME
        fi
        log_info "服务已停止"
    else
        log_info "没有运行中的服务"
    fi
fi
