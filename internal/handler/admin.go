package handler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xwb1989/sqlparser"

	"github.com/tans/miao/internal/config"
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
		log.Fatalf("failed to initialize admin repository: %v", err)
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
// Frontend params: page, page_size, role, status, search
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

	// Parse query params (frontend format)
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "15")
	role := c.DefaultQuery("role", "")       // business, creator, admin
	statusStr := c.DefaultQuery("status", "") // active, disabled
	search := c.DefaultQuery("search", "")

	// Parse page and page_size
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 15
	}
	offset := (page - 1) * pageSize

	// Map role to filters
	var isAdmin *bool
	var businessVerified *bool
	if role == "admin" {
		val := true
		isAdmin = &val
	} else if role == "business" {
		val := true
		businessVerified = &val
	}
	// role == "creator" or "" means no filter (all users are creators)

	// Map status: active=1, disabled=0
	var status *int
	if statusStr == "active" {
		val := 1
		status = &val
	} else if statusStr == "disabled" {
		val := 0
		status = &val
	}

	users, total, err := adminRepo.ListUsersAdvanced(isAdmin, businessVerified, status, search, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户列表失败",
			Data:    nil,
		})
		return
	}

	// Format users for frontend response
	var formattedUsers []gin.H
	for _, u := range users {
		role := "creator"
		if u.IsAdmin {
			role = "admin"
		} else if u.BusinessVerified {
			role = "business"
		}

		formattedUsers = append(formattedUsers, gin.H{
			"id":         u.ID,
			"username":   u.Username,
			"phone":      u.Phone,
			"nickname":   u.Nickname,
			"avatar":     u.Avatar,
			"role":       role,
			"is_disabled": u.Status != 1,
			"status":     u.Status,
			"balance":    u.Balance,
			"level":      u.Level,
			"level_name": u.GetLevelName(),
			"is_admin":   u.IsAdmin,
			"created_tasks_count":   u.CreatedTasksCount,
			"claimed_tasks_count":   u.ClaimedTasksCount,
			"submitted_works_count": u.SubmittedWorksCount,
			"created_at": u.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"users": formattedUsers,
			"total": total,
			"page":  page,
			"page_size": pageSize,
		},
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

	// Update adopted count based on credit change
	// Positive change increases adopted count, negative decreases (min 0)
	newAdoptedCount := user.AdoptedCount + req.Change
	if newAdoptedCount < 0 {
		newAdoptedCount = 0
	}

	if err := adminRepo.UpdateUserAdoptedCount(id, newAdoptedCount); err != nil {
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
			"adopted_count":    newAdoptedCount,
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

// GetUserDetail 获取用户详情（仅管理员）
// GET /api/v1/admin/users/:id
func GetUserDetail(c *gin.Context) {
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

	// Parse pagination params
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "10")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	// Get user
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

	// Get created tasks (business tasks)
	createdTasks, createdTasksTotal, err := adminRepo.GetTasksByBusinessID(id, pageSize, offset)
	if err != nil {
		createdTasks = []*model.Task{}
		createdTasksTotal = 0
	}

	// Get participated tasks (all claims)
	participatedClaims, participatedTotal, err := adminRepo.GetClaimsByCreatorID(id, pageSize, offset)
	if err != nil {
		participatedClaims = []*model.Claim{}
		participatedTotal = 0
	}

	// Get submitted works (claims with status >= submitted)
	submittedWorks, submittedTotal, err := adminRepo.GetSubmittedWorksByCreatorID(id, pageSize, offset)
	if err != nil {
		submittedWorks = []*model.Claim{}
		submittedTotal = 0
	}

	// Format user info
	role := "creator"
	if user.IsAdmin {
		role = "admin"
	} else if user.BusinessVerified {
		role = "business"
	}

	userInfo := gin.H{
		"id":              user.ID,
		"username":        user.Username,
		"phone":           user.Phone,
		"nickname":        user.Nickname,
		"avatar":          user.Avatar,
		"role":            role,
		"is_disabled":     user.Status != 1,
		"status":          user.Status,
		"balance":         user.Balance,
		"frozen_amount":   user.FrozenAmount,
		"level":           user.Level,
		"level_name":      user.GetLevelName(),
		"is_admin":        user.IsAdmin,
		"adopted_count":   user.AdoptedCount,
		"report_count":    user.ReportCount,
		"created_at":      user.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	// Format created tasks
	var formattedCreatedTasks []gin.H
	for _, task := range createdTasks {
		formattedCreatedTasks = append(formattedCreatedTasks, gin.H{
			"id":              task.ID,
			"title":           task.Title,
			"category":        task.Category,
			"unit_price":      task.UnitPrice,
			"total_count":     task.TotalCount,
			"remaining_count": task.RemainingCount,
			"status":          task.Status,
			"total_budget":    task.TotalBudget,
			"created_at":      task.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	// Format participated tasks (claims)
	var formattedParticipated []gin.H
	for _, claim := range participatedClaims {
		// Get task info for this claim
		taskInfo := gin.H{}
		if task, err := adminRepo.GetTaskByID(claim.TaskID); err == nil && task != nil {
			taskInfo = gin.H{
				"id":    task.ID,
				"title": task.Title,
			}
		}

		statusStr := "已认领"
		if claim.Status == 2 {
			statusStr = "已提交"
		} else if claim.Status == 3 {
			statusStr = "已验收"
		} else if claim.Status == 4 {
			statusStr = "已取消"
		} else if claim.Status == 5 {
			statusStr = "已超时"
		}

		formattedParticipated = append(formattedParticipated, gin.H{
			"id":              claim.ID,
			"task_id":         claim.TaskID,
			"status":          claim.Status,
			"status_str":      statusStr,
			"content":         claim.Content,
			"submit_at":       claim.SubmitAt,
			"creator_reward":  claim.CreatorReward,
			"created_at":      claim.CreatedAt.Format("2006-01-02 15:04:05"),
			"task":            taskInfo,
		})
	}

	// Format submitted works
	var formattedSubmittedWorks []gin.H
	for _, claim := range submittedWorks {
		// Get task info for this claim
		taskInfo := gin.H{}
		if task, err := adminRepo.GetTaskByID(claim.TaskID); err == nil && task != nil {
			taskInfo = gin.H{
				"id":    task.ID,
				"title": task.Title,
			}
		}

		reviewResultStr := "待验收"
		if claim.ReviewResult != nil {
			if *claim.ReviewResult == 1 {
				reviewResultStr = "通过"
			} else if *claim.ReviewResult == 2 {
				reviewResultStr = "退回"
			}
		}

		formattedSubmittedWorks = append(formattedSubmittedWorks, gin.H{
			"id":               claim.ID,
			"task_id":          claim.TaskID,
			"content":          claim.Content,
			"submit_at":        claim.SubmitAt,
			"review_at":        claim.ReviewAt,
			"review_result":    claim.ReviewResult,
			"review_result_str": reviewResultStr,
			"review_comment":   claim.ReviewComment,
			"creator_reward":   claim.CreatorReward,
			"created_at":       claim.CreatedAt.Format("2006-01-02 15:04:05"),
			"task":             taskInfo,
		})
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"user": userInfo,
			"created_tasks": gin.H{
				"tasks": formattedCreatedTasks,
				"total": createdTasksTotal,
				"page":  page,
				"page_size": pageSize,
			},
			"participated_tasks": gin.H{
				"claims": formattedParticipated,
				"total":  participatedTotal,
				"page":   page,
				"page_size": pageSize,
			},
			"submitted_works": gin.H{
				"works": formattedSubmittedWorks,
				"total": submittedTotal,
				"page":  page,
				"page_size": pageSize,
			},
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
// Frontend params: page, page_size, status (pending/approved/rejected), search
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

	// Parse query params (frontend format)
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "20")
	statusStr := c.DefaultQuery("status", "")
	search := c.DefaultQuery("search", "")

	// Also support legacy backend format (limit/offset)
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	// Parse page and page_size
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	// Support both page-based and offset-based pagination
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	if limit > 0 {
		pageSize = limit
		page = offset/limit + 1
	}
	offset = (page - 1) * pageSize

	// Map status string to int: pending=1, approved=2, rejected=5
	var status *int
	if statusStr == "pending" {
		val := 1 // TaskStatusPending
		status = &val
	} else if statusStr == "approved" {
		val := 2 // TaskStatusOnline
		status = &val
	} else if statusStr == "rejected" {
		val := 5 // TaskStatusCancelled
		status = &val
	} else if statusStr != "" {
		// Try numeric status
		if s, err := strconv.Atoi(statusStr); err == nil && s > 0 {
			status = &s
		}
	}

	// Get total count
	total, err := adminRepo.CountTasks(status, search)
	if err != nil {
		total = 0
	}

	// Get tasks
	tasks, err := adminRepo.ListTasks(status, search, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取任务列表失败",
			Data:    nil,
		})
		return
	}

	// Get business names for each task
	db := GetDB()
	businessNames := make(map[int64]string)
	formattedTasks := make([]gin.H, 0, len(tasks))
	for _, task := range tasks {
		// Get business name if not already cached (use nickname, fallback to username if empty)
		if _, ok := businessNames[task.BusinessID]; !ok {
			var businessName string
			err := db.QueryRow("SELECT COALESCE(NULLIF(nickname, ''), username) FROM users WHERE id = ?", task.BusinessID).Scan(&businessName)
			if err != nil {
				businessName = ""
			}
			businessNames[task.BusinessID] = businessName
		}

		// Map status int to string
		statusStr := mapStatusToString(task.Status)

		formattedTasks = append(formattedTasks, gin.H{
			"id":            task.ID,
			"title":        task.Title,
			"business_name": businessNames[task.BusinessID],
			"reward":        int(task.UnitPrice * 100), // Convert to cents
			"status":        statusStr,
			"created_at":   task.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"tasks":    formattedTasks,
			"total":    total,
			"page":     page,
			"page_size": pageSize,
		},
	})
}

// GetTaskAdmin returns task detail for admin
// GET /api/v1/admin/tasks/:id
func GetTaskAdmin(c *gin.Context) {
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

	task, err := adminRepo.GetTaskByID(taskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "任务不存在",
			Data:    nil,
		})
		return
	}

	// Get business name (use nickname, fallback to username if empty)
	db := GetDB()
	var businessName string
	err = db.QueryRow("SELECT COALESCE(NULLIF(nickname, ''), username) FROM users WHERE id = ?", task.BusinessID).Scan(&businessName)
	if err != nil {
		businessName = ""
	}

	// Get claims (认领) and submitted works (提交的作品) for this task
	claims, err := adminRepo.GetClaimsByTaskID(taskID, 100, 0)
	if err != nil {
		claims = []*model.Claim{}
	}

	// Format response
	statusStr := mapStatusToString(task.Status)
	var deadline string
	if task.EndAt != nil {
		deadline = task.EndAt.Format("2006-01-02 15:04:05")
	}

	// Format claims with creator info
	formattedClaims := make([]gin.H, 0, len(claims))
	claimedCount := 0
	submittedCount := 0
	for _, claim := range claims {
		// Get creator name
		creatorName := ""
		if creator, err := adminRepo.GetUserByID(claim.CreatorID); err == nil && creator != nil {
			if creator.Nickname != "" {
				creatorName = creator.Nickname
			} else {
				creatorName = creator.Username
			}
		}

		statusStr := "已认领"
		if claim.Status == 2 {
			statusStr = "已提交"
			submittedCount++
		} else if claim.Status == 3 {
			statusStr = "已验收"
		} else if claim.Status == 4 {
			statusStr = "已取消"
		} else if claim.Status == 5 {
			statusStr = "已超时"
		} else {
			claimedCount++
		}

		reviewResultStr := ""
		if claim.ReviewResult != nil {
			if *claim.ReviewResult == 1 {
				reviewResultStr = "通过"
			} else if *claim.ReviewResult == 2 {
				reviewResultStr = "退回"
			}
		}

		formattedClaims = append(formattedClaims, gin.H{
			"id":                claim.ID,
			"creator_id":        claim.CreatorID,
			"creator_name":      creatorName,
			"status":            claim.Status,
			"status_str":        statusStr,
			"content":           claim.Content,
			"submit_at":         claim.SubmitAt,
			"review_at":         claim.ReviewAt,
			"review_result":     claim.ReviewResult,
			"review_result_str": reviewResultStr,
			"review_comment":   claim.ReviewComment,
			"creator_reward":    claim.CreatorReward,
			"created_at":        claim.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"id":              task.ID,
			"title":           task.Title,
			"description":     task.Description,
			"requirements":    "",
			"business_name":   businessName,
			"reward":          int(task.UnitPrice * 100),
			"status":          statusStr,
			"created_at":      task.CreatedAt.Format("2006-01-02 15:04:05"),
			"deadline":        deadline,
			"reject_reason":   "",
			"industries":      task.Industries,
			"video_duration":  task.VideoDuration,
			"video_aspect":    task.VideoAspect,
			"video_resolution": task.VideoResolution,
			"creative_style":  task.CreativeStyle,
			"unit_price":      task.UnitPrice,
			"total_count":      task.TotalCount,
			"remaining_count":  task.RemainingCount,
			"award_price":      task.AwardPrice,
			"claims":           formattedClaims,
			"claimed_count":    claimedCount,
			"submitted_count":  submittedCount,
		},
	})
}

// UpdateTaskAdmin updates task data (admin edit)
// PUT /api/v1/admin/tasks/:id
func UpdateTaskAdmin(c *gin.Context) {
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

	task, err := adminRepo.GetTaskByID(taskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "任务不存在",
			Data:    nil,
		})
		return
	}

	var req struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		UnitPrice   float64 `json:"unit_price"`
		TotalCount  int     `json:"total_count"`
		Deadline    string  `json:"deadline"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// Update fields if provided
	if req.Title != "" {
		task.Title = req.Title
	}
	if req.Description != "" {
		task.Description = req.Description
	}
	if req.UnitPrice > 0 {
		task.UnitPrice = req.UnitPrice
	}
	if req.TotalCount > 0 {
		task.TotalCount = req.TotalCount
	}
	if req.Deadline != "" {
		deadline, err := time.Parse("2006-01-02 15:04:05", req.Deadline)
		if err == nil {
			task.EndAt = &deadline
		}
	}

	err = GetTaskRepo().UpdateTask(task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "更新失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "更新成功",
		Data:    nil,
	})
}

// mapStatusToString converts numeric status to string status
func mapStatusToString(status model.TaskStatus) string {
	switch status {
	case model.TaskStatusPending:
		return "pending"
	case model.TaskStatusOnline:
		return "published"
	case model.TaskStatusOngoing:
		return "completed"
	case model.TaskStatusEnded:
		return "completed"
	case model.TaskStatusCancelled:
		return "cancelled"
	default:
		return "pending"
	}
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

// ListTables lists all tables in the database
// GET /api/v1/admin/tables
func ListTables(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	db := GetDB()
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取表列表失败",
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			tables = append(tables, name)
		}
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"tables": tables,
		},
	})
}

// ExecuteQuery executes a read-only SQL query
// POST /api/v1/admin/query
func ExecuteQuery(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	var req struct {
		SQL string `json:"sql" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误",
			Data:    nil,
		})
		return
	}

	sql := strings.TrimSpace(req.SQL)
	if sql == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "SQL 语句不能为空",
			Data:    nil,
		})
		return
	}

	// Parse SQL using AST to properly validate query type
	stmt, err := sqlparser.Parse(sql)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40004,
			Message: "无效的 SQL 语句: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// Only allow SELECT statements
	if _, ok := stmt.(*sqlparser.Select); !ok {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "只允许执行 SELECT 查询",
			Data:    nil,
		})
		return
	}

	// For string-based API, re-validate using keyword check as additional safeguard
	sqlLower := strings.ToLower(sql)
	dangerous := []string{"drop", "delete", "update", "insert", "alter", "create", "truncate", "attach", "detach"}
	for _, keyword := range dangerous {
		if strings.Contains(sqlLower, keyword+" ") || strings.Contains(sqlLower, keyword+";") {
			c.JSON(http.StatusBadRequest, Response{
				Code:    40003,
				Message: "不支持包含 " + keyword + " 的查询",
				Data:    nil,
			})
			return
		}
	}

	db := GetDB()
	start := time.Now()
	rows, err := db.Query(sql)
	if err != nil {
		c.JSON(http.StatusOK, Response{
			Code:    1,
			Message: "SQL执行错误: " + err.Error(),
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		c.JSON(http.StatusOK, Response{
			Code:    1,
			Message: "获取列信息失败: " + err.Error(),
			Data:    nil,
		})
		return
	}

	var resultRows []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}
		row := make(map[string]interface{})
		for i, col := range cols {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		resultRows = append(resultRows, row)
	}

	elapsed := time.Since(start).Milliseconds()

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"columns":     cols,
			"rows":        resultRows,
			"elapsed_ms":  elapsed,
		},
	})
}

