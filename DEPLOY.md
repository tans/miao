# 创意喵平台部署指南

## 快速开始

### 开发环境

```bash
# 1. 克隆代码
git clone <repository-url>
cd miao

# 2. 安装依赖
go mod download

# 3. 设置环境变量（可选）
export JWT_SECRET="your-secret-key"
export DB_PATH="./data/miao.db"
export SERVER_PORT=8080

# 4. 运行开发服务器（前台运行）
./deploy.sh dev
```

### 生产环境

```bash
# 1. 部署到生产环境
./deploy.sh prod

# 2. 查看日志
tail -f logs/server.log

# 3. 停止服务
./stop.sh

# 4. 重启服务
./restart.sh
```

## 部署脚本说明

### deploy.sh - 部署脚本

支持三种环境：
- `dev`: 开发环境（前台运行，debug模式）
- `staging`: 预发布环境（后台运行，info日志）
- `prod`: 生产环境（后台运行，优化编译，warn日志）

功能：
- ✅ 检查 Go 环境
- ✅ 拉取最新代码（生产环境）
- ✅ 安装依赖
- ✅ 运行测试
- ✅ 编译应用
- ✅ 备份旧版本（生产环境）
- ✅ 停止旧服务
- ✅ 启动新服务
- ✅ 健康检查

### stop.sh - 停止服务

优雅停止服务，先发送 TERM 信号，等待10秒后强制停止。

### restart.sh - 重启服务

停止并重新启动服务，包含健康检查。

## 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `JWT_SECRET` | JWT 密钥（必须设置） | 随机生成 |
| `DB_PATH` | 数据库路径 | `./data/miao.db` |
| `SERVER_PORT` | 服务端口 | `8080` |
| `GIN_MODE` | Gin 模式 | `release` |
| `LOG_LEVEL` | 日志级别 | `info` |

## 目录结构

```
miao/
├── cmd/server/          # 服务入口
├── internal/            # 内部代码
│   ├── config/         # 配置
│   ├── database/       # 数据库
│   ├── handler/        # 处理器
│   ├── middleware/     # 中间件
│   ├── model/          # 模型
│   ├── repository/     # 数据访问层
│   ├── router/         # 路由
│   └── service/        # 业务逻辑
├── web/                # 前端资源
│   ├── static/         # 静态文件
│   └── templates/      # 模板
├── data/               # 数据目录
├── logs/               # 日志目录
├── uploads/            # 上传文件目录
├── backups/            # 备份目录
├── deploy.sh           # 部署脚本
├── stop.sh             # 停止脚本
└── restart.sh          # 重启脚本
```

## 生产环境部署建议

### 1. 使用 systemd 管理服务

创建 `/etc/systemd/system/miao.service`:

```ini
[Unit]
Description=Miao Creative Platform
After=network.target

[Service]
Type=simple
User=miao
WorkingDirectory=/opt/miao
Environment="JWT_SECRET=your-secret-key"
Environment="DB_PATH=/opt/miao/data/miao.db"
Environment="SERVER_PORT=8080"
ExecStart=/opt/miao/miao-server
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

启动服务：
```bash
sudo systemctl daemon-reload
sudo systemctl enable miao
sudo systemctl start miao
sudo systemctl status miao
```

### 2. 使用 Nginx 反向代理

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /static/ {
        alias /opt/miao/web/static/;
        expires 30d;
    }

    location /uploads/ {
        alias /opt/miao/uploads/;
        expires 7d;
    }
}
```

### 3. 数据库备份

```bash
# 创建备份脚本
cat > backup.sh << 'EOF'
#!/bin/bash
BACKUP_DIR="backups/$(date +%Y%m%d)"
mkdir -p $BACKUP_DIR
sqlite3 data/miao.db ".backup $BACKUP_DIR/miao.db"
echo "Backup completed: $BACKUP_DIR/miao.db"
EOF

chmod +x backup.sh

# 添加到 crontab（每天凌晨2点备份）
crontab -e
# 添加: 0 2 * * * cd /opt/miao && ./backup.sh
```

### 4. 日志轮转

创建 `/etc/logrotate.d/miao`:

```
/opt/miao/logs/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0644 miao miao
    postrotate
        systemctl reload miao
    endscript
}
```

## 监控和维护

### 查看服务状态
```bash
# systemd 方式
sudo systemctl status miao

# 脚本方式
ps aux | grep miao-server
```

### 查看日志
```bash
# 实时日志
tail -f logs/server.log

# 错误日志
grep ERROR logs/server.log

# systemd 日志
sudo journalctl -u miao -f
```

### 性能监控
```bash
# CPU 和内存使用
top -p $(cat miao.pid)

# 网络连接
netstat -anp | grep :8080
```

## 故障排查

### 服务无法启动
1. 检查端口是否被占用: `lsof -i:8080`
2. 检查日志: `tail -f logs/server.log`
3. 检查环境变量是否正确设置
4. 检查数据库文件权限

### 数据库错误
1. 检查数据库文件是否存在: `ls -lh data/miao.db`
2. 检查数据库权限
3. 尝试重新运行迁移

### 内存泄漏
1. 监控内存使用: `top -p $(cat miao.pid)`
2. 使用 pprof 分析: `go tool pprof http://localhost:8080/debug/pprof/heap`

## 更新部署

```bash
# 1. 拉取最新代码
git pull origin main

# 2. 重新部署
./deploy.sh prod

# 3. 验证服务
curl http://localhost:8080/health
```

## 回滚

```bash
# 1. 停止当前服务
./stop.sh

# 2. 恢复备份
cp backups/YYYYMMDD_HHMMSS/miao-server ./
cp backups/YYYYMMDD_HHMMSS/miao.db data/

# 3. 启动服务
./restart.sh
```
