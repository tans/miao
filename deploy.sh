#!/bin/bash

# 创意喵平台部署脚本
# 用法: ./deploy.sh [环境]
# 环境: dev (开发), staging (预发布), prod (生产)

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查参数
ENV=${1:-dev}
if [[ ! "$ENV" =~ ^(dev|staging|prod)$ ]]; then
    log_error "无效的环境参数: $ENV"
    echo "用法: ./deploy.sh [dev|staging|prod]"
    exit 1
fi

log_info "开始部署到 $ENV 环境..."

# 1. 检查 Go 环境
log_info "检查 Go 环境..."
if ! command -v go &> /dev/null; then
    log_error "Go 未安装，请先安装 Go 1.20+"
    exit 1
fi
GO_VERSION=$(go version | awk '{print $3}')
log_info "Go 版本: $GO_VERSION"

# 2. 拉取最新代码（生产环境）
if [ "$ENV" = "prod" ]; then
    log_info "拉取最新代码..."
    git pull origin main
fi

# 3. 安装依赖
log_info "安装依赖..."
GOPROXY=https://goproxy.cn,direct go mod download
GOPROXY=https://goproxy.cn,direct go mod verify

# 4. 运行测试
log_info "运行测试..."
if go test ./... -v; then
    log_info "测试通过"
else
    log_error "测试失败，部署中止"
    exit 1
fi

# 5. 编译
log_info "编译应用..."
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION="v1.0.0"

BINARY_NAME="miao-server"
if [ "$ENV" = "prod" ]; then
    # 生产环境优化编译
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GOPROXY=https://goproxy.cn,direct go build \
        -ldflags "-s -w -X main.Version=$VERSION -X main.BuildTime=$BUILD_TIME -X main.GitCommit=$GIT_COMMIT" \
        -o $BINARY_NAME \
        ./cmd/server
else
    # 开发/预发布环境
    GOPROXY=https://goproxy.cn,direct go build \
        -ldflags "-X main.Version=$VERSION -X main.BuildTime=$BUILD_TIME -X main.GitCommit=$GIT_COMMIT" \
        -o $BINARY_NAME \
        ./cmd/server
fi

log_info "编译完成: $BINARY_NAME"

# 6. 创建必要的目录
log_info "创建必要的目录..."
mkdir -p data
mkdir -p logs
mkdir -p uploads

# 7. 备份旧版本（生产环境）
if [ "$ENV" = "prod" ] && [ -f "$BINARY_NAME" ]; then
    log_info "备份旧版本..."
    BACKUP_DIR="backups/$(date +%Y%m%d_%H%M%S)"
    mkdir -p $BACKUP_DIR
    cp $BINARY_NAME $BACKUP_DIR/
    cp data/miao.db $BACKUP_DIR/ 2>/dev/null || true
    log_info "备份保存到: $BACKUP_DIR"
fi

# 8. 停止旧服务
log_info "停止旧服务..."
if pgrep -f $BINARY_NAME > /dev/null; then
    pkill -TERM -f $BINARY_NAME
    sleep 2
    # 如果还在运行，强制杀掉
    if pgrep -f $BINARY_NAME > /dev/null; then
        pkill -9 -f $BINARY_NAME
    fi
    log_info "旧服务已停止"
else
    log_info "没有运行中的服务"
fi

# 9. 设置环境变量
log_info "设置环境变量..."
export GIN_MODE=release
export DB_PATH="./data/miao.db"
export JWT_SECRET=${JWT_SECRET:-$(openssl rand -base64 32)}
export SERVER_PORT=${SERVER_PORT:-8888}

# 根据环境设置不同的配置
case $ENV in
    dev)
        export GIN_MODE=debug
        export LOG_LEVEL=debug
        ;;
    staging)
        export LOG_LEVEL=info
        ;;
    prod)
        export LOG_LEVEL=warn
        ;;
esac

# 10. 数据库迁移
log_info "执行数据库迁移..."
# 数据库迁移在服务启动时自动执行

# 11. 启动服务
log_info "启动服务..."
if [ "$ENV" = "dev" ]; then
    # 开发环境前台运行
    log_info "开发环境，前台运行..."
    ./$BINARY_NAME
else
    # 生产/预发布环境后台运行
    nohup ./$BINARY_NAME > logs/server.log 2>&1 &
    SERVER_PID=$!
    echo $SERVER_PID > miao.pid
    log_info "服务已启动，PID: $SERVER_PID"

    # 等待服务启动
    sleep 3

    # 健康检查
    log_info "执行健康检查..."
    MAX_RETRY=10
    RETRY=0
    while [ $RETRY -lt $MAX_RETRY ]; do
        if curl -s http://localhost:$SERVER_PORT/health > /dev/null; then
            log_info "健康检查通过"
            break
        fi
        RETRY=$((RETRY+1))
        log_warn "健康检查失败，重试 $RETRY/$MAX_RETRY..."
        sleep 2
    done

    if [ $RETRY -eq $MAX_RETRY ]; then
        log_error "健康检查失败，服务可能未正常启动"
        log_error "查看日志: tail -f logs/server.log"
        exit 1
    fi
fi

# 12. 部署完成
log_info "=========================================="
log_info "部署完成！"
log_info "环境: $ENV"
log_info "版本: $VERSION"
log_info "提交: $GIT_COMMIT"
log_info "构建时间: $BUILD_TIME"
log_info "服务地址: http://localhost:$SERVER_PORT"
log_info "=========================================="

if [ "$ENV" != "dev" ]; then
    log_info "查看日志: tail -f logs/server.log"
    log_info "停止服务: kill \$(cat miao.pid)"
fi
