# 部署与配置

## 安装和启动

运行环境需要 Bun、curl、unzip 和 openssl。脚本支持 Linux/macOS 的 x64 和 ARM64；安装时下载固定版本 PocketBase 0.40.4，并按 `bun.lock` 安装 Bun 依赖。

```sh
./scripts/install.sh
./scripts/start.sh
```

`start.sh` 在前台启动 PocketBase 和 Bun 服务，按 `Ctrl-C` 会一并停止。PocketBase 首次启动会应用仓库 migration；PocketBase 管理员邮箱和随机密码写入权限为 `600` 的本地配置文件。安装输出会告诉你配置文件路径。

## 数据目录和配置

- Linux 默认使用 `$XDG_DATA_HOME/miao`，未设置 `XDG_DATA_HOME` 时使用 `~/.local/share/miao`。
- macOS 默认使用 `~/Library/Application Support/Miao/data`。
- 管理员可以在安装前设置 `MIAO_DATA_DIR=/srv/miao-data`，或修改安装目录下的 `miao.env` 中的 `MIAO_DATA_DIR`。该目录保存 PocketBase 数据、文件、日志及运行期生成的 migration。
- PocketBase 默认监听 `127.0.0.1:8090`，不直接暴露到网络；MIAO 默认监听 `0.0.0.0:41874`。使用 `POCKETBASE_PORT`、`MIAO_PORT`、`HOST` 可修改端口和监听地址。
- `POCKETBASE_SUPERUSER_EMAIL` 和 `POCKETBASE_SUPERUSER_PASSWORD` 是 MIAO 服务端使用的 PocketBase 管理员凭据。重新启动时会确保管理员密码与配置一致。
- `AI_GATEWAY_API_KEY` 是可选的企业 Vercel AI Gateway 密钥，只注入服务端，不会发送给浏览器。

配置文件是 Bun dotenv 格式。编辑后重新运行 `start.sh` 即可载入新配置。请限制配置文件访问权限，不要将真实密钥提交到仓库。

生产环境可用 systemd、launchd 或其他进程管理器托管 `scripts/start.sh`，并通过 HTTPS 反向代理提供 MIAO 服务。运行账户需要对 MIAO 安装目录、配置文件和数据目录有读写权限。

## 工作区邀请

工作区所有者在侧栏打开“成员与邀请”，输入同事邮箱并生成邀请链接。链接有效期为 72 小时，只能接受一次；被邀请者必须使用该邮箱注册或登录。MIAO 当前生成并复制链接，不负责发送邮件。所有者可以撤销待接受邀请或移除成员。

用户可以在工作区选择器中切换自己拥有或受邀加入的空间。每个业务 API 和 AI 代理请求都按当前工作区重新校验成员权限。

## AI Gateway 请求限制

MIAO 服务端只代理 fx 所需的固定 AI Gateway 路径，并以服务端配置的 `AI_GATEWAY_API_KEY` 添加上游认证。当前速率限制为每用户每分钟 30 次、每个 MIAO 进程每分钟 120 次。计数保存在进程内存中，重启或多实例部署不会共享限额；这不是持久化的费用预算系统。

## 尚未覆盖的运维项

- PocketBase 数据的定期异地备份与恢复演练。
- 邮件服务、邀请通知和企业管理员 UI。
- 多实例共享的 AI 用量计量与费用上限。
- 注册邮箱验证、密码重置和账号停用流程。
- systemd/launchd 单元模板及备份恢复自动化。
