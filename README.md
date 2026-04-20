# 创意喵 - 视频任务平台

创意喵是一个专注于**视频内容生产与交付**的任务平台，连接商家和视频创作者。商家发布视频任务，创作者认领并完成脚本、拍摄、口播、剪辑等交付。平台采用等级制度和信用体系，保障双方权益。

## 功能特性

### 商家端
- ✅ 企业实名认证
- ✅ 发布视频任务
- ✅ 100%预付制度，资金安全
- ✅ 认领审核和交付验收
- ✅ 账户充值和交易记录
- ✅ 任务数据统计

### 创作者端
- ✅ 等级制度（Lv0-Lv5：试用/新手/活跃/优质/金牌/特约）
- ✅ 任务大厅浏览和认领
- ✅ 动态抽成比例（3%-10%）
- ✅ 每日认领限额管理（3-999单）
- ✅ 钱包和收益明细
- ✅ 信用积分系统

### 管理端
- ✅ 用户管理和状态控制
- ✅ 任务审核
- ✅ 申诉处理
- ✅ 平台数据统计

### 核心机制
- 📊 **等级升级**: 基于累计采纳数自动升级（Lv0-Lv5）
- 💰 **动态抽成**: 等级越高抽成越低（Lv0-Lv3: 10%, Lv4: 5%, Lv5: 3%）
- ⏰ **超时保护**: 24小时交付期限，超时自动退回
- 🛡️ **资金安全**: 100%预付冻结，验收通过后支付

## 技术栈

### 后端
- **语言**: Go 1.21+
- **框架**: Gin Web Framework
- **数据库**: SQLite
- **认证**: JWT (JSON Web Token)
- **架构**: RESTful API

### 前端
- **框架**: 原生 HTML/CSS/JavaScript
- **UI库**: Bootstrap 5
- **图表**: Chart.js
- **模板**: Go Template

### 部署
- **服务器**: 单机部署
- **进程管理**: 后台运行
- **日志**: 文件日志

## 快速开始

### 环境要求

- Go 1.21 或更高版本
- SQLite 3

### 安装步骤

1. **克隆项目**
```bash
git clone <repository-url>
cd miao
```

2. **安装依赖**
```bash
go mod download
```

3. **配置环境变量**
```bash
# 创建 .env 文件
cat > .env << EOF
JWT_SECRET=your-secret-key-here
DATABASE_PATH=./data/miao.db
SERVER_PORT=8080
EOF
```

4. **编译项目**
```bash
go build -o miao-server cmd/server/main.go
```

5. **运行服务器**
```bash
./miao-server
```

服务器将在 `http://localhost:8080` 启动。

### 使用部署脚本

项目提供了便捷的部署脚本：

```bash
# 部署（编译+启动）
./deploy.sh

# 停止服务
./stop.sh

# 重启服务
./restart.sh
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
│   ├── middleware/          # 中间件
│   ├── model/               # 数据模型
│   ├── repository/          # 数据访问层
│   ├── router/              # 路由配置
│   └── service/             # 业务逻辑层
├── web/
│   ├── static/              # 静态资源
│   │   ├── css/            # 样式文件
│   │   └── js/             # JavaScript 文件
│   └── templates/           # HTML 模板
│       ├── auth/           # 认证页面
│       ├── creator/        # 创作者页面
│       ├── business/       # 商家页面
│       └── admin/          # 管理员页面
├── migrations/              # 数据库迁移
├── data/                    # 数据目录
├── docs/                    # 文档
│   └── API.md              # API 接口文档
├── deploy.sh               # 部署脚本
├── stop.sh                 # 停止脚本
├── restart.sh              # 重启脚本
├── go.mod                  # Go 模块定义
└── README.md               # 项目说明
```

## API 文档

完整的 API 接口文档请查看 [API.md](docs/API.md)

### 快速示例

**用户注册**
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "password123",
    "phone": "13800138000",
    "role": "creator"
  }'
```

**用户登录**
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "password123"
  }'
```

