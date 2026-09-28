# 部署与配置

## 安装和启动

运行环境需要 Bun、系统安装的 PM2、curl、unzip 和 openssl。脚本支持 Linux/macOS 的 x64 和 ARM64；安装时下载固定版本 PocketBase 0.40.4，并按 `bun.lock` 安装 Bun 依赖。系统 PM2 管理 PocketBase 和 MIAO 应用进程。

```sh
bun run server:install
bun run server:start
```

安装会构建 fx 浏览器资源。启动时先初始化 PocketBase，再由 PM2 分别托管 PocketBase 和 Bun 服务；关闭终端不会停止服务。PocketBase 管理员邮箱和随机密码写入权限为 `600` 的本地配置文件。安装输出会告诉你配置文件路径。

```sh
bun run server:status
./scripts/logs.sh miao-platform
bun run server:stop
```

`server:logs` 可接收进程名（`miao-platform` 或 `miao-pocketbase`）和行数。`start.sh` 会保存 PM2 进程清单。PM2 自动重启异常退出的进程；当前没有安装系统级开机启动项。

## 数据目录和配置

- Linux 默认使用 `$XDG_DATA_HOME/miao`，未设置 `XDG_DATA_HOME` 时使用 `~/.local/share/miao`。
- macOS 默认使用 `~/Library/Application Support/Miao/data`。
- 管理员可以在安装前设置 `MIAO_DATA_DIR=/srv/miao-data`，或修改安装目录下的 `miao.env` 中的 `MIAO_DATA_DIR`。该目录保存 PocketBase 数据、文件、应用日志及运行期生成的 migration。
- PM2 使用当前系统用户默认的 PM2 daemon 和进程清单；`pm2 list` 可查看 MIAO 与该用户的其他 PM2 服务。应用数据目录可以包含空格。
- PocketBase 默认监听 `127.0.0.1:8090`，不直接暴露到网络；MIAO 默认监听 `0.0.0.0:41874`。使用 `POCKETBASE_PORT`、`MIAO_PORT`、`HOST` 可修改端口和监听地址。
- `POCKETBASE_SUPERUSER_EMAIL` 和 `POCKETBASE_SUPERUSER_PASSWORD` 是 MIAO 服务端使用的 PocketBase 管理员凭据。重新启动时会确保管理员密码与配置一致。
- `AI_GATEWAY_API_KEY` 是可选的企业 Vercel AI Gateway 密钥，只注入服务端，不会发送给浏览器。
- `MIAO_PUBLIC_URL` 配置邮件验证和密码重置链接的公网根地址；`RESEND_API_KEY` 与 `MIAO_MAIL_FROM` 配置 Resend 邮件发送。
- `MIAO_REQUIRE_EMAIL_VERIFICATION=true` 要求新账号验证邮箱后才能使用；`MIAO_REGISTRATION_MODE` 可设为 `open`、`invite` 或 `closed`（默认 `open`）；`MIAO_ALLOWED_EMAIL_DOMAINS` 可用逗号分隔限制注册域名。
- `MIAO_ADMIN_EMAILS` 配置可管理平台级 AI Gateway 密钥的管理员邮箱（逗号分隔）；管理界面保存的密钥使用 `MIAO_SETTINGS_ENCRYPTION_KEY` 加密，此密钥至少 32 个字符，必须长期保管并通过安全配置渠道注入。
- `MIAO_BACKUP_DIR` 与 `MIAO_BACKUP_RETENTION_DAYS` 可设置 PocketBase 备份目录和保留天数；默认位于数据目录的 `backups` 子目录并保留 30 天。备份脚本复用 PocketBase superuser 凭据。

配置文件是 Bun dotenv 格式。编辑后重新运行 `start.sh` 即可载入新配置。请限制配置文件访问权限，不要将真实密钥提交到仓库。

生产环境通过 PM2 管理服务，并通过 HTTPS 反向代理提供 MIAO。运行账户需要对 MIAO 安装目录、配置文件和数据目录有读写权限。

## 工作区邀请

工作区所有者在侧栏打开“成员与邀请”，输入同事邮箱并生成邀请链接。链接有效期为 72 小时，只能接受一次；被邀请者必须使用该邮箱注册或登录。MIAO 当前生成并复制链接，不负责发送邮件。所有者可以撤销待接受邀请或移除成员。

用户可以在工作区选择器中切换自己拥有或受邀加入的空间。每个业务 API 和 AI 代理请求都按当前工作区重新校验成员权限。

## AI Gateway 请求限制与预算

MIAO 服务端只代理 fx 所需的固定 AI Gateway 路径，并以服务端配置的 Gateway 密钥添加上游认证。平台管理员可在界面保存加密密钥或回退至环境变量。平台配置密钥需要稳定保管 `MIAO_SETTINGS_ENCRYPTION_KEY`；更换此密钥前应先迁移/重新保存加密设置。

用户每分钟、工作区每分钟和平台每分钟的请求计数以及工作区每日请求预算保存在 PocketBase 中，可由工作区所有者查看用量和设置预算。该预算按请求计数，不等于供应商账单金额；token 用量取决于 Gateway 响应是否提供可识别的 usage 信息。实际费用仍应与 Gateway 账单核对。

`bun run backup` 创建完整 PocketBase 归档；`bun run restore -- <归档路径> --confirm` 会替换运行数据。配置计划任务和异地复制，并按[备份与恢复手册](BACKUP.md)演练。MIAO 不会自行创建计划任务或异地副本。

邮件验证、密码重置和工作区邀请使用 Resend：配置 `MIAO_PUBLIC_URL`、`RESEND_API_KEY` 和 `MIAO_MAIL_FROM`。未配置邮件服务时，验证/重置邮件不会发送；邀请接口仍返回可复制的邀请链接。
