package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
	"github.com/tans/miao/internal/service"
)

var businessRepo *repository.BusinessRepository
var businessNotificationService *service.NotificationService

func init() {
	cfg := config.Load()
	db, err := database.InitDB(cfg.Database.Path)
	if err != nil {
		panic("failed to initialize db: " + err.Error())
	}
	businessRepo = repository.NewBusinessRepository(db)
	businessNotificationService = service.NewNotificationService(db)
}

// CreateTask 发布任务
// POST /api/v1/business/task
func CreateTask(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	var req model.TaskCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// Get user to check business verification
	user, err := businessRepo.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}

	// Check if user is business
	if !strings.Contains(user.Role, "business") {
		c.JSON(http.StatusForbidden, Response{
			Code:    40301,
			Message: "只有商家可以发布任务",
			Data:    nil,
		})
		return
	}

	// Check business verification
	if !user.BusinessVerified {
		c.JSON(http.StatusForbidden, Response{
			Code:    40302,
			Message: "需要完成企业实名认证才能发布任务",
			Data:    nil,
		})
		return
	}

	// Calculate total budget
	totalBudget := req.UnitPrice * float64(req.TotalCount)

	// Check if user has enough balance (100%预付)
	if user.Balance < totalBudget {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "余额不足，需要预付总金额",
			Data: gin.H{
				"required":  totalBudget,
				"available": user.Balance,
			},
		})
		return
	}

	// Create task
	task := &model.Task{
		BusinessID:     userID,
		Title:         req.Title,
		Description:   req.Description,
		Category:      req.Category,
		UnitPrice:     req.UnitPrice,
		TotalCount:    req.TotalCount,
		RemainingCount: req.TotalCount,
		Status:        model.TaskStatusPending, // 待审核
		TotalBudget:   totalBudget,
		FrozenAmount:  0,
		PaidAmount:    0,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Parse deadline if provided
	if req.Deadline != "" {
		deadline, err := time.Parse(time.RFC3339, req.Deadline)
		if err == nil {
			task.EndAt = &deadline
		}
	}

	err = businessRepo.CreateTask(task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50002,
			Message: "创建任务失败",
			Data:    nil,
		})
		return
	}

	// Freeze 100% budget
	newBalance := user.Balance - totalBudget
	newFrozenAmount := user.FrozenAmount + totalBudget

	err = businessRepo.UpdateUserBalance(userID, newBalance)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50003,
			Message: "冻结金额失败",
			Data:    nil,
		})
		return
	}

	err = businessRepo.UpdateUserFrozenAmount(userID, newFrozenAmount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50004,
			Message: "冻结金额更新失败",
			Data:    nil,
		})
		return
	}

	// Update task frozen amount
	businessRepo.UpdateTaskFrozenAmount(task.ID, totalBudget)

	// Create transaction record
	transaction := &model.Transaction{
		UserID:        userID,
		Type:          model.TransactionTypeFreeze,
		Amount:        totalBudget,
		BalanceBefore: user.Balance,
		BalanceAfter:  user.Balance - totalBudget,
		Remark:        "发布任务冻结: " + task.Title,
		RelatedID:     task.ID,
		CreatedAt:     time.Now(),
	}
	businessRepo.CreateTransaction(transaction)

	// Update publish count
	businessRepo.UpdateUserPublishCount(userID, user.PublishCount+1)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "任务发布成功，等待审核",
		Data: gin.H{
			"task_id": task.ID,
		},
	})
}

// ListMyTasks 获取我的任务列表
// GET /api/v1/business/tasks
func ListMyTasks(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	tasks, err := businessRepo.ListTasksByBusinessID(userID)
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

// GetTaskClaims 获取任务的认领列表
// GET /api/v1/business/task/:id/claims
func GetTaskClaims(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
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

	// Verify task ownership
	task, err := businessRepo.GetTaskByID(taskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "任务不存在",
			Data:    nil,
		})
		return
	}

	if task.BusinessID != userID {
		c.JSON(http.StatusForbidden, Response{
			Code:    40301,
			Message: "无权查看此任务的认领",
			Data:    nil,
		})
		return
	}

	claims, err := businessRepo.ListClaimsByTaskID(taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50002,
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

// GetClaim 获取认领详情
// GET /api/v1/business/claim/:id
func GetClaim(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	claimID := parseInt64(c.Param("id"), 0)
	if claimID == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的认领ID",
			Data:    nil,
		})
		return
	}

	claim, err := businessRepo.GetClaimByID(claimID)
	if err != nil || claim == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "认领不存在",
			Data:    nil,
		})
		return
	}

	// Get task to verify ownership
	task, err := businessRepo.GetTaskByID(claim.TaskID)
	if err != nil || task == nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50003,
			Message: "获取任务信息失败",
			Data:    nil,
		})
		return
	}

	// Verify business owns the task
	if task.BusinessID != userID {
		c.JSON(http.StatusForbidden, Response{
			Code:    40302,
			Message: "无权查看此认领",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    claim,
	})
}

