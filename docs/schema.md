# 数据库表结构

## 1. users（用户表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| username | TEXT | 用户名，唯一 |
| password_hash | TEXT | 密码哈希 |
| is_admin | INTEGER | 是否管理员，默认0 |
| email | TEXT | 邮箱 |
| phone | TEXT | 电话 |
| status | INTEGER | 状态，默认1 |
| nickname | TEXT | 昵称 |
| avatar | TEXT | 头像 |
| real_name | TEXT | 真实姓名 |
| real_name_verified | INTEGER | 实名认证状态 |
| company_name | TEXT | 公司名称 |
| balance | REAL | 余额，默认0 |
| frozen_amount | REAL | 冻结金额，默认0 |
| wechat_openid | TEXT | 微信OpenID |
| level | INTEGER | 等级 |
| adopted_count | INTEGER | 被采纳数 |
| margin_frozen | REAL | 保证金冻结 |
| daily_claim_count | INTEGER | 每日领取数 |
| daily_claim_reset | DATETIME | 每日领取重置时间 |
| report_count | INTEGER | 被举报次数 |
| business_verified | INTEGER | 商家认证状态 |
| publish_count | INTEGER | 发布任务数 |
| credit_score | INTEGER | 信用分，默认100 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

## 2. tasks（任务表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| business_id | INTEGER | 商家ID，外键→users(id) |
| title | TEXT | 任务标题 |
| description | TEXT | 任务描述 |
| category | INTEGER | 分类 |
| unit_price | REAL | 单价 |
| total_count | INTEGER | 总数量 |
| remaining_count | INTEGER | 剩余数量 |
| status | INTEGER | 状态，默认1 |
| review_at | DATETIME | 审核时间 |
| publish_at | DATETIME | 发布时间 |
| end_at | DATETIME | 结束时间 |
| total_budget | REAL | 总预算 |
| frozen_amount | REAL | 冻结金额 |
| paid_amount | REAL | 已支付金额 |
| industries | TEXT | 行业标签 |
| video_duration | TEXT | 视频时长 |
| video_aspect | TEXT | 视频宽高比 |
| video_resolution | TEXT | 视频分辨率 |
| styles | TEXT | 创意风格 |
| award_price | REAL | 奖金金额 |
| public | INTEGER | 是否公开提交（1=公开，0=不公开） |
| service_fee_rate | REAL | 服务费率（0.05/0.10） |
| service_fee_amount | REAL | 服务费金额 |
| award_count | INTEGER | 奖金份数 |
| review_deadline_at | DATETIME | 审核截止时间 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

## 3. claims（投稿/认领表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| task_id | INTEGER | 任务ID，外键→tasks(id) |
| creator_id | INTEGER | 创作者ID，外键→users(id) |
| status | INTEGER | 状态，默认1 |
| content | TEXT | 投稿内容 |
| submit_at | DATETIME | 提交时间 |
| expires_at | DATETIME | 过期时间 |
| review_at | DATETIME | 审核时间 |
| review_result | INTEGER | 审核结果 |
| review_comment | TEXT | 审核备注 |
| creator_reward | REAL | 创作者奖励 |
| platform_fee | REAL | 平台费 |
| margin_returned | REAL | 保证金返还 |
| likes | INTEGER | 点赞数 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

## 4. claim_materials（投稿素材表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| claim_id | INTEGER | 投稿ID，外键→claims(id) |
| file_name | TEXT | 文件名 |
| file_path | TEXT | 文件路径 |
| file_size | INTEGER | 文件大小 |
| file_type | TEXT | 文件类型 |
| thumbnail_path | TEXT | 缩略图路径 |
| source_file_path | TEXT | 源文件路径 |
| processed_file_path | TEXT | 处理后文件路径 |
| process_status | TEXT | 处理状态 |
| process_error | TEXT | 处理错误信息 |
| watermark_applied | INTEGER | 是否添加水印 |
| compressed | INTEGER | 是否压缩 |
| duration | REAL | 视频时长 |
| width | INTEGER | 宽度 |
| height | INTEGER | 高度 |
| created_at | DATETIME | 创建时间 |

