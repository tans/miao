package handler

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
	"github.com/tans/miao/internal/service"
)

var creatorRepo *repository.CreatorRepository
var creatorNotificationService *service.NotificationService
var claimInspirationService *service.ClaimInspirationService

func init() {
	db := GetDB()
	creatorRepo = repository.NewCreatorRepository(db)
	creatorNotificationService = service.NewNotificationService(db)
	claimInspirationService = service.NewClaimInspirationService(db)
}

// ListAvailableTasks 获取可认领的视频任务列表（支持分页、搜索、排序）
// GET /api/v1/creator/tasks?page=1&limit=20&keyword=关键词&sort=price_desc&status=1
func ListAvailableTasks(c *gin.Context) {
	db := GetDB()
	taskRepo := repository.NewTaskRepository(db)

	// 解析查询参数
	page := parseInt(c.DefaultQuery("page", "1"), 1)
	limit := parseInt(c.DefaultQuery("limit", "20"), 20)
	keyword := c.Query("keyword")
	sort := c.DefaultQuery("sort", "created_at")
	statusStr := c.Query("status")
	var status model.TaskStatus
	if statusStr != "" {
		status = model.TaskStatus(parseInt(statusStr, 0))
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	// 单一视频平台模式下，不再按外部分类参数筛选。status参数支持按任务状态筛选
	tasks, total, err := taskRepo.ListTasksWithPagination(0, keyword, sort, limit, offset, status)
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
		Data: gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
			"data":  formatTaskList(tasks),
		},
	})
}

// ClaimTask 认领任务
// POST /api/v1/creator/claim
func ClaimTask(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	var req model.ClaimCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// Get user
	user, err := creatorRepo.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}

	// All users are creators now - no role check needed
	// Check if creator can claim (白银及以上)
	if !user.CanClaim() {
		c.JSON(http.StatusForbidden, Response{
			Code:    40302,
			Message: "只有白银及以上等级才能认领任务",
			Data:    nil,
		})
		return
	}

	// Check if creator has been reported too many times (>=5 means can't submit works)
	if !user.CanSubmitWork() {
		c.JSON(http.StatusForbidden, Response{
			Code:    40304,
			Message: "因被举报次数过多，无法认领新任务",
			Data:    nil,
		})
		return
	}

	// Check pending claims limit (max 3)
	pendingCount, err := creatorRepo.CountPendingClaimsByCreatorID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "检查认领状态失败",
			Data:    nil,
		})
		return
	}
	if pendingCount >= 3 {
		c.JSON(http.StatusForbidden, Response{
			Code:    40303,
			Message: "待提交任务不能超过3个",
			Data:    nil,
		})
		return
	}

	// Get task
	db := GetDB()
	taskRepo := repository.NewTaskRepository(db)
	task, err := taskRepo.GetTaskByID(req.TaskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "任务不存在",
			Data:    nil,
		})
		return
	}

	// Check if task is available
	if !task.IsAvailable() {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "任务不可认领",
			Data:    nil,
		})
		return
	}

	// Check if user already claimed this task
	existingClaim, err := creatorRepo.GetClaimByTaskIDAndCreatorID(req.TaskID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "检查认领状态失败",
			Data:    nil,
		})
		return
	}
	if existingClaim != nil && existingClaim.Status != model.ClaimStatusCancelled {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40004,
			Message: "您已领取过该任务",
			Data:    nil,
		})
		return
	}

	// Check if margin is needed (青铜用户)
	marginAmount := 0.0
	if user.NeedMargin() {
		marginAmount = 10.0 // 10元保证金
		if user.Balance < marginAmount {
			c.JSON(http.StatusBadRequest, Response{
				Code:    40003,
				Message: "余额不足，需要冻结10元保证金",
				Data:    nil,
			})
			return
		}
	}

	// Create claim
	claim := &model.Claim{
		TaskID:    req.TaskID,
		CreatorID: userID,
		Status:    model.ClaimStatusPending,
		ExpiresAt: time.Now().Add(24 * time.Hour), // 24小时生产
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = creatorRepo.CreateClaim(claim)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50002,
			Message: "认领失败",
			Data:    nil,
		})
		return
	}

	// Atomically decrement task remaining count
	success, err := taskRepo.DecrementRemainingCount(task.ID)
	if err != nil || !success {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40003,
			Message: "任务已被认领完",
			Data:    nil,
		})
		return
	}

	// Freeze margin if needed (青铜用户)
	if marginAmount > 0 {
		creatorRepo.UpdateUserMarginFrozen(userID, user.MarginFrozen+marginAmount)
		// Freeze from balance
		creatorRepo.UpdateUserBalance(userID, user.Balance-marginAmount)
	}

	// Send notification to business owner
	creatorNotificationService.NotifyTaskClaimed(task.BusinessID, task.ID, task.Title, user.Username)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "认领成功",
		Data: gin.H{
			"claim_id":   claim.ID,
			"expires_at": claim.ExpiresAt.Format(time.RFC3339),
		},
	})
}

