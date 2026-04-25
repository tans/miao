package model

import "time"

// PaymentOrder 支付订单
type PaymentOrder struct {
	ID            int64      `json:"id"`
	UserID        int64      `json:"user_id"`
	OrderNo       string     `json:"order_no"`        // 商户订单号
	Amount        float64    `json:"amount"`          // 充值金额(元)
	Status        int        `json:"status"`          // 订单状态
	PayResult     string     `json:"pay_result"`      // 支付结果描述
	WechatOrderID string     `json:"wechat_order_id"` // 微信支付订单号
	PaidAt        *time.Time `json:"paid_at"`         // 支付时间
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// PaymentOrderStatus 订单状态
const (
	PaymentOrderStatusPending   = 1 // 待支付
	PaymentOrderStatusPaid      = 2 // 已支付
	PaymentOrderStatusCancelled = 3 // 已取消
	PaymentOrderStatusRefunded  = 4 // 已退款
)

// PaymentOrderQuery 支付订单查询
type PaymentOrderQuery struct {
	Status   int `form:"status"`
	Page     int `form:"page,default=1"`
	PageSize int `form:"page_size,default=20"`
}
