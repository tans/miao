package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
	"github.com/tans/miao/internal/service"
)

// AdminResponse represents the standard API response
type AdminResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

var adminRepo *repository.AdminRepository
var notificationService *service.NotificationService

func initAdminRepo() error {
	if err := initDB(); err != nil {
		return err
	}
	adminRepo = repository.NewAdminRepository(db)
	notificationService = service.NewNotificationServiceWithNotification(db)
	return nil
}

func init() {
	if err := initAdminRepo(); err != nil {
		panic("failed to initialize admin repository: " + err.Error())
	}
}

// GetDashboard handles dashboard statistics
// GET /api/v1/admin/dashboard
func GetDashboard(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	stats, err := adminRepo.GetDashboardStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取统计数据失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// GetStats returns platform statistics
// GET /api/v1/admin/stats
func GetStats(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	stats, err := adminRepo.GetDashboardStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取统计数据失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// ReviewTask 审核任务上架
// PUT /api/v1/admin/task/:id/review
func ReviewTask(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	taskID := parseInt64(c.Param("id"), 0)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的任务ID",
			Data:    nil,
		})
		return
	}

	var req struct {
		Approved bool   `json:"approved"` // true=通过, false=拒绝
		Comment  string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	task, err := adminRepo.GetTaskByID(taskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "任务不存在",
			Data:    nil,
		})
		return
	}

	if task.Status != model.TaskStatusPending {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "任务不在待审核状态",
			Data:    nil,
		})
		return
	}

	now := time.Now()

	if req.Approved {
		// Approve - set status to online
		err = adminRepo.ApproveTask(taskID, now)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50001,
				Message: "审核失败",
				Data:    nil,
			})
			return
		}

		// Send notification to business
		notificationService.NotifyTaskReviewed(task.BusinessID, task.ID, task.Title, true, "")
	} else {
		// Reject - cancel task and unfreeze money
		err = adminRepo.RejectTask(taskID, now, req.Comment)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50002,
				Message: "审核失败",
				Data:    nil,
			})
			return
		}

		// Unfreeze business balance
		business, _ := adminRepo.GetUserByID(task.BusinessID)
		if business != nil {
			adminRepo.UpdateUserBalance(task.BusinessID, business.Balance+task.TotalBudget)
			adminRepo.UpdateUserFrozenAmount(task.BusinessID, business.FrozenAmount-task.TotalBudget)

			// Create transaction
			tx := &model.Transaction{
				UserID:        task.BusinessID,
				Type:          model.TransactionTypeUnfreeze,
				Amount:        task.TotalBudget,
				BalanceBefore: business.Balance,
				BalanceAfter:  business.Balance + task.TotalBudget,
				Remark:        "任务审核拒绝退还: " + task.Title,
				RelatedID:     task.ID,
				CreatedAt:     now,
			}
			adminRepo.CreateTransaction(tx)
		}

		// Send notification to business
		notificationService.NotifyTaskReviewed(task.BusinessID, task.ID, task.Title, false, req.Comment)
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "审核完成",
		Data:    nil,
	})
}

// ListUsers handles listing users with filters
// GET /api/v1/admin/users
func ListUsers(c *gin.Context) {
	// Check admin auth
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	// Parse query params
	isAdminStr := c.DefaultQuery("is_admin", "")
	statusStr := c.DefaultQuery("status", "0")
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	keyword := c.DefaultQuery("keyword", "")

	var isAdmin *bool
	if isAdminStr == "true" || isAdminStr == "1" {
		val := true
		isAdmin = &val
	} else if isAdminStr == "false" || isAdminStr == "0" {
		val := false
		isAdmin = &val
	}

	var status *int
	if s, err := strconv.Atoi(statusStr); err == nil && s > 0 {
		status = &s
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(offsetStr)
	if offset < 0 {
		offset = 0
	}

	users, err := adminRepo.ListUsers(isAdmin, status, keyword, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户列表失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    users,
	})
}