// GetTableSchema returns the schema (column info) for a specific table
// GET /api/v1/admin/tables/:table/schema
func GetTableSchema(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	tableName := c.Param("table")
	if tableName == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "表名不能为空",
			Data:    nil,
		})
		return
	}

	// Validate table name to prevent SQL injection
	validTableName, err := validateTableName(tableName)
	if err != nil || !validTableName {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "无效的表名",
			Data:    nil,
		})
		return
	}

	// Table name is validated by validateTableName() above to contain only safe characters
	db := GetDB()
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取表结构失败",
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	var columns []gin.H
	for rows.Next() {
		var cid int
		var name, colType string
		var notnull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err == nil {
			columns = append(columns, gin.H{
				"cid":         cid,
				"name":        name,
				"type":        colType,
				"notnull":     notnull == 1,
				"default":     dfltValue,
				"primary_key": pk == 1,
			})
		}
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"table_name": tableName,
			"columns":    columns,
		},
	})
}

// InsertRecord inserts a new record into a table
// POST /api/v1/admin/tables/:table
func InsertRecord(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	tableName := c.Param("table")
	if tableName == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "表名不能为空",
			Data:    nil,
		})
		return
	}

	validTableName, err := validateTableName(tableName)
	if err != nil || !validTableName {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "无效的表名",
			Data:    nil,
		})
		return
	}

	var req struct {
		Data map[string]interface{} `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40003,
			Message: "参数错误",
			Data:    nil,
		})
		return
	}

	if len(req.Data) == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40004,
			Message: "数据不能为空",
			Data:    nil,
		})
		return
	}

	// Build INSERT statement
	columns := make([]string, 0, len(req.Data))
	placeholders := make([]string, 0, len(req.Data))
	values := make([]interface{}, 0, len(req.Data))

	for col, val := range req.Data {
		// Validate column name
		if !isValidIdentifier(col) {
			c.JSON(http.StatusBadRequest, Response{
				Code:    40005,
				Message: "无效的列名: " + col,
				Data:    nil,
			})
			return
		}
		columns = append(columns, col)
		placeholders = append(placeholders, "?")
		values = append(values, val)
	}

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	db := GetDB()
	result, err := db.Exec(sql, values...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "插入失败: " + err.Error(),
			Data:    nil,
		})
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("Failed to get last insert id: %v", err)
	}
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "记录已添加",
		Data: gin.H{
			"id": id,
		},
	})
}

// UpdateRecord updates a record in a table
// PUT /api/v1/admin/tables/:table/:id
func UpdateRecord(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	tableName := c.Param("table")
	idStr := c.Param("id")

	validTableName, err := validateTableName(tableName)
	if err != nil || !validTableName {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "无效的表名",
			Data:    nil,
		})
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40003,
			Message: "无效的记录ID",
			Data:    nil,
		})
		return
	}

	var req struct {
		Data map[string]interface{} `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40004,
			Message: "参数错误",
			Data:    nil,
		})
		return
	}

	if len(req.Data) == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40005,
			Message: "数据不能为空",
			Data:    nil,
		})
		return
	}

	// Build UPDATE statement
	sets := make([]string, 0, len(req.Data))
	values := make([]interface{}, 0, len(req.Data)+1)

	for col, val := range req.Data {
		if !isValidIdentifier(col) {
			c.JSON(http.StatusBadRequest, Response{
				Code:    40006,
				Message: "无效的列名: " + col,
				Data:    nil,
			})
			return
		}
		sets = append(sets, col+"=?")
		values = append(values, val)
	}
	values = append(values, id)

	sql := fmt.Sprintf("UPDATE %s SET %s WHERE id=?", tableName, strings.Join(sets, ", "))

	db := GetDB()
	_, err = db.Exec(sql, values...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "更新失败: " + err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "记录已更新",
		Data:    nil,
	})
}