**获取任务列表**
```bash
curl -X GET "http://localhost:8080/api/v1/creator/tasks?page=1&limit=20" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## 数据库设计

### 核心表结构

- **users**: 用户表（商家、创作者、管理员）
- **tasks**: 任务表
- **claims**: 认领表
- **transactions**: 交易记录表
- **messages**: 消息通知表
- **appeals**: 申诉表

详细的数据库 Schema 请查看 `migrations/schema.sql`

## 业务流程

### 任务发布流程

1. 商家完成企业实名认证
2. 商家发布任务，预付100%金额（冻结）
3. 管理员审核任务
4. 审核通过后任务上线

### 任务认领流程

1. 创作者浏览任务大厅
2. 认领任务（Lv0起即可，无需保证金）
3. 24小时内完成并提交交付
4. 商家验收
5. 验收通过：创作者获得报酬，平台扣除抽成
6. 验收退回：创作者可重新提交

### 等级升级规则

| 等级 | 等级名称 | 累计采纳数 | 每日限额 | 抽成比例 |
|------|---------|-----------|---------|---------|
| Lv0 | 试用创作者 | 0 | 3单 | 10% |
| Lv1 | 新手创作者 | ≥1 | 8单 | 10% |
| Lv2 | 活跃创作者 | ≥5 | 15单 | 10% |
| Lv3 | 优质创作者 | ≥20 | 30单 | 10% |
| Lv4 | 金牌创作者 | ≥50 | 50单 | 5% |
| Lv5 | 特约创作者 | ≥100 | 999单 | 3% |

**升级条件**:
- 基于累计采纳数自动升级
- Lv0起即可认领任务

## 开发指南

### 添加新功能

1. 在 `internal/model/` 定义数据模型
2. 在 `internal/repository/` 实现数据访问
3. 在 `internal/handler/` 实现业务逻辑
4. 在 `internal/router/router.go` 注册路由
5. 在 `web/templates/` 创建前端页面

### 代码规范

- 使用 `gofmt` 格式化代码
- 遵循 Go 官方代码规范
- 函数和变量使用驼峰命名
- 添加必要的注释

### 测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/handler/

# 查看测试覆盖率
go test -cover ./...
```

## 安全性

### 已实现的安全措施

- ✅ JWT 认证和授权
- ✅ 密码加密存储（bcrypt）
- ✅ SQL 注入防护（参数化查询）
- ✅ XSS 防护（HTML 转义）
- ✅ CORS 配置
- ✅ 用户状态检查（禁用用户无法登录）
- ✅ 原子操作防止竞态条件

### 安全建议

- 🔒 生产环境使用强 JWT Secret
- 🔒 启用 HTTPS
- 🔒 配置防火墙规则
- 🔒 定期备份数据库
- 🔒 监控异常登录行为

## 性能优化

### 已实现的优化

- ✅ 数据库索引优化
- ✅ 原子操作减少锁竞争
- ✅ 分页查询减少数据传输
- ✅ 静态资源 CDN 加速（Bootstrap、Chart.js）

### 优化建议

- 📈 添加 Redis 缓存热点数据
- 📈 使用连接池管理数据库连接
- 📈 实现 API 请求限流
- 📈 启用 Gzip 压缩

## 部署建议

### 生产环境配置

1. **环境变量**
```bash
export JWT_SECRET="your-production-secret-key"
export DATABASE_PATH="/var/lib/miao/miao.db"
export SERVER_PORT=8080
export GIN_MODE=release
```

2. **使用 systemd 管理服务**
```ini
[Unit]
Description=Miao Server
After=network.target

[Service]
Type=simple
User=miao
WorkingDirectory=/opt/miao
ExecStart=/opt/miao/miao-server
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

3. **Nginx 反向代理**
```nginx
server {
    listen 80;
    server_name miao.example.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## 监控和日志

### 日志文件

- `server.log`: 服务器运行日志
- 包含请求日志、错误日志、业务日志

### 健康检查

```bash
curl http://localhost:8080/health
```

## 常见问题

### Q: 如何重置管理员密码？

A: 直接修改数据库中的用户记录，使用 bcrypt 加密新密码。

### Q: 如何备份数据？

A: 复制 `data/miao.db` 文件即可。

### Q: 如何迁移到 MySQL/PostgreSQL？

A: 修改 `internal/database/database.go` 中的数据库驱动和连接字符串。

### Q: 前端如何处理 Token 过期？

A: 监听 401 响应，清除本地 Token 并跳转到登录页。

## 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 本项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

## 联系方式

- 项目主页: https://github.com/yourusername/miao
- 问题反馈: https://github.com/yourusername/miao/issues

## 致谢

- [Gin](https://github.com/gin-gonic/gin) - Web 框架
- [Bootstrap](https://getbootstrap.com/) - UI 框架
- [Chart.js](https://www.chartjs.org/) - 图表库
- [jwt-go](https://github.com/golang-jwt/jwt) - JWT 实现

---

**创意喵** - 让创意变现更简单 🐱