## 5. transactions（交易流水表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| user_id | INTEGER | 用户ID，外键→users(id) |
| type | INTEGER | 交易类型 |
| amount | REAL | 交易金额 |
| balance_before | REAL | 交易前余额 |
| balance_after | REAL | 交易后余额 |
| remark | TEXT | 备注 |
| related_id | INTEGER | 关联ID |
| created_at | DATETIME | 创建时间 |

## 6. credit_logs（信用日志表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| user_id | INTEGER | 用户ID，外键→users(id) |
| type | INTEGER | 类型 |
| change | INTEGER | 变更值 |
| reason | TEXT | 原因 |
| related_id | INTEGER | 关联ID |
| created_at | DATETIME | 创建时间 |

## 7. appeals（申诉表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| user_id | INTEGER | 用户ID，外键→users(id) |
| type | INTEGER | 申诉类型 |
| target_id | INTEGER | 目标ID |
| reason | TEXT | 申诉原因 |
| evidence | TEXT | 证据 |
| status | INTEGER | 状态，默认1 |
| result | TEXT | 处理结果 |
| admin_id | INTEGER | 管理员ID |
| handle_at | DATETIME | 处理时间 |
| created_at | DATETIME | 创建时间 |

## 8. notifications（通知表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| user_id | INTEGER | 用户ID，外键→users(id) |
| type | INTEGER | 通知类型 |
| title | TEXT | 标题 |
| content | TEXT | 内容 |
| related_id | INTEGER | 关联ID |
| is_read | INTEGER | 是否已读 |
| created_at | DATETIME | 创建时间 |

## 9. inspirations（灵感表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| title | TEXT | 标题 |
| content | TEXT | 内容 |
| creator_name | TEXT | 创作者名称 |
| creator_avatar | TEXT | 创作者头像 |
| cover_url | TEXT | 封面URL |
| cover_type | TEXT | 封面类型，默认image |
| cover_width | INTEGER | 封面宽度 |
| cover_height | INTEGER | 封面高度 |
| status | INTEGER | 状态，默认1 |
| views | INTEGER | 浏览数 |
| likes | INTEGER | 点赞数 |
| tags | TEXT | 标签 |
| sort_order | INTEGER | 排序 |
| created_by | INTEGER | 创建者ID，外键→users(id) |
| source_claim_id | INTEGER | 来源投稿ID |
| published_at | DATETIME | 发布时间 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

## 10. inspiration_materials（灵感素材表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| inspiration_id | INTEGER | 灵感ID，外键→inspirations(id) |
| file_name | TEXT | 文件名 |
| file_path | TEXT | 文件路径 |
| file_size | INTEGER | 文件大小 |
| file_type | TEXT | 文件类型 |
| thumbnail_path | TEXT | 缩略图路径 |
| sort_order | INTEGER | 排序 |
| created_at | DATETIME | 创建时间 |

## 11. inspiration_likes（灵感点赞表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| inspiration_id | INTEGER | 灵感ID，外键→inspirations(id) |
| user_id | INTEGER | 用户ID，外键→users(id) |
| created_at | DATETIME | 创建时间 |

## 12. work_likes（作品点赞表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| work_id | INTEGER | 作品ID，外键→claims(id) |
| user_id | INTEGER | 用户ID，外键→users(id) |
| created_at | DATETIME | 创建时间 |

## 13. system_settings（系统设置表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键，固定为1 |
| review_days | INTEGER | 审核天数，默认7 |
| submit_days | INTEGER | 提交天数，默认7 |
| grace_days | INTEGER | 宽限期天数，默认7 |
| report_action | INTEGER | 举报处理动作，默认1 |
| min_unit_price | REAL | 最小单价，默认2.0 |
| min_award_price | REAL | 最小奖金，默认8.0 |
| updated_at | DATETIME | 更新时间 |

