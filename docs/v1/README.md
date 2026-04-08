# 创意喵平台 V1 快速开始

## 项目简介

创意喵是一个连接商家和创作者的视频任务撮合平台。商家发布视频制作任务，创作者认领并完成，平台提供资金托管和信用评级。

**V1 版本**: 核心功能 MVP，支持完整的任务发布→认领→提交→审核流程。

## 技术栈

- **后端**: Go 1.21+ / Gin
- **数据库**: SQLite
- **前端**: HTML / Bootstrap 5
- **认证**: JWT

## 快速启动

### 1. 环境要求

- Go 1.21 或更高版本
- 无需额外依赖

### 2. 启动服务

```bash
# 克隆项目
cd miao

# 安装依赖
go mod download

# 启动服务
go run cmd/server/main.go
```

服务将在 `http://localhost:8888` 启动。

### 3. 访问应用

- **首页**: http://localhost:8888/
- **登录**: http://localhost:8888/auth/login.html
- **注册**: http://localhost:8888/auth/register.html
- **商家端**: http://localhost:8888/business/dashboard.html
- **创作者端**: http://localhost:8888/creator/dashboard.html
- **健康检查**: http://localhost:8888/health

## 核心功能

### 商家端
1. 注册商家账号
2. 充值账户余额
3. 发布视频任务（标题、描述、单价、数量）
4. 查看任务列表和投稿
5. 审核创作者投稿
6. 选中获胜作品

### 创作者端
1. 注册创作者账号
2. 浏览任务大厅
3. 认领感兴趣的任务
4. 提交作品链接
5. 查看审核结果
6. 查看收益

## 测试账号

### 方式一：使用现有测试账号

```bash
# 商家账号
用户名: test_biz_1775665314
密码: test123456

# 创作者账号
用户名: test_creator_1775665326
密码: test123456
```

### 方式二：注册新账号

访问 http://localhost:8888/auth/register.html

- **商家**: 选择角色 "商家"
- **创作者**: 选择角色 "创作者"

## API 文档

完整 API 文档请查看: [API.md](API.md)

### 核心端点

```bash
# 注册
POST /api/v1/auth/register
{
  "username": "test_user",
  "password": "test123456",
  "phone": "13800138000",
  "role": 2  // 2=商家, 3=创作者
}

# 登录
POST /api/v1/auth/login
{
  "username": "test_user",
  "password": "test123456"
}

# 创建任务（需要 Bearer token）
POST /api/v1/business/tasks
Authorization: Bearer <token>
{
  "title": "视频制作任务",
  "description": "需要制作一个产品介绍视频",
  "unit_price": 100.0,
  "total_count": 5
}

# 认领任务
POST /api/v1/creator/claim
Authorization: Bearer <token>
{
  "task_id": 1,
  "quantity": 1
}
```

## 完整流程示例

### 1. 商家发布任务

```bash
# 注册商家
curl -X POST http://localhost:8888/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username": "business1", "password": "test123456", "phone": "13800138001", "role": 2}'

# 登录获取 token
TOKEN=$(curl -X POST http://localhost:8888/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "business1", "password": "test123456"}' | jq -r '.data.token')

# 充值
curl -X POST http://localhost:8888/api/v1/business/recharge \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"amount": 1000}'

# 创建任务
curl -X POST http://localhost:8888/api/v1/business/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title": "产品视频", "description": "制作产品介绍视频", "unit_price": 100, "total_count": 5}'
```

### 2. 创作者认领并提交

```bash
# 注册创作者
curl -X POST http://localhost:8888/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username": "creator1", "password": "test123456", "phone": "13900139001", "role": 3}'

# 登录
CREATOR_TOKEN=$(curl -X POST http://localhost:8888/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "creator1", "password": "test123456"}' | jq -r '.data.token')

# 查看任务
curl http://localhost:8888/api/v1/tasks \
  -H "Authorization: Bearer $CREATOR_TOKEN"

# 认领任务（需要先审核任务上架）
curl -X POST http://localhost:8888/api/v1/creator/claim \
  -H "Authorization: Bearer $CREATOR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"task_id": 1, "quantity": 1}'

# 提交作品
curl -X PUT http://localhost:8888/api/v1/creator/claim/1/submit \
  -H "Authorization: Bearer $CREATOR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content": "https://example.com/video.mp4", "description": "已完成"}'
```

### 3. 商家审核

```bash
# 查看投稿
curl http://localhost:8888/api/v1/business/claim/1 \
  -H "Authorization: Bearer $TOKEN"

# 审核通过
curl -X PUT http://localhost:8888/api/v1/business/claim/1/review \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"result": 1, "comment": "作品很好"}'
```

## 配置

### 环境变量

```bash
# 服务端口（默认 8888）
export SERVER_PORT=8888

# 数据库路径（默认 ./data/miao.db）
export DB_PATH=./data/miao.db

# JWT 密钥（生产环境必须设置）
export JWT_SECRET=your-secret-key
```

### 数据库

SQLite 数据库位于 `./data/miao.db`，首次启动时自动创建。

## 项目结构

```
miao/
├── cmd/server/main.go          # 入口文件
├── internal/
│   ├── config/                 # 配置
│   ├── database/               # 数据库初始化
│   ├── handler/                # API 处理器
│   ├── middleware/             # 中间件（认证、CORS）
│   ├── model/                  # 数据模型
│   ├── repository/             # 数据访问层
│   ├── service/                # 业务逻辑层
│   └── router/                 # 路由配置
├── web/
│   ├── static/                 # 静态资源
│   └── templates/              # HTML 模板
│       ├── auth/               # 登录注册
│       ├── business/           # 商家端页面
│       ├── creator/            # 创作者端页面
│       └── admin/              # 管理端页面
├── data/                       # 数据库文件
└── _workspace_v1/              # V1 验证工作目录
```

## 开发

### 热重载

```bash
# 安装 air
go install github.com/cosmtrek/air@latest

# 启动热重载
air
```

### 运行测试

```bash
go test ./...
```

## 已知限制（V1）

1. **任务审核**: 商家创建任务后需要管理员审核才能上架（测试时可直接更新数据库）
2. **文件上传**: 暂不支持实际文件上传，使用 URL 链接
3. **支付系统**: 充值为模拟操作，不对接真实支付
4. **消息通知**: 基础实现，无实时推送
5. **管理端**: 功能有限

## 故障排查

### 端口被占用

```bash
# 查看占用进程
lsof -ti:8888

# 杀死进程
kill $(lsof -ti:8888)
```

### 数据库锁定

```bash
# 删除数据库重新初始化
rm data/miao.db
go run cmd/server/main.go
```

### JWT 认证失败

检查 token 是否正确设置在 `Authorization: Bearer <token>` header 中。

## 下一步

- 查看 [API.md](API.md) 了解完整 API 文档
- 查看 [USER_GUIDE.md](USER_GUIDE.md) 了解详细使用指南
- 查看 [KNOWN_ISSUES.md](KNOWN_ISSUES.md) 了解已知问题

## 支持

- 项目地址: https://github.com/tans/miao
- 问题反馈: 提交 Issue

## 许可证

MIT License
