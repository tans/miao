# 部署与配置

## 配置

复制 `.env.example` 为 `.env`，至少设置以下密钥：

- `POCKETBASE_SUPERUSER_EMAIL` 与 `POCKETBASE_SUPERUSER_PASSWORD`：MIAO 服务端连接 PocketBase 的管理员凭据。
- `AI_GATEWAY_API_KEY`：企业 Vercel AI Gateway 密钥。只注入 `miao` 服务端，不填入网页、不发送给浏览器用户。

`.env` 已加入 Git 忽略规则，不要把真实密钥提交到仓库。改动 AI Gateway 密钥后，重新创建 MIAO 服务容器以载入新环境变量。

## 启动

在仓库根目录执行：

```sh
docker compose up -d --build
```

PocketBase 首次启动时会运行仓库中的 migrations。Compose 只发布 MIAO 的 `41874` 端口，PocketBase 管理 API 留在 Compose 网络内。生产环境应在前置 HTTPS 反向代理后提供 MIAO 服务。

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