// UpdateUserStatus handles updating user status
// PUT /api/v1/admin/users/:id/status
func UpdateUserStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的用户ID",
			Data:    nil,
		})
		return
	}

	var req struct {
		Status int `json:"status" binding:"required,oneof=1 2 3"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	user, err := adminRepo.GetUserByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	// Prevent changing admin status
	if user.IsAdmin && req.Status != 1 {
		c.JSON(http.StatusForbidden, Response{
			Code:    40301,
			Message: "无法禁用管理员账户",
			Data:    nil,
		})
		return
	}

	if err := adminRepo.UpdateUserStatus(id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "更新用户状态失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "用户状态已更新",
		Data: gin.H{
			"id":     id,
			"status": req.Status,
		},
	})
}

// UpdateUserCredit handles updating user credit score
// PUT /api/v1/admin/users/:id/credit
func UpdateUserCredit(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的用户ID",
			Data:    nil,
		})
		return
	}

	var req struct {
		Change int    `json:"change" binding:"required"`
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	user, err := adminRepo.GetUserByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	// Update behavior score
	newBehaviorScore := user.BehaviorScore + req.Change
	if newBehaviorScore < -1000 {
		newBehaviorScore = -1000
	}
	if newBehaviorScore > 2000 {
		newBehaviorScore = 2000
	}

	// Update total score
	newTotalScore := newBehaviorScore + int(user.TradeScore)

	if err := adminRepo.UpdateUserScore(id, newBehaviorScore, user.TradeScore, newTotalScore); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "更新积分失败",
			Data:    nil,
		})
		return
	}

	// Create credit log
	creditLog := &model.CreditLog{
		UserID:    id,
		Type:      model.CreditLogTypeReward,
		Change:    req.Change,
		Reason:    req.Reason,
		CreatedAt: time.Now(),
	}
	if req.Change < 0 {
		creditLog.Type = model.CreditLogTypePunish
	}
	adminRepo.CreateCreditLog(creditLog)

	// Check if level needs update
	adminRepo.UpdateCreatorLevel(id)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "积分已更新",
		Data: gin.H{
			"id":               id,
			"change":           req.Change,
			"behavior_score":   newBehaviorScore,
			"total_score":      newTotalScore,
		},
	})
}

// UpdateUserBalance handles updating user wallet balance (admin operation)
// PUT /api/v1/admin/users/:id/balance
func UpdateUserBalance(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的用户ID",
			Data:    nil,
		})
		return
	}

	var req struct {
		Change float64 `json:"change" binding:"required"` // 变更金额，正数增加，负数减少
		Reason string  `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	user, err := adminRepo.GetUserByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	// Calculate new balance
	newBalance := user.Balance + req.Change
	if newBalance < 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "余额不足，无法减少更多",
			Data:    nil,
		})
		return
	}

	// Update balance
	if err := adminRepo.UpdateUserBalance(id, newBalance); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "更新余额失败",
			Data:    nil,
		})
		return
	}

	// Determine transaction type
	txType := model.TransactionTypeReward // 5 = 奖励
	if req.Change < 0 {
		txType = model.TransactionTypeConsume // 2 = 消费
	}

	// Create transaction record for audit
	tx := &model.Transaction{
		UserID:        id,
		Type:          txType,
		Amount:        req.Change,
		BalanceBefore: user.Balance,
		BalanceAfter:  newBalance,
		Remark:        req.Reason,
		CreatedAt:     time.Now(),
	}
	adminRepo.CreateTransaction(tx)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "余额已更新",
		Data: gin.H{
			"id":            id,
			"change":        req.Change,
			"balance":       newBalance,
			"balance_before": user.Balance,
		},
	})
}