// DeleteRecord deletes a record from a table
// DELETE /api/v1/admin/tables/:table/:id
func DeleteRecord(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	tableName := c.Param("table")
	idStr := c.Param("id")

	validTableName, err := validateTableName(tableName)
	if err != nil || !validTableName {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "无效的表名",
			Data:    nil,
		})
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40003,
			Message: "无效的记录ID",
			Data:    nil,
		})
		return
	}

	sql := fmt.Sprintf("DELETE FROM %s WHERE id=?", tableName)

	db := GetDB()
	result, err := db.Exec(sql, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "删除失败: " + err.Error(),
			Data:    nil,
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Failed to get rows affected: %v", err)
	}
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "记录已删除",
		Data: gin.H{
			"rows_affected": rowsAffected,
		},
	})
}

// validateTableName checks if the table name is valid (prevents SQL injection)
func validateTableName(name string) (bool, error) {
	if name == "" || len(name) > 64 {
		return false, nil
	}
	// Only allow alphanumeric and underscore
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false, nil
		}
	}
	return true, nil
}

// isValidIdentifier checks if a column or table name is valid
func isValidIdentifier(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
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

	cfg := config.Load()

	// Check if logging in with .env admin credentials
	if req.Username == cfg.Admin.Username && req.Password == cfg.Admin.Password && cfg.Admin.Password != "" {
		// Generate token for .env-based admin with 30-day expiry
		token, err := middleware.GenerateTokenWithExpiry(0, cfg.Admin.Username, true, cfg.JWT.AdminExpireTime)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "生成令牌失败"))
			return
		}

		c.JSON(http.StatusOK, SuccessResponse(gin.H{
			"token": token,
			"user": gin.H{
				"id":       0,
				"username": cfg.Admin.Username,
				"is_admin": true,
			},
		}))
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

