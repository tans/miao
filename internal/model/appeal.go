package model

import "time"

// AppealType 申诉类型
type AppealType int

const (
	AppealTypeTask AppealType = 1 // 任务申诉
)

// AppealStatus 申诉状态
type AppealStatus int

const (
	AppealStatusPending  AppealStatus = 1 // 待处理
	AppealStatusResolved AppealStatus = 2 // 已处理
)

// Appeal 申诉表
type Appeal struct {
	ID        int64        `json:"id"`
	UserID    int64        `json:"user_id"`
	Type      AppealType   `json:"type"`      // 1=作品申诉
	ClaimID   int64        `json:"claim_id"`  // 关联的作品/投稿ID
	TargetID  int64        `json:"target_id"` // 兼容旧字段
	Reason    string       `json:"reason"`    // 申诉原因
	Evidence  string       `json:"evidence"`  // 证据材料
	Status    AppealStatus `json:"status"`    // 1=待处理, 2=已处理
	Result    string       `json:"result"`    // 处理结果
	AdminID   int64        `json:"admin_id"`  // 处理管理员ID
	HandleAt  *time.Time   `json:"handle_at"` // 处理时间
	CreatedAt time.Time    `json:"created_at"`
}

// CreateAppealRequest 创建申诉请求
type CreateAppealRequest struct {
	Type     int    `json:"type" binding:"required,oneof=1"`
	ClaimID  int64  `json:"claim_id"`
	TargetID int64  `json:"target_id"`
	TaskID   int64  `json:"task_id"`
	Reason   string `json:"reason" binding:"required"`
	Evidence string `json:"evidence"` // 证据材料（可选）
}

// AppealQuery 申诉查询
type AppealQuery struct {
	Status int `form:"status"`
	Type   int `form:"type"`
	Limit  int `form:"limit,default=20"`
	Offset int `form:"offset,default=0"`
}

// ResolveAppealRequest 处理申诉请求
type ResolveAppealRequest struct {
	Result   string `json:"result" binding:"required"`
	Accepted bool   `json:"accepted"` // 申诉是否被接受
}
