package model

import "time"

// MessageType 消息类型
type MessageType int

const (
	MessageTypeTaskReview      MessageType = 1 // 任务审核
	MessageTypeClaimStatus     MessageType = 2 // 认领状态
	MessageTypeReviewResult    MessageType = 3 // 验收结果
	MessageTypeAppealHandled   MessageType = 4 // 申诉处理
	MessageTypeSystem          MessageType = 5 // 系统通知
)

// Message 消息表
type Message struct {
	ID        int64       `json:"id" db:"id"`
	UserID    int64       `json:"user_id" db:"user_id"`
	Type      MessageType `json:"type" db:"type"`
	Title     string      `json:"title" db:"title"`
	Content   string      `json:"content" db:"content"`
	RelatedID *int64      `json:"related_id,omitempty" db:"related_id"`
	IsRead    int         `json:"is_read" db:"is_read"` // 0=未读, 1=已读
	CreatedAt time.Time   `json:"created_at" db:"created_at"`
}

// MessageCreate 创建消息请求
type MessageCreate struct {
	UserID    int64       `json:"user_id"`
	Type      MessageType `json:"type"`
	Title     string      `json:"title"`
	Content   string      `json:"content"`
	RelatedID *int64      `json:"related_id,omitempty"`
}
