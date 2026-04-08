# 创意喵平台开发指南

## 快速开始

### 1. 安装开发工具

```bash
# 安装 air 热重载工具（使用国内镜像）
make install-tools

# 或手动安装
GOPROXY=https://goproxy.cn,direct go install github.com/air-verse/air@latest

# 验证安装
~/go/bin/air -v
```

### 2. 启动开发服务器

```bash
# 推荐：使用热重载开发（代码变更自动重启）
make dev

# 或直接运行（无热重载）
make run
```

## 开发工作流

### 热重载开发（推荐）

使用 `air` 实现代码变更自动编译重启：

```bash
make dev
```

**优势：**
- 保存文件后自动重新编译
- 1 秒延迟避免频繁编译
- 排除测试文件和静态资源
- 彩色日志输出
- 编译错误时停止旧进程

**监听范围：**
- `.go` 文件（排除 `*_test.go`）
- `.html`, `.tmpl`, `.tpl` 模板文件
- 排除目录：`tmp/`, `vendor/`, `web/static/`, `.git/`, `.claude/`

### 编译优化

```bash
# 开发快速编译（禁用优化，加快编译速度）
make build-dev

# 生产编译（优化体积和性能）
make build
```

**编译速度对比：**
- 开发模式：`-gcflags="all=-N -l"` 禁用内联和优化，编译快 30-50%
- 生产模式：`-ldflags="-s -w"` 去除调试信息，体积减少 20-30%

## 测试策略

### 运行测试

```bash
# 运行所有测试
make test

# 运行指定包测试
make test-pkg PKG=./internal/handler

# 运行指定测试函数
make test-func FUNC=TestCreateTask PKG=./internal/handler

# 生成覆盖率报告
make test-coverage
```

### 测试优化技巧

**1. 使用内存数据库（测试时）**

```go
// 测试时使用 :memory: 避免磁盘 I/O
db, _ := sql.Open("sqlite3", ":memory:")
```

**2. 并行测试**

```bash
# 并行运行测试（默认 GOMAXPROCS）
go test -parallel 4 ./...
```

**3. 缓存控制**

```bash
# 使用缓存（默认，跳过未变更的测试）
go test ./...

# 禁用缓存（调试时）
go test -count=1 ./...
```

**4. 只测试变更的包**

```bash
# 使用 git 找出变更的包
git diff --name-only | grep '\.go$' | xargs -I {} dirname {} | sort -u | xargs go test
```

## 性能优化

### 1. 模块缓存

```bash
# 预下载所有依赖
make deps

# 清理无用依赖
make tidy
```

### 2. 编译缓存

Go 自动缓存编译结果，首次编译慢，后续编译快。

```bash
# 查看缓存位置
go env GOCACHE

# 清理缓存（不推荐，除非调试编译问题）
go clean -cache
```

### 3. SQLite 优化

**开发环境配置：**

```go
// internal/database/db.go
db.SetMaxOpenConns(1)  // SQLite 单写入连接
db.SetMaxIdleConns(1)
db.Exec("PRAGMA journal_mode=WAL")      // 写前日志，提升并发
db.Exec("PRAGMA synchronous=NORMAL")    // 平衡性能和安全
db.Exec("PRAGMA cache_size=-64000")     // 64MB 缓存
```

### 4. Gin 开发模式

```go
// 开发时使用 debug 模式（默认）
gin.SetMode(gin.DebugMode)

// 生产时使用 release 模式
gin.SetMode(gin.ReleaseMode)
```

## 常用命令

```bash
# 开发
make dev              # 热重载开发（推荐）
make run              # 直接运行
make build-dev        # 快速编译

# 测试
make test             # 所有测试
make test-pkg PKG=./internal/handler
make test-func FUNC=TestCreateTask PKG=./internal/handler

# 维护
make clean            # 清理编译产物
make deps             # 下载依赖
make tidy             # 整理依赖
make db-reset         # 重置数据库

# 生产
make build            # 生产编译
```

## 目录结构

```
miao/
├── .air.toml              # Air 热重载配置
├── Makefile               # 开发命令
├── tmp/                   # 临时编译产物（air 使用）
├── bin/                   # 生产编译产物
├── cmd/server/main.go     # 入口文件
├── internal/              # 内部包
│   ├── handler/           # HTTP 处理器
│   ├── service/           # 业务逻辑
│   ├── repository/        # 数据访问
│   ├── model/             # 数据模型
│   ├── middleware/        # 中间件
│   └── router/            # 路由配置
├── web/                   # 前端资源
│   ├── static/            # 静态文件
│   └── templates/         # HTML 模板
└── docs/                  # 文档
```

## 故障排查

### 编译慢

1. 检查是否使用了 `make dev`（热重载）
2. 首次编译会下载依赖，后续会快很多
3. 使用 `make build-dev` 禁用优化

### 热重载不生效

1. 检查 `air` 是否安装：`which air`
2. 检查 `.air.toml` 配置中的 `include_ext` 和 `exclude_dir`
3. 查看 `tmp/air.log` 日志

### 测试慢

1. 使用 `:memory:` 数据库
2. 只测试变更的包
3. 使用 `-parallel` 并行测试
4. 检查是否有网络请求或 sleep

### 数据库锁定

```bash
# SQLite 数据库被锁定时重置
make db-reset
```

## Agent 开发注意事项

**backend-dev、frontend-dev、qa agent 必读：**

1. **开发时使用 `make dev`**：自动热重载，无需手动重启
2. **测试时使用内存数据库**：`sql.Open("sqlite3", ":memory:")`
3. **编译优化**：开发用 `make build-dev`，生产用 `make build`
4. **测试策略**：先测试单个包 `make test-pkg`，再测试全部
5. **清理产物**：遇到奇怪问题时 `make clean` 重新编译

## 参考资料

- [Air 文档](https://github.com/air-verse/air)
- [Gin 文档](https://gin-gonic.com/docs/)
- [SQLite 优化](https://www.sqlite.org/pragma.html)
- [Go 测试最佳实践](https://go.dev/doc/tutorial/add-a-test)
