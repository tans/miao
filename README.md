# 创意喵 - 视频任务撮合平台

创意喵是一个连接**商家**和**创作者**的视频任务撮合平台。商家发布视频任务，创作者认领并完成交付。平台提供等级制度、信用体系和资金托管，保障双方权益。

## 功能特性

### 商家端
- 企业实名认证
- 发布视频任务（支持图文/视频素材）
- 100%预付制度，资金托管安全
- 认领审核和交付验收
- 账户充值和交易记录
- 任务数据统计

### 创作者端
- 四级体系（青铜/白银/黄金/钻石）
- 任务大厅浏览和认领
- 动态抽成比例（10%-20%，等级越高越低）
- 每日认领限额管理（3-50单）
- 钱包和收益明细
- 信用积分系统

### 管理端
- 用户管理和状态控制
- 任务审核
- 申诉处理
- 平台数据统计

### 核心机制
- 📊 **等级升级**: 基于累计积分自动升级（青铜→白银→黄金→钻石）
- 💰 **动态抽成**: 等级越高抽成越低（青铜20%→白银15%→黄金12%→钻石10%）
- ⏰ **超时保护**: 24小时交付期限，超时自动退回
- 🛡️ **资金安全**: 100%预付冻结，验收通过后支付

## 技术栈

| 层级 | 技术选型 |
|------|---------|
| 后端 | Go 1.25 + Gin Web Framework |
| 数据库 | PostgreSQL |
| 认证 | JWT |
| 前端 | HTML + Bootstrap 5 + 原生 JS |
| 架构 | RESTful API `/api/v1/` |

## 快速开始

### 环境要求

- Go 1.21+
- PostgreSQL 13+

### 安装步骤

```bash
# 克隆项目
git clone <repository-url>
cd miao

# 安装依赖
go mod download

# 配置环境变量
cp .env.example .env
# 编辑 .env 设置 JWT_SECRET、DATABASE_URL 等

# 编译
go build -o miao-server cmd/server/main.go

# 运行
./miao-server
```

服务器启动后访问 `http://localhost:8888`

### 部署脚本

```bash
./deploy.sh    # 部署（编译+启动）
./stop.sh      # 停止服务
./restart.sh   # 重启服务
```

## 项目结构

```
miao/
├── cmd/
│   └── server/
│       └── main.go           # 服务器入口
├── internal/
│   ├── config/              # 配置管理
│   ├── database/            # 数据库初始化
│   ├── handler/             # HTTP 处理器
│   ├── middleware/          # 中间件（认证、日志、跨域）
│   ├── model/               # 数据模型
│   ├── repository/          # 数据访问层
│   ├── router/              # 路由配置
│   └── service/             # 业务逻辑层
├── web/
│   ├── static/              # 静态资源
│   │   ├── css/
│   │   ├── js/
│   │   ├── images/
│   │   └── mobile/          # 移动端资源
│   └── templates/           # HTML 模板
│       ├── auth/
│       ├── business/        # 商家端页面
│       ├── creator/         # 创作者页面
│       ├── admin/           # 管理端页面
│       ├── mobile/          # 移动端页面
│       └── ...
├── migrations/              # 数据库迁移
├── docs/                    # 文档
│   ├── API.md              # API 接口文档
│   ├── V1_0_PRD.md         # 产品需求文档
│   └── development-guide.md
├── scripts/                 # 辅助脚本
├── test/                    # 测试文件
├── go.mod
└── README.md
```

## API 文档

完整的 API 接口文档请查看 [docs/API.md](docs/API.md)

### 认证示例

```bash
# 用户注册
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"password123","phone":"13800138000","role":"creator"}'

# 用户登录
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"password123"}'

# 获取任务列表
curl -X GET "http://localhost:8080/api/v1/creator/tasks?page=1&limit=20" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## 创作者等级体系

| 等级 | 保证金 | 日接单上限 | 平台抽成 | 升级条件 |
|------|--------|----------|---------|---------|
| 青铜 | 10元/条 | 3条 | 20% | 新注册+完成新手培训 |
| 白银 | 10元/条 | 10条 | 15% | 完成10单+通过率≥70% |
| 黄金 | 0元 | 20条 | 12% | 总积分≥800+完成50单 |
| 钻石 | 0元 | 50条 | 10% | 总积分≥1500+完成200单 |

**积分规则**: 总积分 = 行为分(-1000~+2000) + 交易分(收入1元=0.1分，封顶500分)

## 开发指南

```bash
# 运行测试
go test ./...

# 运行特定包测试
go test ./internal/handler/

# 查看测试覆盖率
go test -cover ./...
```

### 代码规范

- 使用 `gofmt` 格式化代码
- 遵循 Go 官方代码规范
- 函数和变量使用驼峰命名

## 安全性

- ✅ JWT 认证和授权
- ✅ 密码加密存储（bcrypt）
- ✅ SQL 注入防护（参数化查询）
- ✅ XSS 防护（HTML 转义）
- ✅ CORS 配置
- ✅ 用户状态检查（禁用用户无法登录）
- ✅ 原子操作防止竞态条件

## 部署建议

### 生产环境配置

```bash
export JWT_SECRET="your-production-secret-key"
export DATABASE_URL="postgres://miao:miao@127.0.0.1:5432/miao?sslmode=disable"
export SERVER_PORT=8080
export GIN_MODE=release
```

### systemd 服务（Linux）

详细配置请查看 [docs/deployment-guide.md](docs/deployment-guide.md)

## 文档

- [API 文档](docs/API.md) - 完整的 API 接口说明
- [V1.0 PRD](docs/V1_0_PRD.md) - 产品需求文档
- [开发计划](docs/开发计划.md) - 版本规划和功能清单
- [部署指南](docs/deployment-guide.md) - 生产环境部署详细说明

## 许可证

MIT License
