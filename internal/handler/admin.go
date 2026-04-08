package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
)

// AdminResponse represents the standard API response
type AdminResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

var adminRepo *repository.AdminRepository

func initAdminRepo() error {
	if err := initDB(); err != nil {
		return err
	}
	adminRepo = repository.NewAdminRepository(db)
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
	roleStr := c.DefaultQuery("role", "0")
	statusStr := c.DefaultQuery("status", "0")
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	keyword := c.DefaultQuery("keyword", "")

	var role *int
	if r, err := strconv.Atoi(roleStr); err == nil && r > 0 {
		role = &r
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

	users, err := adminRepo.ListUsers(role, status, keyword, limit, offset)
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
	if user.Role == "admin" && req.Status != 1 {
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
		if appeal.Type == model.AppealTypeSubmission {
			typeStr = "投稿申诉"
		}
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
	if appeal.Type == model.AppealTypeSubmission {
		typeStr = "投稿申诉"
	}
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

// ResolveAppealAdmin handles resolving an appeal (admin)
// PUT /api/v1/admin/appeals/:id
func ResolveAppealAdmin(c *gin.Context) {
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

// RequireAdmin is a middleware that requires admin role
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := middleware.GetRoleFromContext(c)
		if !ok || role != "admin" {
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
