package handler

import (
	"database/sql"
	"fmt"
	"github.com/tans/miao/internal/database"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
)

// AccountResponse represents the standard API response
type AccountResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func calculateWithdrawActualAmount(amount, commissionRate float64) float64 {
	return amount * (1 - commissionRate)
}

const (
	withdrawOrderStatusProcessing = 1
	withdrawOrderStatusSuccess    = 2
	withdrawOrderStatusFailed     = 3
)

var withdrawCommissionRemarkRe = regexp.MustCompile(`扣除佣金([0-9]+(?:\.[0-9]+)?)元`)

type withdrawOrder struct {
	WithdrawNo       string
	Amount           float64
	ActualAmount     float64
	CommissionAmount float64
	Status           int
}

func generateWithdrawNo(userID int64) string {
	return fmt.Sprintf("W%d%d", time.Now().UnixNano(), userID%10000)
}

func getWithdrawOrderByIdempotencyKeyTx(tx database.Tx, userID int64, key string) (*withdrawOrder, error) {
	if strings.TrimSpace(key) == "" {
		return nil, nil
	}

	order := &withdrawOrder{}
	err := tx.QueryRow(`
		SELECT withdraw_no, amount, actual_amount, commission_amount, status
		FROM withdraw_orders
		WHERE user_id = ? AND idempotency_key = ?
		LIMIT 1
	`, userID, key).Scan(
		&order.WithdrawNo,
		&order.Amount,
		&order.ActualAmount,
		&order.CommissionAmount,
		&order.Status,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return order, nil
}

func createWithdrawOrderTx(tx database.Tx, userID int64, withdrawNo, idempotencyKey string, amount, actualAmount, commissionAmount float64, status int) (int64, error) {
	now := time.Now()
	id, err := database.InsertReturningID(tx, `
		INSERT INTO withdraw_orders (
			user_id, withdraw_no, idempotency_key, amount, actual_amount, commission_amount, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, withdrawNo, idempotencyKey, amount, actualAmount, commissionAmount, status, now, now)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Withdraw 处理创作者提现
// POST /api/v1/creator/withdraw
func Withdraw(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, AccountResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
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
			Data:    nil,
		})
		return
	}

	userRepo := repository.NewUserRepository(GetDB())
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))

	// 开启事务，使用 IMMEDIATE 模式获取排他锁防止竞态条件
	tx, err := GetDB().Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    50004,
			Message: "开启事务失败",
			Data:    nil,
		})
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 在事务内使用 FOR UPDATE 查询用户，锁定用户行防止并发修改
	user, err := userRepo.GetUserByIDForUpdate(tx, userID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    50001,
			Message: "获取用户失败",
			Data:    nil,
		})
		return
	}

	if user == nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, AccountResponse{
			Code:    40401,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	// 检查是否已实名认证
	if !user.RealNameVerified {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, AccountResponse{
			Code:    40002,
			Message: "请先完成实名认证再提现",
			Data:    nil,
		})
		return
	}

	// 检查可提现余额 (balance - frozen)
	availableBalance := user.Balance - user.FrozenAmount
	if availableBalance < req.Amount {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, AccountResponse{
			Code:    40003,
			Message: "可提现余额不足",
			Data:    nil,
		})
		return
	}

	// 幂等处理：同一用户 + 相同幂等键，直接返回已存在的提现单
	if idempotencyKey != "" {
		existing, idemErr := getWithdrawOrderByIdempotencyKeyTx(tx, userID, idempotencyKey)
		if idemErr != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, AccountResponse{
				Code:    50006,
				Message: "查询幂等提现单失败",
				Data:    nil,
			})
			return
		}
		if existing != nil {
			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, AccountResponse{
					Code:    50005,
					Message: "提交事务失败",
					Data:    nil,
				})
				return
			}
			c.JSON(http.StatusOK, AccountResponse{
				Code:    0,
				Message: "提现申请已存在",
				Data: gin.H{
					"withdraw_no":     existing.WithdrawNo,
					"withdraw_amount": existing.Amount,
					"actual_amount":   existing.ActualAmount,
					"commission":      existing.CommissionAmount,
					"status":          existing.Status,
				},
			})
			return
		}
	}

	// 计算实际到账金额 (扣除平台抽成)
	commissionRate := user.GetCommission()
	actualAmount := calculateWithdrawActualAmount(req.Amount, commissionRate)

	// 计算余额变动
	balanceBefore := user.Balance
	newBalance := user.Balance - req.Amount

	// 在事务中更新余额（行已被锁定，防止竞态）
	err = userRepo.UpdateUserBalanceWithTx(tx, userID, newBalance)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    50002,
			Message: "更新余额失败",
			Data:    nil,
		})
		return
	}

	// 记录提现交易 (在事务中执行)
	withdrawNo := generateWithdrawNo(userID)
	commissionAmount := req.Amount - actualAmount
	withdrawOrderID, err := createWithdrawOrderTx(tx, userID, withdrawNo, idempotencyKey, req.Amount, actualAmount, commissionAmount, withdrawOrderStatusProcessing)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    50003,
			Message: "创建提现单失败",
			Data:    nil,
		})
		return
	}

	transaction := &model.Transaction{
		UserID:        userID,
		Type:          model.TransactionTypeWithdraw,
		Amount:        -req.Amount, // 支出为负
		BalanceBefore: balanceBefore,
		BalanceAfter:  newBalance,
		Remark:        fmt.Sprintf("提现到账%.2f元(扣除佣金%.2f元)", actualAmount, commissionAmount),
		RelatedID:     withdrawOrderID,
		CreatedAt:     time.Now(),
	}

	accountRepo := repository.NewAccountRepository(GetDB())
	if err := accountRepo.CreateTransactionTx(tx, transaction); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    50007,
			Message: "记录交易失败",
			Data:    nil,
		})
		return
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    50005,
			Message: "提交事务失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, AccountResponse{
		Code:    0,
		Message: "提现申请已提交",
		Data: gin.H{
			"withdraw_no":     withdrawNo,
			"withdraw_amount": req.Amount,
			"actual_amount":   actualAmount,
			"commission":      commissionAmount,
			"status":          withdrawOrderStatusProcessing,
			"balance":         newBalance,
		},
	})
}

// Prepay handles task reward pre-payment (freeze amount) - legacy function
// POST /api/v1/account/prepay
func Prepay(c *gin.Context) {
	// All users can prepay now - no role check needed
	businessID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, AccountResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	var req struct {
		TaskID int64 `json:"task_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AccountResponse{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// Get task
	task, err := GetTaskRepo().GetTaskByID(req.TaskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    50001,
			Message: "获取任务失败",
			Data:    nil,
		})
		return
	}

	if task == nil {
		c.JSON(http.StatusNotFound, AccountResponse{
			Code:    40401,
			Message: "任务不存在",
			Data:    nil,
		})
		return
	}

	// Check if business owns the task
	if task.BusinessID != businessID {
		c.JSON(http.StatusForbidden, AccountResponse{
			Code:    40302,
			Message: "无权操作此任务",
			Data:    nil,
		})
		return
	}

	// Get user
	userRepo := repository.NewUserRepository(GetDB())
	user, err := userRepo.GetUserByID(businessID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    50001,
			Message: "获取用户失败",
			Data:    nil,
		})
		return
	}

	// Check if balance is sufficient
	frozenAmount := task.TotalBudget
	if user.Balance < frozenAmount {
		c.JSON(http.StatusBadRequest, AccountResponse{
			Code:    40002,
			Message: "余额不足，无法预付悬赏金",
			Data:    nil,
		})
		return
	}

	// Freeze amount
	balanceBefore := user.Balance
	newBalance := user.Balance - frozenAmount
	newFrozen := user.FrozenAmount + frozenAmount

	err = userRepo.UpdateUserBalance(businessID, newBalance)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    50001,
			Message: "更新余额失败",
			Data:    nil,
		})
		return
	}

	// Update frozen amount
	err = userRepo.UpdateUserFrozenAmount(businessID, newFrozen)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    50001,
			Message: "更新冻结金额失败",
			Data:    nil,
		})
		return
	}

	// Record freeze transaction
	transaction := &model.Transaction{
		UserID:        businessID,
		Type:          model.TransactionTypeFreeze,
		Amount:        frozenAmount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  newBalance,
		Remark:        "悬赏金预付(task_id:" + strconv.FormatInt(req.TaskID, 10) + ")",
		RelatedID:     req.TaskID,
		CreatedAt:     time.Now(),
	}

	accountRepo := repository.NewAccountRepository(GetDB())
	if err := accountRepo.CreateTransaction(transaction); err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    50002,
			Message: "记录交易失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, AccountResponse{
		Code:    0,
		Message: "预付成功",
		Data: gin.H{
			"frozen_amount":  newFrozen,
			"balance":        newBalance,
			"balance_before": balanceBefore,
		},
	})
}

