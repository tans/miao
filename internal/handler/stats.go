package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
)

// GetBusinessStats 商家端统计
// GET /api/v1/business/stats
func GetBusinessStats(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	db := GetDB()

	// 统计任务数
	var totalTasks, ongoingTasks, completedTasks int
	db.QueryRow("SELECT COUNT(*) FROM tasks WHERE business_id = ?", userID).Scan(&totalTasks)
	db.QueryRow("SELECT COUNT(*) FROM tasks WHERE business_id = ? AND status = ?", userID, model.TaskStatusOngoing).Scan(&ongoingTasks)
	db.QueryRow("SELECT COUNT(*) FROM tasks WHERE business_id = ? AND status = ?", userID, model.TaskStatusEnded).Scan(&completedTasks)

	// 统计总支出
	var totalExpense float64
	db.QueryRow("SELECT COALESCE(SUM(paid_amount), 0) FROM tasks WHERE business_id = ?", userID).Scan(&totalExpense)

	// 统计待验收数
	var pendingReviews int
	db.QueryRow(`
		SELECT COUNT(*) FROM claims c
		JOIN tasks t ON c.task_id = t.id
		WHERE t.business_id = ? AND c.status = ?
	`, userID, model.ClaimStatusSubmitted).Scan(&pendingReviews)

	// 获取账户余额
	var balance, frozenAmount float64
	db.QueryRow("SELECT balance, frozen_amount FROM users WHERE id = ?", userID).Scan(&balance, &frozenAmount)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total_tasks":     totalTasks,
			"ongoing_tasks":   ongoingTasks,
			"completed_tasks": completedTasks,
			"total_expense":   totalExpense,
			"pending_reviews": pendingReviews,
			"balance":         balance,
			"frozen_amount":   frozenAmount,
		},
	})
}

// GetBusinessExpenseChart 商家支出趋势
// GET /api/v1/business/chart/expense?period=7d
func GetBusinessExpenseChart(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	period := c.DefaultQuery("period", "7d")
	days := 7
	if period == "30d" {
		days = 30
	}

	db := GetDB()
	startDate := time.Now().AddDate(0, 0, -days)

	// 按日期统计支出
	rows, err := db.Query(`
		SELECT DATE(created_at) as date, COALESCE(SUM(amount), 0) as total
		FROM transactions
		WHERE user_id = ? AND type = ? AND created_at >= ?
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`, userID, model.TransactionTypeFreeze, startDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "查询失败",
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	type ChartData struct {
		Date  string  `json:"date"`
		Total float64 `json:"total"`
	}

	var data []ChartData
	for rows.Next() {
		var item ChartData
		rows.Scan(&item.Date, &item.Total)
		data = append(data, item)
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// GetCreatorStats 创作者端统计
// GET /api/v1/creator/stats
func GetCreatorStats(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	db := GetDB()

	// 统计累计采纳数
	var adoptedCount int
	db.QueryRow("SELECT adopted_count FROM users WHERE id = ?", userID).Scan(&adoptedCount)

	// 统计总收益
	var totalIncome float64
	db.QueryRow(`
		SELECT COALESCE(SUM(amount), 0) FROM transactions
		WHERE user_id = ? AND type IN (?, ?, ?)
	`, userID, model.TransactionTypeReward, model.TransactionTypePayment, model.TransactionTypeAwardPayment).Scan(&totalIncome)

	// 统计进行中任务
	var ongoingClaims int
	db.QueryRow("SELECT COUNT(*) FROM claims WHERE creator_id = ? AND status IN (?, ?)",
		userID, model.ClaimStatusPending, model.ClaimStatusSubmitted).Scan(&ongoingClaims)

	// 获取用户信息
	var level int
	var balance, marginFrozen float64
	var dailyClaimCount int
	db.QueryRow(`
		SELECT level, balance, margin_frozen, daily_claim_count
		FROM users WHERE id = ?
	`, userID).Scan(&level, &balance, &marginFrozen, &dailyClaimCount)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"adopted_count":    adoptedCount,
			"total_income":      totalIncome,
			"ongoing_claims":    ongoingClaims,
			"level":             level,
			"balance":           balance,
			"margin_frozen":     marginFrozen,
			"daily_claim_count": dailyClaimCount,
		},
	})
}

// GetCreatorIncomeChart 创作者收益趋势
// GET /api/v1/creator/chart/income?period=7d
func GetCreatorIncomeChart(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	period := c.DefaultQuery("period", "7d")
	days := 7
	if period == "30d" {
		days = 30
	}

	db := GetDB()
	startDate := time.Now().AddDate(0, 0, -days)

	// 按日期统计收益
	rows, err := db.Query(`
		SELECT DATE(created_at) as date, COALESCE(SUM(amount), 0) as total
		FROM transactions
		WHERE user_id = ? AND type IN (?, ?, ?) AND created_at >= ?
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`, userID, model.TransactionTypeReward, model.TransactionTypePayment, model.TransactionTypeAwardPayment, startDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "查询失败",
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	type ChartData struct {
		Date  string  `json:"date"`
		Total float64 `json:"total"`
	}

	var data []ChartData
	for rows.Next() {
		var item ChartData
		rows.Scan(&item.Date, &item.Total)
		data = append(data, item)
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}
