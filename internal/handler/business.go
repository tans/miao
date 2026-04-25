package handler

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
	"github.com/tans/miao/internal/service"
)

// AIWriteTaskDescription generates a task description using AI
// POST /api/v1/business/tasks/ai-write
func AIWriteTaskDescription(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	_ = userID // userID not used in this endpoint as we don't require auth context for AI generation

	var req struct {
		Title      string   `json:"title"`
		Industries []string `json:"industries"`
		Styles     []string `json:"styles"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	if req.Title == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "请提供任务标题",
			Data:    nil,
		})
		return
	}

	aiService := service.GetAIService()
	result, err := aiService.GenerateTaskDescription(&service.AIWriteRequest{
		Title:      req.Title,
		Industries: req.Industries,
		Styles:     req.Styles,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "AI服务调用失败",
			Data:    nil,
		})
		return
	}

	if !result.Success {
		c.JSON(http.StatusOK, Response{
			Code:    0,
			Message: result.Error,
			Data: gin.H{
				"success":     false,
				"description": "",
				"error":       result.Error,
			},
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"success":     true,
			"description": result.Description,
		},
	})
}

var businessRepo *repository.BusinessRepository
var businessNotificationService *service.NotificationService

func init() {
	_ = initDB() // ensure shared DB is initialized first
	db := GetDB()
	businessRepo = repository.NewBusinessRepository(db)
	businessNotificationService = service.NewNotificationService(db)
}

// CreateTask 发布任务
// POST /api/v1/business/tasks
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

	// All users are businesses now - no role check needed
	// Check business verification
	if !user.BusinessVerified {
		c.JSON(http.StatusForbidden, Response{
			Code:    40302,
			Message: "需要完成企业实名认证才能发布任务",
			Data:    nil,
		})
		return
	}

	// Calculate total budget = 基础预算 + 服务费
	// 基础预算 = 报名上限 × (参与奖励 + 采纳奖励)
	baseBudget := float64(req.TotalCount) * (req.UnitPrice + req.AwardPrice)
	serviceFeeRate := 0.10
	if req.Public {
		serviceFeeRate = 0.05
	}
	serviceFeeAmount := baseBudget * serviceFeeRate
	totalBudget := baseBudget + serviceFeeAmount

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
		Title:          req.Title,
		Description:    req.Description,
		Category:       model.NormalizeTaskCategory(req.Category),
		UnitPrice:      req.UnitPrice,
		TotalCount:     req.TotalCount,
		RemainingCount: req.TotalCount,
		Status:         model.TaskStatusOnline, // 已上线，无需审核
		TotalBudget:    totalBudget,
		FrozenAmount:   0,
		PaidAmount:     0,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),

		// v1.md 规范新增字段
		Industries:      strings.Join(req.Industries, ","),
		VideoDuration:   req.VideoDuration,
		VideoAspect:     req.VideoAspect,
		VideoResolution: req.VideoResolution,
		Styles:          string(req.Styles),
		AwardPrice:      req.AwardPrice,

		// 即梦合拍字段
		JimengLink: req.JimengLink,
		JimengCode: req.JimengCode,

		// 投稿开放与服务费
		Public:           req.Public,
		ServiceFeeRate:   serviceFeeRate,
		ServiceFeeAmount: serviceFeeAmount,
	}

	// Parse deadline if provided - must be in the future
	// If not provided, automatically set to created date + 7 days
	now := time.Now()
	var deadline time.Time
	if req.Deadline == "" {
		// Auto-set deadline to created date + 7 days
		deadline = now.AddDate(0, 0, 7)
	} else {
		var err error
		deadline, err = time.Parse(time.RFC3339, req.Deadline)
		if err != nil {
			// datetime-local format: 2026-04-20T15:30 -> treat as local time
			deadline, err = time.Parse("2006-01-02T15:04", req.Deadline)
			if err != nil {
				// date-only format: 2026-04-20 (used by mini program)
				deadline, err = time.Parse("2006-01-02", req.Deadline)
				if err != nil {
					c.JSON(http.StatusBadRequest, Response{
						Code:    40006,
						Message: "截止日期格式错误",
						Data:    nil,
					})
					return
				}
			}
		}
		if deadline.Before(now) {
			c.JSON(http.StatusBadRequest, Response{
				Code:    40007,
				Message: "截止日期必须是将来的时间",
				Data:    nil,
			})
			return
		}
	}
	task.EndAt = &deadline

	// ReviewDeadlineAt = deadline + 7 days (审核截止日期，超过此时间未审核的提交自动通过)
	reviewDeadline := deadline.AddDate(0, 0, 7)
	task.ReviewDeadlineAt = &reviewDeadline

	// Materials: use provided or default placeholder
	materials := req.Materials
	if len(materials) == 0 {
		materials = []model.TaskMaterialInput{
			{FileName: "placeholder.jpg", FilePath: "/static/images/task-placeholder.jpg", FileType: "image"},
		}
	} else if materials[0].FileType != "image" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40004,
			Message: "第一个素材必须是图片",
			Data:    nil,
		})
		return
	}

	// Use atomic transaction to create task and freeze budget
	err = businessRepo.CreateTaskWithFreeze(task, materials, userID, totalBudget, user.Balance, user.FrozenAmount, user.PublishCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50002,
			Message: "创建任务失败",
			Data:    nil,
		})
		return
	}

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

	// Enrich claims with creator info and materials
	userRepo := repository.NewUserRepository(GetDB())
	creatorRepo := repository.NewCreatorRepository(GetDB())
	enrichedClaims := make([]gin.H, 0, len(claims))
	for _, claim := range claims {
		creator, _ := userRepo.GetUserByID(claim.CreatorID)
		materials, _ := creatorRepo.GetClaimMaterials(claim.ID)

		creatorName := ""
		creatorAvatar := ""
		if creator != nil {
			if creator.Nickname != "" {
				creatorName = creator.Nickname
			} else {
				creatorName = creator.Username
			}
			creatorAvatar = creator.Avatar
		}

		enrichedClaims = append(enrichedClaims, gin.H{
			"id":                     claim.ID,
			"task_id":                claim.TaskID,
			"creator_id":             claim.CreatorID,
			"status":                 claim.Status,
			"content":                claim.Content,
			"submit_at":              claim.SubmitAt,
			"expires_at":             claim.ExpiresAt,
			"review_at":              claim.ReviewAt,
			"review_result":          claim.ReviewResult,
			"review_comment":         claim.ReviewComment,
			"creator_reward":         claim.CreatorReward,
			"creator_name":           creatorName,
			"creator_avatar":         creatorAvatar,
			"materials":              formatClaimMaterialsForBusiness(materials),
			"process_status_summary": summarizeMaterialProcessing(materials),
			"created_at":             claim.CreatedAt,
			"updated_at":             claim.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    enrichedClaims,
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

	db := GetDB()
	userRepo := repository.NewUserRepository(db)
	creatorRepo := repository.NewCreatorRepository(db)

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

	// Get creator info
	creator, _ := userRepo.GetUserByID(claim.CreatorID)
	creatorName := ""
	creatorAvatar := ""
	if creator != nil {
		if creator.Nickname != "" {
			creatorName = creator.Nickname
		} else {
			creatorName = creator.Username
		}
		creatorAvatar = creator.Avatar
	}

	// Get claim materials
	materials, _ := creatorRepo.GetClaimMaterials(claim.ID)

	// Format response
	result := gin.H{
		"id":             claim.ID,
		"task_id":        claim.TaskID,
		"creator_id":     claim.CreatorID,
		"creator_name":   creatorName,
		"creator_avatar": creatorAvatar,
		"status":         claim.Status,
		"content":        claim.Content,
		"created_at":     claim.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if claim.SubmitAt != nil {
		result["submit_at"] = claim.SubmitAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if claim.ReviewAt != nil {
		result["review_at"] = claim.ReviewAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if claim.ReviewResult != nil {
		result["review_result"] = *claim.ReviewResult
	}
	if claim.ReviewComment != "" {
		result["review_comment"] = claim.ReviewComment
	}

	result["materials"] = formatClaimMaterialsForBusiness(materials)
	result["process_status_summary"] = summarizeMaterialProcessing(materials)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// formatClaimMaterialsForBusiness converts claim materials and prefixes their URLs with CDN
func formatClaimMaterialsForBusiness(materials []*model.ClaimMaterial) []*model.ClaimMaterial {
	return formatVisibleClaimMaterials(materials)
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

		// 参与奖励 (UnitPrice) - 支付给所有合格提交者
		creatorReward := task.UnitPrice * (1.0 - commissionRate)
		platformFee := task.UnitPrice * commissionRate

		// 采纳奖励 (AwardPrice) - 全部支付给被采纳的创作者
		awardReward := task.AwardPrice

		// Total payment to creator (participation reward + adopted reward)
		payment := creatorReward + awardReward

		// Begin transaction for all database writes
		tx, err := businessRepo.BeginTx()
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50001,
				Message: "开启事务失败",
				Data:    nil,
			})
			return
		}
		ensureRollback := true
		defer func() {
			if ensureRollback {
				tx.Rollback()
			}
		}()

		// Approve claim
		if err := businessRepo.ApproveClaimTx(tx, claimID, now, req.Comment, creatorReward+awardReward, platformFee); err != nil {
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
			marginReturned = config.Load().Margin.Amount
			if err := businessRepo.UpdateUserMarginFrozenTx(tx, claim.CreatorID, creator.MarginFrozen-marginReturned); err != nil {
				c.JSON(http.StatusInternalServerError, Response{
					Code:    50007,
					Message: "更新创作者保证金失败",
					Data:    nil,
				})
				return
			}
			payment += marginReturned
		}

		// Update creator balance (participation + adopted + margin return)
		if err := businessRepo.UpdateUserBalanceTx(tx, claim.CreatorID, creator.Balance+payment); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50008,
				Message: "更新创作者余额失败",
				Data:    nil,
			})
			return
		}

		// Update task paid amount (includes both participation and adopted rewards)
		if err := businessRepo.UpdateTaskPaidAmountTx(tx, task.ID, task.PaidAmount+task.UnitPrice+task.AwardPrice); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50009,
				Message: "更新任务支付金额失败",
				Data:    nil,
			})
			return
		}

		// Create transaction record for creator - 参与奖励
		creatorTx := &model.Transaction{
			UserID:        claim.CreatorID,
			Type:          model.TransactionTypePayment,
			Amount:        creatorReward,
			BalanceBefore: creator.Balance,
			BalanceAfter:  creator.Balance + creatorReward,
			Remark:        "参与奖励: " + task.Title,
			RelatedID:     claim.ID,
			CreatedAt:     now,
		}
		if err := businessRepo.CreateTransactionTx(tx, creatorTx); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50010,
				Message: "记录创作者参与奖励流水失败",
				Data:    nil,
			})
			return
		}

		// Create transaction record for creator - 采纳奖励
		awardTx := &model.Transaction{
			UserID:        claim.CreatorID,
			Type:          model.TransactionTypeAwardPayment,
			Amount:        awardReward,
			BalanceBefore: creator.Balance + creatorReward,
			BalanceAfter:  creator.Balance + creatorReward + awardReward,
			Remark:        "采纳奖励: " + task.Title,
			RelatedID:     claim.ID,
			CreatedAt:     now,
		}
		if err := businessRepo.CreateTransactionTx(tx, awardTx); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50011,
				Message: "记录创作者采纳奖励流水失败",
				Data:    nil,
			})
			return
		}

		// Create transaction record for platform income (from participation reward only)
		if platformFee > 0 {
			platformTx := &model.Transaction{
				UserID:        0, // 平台账户
				Type:          model.TransactionTypePlatformIncome,
				Amount:        platformFee,
				BalanceBefore: 0,
				BalanceAfter:  0,
				Remark:        "平台抽成: " + task.Title,
				RelatedID:     claim.ID,
				CreatedAt:     now,
			}
			if err := businessRepo.CreateTransactionTx(tx, platformTx); err != nil {
				c.JSON(http.StatusInternalServerError, Response{
					Code:    50012,
					Message: "记录平台抽成流水失败",
					Data:    nil,
				})
				return
			}
		}

		// Create transaction record for business expense (支付奖励)
		businessExpense := task.UnitPrice + task.AwardPrice
		businessExpenseTx := &model.Transaction{
			UserID:        userID,
			Type:          model.TransactionTypeConsume,
			Amount:        -businessExpense, // 负数表示支出
			BalanceBefore: businessUser.Balance,
			BalanceAfter:  businessUser.Balance, // 余额不变，冻结金额减少
			Remark:        "支付奖励: " + task.Title,
			RelatedID:     claim.ID,
			CreatedAt:     now,
		}
		if err := businessRepo.CreateTransactionTx(tx, businessExpenseTx); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50013,
				Message: "记录商家支出流水失败",
				Data:    nil,
			})
			return
		}

		// Update adopted count for creator
		if err := businessRepo.UpdateCreatorAdoptedCountTx(tx, claim.CreatorID, creator.AdoptedCount+1); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50014,
				Message: "更新创作者采纳数失败",
				Data:    nil,
			})
			return
		}

		// Unfreeze remaining budget for this claim (participation + adopted rewards)
		newTaskFrozen := task.FrozenAmount - task.UnitPrice - task.AwardPrice
		if newTaskFrozen < 0 {
			newTaskFrozen = 0
		}
		if err := businessRepo.UpdateTaskFrozenAmountTx(tx, task.ID, newTaskFrozen); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50016,
				Message: "更新任务冻结金额失败",
				Data:    nil,
			})
			return
		}

		// Also update business user's frozen amount
		newBusinessFrozen := businessUser.FrozenAmount - task.UnitPrice - task.AwardPrice
		if newBusinessFrozen < 0 {
			newBusinessFrozen = 0
		}
		if err := businessRepo.UpdateUserFrozenAmountTx(tx, userID, newBusinessFrozen); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50017,
				Message: "更新商家冻结金额失败",
				Data:    nil,
			})
			return
		}

		// Commit transaction before publishing inspiration (which may have its own transactions)
		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50001,
				Message: "提交事务失败",
				Data:    nil,
			})
			return
		}
		ensureRollback = false

		// Publish inspiration after transaction commits (outside transaction)
		if claimInspirationService != nil {
			savedMaterials, materialsErr := creatorRepo.GetClaimMaterials(claim.ID)
			if materialsErr != nil {
				log.Printf("Failed to load claim materials for claim %d: %v", claimID, materialsErr)
			} else if _, err := claimInspirationService.PublishFromClaim(claim, task, creator, savedMaterials); err != nil {
				log.Printf("Failed to publish inspiration for claim %d: %v", claimID, err)
			}
		}

		// Send notification to creator
		businessNotificationService.NotifyReviewResult(claim.CreatorID, claim.ID, task.Title, true, req.Comment)

	} else if req.Result == 2 {
		// 拒绝 - 创作者获得基础奖励，不获得采纳奖励
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

		// Begin transaction for all database writes
		tx, err := businessRepo.BeginTx()
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50001,
				Message: "开启事务失败",
				Data:    nil,
			})
			return
		}
		ensureRollback := true
		defer func() {
			if ensureRollback {
				tx.Rollback()
			}
		}()

		// 拒绝：标记为退回状态，记录基础奖励
		if err := businessRepo.ReturnClaimTx(tx, claimID, now, req.Comment); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50005,
				Message: "验收失败",
				Data:    nil,
			})
			return
		}

		// 更新认领的基础奖励金额
		if err := businessRepo.UpdateClaimRewardTx(tx, claimID, creatorReward, platformFee); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50007,
				Message: "更新认领奖励失败",
				Data:    nil,
			})
			return
		}

		// 支付基础奖励给创作者
		if err := businessRepo.UpdateUserBalanceTx(tx, claim.CreatorID, creator.Balance+creatorReward); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50008,
				Message: "更新创作者余额失败",
				Data:    nil,
			})
			return
		}

		// 解冻相应金额
		newTaskFrozen := task.FrozenAmount - task.UnitPrice
		if newTaskFrozen < 0 {
			newTaskFrozen = 0
		}
		if err := businessRepo.UpdateTaskFrozenAmountTx(tx, task.ID, newTaskFrozen); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50016,
				Message: "更新任务冻结金额失败",
				Data:    nil,
			})
			return
		}

		newBusinessFrozen := businessUser.FrozenAmount - task.UnitPrice
		if newBusinessFrozen < 0 {
			newBusinessFrozen = 0
		}
		if err := businessRepo.UpdateUserFrozenAmountTx(tx, userID, newBusinessFrozen); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50017,
				Message: "更新商家冻结金额失败",
				Data:    nil,
			})
			return
		}

		// 创建交易记录 - 基础奖励
		creatorTx := &model.Transaction{
			UserID:        claim.CreatorID,
			Type:          model.TransactionTypePayment,
			Amount:        creatorReward,
			BalanceBefore: creator.Balance,
			BalanceAfter:  creator.Balance + creatorReward,
			Remark:        "基础奖励(拒绝): " + task.Title,
			RelatedID:     claim.ID,
			CreatedAt:     now,
		}
		if err := businessRepo.CreateTransactionTx(tx, creatorTx); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50010,
				Message: "记录创作者基础奖励流水失败",
				Data:    nil,
			})
			return
		}

		// 平台抽成
		if platformFee > 0 {
			platformTx := &model.Transaction{
				UserID:        0,
				Type:          model.TransactionTypePlatformIncome,
				Amount:        platformFee,
				BalanceBefore: 0,
				BalanceAfter:  0,
				Remark:        "平台抽成(拒绝): " + task.Title,
				RelatedID:     claim.ID,
				CreatedAt:     now,
			}
			if err := businessRepo.CreateTransactionTx(tx, platformTx); err != nil {
				c.JSON(http.StatusInternalServerError, Response{
					Code:    50012,
					Message: "记录平台抽成流水失败",
					Data:    nil,
				})
				return
			}
		}

		// Create transaction record for business expense (支付基础奖励)
		businessExpenseTx := &model.Transaction{
			UserID:        userID,
			Type:          model.TransactionTypeConsume,
			Amount:        -task.UnitPrice, // 负数表示支出
			BalanceBefore: businessUser.Balance,
			BalanceAfter:  businessUser.Balance, // 余额不变，冻结金额减少
			Remark:        "支付基础奖励(拒绝): " + task.Title,
			RelatedID:     claim.ID,
			CreatedAt:     now,
		}
		if err := businessRepo.CreateTransactionTx(tx, businessExpenseTx); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50013,
				Message: "记录商家基础支出流水失败",
				Data:    nil,
			})
			return
		}

		// 更新任务已支付金额（只包含基础奖励）
		if err := businessRepo.UpdateTaskPaidAmountTx(tx, task.ID, task.PaidAmount+task.UnitPrice); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50009,
				Message: "更新任务支付金额失败",
				Data:    nil,
			})
			return
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50001,
				Message: "提交事务失败",
				Data:    nil,
			})
			return
		}
		ensureRollback = false

		// 发送通知给创作者
		businessNotificationService.NotifyReviewResult(claim.CreatorID, claim.ID, task.Title, false, req.Comment)

	} else if req.Result == 3 {
		// 举报 - 创作者不获得任何奖励，基础奖励归平台，同时增加举报次数
		creator, err := businessRepo.GetUserByID(claim.CreatorID)
		if err != nil || creator == nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50006,
				Message: "获取创作者信息失败",
				Data:    nil,
			})
			return
		}

		// Begin transaction for all database writes
		tx, err := businessRepo.BeginTx()
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50001,
				Message: "开启事务失败",
				Data:    nil,
			})
			return
		}
		ensureRollback := true
		defer func() {
			if ensureRollback {
				tx.Rollback()
			}
		}()

		// 增加举报次数
		if err := businessRepo.UpdateUserReportCountTx(tx, claim.CreatorID, creator.ReportCount+1); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50018,
				Message: "更新创作者举报次数失败",
				Data:    nil,
			})
			return
		}

		// 标记为退回状态（使用ReportClaim设置举报标记）
		if err := businessRepo.ReportClaimTx(tx, claimID, now, req.Comment); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50005,
				Message: "验收失败",
				Data:    nil,
			})
			return
		}

		// 基础奖励归平台（不支付给创作者）
		// 平台获得 UnitPrice 作为举报罚款
		platformTx := &model.Transaction{
			UserID:        0,
			Type:          model.TransactionTypePlatformIncome,
			Amount:        task.UnitPrice,
			BalanceBefore: 0,
			BalanceAfter:  0,
			Remark:        "举报罚款: " + task.Title,
			RelatedID:     claim.ID,
			CreatedAt:     now,
		}
		if err := businessRepo.CreateTransactionTx(tx, platformTx); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50012,
				Message: "记录平台罚款流水失败",
				Data:    nil,
			})
			return
		}

		// 解冻全部金额（UnitPrice + AwardPrice）
		// UnitPrice 作为举报罚款归平台（商家支出），AwardPrice 返还给商家可用余额
		frozenToUnfreeze := task.UnitPrice + task.AwardPrice
		newTaskFrozen := task.FrozenAmount - frozenToUnfreeze
		if newTaskFrozen < 0 {
			newTaskFrozen = 0
		}
		if err := businessRepo.UpdateTaskFrozenAmountTx(tx, task.ID, newTaskFrozen); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50016,
				Message: "更新任务冻结金额失败",
				Data:    nil,
			})
			return
		}

		// AwardPrice 返还给商家可用余额
		newBusinessBalance := businessUser.Balance + task.AwardPrice
		newBusinessFrozen := businessUser.FrozenAmount - frozenToUnfreeze
		if newBusinessFrozen < 0 {
			newBusinessFrozen = 0
		}
		if err := businessRepo.UpdateUserBalanceTx(tx, userID, newBusinessBalance); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50019,
				Message: "更新商家余额失败",
				Data:    nil,
			})
			return
		}
		if err := businessRepo.UpdateUserFrozenAmountTx(tx, userID, newBusinessFrozen); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50017,
				Message: "更新商家冻结金额失败",
				Data:    nil,
			})
			return
		}

		// 创建商家支出交易记录（举报罚款 - UnitPrice）
		penaltyTx := &model.Transaction{
			UserID:        userID,
			Type:          model.TransactionTypeConsume,
			Amount:        -task.UnitPrice, // 负数表示支出
			BalanceBefore: businessUser.Balance,
			BalanceAfter:  businessUser.Balance, // 余额不变，冻结金额减少
			Remark:        "举报罚款: " + task.Title,
			RelatedID:     claim.ID,
			CreatedAt:     now,
		}
		if err := businessRepo.CreateTransactionTx(tx, penaltyTx); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50013,
				Message: "记录商家罚款流水失败",
				Data:    nil,
			})
			return
		}

		// 创建商家解冻交易记录（返还AwardPrice）
		unfreezeTx := &model.Transaction{
			UserID:        userID,
			Type:          model.TransactionTypeUnfreeze,
			Amount:        task.AwardPrice,
			BalanceBefore: businessUser.Balance,
			BalanceAfter:  newBusinessBalance,
			Remark:        "解冻返还(举报): " + task.Title,
			RelatedID:     claim.ID,
			CreatedAt:     now,
		}
		if err := businessRepo.CreateTransactionTx(tx, unfreezeTx); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50020,
				Message: "记录商家解冻流水失败",
				Data:    nil,
			})
			return
		}

		// 更新任务已支付金额（举报情况下平台只获得 UnitPrice）
		if err := businessRepo.UpdateTaskPaidAmountTx(tx, task.ID, task.PaidAmount+task.UnitPrice); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50009,
				Message: "更新任务支付金额失败",
				Data:    nil,
			})
			return
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50001,
				Message: "提交事务失败",
				Data:    nil,
			})
			return
		}
		ensureRollback = false

		// 发送通知给创作者（举报结果）
		businessNotificationService.NotifyReviewResult(claim.CreatorID, claim.ID, task.Title, false, "举报: "+req.Comment)

	} else {
		// 退回 - 发回给创作者重新提交（无奖励）
		err = businessRepo.ReturnClaim(claimID, now, req.Comment)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50005,
				Message: "验收失败",
				Data:    nil,
			})
			return
		}

		// 发送通知给创作者
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
	// Use TotalBudget - PaidAmount instead of FrozenAmount - PaidAmount
	// because FrozenAmount maintenance across multiple code paths can be inconsistent
	frozenAmount := task.TotalBudget - task.PaidAmount
	if frozenAmount < 0 {
		frozenAmount = 0
	}

	// Cancel all pending claims and return margins
	claims, err := businessRepo.ListClaimsByTaskID(taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50006,
			Message: "获取任务认领列表失败",
			Data:    nil,
		})
		return
	}
	for _, claim := range claims {
		if claim.Status == model.ClaimStatusPending {
			// Get creator for margin
			creator, err := businessRepo.GetUserByID(claim.CreatorID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, Response{
					Code:    50007,
					Message: "获取创作者信息失败",
					Data:    nil,
				})
				return
			}
			if creator != nil && creator.NeedMargin() {
				if err := businessRepo.UpdateUserMarginFrozen(claim.CreatorID, creator.MarginFrozen-10); err != nil {
					c.JSON(http.StatusInternalServerError, Response{
						Code:    50008,
						Message: "退还创作者保证金失败",
						Data:    nil,
					})
					return
				}
				if err := businessRepo.UpdateUserBalance(claim.CreatorID, creator.Balance+10); err != nil {
					c.JSON(http.StatusInternalServerError, Response{
						Code:    50009,
						Message: "更新创作者余额失败",
						Data:    nil,
					})
					return
				}
			}
			// Delete claim and related materials (取消后直接删除，不再显示)
			if err := businessRepo.DeleteClaimMaterials(claim.ID); err != nil {
				c.JSON(http.StatusInternalServerError, Response{
					Code:    50010,
					Message: "删除认领素材失败",
					Data:    nil,
				})
				return
			}
			if err := businessRepo.DeleteClaim(claim.ID); err != nil {
				c.JSON(http.StatusInternalServerError, Response{
					Code:    50011,
					Message: "删除认领记录失败",
					Data:    nil,
				})
				return
			}
		}
	}

	// Refund remaining frozen amount to business
	if frozenAmount > 0 {
		newBalance := business.Balance + frozenAmount
		newFrozen := business.FrozenAmount - frozenAmount
		if err := businessRepo.UpdateUserBalance(userID, newBalance); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50012,
				Message: "更新商家余额失败",
				Data:    nil,
			})
			return
		}
		if err := businessRepo.UpdateUserFrozenAmount(userID, newFrozen); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50013,
				Message: "更新商家冻结金额失败",
				Data:    nil,
			})
			return
		}

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
		if err := businessRepo.CreateTransaction(tx); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    50014,
				Message: "记录解冻流水失败",
				Data:    nil,
			})
			return
		}
	}

	// Update task status to cancelled
	if err := businessRepo.UpdateTaskStatus(task.ID, model.TaskStatusCancelled); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50015,
			Message: "更新任务状态失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "任务已取消",
		Data: gin.H{
			"refunded": frozenAmount,
		},
	})
}