// GetUserTransactionsAdmin 获取指定用户的交易记录（仅管理员）
// GET /api/v1/admin/users/:id/transactions
func GetUserTransactionsAdmin(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的用户ID",
			Data:    nil,
		})
		return
	}

	// Parse query params
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(offsetStr)
	if offset < 0 {
		offset = 0
	}

	// Check if user exists
	user, err := adminRepo.GetUserByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}
	if user == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	// Get transactions
	accountRepo := GetAccountRepo()
	transactions, total, err := accountRepo.ListTransactionsByUserID(id, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取交易记录失败",
			Data:    nil,
		})
		return
	}

	// Format transactions for response
	var formattedTx []gin.H
	typeNames := map[int]string{
		1: "充值",
		2: "消费",
		3: "冻结",
		4: "解冻",
		5: "奖励",
		6: "提现",
		7: "退保证金",
		8: "抽成",
	}
	for _, tx := range transactions {
		typeName := typeNames[int(tx.Type)]
		if typeName == "" {
			typeName = "未知"
		}
		formattedTx = append(formattedTx, gin.H{
			"id":             tx.ID,
			"user_id":        tx.UserID,
			"type":           tx.Type,
			"type_str":       typeName,
			"amount":         tx.Amount,
			"balance_before": tx.BalanceBefore,
			"balance_after":  tx.BalanceAfter,
			"remark":         tx.Remark,
			"related_id":     tx.RelatedID,
			"created_at":     tx.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"transactions": formattedTx,
			"total":         total,
			"limit":         limit,
			"offset":        offset,
		},
	})
}

// ListTasksAdmin handles listing tasks (admin view)
// GET /api/v1/admin/tasks
func ListTasksAdmin(c *gin.Context) {
	// Check admin auth
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	// Parse query params
	statusStr := c.DefaultQuery("status", "0")
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	keyword := c.DefaultQuery("keyword", "")

	var status *int
	if s, err := strconv.Atoi(statusStr); err == nil && s > 0 {
		status = &s
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(offsetStr)
	if offset < 0 {
		offset = 0
	}

	tasks, err := adminRepo.ListTasks(status, keyword, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取任务列表失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    tasks,
	})
}

// ListClaimsAdmin 获取所有认领
// GET /api/v1/admin/claims
func ListClaimsAdmin(c *gin.Context) {
	// Check admin auth
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	// Parse query params
	statusStr := c.DefaultQuery("status", "0")
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	var status *int
	if s, err := strconv.Atoi(statusStr); err == nil && s > 0 {
		status = &s
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(offsetStr)
	if offset < 0 {
		offset = 0
	}

	claims, err := adminRepo.ListClaims(status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取认领列表失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    claims,
	})
}

// ListAppealsAdmin handles listing all appeals (admin view)
// GET /api/v1/admin/appeals
func ListAppealsAdmin(c *gin.Context) {
	// Parse query params
	statusStr := c.DefaultQuery("status", "0")
	typeStr := c.DefaultQuery("type", "0")
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	status, _ := strconv.Atoi(statusStr)
	appealType, _ := strconv.Atoi(typeStr)
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(offsetStr)
	if offset < 0 {
		offset = 0
	}

	appeals, total, err := adminRepo.GetAllAppeals(status, appealType, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取申诉列表失败",
			Data:    nil,
		})
		return
	}

	var formattedAppeals []gin.H
	for _, appeal := range appeals {
		typeStr := "任务申诉"
		statusStr := "待处理"
		if appeal.Status == model.AppealStatusResolved {
			statusStr = "已处理"
		}
		formattedAppeals = append(formattedAppeals, gin.H{
			"id":         appeal.ID,
			"user_id":    appeal.UserID,
			"type":       appeal.Type,
			"type_str":   typeStr,
			"target_id":  appeal.TargetID,
			"reason":     appeal.Reason,
			"status":     appeal.Status,
			"status_str": statusStr,
			"result":     appeal.Result,
			"created_at": appeal.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"appeals": formattedAppeals,
			"total":   total,
			"limit":   limit,
			"offset":  offset,
		},
	})
}

