package model

import "time"

// TransactionType 交易类型
type TransactionType int

const (
	TransactionTypeRecharge       TransactionType = 1  // 充值
	TransactionTypeConsume        TransactionType = 2  // 奖励支出
	TransactionTypeFreeze         TransactionType = 3  // 冻结
	TransactionTypeUnfreeze       TransactionType = 4  // 解冻
	TransactionTypeReward         TransactionType = 5  // 奖励
	TransactionTypeWithdraw       TransactionType = 6  // 提现
	TransactionTypeReturnMargin   TransactionType = 7  // 退保证金
	TransactionTypeCommission     TransactionType = 8  // 抽成
	TransactionTypePayment        TransactionType = 9  // 支付参与奖励
	TransactionTypeAwardPayment   TransactionType = 10 // 支付采纳奖励
	TransactionTypePlatformIncome TransactionType = 11 // 平台收入
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
	RelatedID     int64           `json:"related_id"` // 关联ID (task_id, claim_id等)
	CreatedAt     time.Time       `json:"created_at"`
}

// TransactionQuery 交易查询
type TransactionQuery struct {
	Type     *int `form:"type"`
	Page     int  `form:"page,default=1"`
	PageSize int  `form:"page_size,default=20"`
}

// Name returns the user-facing Chinese label for a transaction type.
func (t TransactionType) Name() string {
	switch t {
	case TransactionTypeRecharge:
		return "充值"
	case TransactionTypeConsume:
		return "奖励支出"
	case TransactionTypeFreeze:
		return "冻结"
	case TransactionTypeUnfreeze:
		return "解冻"
	case TransactionTypeReward:
		return "奖励"
	case TransactionTypeWithdraw:
		return "提现"
	case TransactionTypeReturnMargin:
		return "退保证金"
	case TransactionTypeCommission:
		return "平台抽成"
	case TransactionTypePayment:
		return "参与奖励"
	case TransactionTypeAwardPayment:
		return "采纳奖励"
	case TransactionTypePlatformIncome:
		return "平台收入"
	default:
		return "未知"
	}
}

// Code returns the admin API code used for filtering/display.
func (t TransactionType) Code() string {
	switch t {
	case TransactionTypeRecharge:
		return "recharge"
	case TransactionTypeConsume:
		return "task_payment"
	case TransactionTypeFreeze:
		return "freeze"
	case TransactionTypeUnfreeze:
		return "unfreeze"
	case TransactionTypeReward:
		return "task_reward"
	case TransactionTypeWithdraw:
		return "withdraw"
	case TransactionTypeReturnMargin:
		return "refund"
	case TransactionTypeCommission:
		return "commission"
	case TransactionTypePayment:
		return "participation_payment"
	case TransactionTypeAwardPayment:
		return "award_payment"
	case TransactionTypePlatformIncome:
		return "platform_income"
	default:
		return "unknown"
	}
}

// DisplayAmount returns a signed amount suitable for transaction-list display.
// Historical rows sometimes stored expense amounts as positive numbers; the
// balance delta is the authoritative direction when it exists.
func (t *Transaction) DisplayAmount() float64 {
	if t == nil {
		return 0
	}

	delta := t.BalanceAfter - t.BalanceBefore
	absAmount := t.Amount
	if absAmount < 0 {
		absAmount = -absAmount
	}

	if delta < 0 {
		return -absAmount
	}
	if delta > 0 {
		return absAmount
	}

	if t.Type.IsExpense() {
		return -absAmount
	}
	return absAmount
}

// IsExpense returns whether a transaction type should display as an outflow
// when the balance delta is neutral or unavailable.
func (t TransactionType) IsExpense() bool {
	switch t {
	case TransactionTypeConsume, TransactionTypeFreeze, TransactionTypeWithdraw, TransactionTypeCommission:
		return true
	default:
		return false
	}
}
