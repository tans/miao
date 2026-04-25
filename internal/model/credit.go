package model

import "time"

// CreditRecordType 信用记录类型
type CreditRecordType int

const (
	CreditRecordTypeReward  CreditRecordType = 1 // 奖励
	CreditRecordTypePenalty CreditRecordType = 2 // 处罚
)

// CreditRecord 信用记录表
type CreditRecord struct {
	ID        int64            `json:"id"`
	UserID    int64            `json:"user_id"`
	Type      CreditRecordType `json:"type"`   // 1=奖励, 2=处罚
	Change    int              `json:"change"` // 信用分变化（正或负）
	Reason    string           `json:"reason"` // 变更原因
	CreatedAt time.Time        `json:"created_at"`
}

// CreditQuery 信用记录查询
type CreditQuery struct {
	Limit  int `form:"limit,default=20"`
	Offset int `form:"offset,default=0"`
}
