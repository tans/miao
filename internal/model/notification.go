package model

import (
	"fmt"
	"time"
)

// NotificationType 通知类型
type NotificationType string

const (
	NotificationTypeSystem              NotificationType = "system"
	NotificationTypeTaskCreated         NotificationType = "task_created"
	NotificationTypeTaskReviewPassed    NotificationType = "task_review_passed"
	NotificationTypeTaskReviewRejected  NotificationType = "task_review_rejected"
	NotificationTypeTaskClaimed         NotificationType = "task_claimed"
	NotificationTypeClaimCreated        NotificationType = "claim_created"
	NotificationTypeSubmissionSubmitted NotificationType = "submission_submitted"
	NotificationTypeSubmissionReceived  NotificationType = "submission_received"
	NotificationTypeReviewPassed        NotificationType = "review_passed"
	NotificationTypeReviewRejected      NotificationType = "review_rejected"
	NotificationTypeTaskCancelled       NotificationType = "task_cancelled"
	NotificationTypeAppealCreated       NotificationType = "appeal_created"
	NotificationTypeAppealHandled       NotificationType = "appeal_handled"

	NotificationTypeTaskStatus     NotificationType = "task_status"     // legacy: 任务状态变更
	NotificationTypeClaimApproved  NotificationType = "claim_approved"  // legacy: 认领通过
	NotificationTypeIncomeReceived NotificationType = "income_received" // legacy: 收益到账
)

// Notification 通知表
type Notification struct {
	ID        uint             `json:"id" gorm:"primaryKey"`
	UserID    uint             `json:"user_id" gorm:"index"`
	Type      NotificationType `json:"type"`
	Title     string           `json:"title"`
	Content   string           `json:"content"`
	IsRead    bool             `json:"is_read" gorm:"default:false"`
	RelatedID *uint            `json:"related_id,omitempty" gorm:"index"`
	CreatedAt time.Time        `json:"created_at"`
}

// TableName specifies the table name for GORM
func (Notification) TableName() string {
	return "notifications"
}

// CreateNotificationRequest 创建通知请求
type CreateNotificationRequest struct {
	UserID    uint             `json:"user_id"`
	Type      NotificationType `json:"type" binding:"required"`
	Title     string           `json:"title" binding:"required"`
	Content   string           `json:"content" binding:"required"`
	RelatedID *uint            `json:"related_id,omitempty"`
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
	ID        uint             `json:"id"`
	UserID    uint             `json:"user_id"`
	Type      NotificationType `json:"type"`
	TypeStr   string           `json:"type_str"`
	Title     string           `json:"title"`
	Content   string           `json:"content"`
	IsRead    bool             `json:"is_read"`
	RelatedID *uint            `json:"related_id,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
}

// GetTypeStr returns the string representation of notification type
func (n *Notification) GetTypeStr() string {
	switch n.Type {
	case NotificationTypeTaskCreated:
		return "任务已发布"
	case NotificationTypeTaskReviewPassed:
		return "任务审核通过"
	case NotificationTypeTaskReviewRejected:
		return "任务审核未通过"
	case NotificationTypeTaskClaimed:
		return "收到创作者的报名"
	case NotificationTypeClaimCreated:
		return "报名成功"
	case NotificationTypeSubmissionSubmitted:
		return "作品提交成功"
	case NotificationTypeSubmissionReceived:
		return "收到新的稿件"
	case NotificationTypeReviewPassed:
		return "作品审核通过"
	case NotificationTypeReviewRejected:
		return "作品审核未通过"
	case NotificationTypeTaskCancelled:
		return "任务已取消"
	case NotificationTypeAppealCreated:
		return "申诉已提交"
	case NotificationTypeAppealHandled:
		return "申诉已处理"
	case NotificationTypeSystem:
		return "系统消息"
	case NotificationTypeTaskStatus:
		return "任务状态"
	case NotificationTypeClaimApproved:
		return "报名通知"
	case NotificationTypeIncomeReceived:
		return "收益通知"
	default:
		return "系统消息"
	}
}

func (n *Notification) GetBizType() string {
	switch n.Type {
	case NotificationTypeTaskCreated,
		NotificationTypeTaskReviewPassed,
		NotificationTypeTaskReviewRejected,
		NotificationTypeTaskClaimed,
		NotificationTypeTaskCancelled,
		NotificationTypeTaskStatus:
		return "task"
	case NotificationTypeClaimCreated,
		NotificationTypeSubmissionSubmitted,
		NotificationTypeSubmissionReceived,
		NotificationTypeReviewPassed,
		NotificationTypeReviewRejected,
		NotificationTypeClaimApproved,
		NotificationTypeIncomeReceived:
		return "claim"
	case NotificationTypeAppealCreated,
		NotificationTypeAppealHandled:
		return "appeal"
	case NotificationTypeSystem:
		return "system"
	default:
		return "system"
	}
}

func (n *Notification) GetTargetPath() string {
	if n.RelatedID == nil {
		return ""
	}

	id := *n.RelatedID
	switch n.Type {
	case NotificationTypeTaskCreated,
		NotificationTypeTaskReviewPassed,
		NotificationTypeTaskReviewRejected,
		NotificationTypeTaskClaimed,
		NotificationTypeSubmissionReceived,
		NotificationTypeTaskStatus:
		return fmt.Sprintf("/pages/employer/task-detail/index?id=%d", id)
	case NotificationTypeClaimCreated,
		NotificationTypeSubmissionSubmitted,
		NotificationTypeReviewPassed,
		NotificationTypeReviewRejected:
		return fmt.Sprintf("/pages/creator/task-detail/index?id=%d", id)
	case NotificationTypeAppealCreated,
		NotificationTypeAppealHandled:
		return "/pages/mine/appeal/index"
	default:
		return ""
	}
}