// ListMyClaims 获取我的认领列表
// GET /api/v1/creator/claims
func ListMyClaims(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	claims, err := creatorRepo.ListClaimsByCreatorID(userID)
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

// SubmitClaim 提交交付
// PUT /api/v1/creator/claim/:id/submit
func SubmitClaim(c *gin.Context) {
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

	var req model.ClaimSubmit
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// Get claim
	claim, err := creatorRepo.GetClaimByID(claimID)
	if err != nil || claim == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "认领不存在",
			Data:    nil,
		})
		return
	}

	// Check ownership
	if claim.CreatorID != userID {
		c.JSON(http.StatusForbidden, Response{
			Code:    40301,
			Message: "无权操作此认领",
			Data:    nil,
		})
		return
	}

	// Get creator for margin check
	creator, err := creatorRepo.GetUserByID(userID)
	if err != nil || creator == nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50003,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}

	taskRepo := repository.NewTaskRepository(GetDB())
	task, err := taskRepo.GetTaskByID(claim.TaskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "任务不存在",
			Data:    nil,
		})
		return
	}

	// Check status
	if claim.Status != model.ClaimStatusPending {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "当前状态不允许提交",
			Data:    nil,
		})
		return
	}

	// Check if expired
	if time.Now().After(claim.ExpiresAt) {
		// Mark as expired
		creatorRepo.UpdateClaimStatus(claimID, model.ClaimStatusExpired)

		// Return margin to creator if applicable (青铜用户)
		if creator.NeedMargin() && creator.MarginFrozen >= 10 {
			creatorRepo.UpdateUserMarginFrozen(userID, creator.MarginFrozen-10)
			creatorRepo.UpdateUserBalance(userID, creator.Balance+10)
		}

		// Return task remaining count
		task.RemainingCount++
		if task.Status == model.TaskStatusOngoing {
			task.Status = model.TaskStatusOnline
		}
		taskRepo.UpdateTask(task)
		if claimInspirationService != nil {
			if err := claimInspirationService.DeleteByClaimID(claim.ID); err != nil {
				log.Printf("Failed to delete inspiration for expired claim %d: %v", claimID, err)
			}
		}

		c.JSON(http.StatusBadRequest, Response{
			Code:    40003,
			Message: "认领已超时",
			Data:    nil,
		})
		return
	}

	// Submit
	now := time.Now()
	err = creatorRepo.SubmitClaim(claimID, req.Content, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50002,
			Message: "提交失败",
			Data:    nil,
		})
		return
	}

	// 保存媒体文件
	for _, mat := range req.Materials {
		material := &model.ClaimMaterial{
			ClaimID:       claimID,
			FileName:      mat.FileName,
			FilePath:      mat.FilePath,
			FileSize:      mat.FileSize,
			FileType:      mat.FileType,
			ThumbnailPath: mat.ThumbnailPath,
		}
		if err := creatorRepo.CreateClaimMaterial(material); err != nil {
			log.Printf("Failed to save claim material for claim %d: %v", claimID, err)
		}
	}

	claim.Content = req.Content
	savedMaterials, err := creatorRepo.GetClaimMaterials(claimID)
	if err != nil {
		log.Printf("Failed to load claim materials for claim %d: %v", claimID, err)
	} else if claimInspirationService != nil {
		if _, err := claimInspirationService.SaveDraftFromClaim(claim, task, creator, savedMaterials); err != nil {
			log.Printf("Failed to sync draft inspiration for claim %d: %v", claimID, err)
		}
	}

	// Get task info for notification
	if task != nil {
		// Send notification to business owner
		creatorNotificationService.NotifySubmissionSubmitted(task.BusinessID, task.ID, task.Title, creator.Username)
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "提交成功",
		Data:    nil,
	})
}

// GetWallet 获取我的钱包
// GET /api/v1/creator/wallet
func GetWallet(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	user, err := creatorRepo.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}

	wallet := model.UserWallet{
		Balance:      user.Balance,
		FrozenAmount: user.FrozenAmount,
		MarginFrozen: user.MarginFrozen,
		Level:        int(user.Level),
		LevelName:    user.GetLevelName(),
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    wallet,
	})
}

