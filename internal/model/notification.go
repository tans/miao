package model

import "time"

// NotificationType 通知类型
type NotificationType string

const (
	NotificationTypeTaskStatus    NotificationType = "task_status"      // 任务状态变更
	NotificationTypeNewSubmission  NotificationType = "new_submission"   // 新投稿通知
	NotificationTypeClaimApproved NotificationType = "claim_approved"   // 认领通过
	NotificationTypeIncomeReceived NotificationType = "income_received" // 收益到账
)

// Notification 通知表
type Notification struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	UserID    uint           `json:"user_id" gorm:"index"`
	Type      NotificationType `json:"type"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	IsRead    bool           `json:"is_read" gorm:"default:false"`
	RelatedID *uint          `json:"related_id,omitempty" gorm:"index"`
	CreatedAt time.Time      `json:"created_at"`
}

// TableName specifies the table name for GORM
func (Notification) TableName() string {
	return "notifications"
}

// CreateNotificationRequest 创建通知请求
type CreateNotificationRequest struct {
	UserID    uint           `json:"user_id"`
	Type      NotificationType `json:"type" binding:"required"`
	Title     string         `json:"title" binding:"required"`
	Content   string         `json:"content" binding:"required"`
	RelatedID *uint          `json:"related_id,omitempty"`
}

// NotificationQuery 通知查询
type NotificationQuery struct {
	Type   string `form:"type"`
	IsRead *bool  `form:"is_read"`
	Page   int    `form:"page,default=1"`
	Limit  int    `form:"limit,default=20"`
}

// NotificationResponse 通知响应
type NotificationResponse struct {
	ID        uint           `json:"id"`
	UserID    uint           `json:"user_id"`
	Type      NotificationType `json:"type"`
	TypeStr   string         `json:"type_str"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	IsRead    bool           `json:"is_read"`
	RelatedID *uint          `json:"related_id,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// GetTypeStr returns the string representation of notification type
func (n *Notification) GetTypeStr() string {
	switch n.Type {
	case NotificationTypeTaskStatus:
		return "任务状态"
	case NotificationTypeNewSubmission:
		return "新投稿"
	case NotificationTypeClaimApproved:
		return "认领通过"
	case NotificationTypeIncomeReceived:
		return "收益到账"
	default:
		return "未知"
	}
}