func transactionFeeFromRemark(remark string) float64 {
	match := withdrawCommissionRemarkRe.FindStringSubmatch(remark)
	if len(match) != 2 {
		return 0
	}

	fee, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0
	}
	return fee
}

func lookupWithdrawCommissionAmount(relatedID int64) float64 {
	if relatedID <= 0 {
		return 0
	}

	db := GetDB()
	if db == nil {
		return 0
	}

	var commissionAmount float64
	if err := db.QueryRow(`
		SELECT commission_amount
		FROM withdraw_orders
		WHERE id = ?
		LIMIT 1
	`, relatedID).Scan(&commissionAmount); err != nil {
		return 0
	}

	return commissionAmount
}

func lookupTaskServiceFeeAmount(relatedID int64) float64 {
	if relatedID <= 0 {
		return 0
	}

	db := GetDB()
	if db == nil {
		return 0
	}

	var serviceFeeAmount float64
	if err := db.QueryRow(`
		SELECT service_fee_amount
		FROM tasks
		WHERE id = ?
		LIMIT 1
	`, relatedID).Scan(&serviceFeeAmount); err != nil {
		return 0
	}

	return serviceFeeAmount
}

func lookupClaimRewardFee(relatedID int64, txType model.TransactionType, netAmount float64) float64 {
	if relatedID <= 0 {
		return 0
	}

	db := GetDB()
	if db == nil {
		return 0
	}

	var unitPrice float64
	var awardPrice float64
	if err := db.QueryRow(`
		SELECT COALESCE(t.unit_price, 0), COALESCE(t.award_price, 0)
		FROM claims c
		JOIN tasks t ON t.id = c.task_id
		WHERE c.id = ?
		LIMIT 1
	`, relatedID).Scan(&unitPrice, &awardPrice); err != nil {
		return 0
	}

	grossAmount := 0.0
	switch txType {
	case model.TransactionTypePayment:
		grossAmount = unitPrice
	case model.TransactionTypeAwardPayment:
		grossAmount = awardPrice
	default:
		return 0
	}

	if grossAmount <= 0 {
		return 0
	}

	if netAmount < 0 {
		netAmount = -netAmount
	}
	fee := grossAmount - netAmount
	if fee <= 0 {
		return 0
	}
	return math.Round(fee*100) / 100
}