## 14. payment_orders（支付订单表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| user_id | INTEGER | 用户ID |
| order_no | TEXT | 订单号，唯一 |
| amount | REAL | 金额 |
| status | INTEGER | 状态，默认1 |
| pay_result | TEXT | 支付结果 |
| wechat_order_id | TEXT | 微信订单ID |
| paid_at | DATETIME | 支付时间 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

## 15. video_processing_jobs（视频处理任务表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| job_id | TEXT | 任务ID，唯一 |
| material_id | INTEGER | 素材ID，外键→claim_materials(id) |
| biz_type | TEXT | 业务类型 |
| biz_id | INTEGER | 业务ID |
| source_url | TEXT | 源文件URL |
| status | TEXT | 状态，默认pending |
| processed_url | TEXT | 处理后URL |
| thumbnail_url | TEXT | 缩略图URL |
| watermark_template | TEXT | 水印模板 |
| target_format | TEXT | 目标格式，默认mp4 |
| target_resolution | TEXT | 目标分辨率，默认1080P |
| error_message | TEXT | 错误信息 |
| duration | REAL | 时长 |
| width | INTEGER | 宽度 |
| height | INTEGER | 高度 |
| watermark_applied | INTEGER | 是否添加水印 |
| compressed | INTEGER | 是否压缩 |
| completed_at | DATETIME | 完成时间 |
| last_callback_at | DATETIME | 最后回调时间 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

## 16. task_materials（任务素材表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| task_id | INTEGER | 任务ID，外键→tasks(id) |
| file_name | TEXT | 文件名 |
| file_path | TEXT | 文件路径 |
| file_size | INTEGER | 文件大小 |
| file_type | TEXT | 文件类型 |
| sort_order | INTEGER | 排序 |
| created_at | DATETIME | 创建时间 |

## 索引

| 索引名 | 字段 | 表 |
|--------|------|-----|
| idx_users_username | username | users |
| idx_users_is_admin | is_admin | users |
| idx_users_phone | phone | users |
| idx_users_status | status | users |
| idx_users_wechat_openid | wechat_openid | users |
| idx_tasks_status | status | tasks |
| idx_tasks_category | category | tasks |
| idx_tasks_business_id | business_id | tasks |
| idx_tasks_created_at | created_at | tasks |
| idx_claims_task_id | task_id | claims |
| idx_claims_creator_id | creator_id | claims |
| idx_claims_status | status | claims |
| idx_claims_expires_at | expires_at | claims |
| idx_transactions_user_id | user_id | transactions |
| idx_transactions_type | type | transactions |
| idx_transactions_created_at | created_at | transactions |
| idx_credit_logs_user_id | user_id | credit_logs |
| idx_appeals_user_id | user_id | appeals |
| idx_appeals_status | status | appeals |
| idx_notifications_user_id | user_id | notifications |
| idx_notifications_is_read | is_read | notifications |
| idx_inspirations_status_sort | status, sort_order, published_at, created_at | inspirations |
| idx_inspirations_created_by | created_by | inspirations |
| idx_inspirations_source_claim_id | source_claim_id | inspirations |
| idx_inspiration_materials_inspiration_id | inspiration_id | inspiration_materials |
| idx_inspiration_likes_unique | inspiration_id, user_id | inspiration_likes |
| idx_inspiration_likes_user_id | user_id | inspiration_likes |
| idx_work_likes_unique | work_id, user_id | work_likes |
| idx_work_likes_user_id | user_id | work_likes |
| idx_claim_materials_claim_id | claim_id | claim_materials |
| idx_task_materials_task_id | task_id | task_materials |
| idx_payment_orders_user_id | user_id | payment_orders |
| idx_payment_orders_order_no | order_no | payment_orders |
| idx_payment_orders_status | status | payment_orders |
| idx_video_processing_jobs_material_id | material_id | video_processing_jobs |
| idx_video_processing_jobs_status | status | video_processing_jobs |