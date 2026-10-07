# 推进 #61「应用界面:真实绑定、预览发布与持续编辑」

## 关键判断(代码审计结论)

#61 正文写于 10-06,其后 main 已合入大量相关工作(PR #74 及 16 个后续提交)。对照代码,**第一轮 8 条几乎全部已有实现**:

| #61 第一轮条目 | 已有实现(证据) |
|---|---|
| 受控组件 catalog/registry | `public/modules/ui-renderer.jsx:77`(Page/Section/Text/Metric/RecordTable/RecordCards/RecordDetail/RecordForm registry);`internal/httpapi/versions.go:81` validateAppUIDefinition/validateUISources/validateUISpec |
| 按表/字段/关联生成绑定 | `internal/httpapi/app_builder.go:323` mergeUIDefinition:按表生成列表+表单+详情页,select 字段自动生成状态动作,关联表自动加详情数据源 |
| 草稿→只读真实预览→差异→确认发布 | `versions.go:1232/1097/1461`(previewVersion read_only=true、diffVersion、publishVersion)+ CAS 防冲突(expected_latest/published_version_id);发布状态与版本独立(publications) |
| 加字段/改名/筛选保留数据 | mergeUIDefinition 追加新字段到列表列/表单绑定、仅重命名仍为生成默认值的标签;`ui_composer.go` fields/form_fields/query 受控编辑 |
| CMS 公开内容+SEO | `publications.go:62` servePublicSite/publicHTML/publicImage/publicRecordFields;`public_markup.go` sanitizePublicMarkup;`public/site.js` og/canonical 元信息 |
| 采集界面 | `collection_scripts.go` enable/pause/preview(试运行)/run/runs 端点;`public/modules/collection-results.js`、`app-tasks.js` 展示来源/版本/状态/记录/通知 |
| 服务端校验引用 | validateBusinessActionReferences + validateUIRecordContexts(跨应用/失效引用拒绝、失败保留草稿) |
| 已发布 UI 确定性读写 | `versions.go:26` publishedRuntime + runRuntimeAction,不经模型 |

另外:三份模板(crm/cms/collection.json)已存在;对话确定性模板路径("crm:xxx")无模型可用(`app_builder.go:725`);PR #74 已合并(ec9e78c)但 #72/#73 仍 open。

**结论:推进 #61 = 真实走查取证 + 修复走查发现的缺口 + 回写 issue,而非从零开发。**

## 实施阶段

**阶段 0 — 跟踪器同步**:#73 勾掉"PR #74 评审合并"项并评论合并 SHA(ec9e78c),关闭 #72/#73。

**阶段 1 — #61 逐条审计评论**:把上表整理成正式审计发到 #61(代码证据 + 提交 + 验证方式),标注"已实现、待现场验证",不提前勾选。

**阶段 2 — CRM 主链端到端走查**(本地 PM2 服务,`npm run server:start`,凭据用 `.local/admin-credentials.txt`):
- 新建测试工作区 → 对话输入"crm:团队客户"(确定性模板,无模型)→ 审阅计划 → 落地表 → ui.compose 生成界面草稿 → 只读真实预览 → 查看差异 → 确认发布
- 已发布应用实测:新增记录(含成员负责人)、列表/搜索、详情、编辑、状态动作、删除
- 三角色边界:临时邀请管理员/编辑者/只读者,验证只读写入拒绝
- 持续修改:加字段 → UI 自动合并(新列表列/表单绑定)→ 发布;改显示名称、筛选条件;确认既有记录保留
- 全部临时数据清理(删记录/成员/应用)

**阶段 3 — CMS 公开链路**:"cms:产品官网" → 内容草稿/发布 → 匿名 curl/浏览器访问 `/s/{slug}`:列表/详情、草稿与私有字段不可见、图片显式公开、正文净化 HTML、SEO title/description/canonical。

**阶段 4 — 采集界面**:"collection:采集发现" → 连接器(静态本地来源)+ 采集脚本 → 只读试运行 → 立即运行 → 入库、站内通知、重跑去重、暂停。

**阶段 5 — 缺口修复**:走查发现的缺陷按最小修复,每个修复独立 commit;每个 commit 跑 `npm test` + `npm run check` + `git diff --check`;直接提交 main 并推送。

**阶段 6 — 回写 GitHub**:
- #61:勾选已验证的第一轮条目(附证据),更新过时的"职责与状态"描述(不声称第二轮已完成)
- #10:登记回执(SHA、URL、脱敏角色、操作、持久结果)
- #53:进度评论;联合交付总项仅在证据完整时勾选

## 约束
- 不写测试代码;不改数据库迁移;全程确定性路径(真实 Jev/AI 归 #55,不在本次范围)
- 服务生命周期只用 npm run server:start/status/logs
- 每阶段结束向跟踪 issue 回写,保持勾选状态真实