package model

import "time"

// CreditLogType 积分日志类型
type CreditLogType int

const (
	CreditLogTypeReward CreditLogType = 1 // 奖励
	CreditLogTypePunish CreditLogType = 2 // 处罚
)

// CreditLog 积分日志表
type CreditLog struct {
	ID        int64         `json:"id"`
	UserID    int64         `json:"user_id"`
	Type      CreditLogType `json:"type"`   // 1=奖励, 2=处罚
	Change    int           `json:"change"` // 分数变化 (正或负)
	Reason    string        `json:"reason"`
	RelatedID int64         `json:"related_id"` // 关联ID
	CreatedAt time.Time     `json:"created_at"`
}

// CreditLogQuery 积分日志查询
type CreditLogQuery struct {
	Type     *int `form:"type"`
	Page     int  `form:"page,default=1"`
	PageSize int  `form:"page_size,default=20"`
}
