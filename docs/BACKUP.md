# PocketBase 备份与恢复

## 备份策略

`bun run backup` 使用 PocketBase 内置备份接口生成完整数据归档（数据库、文件和集合结构），下载到 `$MIAO_BACKUP_DIR`；默认目录为 `$MIAO_DATA_DIR/backups`。归档文件权限为仅当前操作系统用户可读写。默认保留 30 天，可配置 `MIAO_BACKUP_RETENTION_DAYS` 与 `MIAO_BACKUP_DIR`。

建议每天至少备份一次，并将已完成的归档复制到另一台主机或对象存储。仅在同一块服务器磁盘保存备份不能应对磁盘损坏、勒索软件或整机丢失。备份文件包含用户数据和上传文件，应按生产数据同等保护。

Linux 可用 cron 每天凌晨 02:20 运行（将路径替换成安装目录）：

```cron
20 2 * * * cd /path/to/miao && bun run backup >> /var/log/miao-backup.log 2>&1
```

## 恢复

恢复会替换正在运行的 PocketBase 数据，并重启 PocketBase。先停止 MIAO 写入流量，确认恢复归档和 MIAO/PocketBase 版本兼容，再在隔离的 staging 实例演练。恢复前必须另外保存当前数据的归档。

```sh
bun run restore -- /path/to/miao_backup_20260929022000z.zip --confirm
```

恢复后检查 PocketBase `/api/health`、用户登录、工作区切换、应用/表/附件读取、记录新增与编辑，以及 migration 状态。确认正常后恢复 MIAO 流量。若恢复失败，保留原文件和日志；不要直接删除旧的数据目录。

## 演练和保留

- 每季度至少在隔离实例恢复一份近期归档，记录开始时间、完成时间和抽样校验结果。
- 定期确认异地复制完成、归档大小合理、文件可读取，并覆盖已有附件数据。
- 生产保留期默认 30 天；更长保留和法律保全要求由运营方按企业策略调整。
- `MIAO_SETTINGS_ENCRYPTION_KEY` 必须与备份一同安全保管；恢复管理界面保存的 AI Gateway 密钥时需要相同的加密密钥。
- 账号删除、删除应用/数据表和删除字段属于在线永久删除。备份会按保留期限暂时保留此前快照；到期后归档清理会永久删除这些历史数据。

脚本依赖 `POCKETBASE_SUPERUSER_EMAIL`、`POCKETBASE_SUPERUSER_PASSWORD`，可通过安装配置 `miao.env` 提供。PocketBase 的备份创建和恢复由其官方 superuser 备份 API 执行。
