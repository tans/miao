# 创意喵平台 Makefile

.PHONY: help dev build test clean install-tools run

# 默认目标
help:
	@echo "创意喵平台开发命令:"
	@echo ""
	@echo "开发:"
	@echo "  make dev          - 启动热重载开发服务器 (推荐)"
	@echo "  make run          - 直接运行 (无热重载)"
	@echo "  make build-dev    - 快速编译 (开发模式)"
	@echo ""
	@echo "测试:"
	@echo "  make test         - 运行所有测试"
	@echo "  make test-pkg     - 运行指定包测试 (PKG=./internal/handler)"
	@echo "  make test-func    - 运行指定测试函数 (FUNC=TestCreateTask PKG=./internal/handler)"
	@echo "  make test-coverage - 生成测试覆盖率报告"
	@echo ""
	@echo "生产:"
	@echo "  make build        - 编译生产版本"
	@echo "  make deploy       - 部署到生产环境"
	@echo "  make logs         - 查看生产日志"
	@echo "  make restart      - 重启生产服务"
	@echo ""
	@echo "维护:"
	@echo "  make clean        - 清理编译产物"
	@echo "  make db-reset     - 重置数据库"
	@echo "  make install-tools - 安装开发工具 (air)"

# 安装开发工具
install-tools:
	@echo "安装 air 热重载工具..."
	@GOPROXY=https://goproxy.cn,direct go install github.com/air-verse/air@latest
	@echo "✓ air 安装完成"

# 热重载开发 (推荐)
dev:
	@if ! command -v air > /dev/null; then \
		if [ -f ~/go/bin/air ]; then \
			echo "使用 ~/go/bin/air 启动热重载..."; \
			~/go/bin/air; \
		else \
			echo "air 未安装，正在安装..."; \
			make install-tools; \
			~/go/bin/air; \
		fi \
	else \
		echo "启动热重载开发服务器..."; \
		air; \
	fi

# 快速编译 (开发模式，禁用优化)
build-dev:
	@echo "编译开发版本 (快速模式)..."
	@go build -gcflags="all=-N -l" -o tmp/main ./cmd/server
	@echo "✓ 编译完成: tmp/main"

# 生产编译
build:
	@echo "编译生产版本..."
	@go build -ldflags="-s -w" -o bin/miao ./cmd/server
	@echo "✓ 编译完成: bin/miao"

# 直接运行 (无热重载)
run:
	@echo "运行服务器..."
	@go run cmd/server/main.go

# 运行所有测试
test:
	@echo "运行所有测试..."
	@go test -v ./...

# 运行指定包测试
# 用法: make test-pkg PKG=./internal/handler
test-pkg:
	@if [ -z "$(PKG)" ]; then \
		echo "错误: 请指定 PKG 参数"; \
		echo "示例: make test-pkg PKG=./internal/handler"; \
		exit 1; \
	fi
	@echo "运行测试: $(PKG)"
	@go test -v $(PKG)

# 运行指定测试函数
# 用法: make test-func FUNC=TestCreateTask PKG=./internal/handler
test-func:
	@if [ -z "$(FUNC)" ] || [ -z "$(PKG)" ]; then \
		echo "错误: 请指定 FUNC 和 PKG 参数"; \
		echo "示例: make test-func FUNC=TestCreateTask PKG=./internal/handler"; \
		exit 1; \
	fi
	@echo "运行测试函数: $(FUNC) in $(PKG)"
	@go test -v -run $(FUNC) $(PKG)

# 测试覆盖率
test-coverage:
	@echo "生成测试覆盖率报告..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ 覆盖率报告: coverage.html"

# 清理编译产物
clean:
	@echo "清理编译产物..."
	@rm -rf tmp/ bin/ coverage.out coverage.html
	@echo "✓ 清理完成"

# 依赖管理
deps:
	@echo "下载依赖..."
	@go mod download
	@echo "✓ 依赖下载完成"

tidy:
	@echo "整理依赖..."
	@go mod tidy
	@echo "✓ 依赖整理完成"

# 数据库相关
db-reset:
	@echo "重置数据库..."
	@rm -f miao.db
	@echo "✓ 数据库已删除，下次启动将重新创建"

# 生产部署相关
deploy:
	@echo "部署到生产环境..."
	@if [ ! -f deploy.sh ]; then \
		echo "错误: deploy.sh 不存在"; \
		echo "请根据 docs/deployment-guide.md 创建部署脚本"; \
		exit 1; \
	fi
	@./deploy.sh

logs:
	@echo "查看生产日志..."
	@sudo journalctl -u miao -f

restart:
	@echo "重启生产服务..."
	@sudo systemctl restart miao
	@sudo systemctl status miao