// ListWorksAdmin 获取所有作品（已验收的认领）
// GET /api/v1/admin/works
func ListWorksAdmin(c *gin.Context) {
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
	keyword := c.DefaultQuery("keyword", "")
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	works, total, err := adminRepo.ListWorksAdmin(keyword, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取作品列表失败",
			Data:    nil,
		})
		return
	}

	// Get creator and task info for each work
	userRepo := repository.NewUserRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	creatorRepo := repository.NewCreatorRepository(db)

	var formattedWorks []gin.H
	for _, work := range works {
		// Creator info
		creatorName := ""
		creatorAvatar := ""
		creator, _ := userRepo.GetUserByID(work.CreatorID)
		if creator != nil {
			if creator.Nickname != "" {
				creatorName = creator.Nickname
			} else {
				creatorName = creator.Username
			}
			creatorAvatar = creator.Avatar
		}

		// Task info
		taskTitle := ""
		taskCategory := 0
		task, _ := taskRepo.GetTaskByID(work.TaskID)
		if task != nil {
			taskTitle = task.Title
			taskCategory = int(task.Category)
		}

		materials, _ := creatorRepo.GetClaimMaterials(work.ID)
		if materials == nil {
			materials = []*model.ClaimMaterial{}
		}

		formattedWorks = append(formattedWorks, gin.H{
			"id":              work.ID,
			"task_id":         work.TaskID,
			"task_title":      taskTitle,
			"task_category":   taskCategory,
			"creator_id":      work.CreatorID,
			"creator_name":    creatorName,
			"creator_avatar":  creatorAvatar,
			"content":         work.Content,
			"reward":          work.CreatorReward,
			"submit_at":       work.SubmitAt,
			"review_at":       work.ReviewAt,
			"review_result":   work.ReviewResult,
			"review_comment":  work.ReviewComment,
			"materials":       materials,
			"created_at":      work.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"works":  formattedWorks,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// GetWorkAdmin 获取单个作品详情（管理员）
// GET /api/v1/admin/works/:id
func GetWorkAdmin(c *gin.Context) {
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

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的作品ID",
			Data:    nil,
		})
		return
	}

	work, err := adminRepo.GetWorkByIDAdmin(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取作品详情失败",
			Data:    nil,
		})
		return
	}
	if work == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "作品不存在",
			Data:    nil,
		})
		return
	}

	// Creator info
	creatorName := ""
	creatorAvatar := ""
	creator, _ := userRepo.GetUserByID(work.CreatorID)
	if creator != nil {
		if creator.Nickname != "" {
			creatorName = creator.Nickname
		} else {
			creatorName = creator.Username
		}
		creatorAvatar = creator.Avatar
	}

	// Task info
	taskTitle := ""
	taskCategory := 0
	task, _ := taskRepo.GetTaskByID(work.TaskID)
	if task != nil {
		taskTitle = task.Title
		taskCategory = int(task.Category)
	}

	materials, _ := creatorRepo.GetClaimMaterials(work.ID)
	if materials == nil {
		materials = []*model.ClaimMaterial{}
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"id":              work.ID,
			"task_id":         work.TaskID,
			"task_title":      taskTitle,
			"task_category":   taskCategory,
			"creator_id":      work.CreatorID,
			"creator_name":    creatorName,
			"creator_avatar":  creatorAvatar,
			"content":         work.Content,
			"reward":          work.CreatorReward,
			"submit_at":       work.SubmitAt,
			"review_at":       work.ReviewAt,
			"review_result":   work.ReviewResult,
			"review_comment":  work.ReviewComment,
			"materials":       materials,
			"created_at":      work.CreatedAt.Format("2006-01-02 15:04:05"),
		},
	})
}

