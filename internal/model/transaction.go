package model

import "time"

// TransactionType 交易类型
type TransactionType int

const (
	TransactionTypeRecharge     TransactionType = 1 // 充值
	TransactionTypeConsume      TransactionType = 2 // 消费
	TransactionTypeFreeze       TransactionType = 3 // 冻结
	TransactionTypeUnfreeze     TransactionType = 4 // 解冻
	TransactionTypeReward       TransactionType = 5 // 奖励
	TransactionTypeWithdraw     TransactionType = 6 // 提现
	TransactionTypeReturnMargin TransactionType = 7 // 退保证金
	TransactionTypeCommission   TransactionType = 8 // 抽成
)

// Transaction 交易记录表
type Transaction struct {
	ID            int64           `json:"id"`
	UserID        int64           `json:"user_id"`
	Type          TransactionType `json:"type"`
	Amount        float64         `json:"amount"`
	BalanceBefore float64         `json:"balance_before"`
	BalanceAfter  float64         `json:"balance_after"`
	Remark        string          `json:"remark"`
	RelatedID     int64           `json:"related_id"`      // 关联ID (task_id, claim_id等)
	CreatedAt     time.Time       `json:"created_at"`
}

// TransactionQuery 交易查询
type TransactionQuery struct {
	Type   *int `form:"type"`
	Page   int  `form:"page,default=1"`
	PageSize int `form:"page_size,default=20"`
}
