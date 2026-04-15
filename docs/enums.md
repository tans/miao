# 创意喵枚举定义

> **注意**：所有枚举值必须与本文档保持一致，不得在不同端单独定义或偏移。

## TaskStatus（任务状态）

| 值 | Go 常量 | 文本 | 说明 |
|---|---|---|---|
| 1 | `TaskStatusPending` | 待审核 | 商家创建后待平台审核 |
| 2 | `TaskStatusOnline` | 已上架 | 审核通过，可被认领 |
| 3 | `TaskStatusOngoing` | 进行中 | 有创作者认领并交付中 |
| 4 | `TaskStatusEnded` | 已结束 | 任务到期或名额已满 |
| 5 | `TaskStatusCancelled` | 已取消 | 商家取消或平台下架 |

## ClaimStatus（认领/交付状态）

| 值 | Go 常量 | 文本 | 说明 |
|---|---|---|---|
| 1 | `ClaimStatusPending` | 待提交 | 已认领，待创作者提交内容 |
| 2 | `ClaimStatusSubmitted` | 待验收 | 已提交，待商家验收 |
| 3 | `ClaimStatusApproved` | 已完成 | 商家验收通过 |
| 4 | `ClaimStatusCancelled` | 已取消 | 认领后取消 |
| 5 | `ClaimStatusExpired` | 已超时 | 24小时内未提交 |

## ReviewResult（验收结果）

| 值 | 文本 | 说明 |
|---|---|---|
| 1 | 通过 | 验收通过 |
| 2 | 退回 | 退回修改 |

---

## 实现要求

1. **Go 后端**：`internal/model/task.go` 和 `internal/model/claim.go` 中的常量定义为本源
2. **Web 前端**：通过 `web/static/js/enums.js` 引用，使用 `TaskStatus.ONLINE` 而非硬编码数字
3. **小程序**：通过 `miniprogram/utils/enums.js` 引用，保持相同结构
4. **API 传输**：统一使用整数值（1-5），不在请求/响应中出现字符串状态
