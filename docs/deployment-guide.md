# 创意喵平台部署指南

## 生产环境持续运行方案

### 方案 1: systemd（推荐 - Linux）

创建 systemd 服务，开机自启、崩溃自动重启。

**1. 编译生产版本**

```bash
make build
# 生成 bin/miao
```

**2. 创建 systemd 服务文件**

```bash
sudo nano /etc/systemd/system/miao.service
```

```ini
[Unit]
Description=Creative Meow Platform
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/miao
ExecStart=/opt/miao/bin/miao
Restart=always
RestartSec=5
StandardOutput=append:/var/log/miao/access.log
StandardError=append:/var/log/miao/error.log

# 环境变量
Environment="GIN_MODE=release"
Environment="SERVER_PORT=8888"

# 安全加固
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

**3. 部署和启动**

```bash
# 创建目录
sudo mkdir -p /opt/miao /var/log/miao
sudo chown www-data:www-data /opt/miao /var/log/miao

# 复制文件
sudo cp -r bin/ web/ /opt/miao/
sudo cp miao.db /opt/miao/  # 如果需要

# 启动服务
sudo systemctl daemon-reload
sudo systemctl enable miao  # 开机自启
sudo systemctl start miao

# 查看状态
sudo systemctl status miao

# 查看日志
sudo journalctl -u miao -f
```

**4. 管理命令**

```bash
sudo systemctl start miao    # 启动
sudo systemctl stop miao     # 停止
sudo systemctl restart miao  # 重启
sudo systemctl status miao   # 状态
```

---

### 方案 2: Docker（推荐 - 跨平台）

使用 Docker 容器化部署，隔离环境、易于迁移。

**1. 创建 Dockerfile**

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o miao ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates sqlite

WORKDIR /app
COPY --from=builder /app/miao .
COPY --from=builder /app/web ./web

EXPOSE 8888
CMD ["./miao"]
```

**2. 创建 docker-compose.yml**

```yaml
version: '3.8'

services:
  miao:
    build: .
    container_name: miao
    restart: always
    ports:
      - "8888:8888"
    volumes:
      - ./data:/app/data
      - ./logs:/app/logs
    environment:
      - GIN_MODE=release
      - SERVER_PORT=8888
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8888/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

**3. 部署**

```bash
# 构建和启动
docker-compose up -d

# 查看日志
docker-compose logs -f

# 重启
docker-compose restart

# 停止
docker-compose down
```

---

### 方案 3: Supervisor（简单 - Linux）

轻量级进程管理工具。

**1. 安装 Supervisor**

```bash
sudo apt install supervisor  # Ubuntu/Debian
sudo yum install supervisor  # CentOS/RHEL
```

**2. 创建配置文件**

```bash
sudo nano /etc/supervisor/conf.d/miao.conf
```

```ini
[program:miao]
command=/opt/miao/bin/miao
directory=/opt/miao
user=www-data
autostart=true
autorestart=true
startretries=3
redirect_stderr=true
stdout_logfile=/var/log/miao/access.log
stderr_logfile=/var/log/miao/error.log
environment=GIN_MODE="release",SERVER_PORT="8888"
```

**3. 启动**

```bash
sudo supervisorctl reread
sudo supervisorctl update
sudo supervisorctl start miao

# 管理命令
sudo supervisorctl status miao
sudo supervisorctl restart miao
sudo supervisorctl stop miao
```

---

### 方案 4: nohup（最简单 - 临时方案）

后台运行，不推荐生产环境长期使用。

```bash
# 启动
nohup ./bin/miao > logs/miao.log 2>&1 &

# 查看进程
ps aux | grep miao

# 停止
pkill -f miao
# 或
kill $(cat miao.pid)
```

---

### 方案 5: PM2（Node.js 生态）

虽然 PM2 主要用于 Node.js，但也支持其他二进制程序。

```bash
# 安装 PM2
npm install -g pm2

# 启动
pm2 start bin/miao --name miao

# 管理
pm2 list
pm2 logs miao
pm2 restart miao
pm2 stop miao

# 开机自启
pm2 startup
pm2 save
```

---

## 推荐方案对比

| 方案 | 优点 | 缺点 | 适用场景 |
|------|------|------|----------|
| **systemd** | 系统原生、稳定、开机自启 | 仅限 Linux | Linux 生产环境（推荐） |
| **Docker** | 隔离环境、易迁移、跨平台 | 需要学习 Docker | 容器化部署、微服务 |
| **Supervisor** | 简单易用、配置灵活 | 需要额外安装 | 小型项目、快速部署 |
| **nohup** | 无需安装、极简 | 不稳定、难管理 | 临时测试、开发环境 |
| **PM2** | 功能丰富、监控面板 | Node.js 依赖 | 已有 Node.js 环境 |

---

## 生产环境最佳实践

### 1. 反向代理（Nginx）

```nginx
# /etc/nginx/sites-available/miao
server {
    listen 80;
    server_name miao.example.com;

    location / {
        proxy_pass http://localhost:8888;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 静态文件直接服务
    location /static/ {
        alias /opt/miao/web/static/;
        expires 30d;
    }
}
```

### 2. HTTPS（Let's Encrypt）

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d miao.example.com
```

### 3. 日志轮转

```bash
# /etc/logrotate.d/miao
/var/log/miao/*.log {
    daily
    rotate 14
    compress
    delaycompress
    notifempty
    create 0640 www-data www-data
    sharedscripts
    postrotate
        systemctl reload miao
    endscript
}
```

### 4. 监控和告警

```bash
# 健康检查端点
curl http://localhost:8888/health

# 使用 Prometheus + Grafana 监控
# 或简单的 cron 脚本
*/5 * * * * curl -f http://localhost:8888/health || systemctl restart miao
```

### 5. 数据库备份

```bash
# 每日备份 SQLite
0 2 * * * cp /opt/miao/miao.db /backup/miao-$(date +\%Y\%m\%d).db
```

---

## 快速部署脚本

创建 `deploy.sh`：

```bash
#!/bin/bash
set -e

echo "==> 编译生产版本"
make build

echo "==> 停止旧服务"
sudo systemctl stop miao || true

echo "==> 备份数据库"
sudo cp /opt/miao/miao.db /opt/miao/miao.db.backup || true

echo "==> 部署新版本"
sudo cp bin/miao /opt/miao/bin/
sudo cp -r web/ /opt/miao/

echo "==> 启动服务"
sudo systemctl start miao

echo "==> 检查状态"
sleep 2
sudo systemctl status miao

echo "✓ 部署完成"
```

使用：

```bash
chmod +x deploy.sh
./deploy.sh
```

---

## 故障排查

### 服务无法启动

```bash
# 查看详细日志
sudo journalctl -u miao -n 50 --no-pager

# 检查端口占用
sudo lsof -i :8888

# 检查文件权限
ls -la /opt/miao/bin/miao
```

### 性能问题

```bash
# 查看资源使用
top -p $(pgrep miao)

# 查看连接数
ss -tnp | grep :8888

# 数据库优化
sqlite3 miao.db "PRAGMA optimize;"
```

### 内存泄漏

```bash
# 定期重启（临时方案）
0 3 * * * systemctl restart miao

# 使用 pprof 分析
# 在代码中添加 pprof 端点后
go tool pprof http://localhost:6060/debug/pprof/heap
```

---

## Makefile 集成

已在 `Makefile` 中添加部署相关命令：

```bash
make build          # 编译生产版本
make deploy         # 部署到生产环境（需配置）
make logs           # 查看生产日志
make restart        # 重启生产服务
```