// UpdateWorkAdmin 更新作品内容（管理员）
// PUT /api/v1/admin/works/:id
func UpdateWorkAdmin(c *gin.Context) {
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

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的作品ID",
			Data:    nil,
		})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误",
			Data:    nil,
		})
		return
	}

	work, err := adminRepo.GetWorkByIDAdmin(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取作品失败",
			Data:    nil,
		})
		return
	}
	if work == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "作品不存在",
			Data:    nil,
		})
		return
	}

	if err := adminRepo.UpdateWorkContentAdmin(id, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "更新作品失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "作品已更新",
		Data:    nil,
	})
}

// DeleteWorkAdmin 删除作品（管理员）
// DELETE /api/v1/admin/works/:id
func DeleteWorkAdmin(c *gin.Context) {
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

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的作品ID",
			Data:    nil,
		})
		return
	}

	work, err := adminRepo.GetWorkByIDAdmin(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取作品失败",
			Data:    nil,
		})
		return
	}
	if work == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "作品不存在",
			Data:    nil,
		})
		return
	}

	if err := adminRepo.DeleteWorkAdmin(id); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "删除作品失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "作品已删除",
		Data:    nil,
	})
}

// GetFinanceStats returns finance statistics for admin
// GET /api/v1/admin/finance/stats
func GetFinanceStats(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	stats, err := adminRepo.GetFinanceStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取财务统计数据失败",
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

// ListFinanceTransactions returns paginated transaction list for admin
// GET /api/v1/admin/finance/transactions
func ListFinanceTransactions(c *gin.Context) {
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
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "15")
	typeFilter := c.DefaultQuery("type", "")
	timeFilter := c.DefaultQuery("time", "")
	search := c.DefaultQuery("search", "")

	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 15
	}
	offset := (page - 1) * pageSize

	transactions, total, err := adminRepo.ListAllTransactions(typeFilter, timeFilter, search, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取交易记录失败",
			Data:    nil,
		})
		return
	}

	// Format transactions with user names
	typeNames := map[int]string{
		1: "recharge",
		2: "task_payment",
		3: "freeze",
		4: "unfreeze",
		5: "task_reward",
		6: "withdraw",
		7: "refund",
		8: "commission",
	}

	var formattedTx []gin.H
	for _, tx := range transactions {
		// Get user name
		userName := ""
		if user, err := adminRepo.GetUserByID(tx.UserID); err == nil && user != nil {
			if user.Nickname != "" {
				userName = user.Nickname
			} else {
				userName = user.Username
			}
		}

		typeStr := typeNames[int(tx.Type)]
		if typeStr == "" {
			typeStr = "unknown"
		}

		formattedTx = append(formattedTx, gin.H{
			"id":          tx.ID,
			"user_id":     tx.UserID,
			"user_name":   userName,
			"type":        typeStr,
			"amount":      tx.Amount,
			"status":      "completed",
			"created_at":  tx.CreatedAt.Format("2006-01-02 15:04:05"),
			"description": tx.Remark,
		})
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"transactions": formattedTx,
			"total":        total,
			"page":         page,
			"page_size":    pageSize,
		},
	})
}