// CancelClaim 取消/放弃认领
// DELETE /api/v1/creator/claim/:id
func CancelClaim(c *gin.Context) {
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

	// Get claim
	claim, err := creatorRepo.GetClaimByID(claimID)
	if err != nil || claim == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "认领不存在",
			Data:    nil,
		})
		return
	}

	// Check ownership
	if claim.CreatorID != userID {
		c.JSON(http.StatusForbidden, Response{
			Code:    40301,
			Message: "无权取消该认领",
			Data:    nil,
		})
		return
	}

	// Can only cancel pending claims
	if claim.Status != model.ClaimStatusPending {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "只能取消待提交状态的认领",
			Data:    nil,
		})
		return
	}

	// Delete claim and related materials (取消后不再显示)
	err = creatorRepo.DeleteClaimMaterials(claimID)
	if err != nil {
		log.Printf("Failed to delete claim materials: %v", err)
	}
	err = creatorRepo.DeleteClaim(claimID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "取消认领失败",
			Data:    nil,
		})
		return
	}

	// Return remaining count to task
	db := GetDB()
	taskRepo := repository.NewTaskRepository(db)
	task, _ := taskRepo.GetTaskByID(claim.TaskID)
	if task != nil {
		taskRepo.IncrementRemainingCount(task.ID)
	}

	// Return margin if applicable (青铜用户)
	user, _ := creatorRepo.GetUserByID(userID)
	if user != nil && user.NeedMargin() && user.MarginFrozen >= 10.0 {
		creatorRepo.UpdateUserMarginFrozen(userID, user.MarginFrozen-10.0)
		creatorRepo.UpdateUserBalance(userID, user.Balance+10.0)
	}

	if claimInspirationService != nil {
		if err := claimInspirationService.DeleteByClaimID(claim.ID); err != nil {
			log.Printf("Failed to delete inspiration for cancelled claim %d: %v", claimID, err)
		}
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "已取消认领",
		Data:    nil,
	})
}

// GetClaimByID 获取认领详情
// GET /api/v1/creator/claim/:id
func GetClaimByID(c *gin.Context) {
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

	claim, err := creatorRepo.GetClaimByID(claimID)
	if err != nil || claim == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "认领不存在",
			Data:    nil,
		})
		return
	}

	// Check ownership
	if claim.CreatorID != userID {
		c.JSON(http.StatusForbidden, Response{
			Code:    40301,
			Message: "无权查看该认领",
			Data:    nil,
		})
		return
	}

	// Get creator info for avatar and name
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
	if claim.CreatorReward > 0 {
		result["creator_reward"] = claim.CreatorReward
	}

	// Add materials with CDN prefix
	if len(materials) > 0 {
		result["materials"] = formatCreatorClaimMaterials(materials)
	} else {
		result["materials"] = []*model.ClaimMaterial{}
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// formatCreatorClaimMaterials converts creator claim materials and prefixes their URLs with CDN
func formatCreatorClaimMaterials(materials []*model.ClaimMaterial) []*model.ClaimMaterial {
	cfg := config.Load()
	cdn := cfg.Static.CDN
	if cdn == "" {
		cdn = cfg.Static.Host
	}
	result := make([]*model.ClaimMaterial, len(materials))
	for i, m := range materials {
		result[i] = m
		if result[i].FilePath != "" && !strings.HasPrefix(result[i].FilePath, "http") {
			result[i].FilePath = cdn + result[i].FilePath
		}
		if result[i].ThumbnailPath != "" && !strings.HasPrefix(result[i].ThumbnailPath, "http") {
			result[i].ThumbnailPath = cdn + result[i].ThumbnailPath
		}
	}
	return result
}

// GetClaimByTaskID 获取当前用户对指定任务的认领
// GET /api/v1/creator/claim/by-task/:taskId
func GetClaimByTaskID(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	taskID := parseInt64(c.Param("taskId"), 0)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的任务ID",
			Data:    nil,
		})
		return
	}

	db := GetDB()
	userRepo := repository.NewUserRepository(db)

	claim, err := creatorRepo.GetClaimByTaskIDAndCreatorID(taskID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取认领信息失败",
			Data:    nil,
		})
		return
	}

	// Return null if no claim exists
	if claim == nil || claim.Status == model.ClaimStatusCancelled {
		c.JSON(http.StatusOK, Response{
			Code:    0,
			Message: "success",
			Data:    nil,
		})
		return
	}

	// Get creator info for avatar and name
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
	if claim.CreatorReward > 0 {
		result["creator_reward"] = claim.CreatorReward
	}

	// Add materials with CDN prefix
	if len(materials) > 0 {
		result["materials"] = formatCreatorClaimMaterials(materials)
	} else {
		result["materials"] = []*model.ClaimMaterial{}
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// parseInt 辅助函数：解析整数
func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			val = val*10 + int(c-'0')
		} else {
			return defaultVal
		}
	}
	return val
}
