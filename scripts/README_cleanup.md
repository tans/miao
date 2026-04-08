# 上传文件清理脚本

定期清理旧的上传文件，释放磁盘空间。

## 使用方法

```bash
# 清理 7 天前的文件（默认）
go run scripts/cleanup_uploads.go

# 清理 30 天前的文件
go run scripts/cleanup_uploads.go 30

# 清理 1 天前的文件（测试环境）
go run scripts/cleanup_uploads.go 1
```

## 定时任务

可以使用 cron 定期执行：

```bash
# 每天凌晨 2 点清理 7 天前的文件
0 2 * * * cd /path/to/miao && go run scripts/cleanup_uploads.go 7 >> logs/cleanup.log 2>&1
```

## 注意事项

- 脚本会递归扫描 `web/static/uploads` 目录
- 只删除文件，不删除目录
- 根据文件修改时间判断是否过期
- 删除前会显示文件信息