// ReviewClaim 验收认领
// PUT /api/v1/business/claim/:id/review
func ReviewClaim(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	claimID := parseInt64(c.Param("id"), 0)
	if claimID == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的认领ID",
			Data:    nil,
		})
		return
	}

	var req model.ClaimReview
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// Get claim
	claim, err := businessRepo.GetClaimByID(claimID)
	if err != nil || claim == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "认领不存在",
			Data:    nil,
		})
		return
	}

	// Get task to verify ownership
	task, err := businessRepo.GetTaskByID(claim.TaskID)
	if err != nil || task == nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50003,
			Message: "获取任务信息失败",
			Data:    nil,
		})
		return
	}

	if task.BusinessID != userID {
		c.JSON(http.StatusForbidden, Response{
			Code:    40301,
			Message: "无权验收此认领",
			Data:    nil,
		})
		return
	}

	// Get business user for frozen amount update
	businessUser, err := businessRepo.GetUserByID(userID)
	if err != nil || businessUser == nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50005,
			Message: "获取商家信息失败",
			Data:    nil,
		})
		return
	}

	// Check claim status (must be submitted)
	if claim.Status != model.ClaimStatusSubmitted {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "当前状态不允许验收",
			Data:    nil,
		})
		return
	}

	now := time.Now()

	if req.Result == 1 {
		// Get creator to calculate dynamic commission
		creator, err := businessRepo.GetUserByID(claim.CreatorID)
		if err != nil || creator == nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50006,
				Message: "获取创作者信息失败",
				Data:    nil,
			})
			return
		}

		// Calculate reward based on creator level (dynamic commission)
		commissionRate := creator.GetCommission()
		creatorReward := task.UnitPrice * (1.0 - commissionRate)
		platformFee := task.UnitPrice * commissionRate

		err = businessRepo.ApproveClaim(claimID, now, req.Comment, creatorReward, platformFee)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50004,
				Message: "验收失败",
				Data:    nil,
			})
			return
		}

		// Return margin if applicable
		marginReturned := 0.0
		if creator.NeedMargin() {
			marginReturned = 10.0
			businessRepo.UpdateUserMarginFrozen(claim.CreatorID, creator.MarginFrozen-marginReturned)
		}

		// Pay creator (reward + margin return)
		payment := creatorReward + marginReturned
		businessRepo.UpdateUserBalance(claim.CreatorID, creator.Balance+payment)

		// Update task paid amount
		businessRepo.UpdateTaskPaidAmount(task.ID, task.PaidAmount+task.UnitPrice)

		// Create transaction for creator
		creatorTx := &model.Transaction{
			UserID:        claim.CreatorID,
			Type:          model.TransactionTypeReward,
			Amount:        creatorReward,
			BalanceBefore: creator.Balance,
			BalanceAfter:  creator.Balance + creatorReward,
			Remark:        "任务交付收入: " + task.Title,
			RelatedID:     claim.ID,
			CreatedAt:     now,
		}
		businessRepo.CreateTransaction(creatorTx)

		// Update creator trade score
		newTradeScore := creator.TradeScore + task.UnitPrice*0.1
		if newTradeScore > 500 {
			newTradeScore = 500
		}
		newTotalScore := creator.BehaviorScore + int(newTradeScore)
		businessRepo.UpdateUserScore(claim.CreatorID, creator.BehaviorScore, newTradeScore, newTotalScore)

		// Update level based on total score and completed orders
		businessRepo.UpdateCreatorLevel(claim.CreatorID)

		// Unfreeze remaining budget for this claim
		newTaskFrozen := task.FrozenAmount - task.UnitPrice
		if newTaskFrozen < 0 {
			newTaskFrozen = 0
		}
		businessRepo.UpdateTaskFrozenAmount(task.ID, newTaskFrozen)

		// Also update business user's frozen amount
		newBusinessFrozen := businessUser.FrozenAmount - task.UnitPrice
		if newBusinessFrozen < 0 {
			newBusinessFrozen = 0
		}
		businessRepo.UpdateUserFrozenAmount(userID, newBusinessFrozen)

		// Send notification to creator
		businessNotificationService.NotifyReviewResult(claim.CreatorID, claim.ID, task.Title, true, req.Comment)

	} else {
		// Returned - 发回给创作者重新提交
		err = businessRepo.ReturnClaim(claimID, now, req.Comment)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50005,
				Message: "验收失败",
				Data:    nil,
			})
			return
		}

		// Send notification to creator
		businessNotificationService.NotifyReviewResult(claim.CreatorID, claim.ID, task.Title, false, req.Comment)
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "验收成功",
		Data:    nil,
	})
}