// GetAppealAdmin gets a single appeal by ID
// GET /api/v1/admin/appeals/:id
func GetAppealAdmin(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的申诉ID",
			Data:    nil,
		})
		return
	}

	appeal, err := appealRepo.GetAppealByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取申诉详情失败",
			Data:    nil,
		})
		return
	}

	if appeal == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "申诉不存在",
			Data:    nil,
		})
		return
	}

	typeStr := "任务申诉"
	statusStr := "待处理"
	if appeal.Status == model.AppealStatusResolved {
		statusStr = "已处理"
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"id":         appeal.ID,
			"user_id":    appeal.UserID,
			"type":       appeal.Type,
			"type_str":   typeStr,
			"target_id":  appeal.TargetID,
			"reason":     appeal.Reason,
			"status":     appeal.Status,
			"status_str": statusStr,
			"result":     appeal.Result,
			"created_at": appeal.CreatedAt.Format(time.RFC3339),
		},
	})
}

// HandleAppeal handles resolving an appeal (admin)
// PUT /api/v1/admin/appeals/:id/handle
func HandleAppeal(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的申诉ID",
			Data:    nil,
		})
		return
	}

	var req struct {
		Result string `json:"result" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	appeal, err := appealRepo.GetAppealByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取申诉详情失败",
			Data:    nil,
		})
		return
	}

	if appeal == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "申诉不存在",
			Data:    nil,
		})
		return
	}

	if err := adminRepo.ResolveAppeal(id, req.Result); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "处理申诉失败",
			Data:    nil,
		})
		return
	}

	// Send notification to user
	notificationService.NotifyAppealHandled(appeal.UserID, appeal.ID, req.Result)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "申诉已处理",
		Data: gin.H{
			"id":     appeal.ID,
			"status": 2,
			"result": req.Result,
		},
	})
}

// ResolveAppealAdmin is deprecated, use HandleAppeal instead
func ResolveAppealAdmin(c *gin.Context) {
	HandleAppeal(c)
}

// RequireAdmin is a middleware that requires admin role
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, ok := middleware.GetIsAdminFromContext(c)
		if !ok || !isAdmin {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    40301,
				"message": "需要管理员权限",
				"data":    nil,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// AdminLogin handles administrator authentication
// POST /api/v1/admin/login
func AdminLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	// Admin users have is_admin=true in the user table
	token, user, err := authService.Login(req.Username, req.Password)
	if err != nil {
		if err == service.ErrInvalidUsername {
			c.JSON(http.StatusNotFound, ErrorResponse(CodeUserNotFound, "用户名不存在"))
			return
		}
		if err == service.ErrInvalidPassword {
			c.JSON(http.StatusUnauthorized, ErrorResponse(CodeInvalidPassword, "密码错误"))
			return
		}
		if err == service.ErrUserDisabled {
			c.JSON(http.StatusForbidden, ErrorResponse(CodeForbidden, "账户已被禁用"))
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "登录失败："+err.Error()))
		return
	}

	// Verify the user is actually an admin
	if !user.IsAdmin {
		c.JSON(http.StatusForbidden, ErrorResponse(CodeForbidden, "非管理员账户无权访问后台"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"token": token,
		"user":  buildAuthUserData(user),
	}))
}

// AdminRegister handles administrator registration
// POST /api/v1/admin/register
func AdminRegister(c *gin.Context) {
	var req model.AdminRegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	// Admin registration always sets isAdmin to true
	_, err := authService.Register(req.Username, req.Password, req.Phone, true, req.RealName, "")
	if err != nil {
		if err == service.ErrUserExists {
			c.JSON(http.StatusConflict, ErrorResponse(CodeUsernameExists))
			return
		}
		if err == service.ErrPhoneExists {
			c.JSON(http.StatusConflict, ErrorResponse(CodePhoneExists))
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "注册失败："+err.Error()))
		return
	}

	token, user, err := authService.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "注册成功但自动登录失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"token": token,
		"user":  buildAuthUserData(user),
	}))
}