func deriveTransactionFee(t *model.Transaction) float64 {
	if t == nil {
		return 0
	}

	switch t.Type {
	case model.TransactionTypeWithdraw:
		if fee := lookupWithdrawCommissionAmount(t.RelatedID); fee > 0 {
			return fee
		}
		return transactionFeeFromRemark(t.Remark)
	case model.TransactionTypeFreeze:
		return lookupTaskServiceFeeAmount(t.RelatedID)
	case model.TransactionTypePayment, model.TransactionTypeAwardPayment:
		return lookupClaimRewardFee(t.RelatedID, t.Type, t.Amount)
	default:
		return 0
	}
}

func deriveTransactionFeeLabel(t *model.Transaction) string {
	if t == nil {
		return ""
	}

	switch t.Type {
	case model.TransactionTypeWithdraw:
		return "提现手续费"
	case model.TransactionTypeFreeze:
		if deriveTransactionFee(t) > 0 {
			return "服务费"
		}
		return ""
	case model.TransactionTypePayment:
		return "参与奖励手续费"
	case model.TransactionTypeAwardPayment:
		return "采纳奖励手续费"
	default:
		return ""
	}
}

// ListTransactions handles listing transactions
// GET /api/v1/account/transactions
func ListTransactions(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, AccountResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Get transactions
	accountRepo := repository.NewAccountRepository(GetDB())
	transactions, total, err := accountRepo.ListTransactionsByUserID(userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    50001,
			Message: "获取交易记录失败",
			Data:    nil,
		})
		return
	}

	var formattedTransactions []gin.H
	for _, t := range transactions {
		formattedTransactions = append(formattedTransactions, formatTransaction(t))
	}

	c.JSON(http.StatusOK, AccountResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"transactions": formattedTransactions,
			"total":        total,
		},
	})
}

// formatTransaction converts a Transaction model to a gin.H map
func formatTransaction(t *model.Transaction) gin.H {
	return gin.H{
		"id":             t.ID,
		"user_id":        t.UserID,
		"type":           t.Type,
		"type_str":       t.Type.Name(),
		"type_code":      t.Type.Code(),
		"amount":         t.DisplayAmount(),
		"raw_amount":     t.Amount,
		"fee":            deriveTransactionFee(t),
		"fee_label":      deriveTransactionFeeLabel(t),
		"balance_before": t.BalanceBefore,
		"balance_after":  t.BalanceAfter,
		"remark":         t.Remark,
		"related_id":     t.RelatedID,
		"created_at":     t.CreatedAt.Format(time.RFC3339),
	}
}