// GetAllClaims 获取商家所有认领列表
// GET /api/v1/business/claims
func GetAllClaims(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	statusStr := c.Query("status")
	var status *int
	if statusStr != "" {
		if s, err := strconv.Atoi(statusStr); err == nil && s >= 0 {
			status = &s
		}
	}

	claims, err := businessRepo.ListClaimsByBusinessID(userID, status)
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

// GetBalance 获取账户余额
// GET /api/v1/business/balance
func GetBalance(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	user, err := businessRepo.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"balance":       user.Balance,
			"frozen_amount": user.FrozenAmount,
		},
	})
}

// CancelTask 取消任务
// DELETE /api/v1/business/task/:id
func CancelTask(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
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

	// Get task
	task, err := businessRepo.GetTaskByID(taskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "任务不存在",
			Data:    nil,
		})
		return
	}

	// Verify ownership
	if task.BusinessID != userID {
		c.JSON(http.StatusForbidden, Response{
			Code:    40301,
			Message: "无权取消此任务",
			Data:    nil,
		})
		return
	}

	// Check if task can be cancelled (only online or ongoing)
	if task.Status != model.TaskStatusOnline && task.Status != model.TaskStatusOngoing {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "当前状态不允许取消",
			Data:    nil,
		})
		return
	}

	// Get business user
	business, err := businessRepo.GetUserByID(userID)
	if err != nil || business == nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50005,
			Message: "获取商家信息失败",
			Data:    nil,
		})
		return
	}

	// Calculate refund amount
	frozenAmount := task.FrozenAmount - task.PaidAmount

	// Cancel all pending claims and return margins
	claims, _ := businessRepo.ListClaimsByTaskID(taskID)
	for _, claim := range claims {
		if claim.Status == model.ClaimStatusPending {
			// Get creator for margin
			creator, _ := businessRepo.GetUserByID(claim.CreatorID)
			if creator != nil && creator.NeedMargin() {
				businessRepo.UpdateUserMarginFrozen(claim.CreatorID, creator.MarginFrozen-10)
				businessRepo.UpdateUserBalance(claim.CreatorID, creator.Balance+10)
			}
			// Cancel claim
			businessRepo.UpdateClaimStatus(claim.ID, model.ClaimStatusCancelled)
		}
	}

	// Refund remaining frozen amount to business
	if frozenAmount > 0 {
		newBalance := business.Balance + frozenAmount
		newFrozen := business.FrozenAmount - frozenAmount
		businessRepo.UpdateUserBalance(userID, newBalance)
		businessRepo.UpdateUserFrozenAmount(userID, newFrozen)

		// Create transaction record
		tx := &model.Transaction{
			UserID:        userID,
			Type:          model.TransactionTypeUnfreeze,
			Amount:        frozenAmount,
			BalanceBefore: business.Balance,
			BalanceAfter:  newBalance,
			Remark:        "任务取消解冻: " + task.Title,
			RelatedID:     task.ID,
			CreatedAt:     time.Now(),
		}
		businessRepo.CreateTransaction(tx)
	}

	// Update task status to cancelled
	businessRepo.UpdateTaskStatus(task.ID, model.TaskStatusCancelled)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "任务已取消",
		Data: gin.H{
			"refunded": frozenAmount,
		},
	})
}
