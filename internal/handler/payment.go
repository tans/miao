package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
	"github.com/tans/miao/internal/service"
)

// PaymentHandler 支付处理器
type PaymentHandler struct {
	payService  *service.WechatPayService
	paymentRepo *repository.PaymentRepository
	userRepo    *repository.UserRepository
}

// NewPaymentHandler 创建支付处理器
func NewPaymentHandler(payService *service.WechatPayService) *PaymentHandler {
	return &PaymentHandler{
		payService:  payService,
		paymentRepo: repository.NewPaymentRepository(GetDB()),
		userRepo:    repository.NewUserRepository(GetDB()),
	}
}

// CreateRechargeOrder 创建充值订单
// POST /api/v1/account/recharge
func (h *PaymentHandler) CreateRechargeOrder(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, AccountResponse{
			Code:    40101,
			Message: "未登录",
		})
		return
	}

	var req struct {
		Amount float64 `json:"amount" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AccountResponse{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	// 生成订单号
	orderNo := fmt.Sprintf("R%d%d", time.Now().UnixNano(), userID%10000)

	// 创建支付订单
	order := &model.PaymentOrder{
		UserID:  userID,
		OrderNo: orderNo,
		Amount:  req.Amount,
		Status:  model.PaymentOrderStatusPending,
	}

	if err := h.paymentRepo.CreatePaymentOrder(order); err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    50001,
			Message: "创建订单失败",
		})
		return
	}

	// 调用微信支付统一下单
	wechatReq := &service.UnifiedOrderRequest{}
	wechatReq.Description = "创意喵充值"
	wechatReq.OutTradeNo = orderNo
	wechatReq.Amount.Total = int(req.Amount * 100) // 转换为分
	wechatReq.Amount.Currency = "CNY"
	wechatReq.NotifyURL = "https://your-domain.com/api/v1/payment/callback"

	result, err := h.payService.UnifiedOrder(wechatReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    50002,
			Message: "创建支付失败: " + err.Error(),
		})
		return
	}

	if result.Code != "" && result.Code != "SUCCESS" {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    50003,
			Message: "微信支付错误: " + result.Message,
		})
		return
	}

	// 返回支付参数
	c.JSON(http.StatusOK, AccountResponse{
		Code:    0,
		Message: "创建订单成功",
		Data: gin.H{
			"order_no": orderNo,
			"amount":   req.Amount,
			"code_url": result.CodeUrl,
		},
	})
}

// PaymentCallback 支付回调
// POST /api/v1/payment/callback
func (h *PaymentHandler) PaymentCallback(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERROR", "message": err.Error()})
		return
	}

	var callback struct {
		EventType string `json:"event_type"`
		Resource   struct {
			TransactionID string `json:"transaction_id"`
			OutTradeNo    string `json:"out_trade_no"`
			TradeState    string `json:"trade_state"`
		} `json:"resource"`
	}

	if err := json.Unmarshal(body, &callback); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERROR", "message": "解析失败"})
		return
	}

	if callback.EventType == "TRANSACTION_SUCCESS" || callback.Resource.TradeState == "SUCCESS" {
		outTradeNo := callback.Resource.OutTradeNo
		transactionID := callback.Resource.TransactionID

		if err := h.paymentRepo.UpdatePaymentOrderPaid(outTradeNo, transactionID); err != nil {
			errorLog.Printf("更新订单失败: %s, err: %v", outTradeNo, err)
		} else {
			order, _ := h.paymentRepo.GetPaymentOrderByOrderNo(outTradeNo)
			if order != nil {
				h.creditUserBalance(order.UserID, order.Amount)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
}

// QueryRechargeOrder 查询充值订单状态
// GET /api/v1/account/recharge/:order_no
func (h *PaymentHandler) QueryRechargeOrder(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, AccountResponse{
			Code:    40101,
			Message: "未登录",
		})
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		c.JSON(http.StatusBadRequest, AccountResponse{
			Code:    40001,
			Message: "缺少订单号",
		})
		return
	}

	order, err := h.paymentRepo.GetPaymentOrderByOrderNo(orderNo)
	if err != nil || order == nil {
		c.JSON(http.StatusNotFound, AccountResponse{
			Code:    40401,
			Message: "订单不存在",
		})
		return
	}

	if order.UserID != userID {
		c.JSON(http.StatusForbidden, AccountResponse{
			Code:    40301,
			Message: "无权访问",
		})
		return
	}

	if order.Status == model.PaymentOrderStatusPending {
		result, err := h.payService.QueryOrder(orderNo)
		if err == nil && result.TransactionID != "" {
			h.paymentRepo.UpdatePaymentOrderPaid(orderNo, result.TransactionID)
			order.Status = model.PaymentOrderStatusPaid
			order.WechatOrderID = result.TransactionID
		}
	}

	c.JSON(http.StatusOK, AccountResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"order_no":        order.OrderNo,
			"amount":          order.Amount,
			"status":          order.Status,
			"wechat_order_id": order.WechatOrderID,
			"paid_at":         order.PaidAt,
		},
	})
}

// creditUserBalance 发放充值金额到用户账户
func (h *PaymentHandler) creditUserBalance(userID int64, amount float64) error {
	user, err := h.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return fmt.Errorf("用户不存在")
	}

	balanceBefore := user.Balance
	newBalance := user.Balance + amount

	tx, err := GetDB().Begin()
	if err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := h.userRepo.UpdateUserBalanceWithTx(tx, userID, newBalance); err != nil {
		tx.Rollback()
		return err
	}

	transaction := &model.Transaction{
		UserID:        userID,
		Type:          model.TransactionTypeRecharge,
		Amount:        amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  newBalance,
		Remark:        "充值",
		CreatedAt:     time.Now(),
	}

	accountRepo := repository.NewAccountRepository(GetDB())
	if err := accountRepo.CreateTransactionTx(tx, transaction); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}