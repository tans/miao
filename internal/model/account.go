package model

import "time"

// Account 账户表
type Account struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Balance      float64   `json:"balance"`        // 账户余额
	FrozenAmount float64   `json:"frozen_amount"` // 冻结金额（悬赏金预付后冻结）
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RechargeRequest 充值请求
type RechargeRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
}

// PrepayRequest 悬赏金预付请求
type PrepayRequest struct {
	TaskID int64 `json:"task_id" binding:"required"`
}