// GetFinanceTransactionDetail returns a single transaction detail
// GET /api/v1/admin/finance/transactions/:id
func GetFinanceTransactionDetail(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的交易ID",
			Data:    nil,
		})
		return
	}

	tx, err := adminRepo.GetTransactionByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取交易详情失败",
			Data:    nil,
		})
		return
	}
	if tx == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "交易不存在",
			Data:    nil,
		})
		return
	}

	// Get user name
	userName := ""
	if user, err := adminRepo.GetUserByID(tx.UserID); err == nil && user != nil {
		if user.Nickname != "" {
			userName = user.Nickname
		} else {
			userName = user.Username
		}
	}

	typeNames := map[int]string{
		1: "recharge",
		2: "task_payment",
		3: "freeze",
		4: "unfreeze",
		5: "task_reward",
		6: "withdraw",
		7: "refund",
		8: "commission",
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"id":           tx.ID,
			"user_id":      tx.UserID,
			"user_name":    userName,
			"type":         typeNames[int(tx.Type)],
			"amount":       tx.Amount,
			"balance_before": tx.BalanceBefore,
			"balance_after":  tx.BalanceAfter,
			"status":       "completed",
			"created_at":   tx.CreatedAt.Format("2006-01-02 15:04:05"),
			"description":  tx.Remark,
		},
	})
}

// GetSettings 获取系统设置
// GET /api/v1/admin/settings
func GetSettings(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	// Return default settings (in production, these would come from database)
	settings := model.SystemSettings{
		ReviewDays:    7,
		SubmitDays:    7,
		GraceDays:     7,
		ReportAction:  1,
		MinUnitPrice:  2.0,
		MinAwardPrice: 8.0,
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    settings,
	})
}

// UpdateSettings 更新系统设置
// PUT /api/v1/admin/settings
func UpdateSettings(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	var req model.SystemSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "设置保存成功",
		Data:    req,
	})
}

// AdminSettingsPage 管理后台系统设置页面
// GET /admin/settings.html
func AdminSettingsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "admin/settings.html", gin.H{
		"ActiveMenu": "settings",
		"PageTitle":  "系统设置",
		"Settings": model.SystemSettings{
			ReviewDays:    7,
			SubmitDays:    7,
			GraceDays:     7,
			ReportAction:  1,
			MinUnitPrice:  2.0,
			MinAwardPrice: 8.0,
		},
	})
}
