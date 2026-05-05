package handler

import (
	"fmt"
	"log"
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
	"github.com/tans/miao/internal/storage"
)

// AppealResponse represents the standard API response
type AppealResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

var appealRepo *repository.AppealRepository

func initAppealRepo() error {
	if err := initDB(); err != nil {
		return err
	}
	appealRepo = repository.NewAppealRepository(db)
	return nil
}

func init() {
	if err := initAppealRepo(); err != nil {
		log.Fatalf("failed to initialize appeal repository: %v", err)
	}
}

func getAppealAdminRepo() *repository.AdminRepository {
	if adminRepo != nil {
		return adminRepo
	}
	if db == nil {
		if err := initDB(); err != nil {
			return nil
		}
	}
	if db != nil && adminRepo == nil {
		adminRepo = repository.NewAdminRepository(db)
	}
	return adminRepo
}

func resolveAppealTaskID(appeal *model.Appeal) int64 {
	if appeal == nil {
		return 0
	}

	claimID := appeal.ClaimID
	if claimID <= 0 {
		return 0
	}

	repo := getAppealAdminRepo()
	if repo == nil {
		return 0
	}

	claim, err := repo.GetClaimByID(claimID)
	if err == nil && claim != nil {
		return claim.TaskID
	}

	return 0
}

func resolveAppealReviewDeadline() time.Time {
	now := time.Now()
	reviewDays := 7
	if repo := getAppealAdminRepo(); repo != nil {
		if settings, err := repo.GetSettings(); err == nil && settings != nil && settings.ReviewDays > 0 {
			reviewDays = settings.ReviewDays
		}
	}
	return now.AddDate(0, 0, reviewDays)
}

func resolveAppealWithinTx(tx database.Tx, appealID int64, result string, adminID int64) error {
	if tx == nil {
		return fmt.Errorf("missing transaction")
	}
	if adminID > 0 {
		_, err := tx.Exec(`
			UPDATE appeals
			SET status = ?, result = ?, admin_id = ?, handle_at = ?
			WHERE id = ?
		`, model.AppealStatusResolved, result, adminID, time.Now(), appealID)
		return err
	}
	_, err := tx.Exec(`
		UPDATE appeals
		SET status = ?, result = ?, handle_at = ?
		WHERE id = ?
	`, model.AppealStatusResolved, result, time.Now(), appealID)
	return err
}

func handleAppealResolutionTx(repo *repository.AdminRepository, appeal *model.Appeal, req model.ResolveAppealRequest, adminID int64, claim *model.Claim, task *model.Task, creator *model.User, businessUser *model.User) error {
	if repo == nil {
		return fmt.Errorf("missing repository")
	}
	if appeal == nil {
		return fmt.Errorf("missing appeal")
	}

	tx, err := repo.BeginTx()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if req.Accepted {
		if claim == nil || task == nil || creator == nil || businessUser == nil {
			return fmt.Errorf("missing related records")
		}

		if err := restoreAcceptedAppealTx(tx, claim, task, creator, businessUser, time.Now()); err != nil {
			return err
		}
	}

	if err := resolveAppealWithinTx(tx, appeal.ID, req.Result, adminID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func restoreAcceptedAppealTx(tx database.Tx, claim *model.Claim, task *model.Task, creator *model.User, businessUser *model.User, now time.Time) error {
	if tx == nil {
		return fmt.Errorf("missing transaction")
	}
	if claim == nil || task == nil || creator == nil || businessUser == nil {
		return fmt.Errorf("missing related records")
	}
	if claim.ReviewResult == nil {
		return fmt.Errorf("claim has no review result")
	}

	reviewResult := *claim.ReviewResult

	updateClaim := func() error {
		_, err := tx.Exec(`
			UPDATE claims
			SET status = ?,
				review_at = NULL,
				review_result = NULL,
				review_comment = '',
				creator_reward = 0,
				platform_fee = 0,
				margin_returned = 0,
				updated_at = ?
			WHERE id = ?
		`, model.ClaimStatusSubmitted, now, claim.ID)
		return err
	}

	reviewDeadline := resolveAppealReviewDeadline()

	switch reviewResult {
	case int(model.ReviewResultReturn):
		creatorReward := claim.CreatorReward
		platformFee := claim.PlatformFee
		if creatorReward <= 0 && task.UnitPrice > 0 {
			commissionRate := creator.GetCommission()
			creatorReward = task.UnitPrice * (1 - commissionRate)
			platformFee = task.UnitPrice * commissionRate
		}

		if creatorReward > 0 {
			nextBalance := creator.Balance - creatorReward
			if nextBalance < 0 {
				nextBalance = 0
			}
			if _, err := tx.Exec(`
				UPDATE users
				SET balance = ?, updated_at = ?
				WHERE id = ?
			`, nextBalance, now, creator.ID); err != nil {
				return fmt.Errorf("restore creator balance: %w", err)
			}
			if _, err := tx.Exec(`
				INSERT INTO transactions (user_id, type, amount, balance_before, balance_after, remark, related_id, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`, creator.ID, model.TransactionTypePayment, -creatorReward, creator.Balance, nextBalance, "申诉回滚：撤销基础奖励", claim.ID, now); err != nil {
				return fmt.Errorf("insert creator reversal transaction: %w", err)
			}
		}

		if platformFee > 0 {
			if _, err := tx.Exec(`
				INSERT INTO transactions (user_id, type, amount, balance_before, balance_after, remark, related_id, created_at)
				VALUES (0, ?, ?, 0, 0, ?, ?, ?)
			`, model.TransactionTypePlatformIncome, -platformFee, "申诉回滚：冲正平台抽成", claim.ID, now); err != nil {
				return fmt.Errorf("insert platform reversal transaction: %w", err)
			}
		}

		if _, err := tx.Exec(`
			UPDATE tasks
			SET paid_amount = CASE WHEN paid_amount >= ? THEN paid_amount - ? ELSE 0 END,
				frozen_amount = frozen_amount + ?,
				updated_at = ?
			WHERE id = ?
		`, task.UnitPrice, task.UnitPrice, task.UnitPrice, now, task.ID); err != nil {
			return fmt.Errorf("restore task paid amount: %w", err)
		}
		if _, err := tx.Exec(`
			UPDATE users
			SET frozen_amount = ?, updated_at = ?
			WHERE id = ?
		`, businessUser.FrozenAmount+task.UnitPrice, now, businessUser.ID); err != nil {
			return fmt.Errorf("restore business frozen amount: %w", err)
		}
		if err := businessRepo.RestoreTaskForAppeal(tx, task.ID, reviewDeadline); err != nil {
			return fmt.Errorf("restore task for appeal: %w", err)
		}

		if err := updateClaim(); err != nil {
			return fmt.Errorf("restore claim state: %w", err)
		}

		return nil
	case int(model.ReviewResultReport):
		refundAmount := model.ClaimRewardBudget(task.UnitPrice, task.AwardPrice)
		newReportCount := creator.ReportCount - 1
		if newReportCount < 0 {
			newReportCount = 0
		}

		nextBalance := businessUser.Balance - refundAmount
		if nextBalance < 0 {
			nextBalance = 0
		}
		if _, err := tx.Exec(`
			UPDATE users
			SET balance = ?, frozen_amount = ?, updated_at = ?
			WHERE id = ?
		`, nextBalance, businessUser.FrozenAmount+refundAmount, now, businessUser.ID); err != nil {
			return fmt.Errorf("restore business refund: %w", err)
		}
		if _, err := tx.Exec(`
			UPDATE users
			SET report_count = ?,
				updated_at = ?
			WHERE id = ?
		`, newReportCount, now, creator.ID); err != nil {
			return fmt.Errorf("restore creator report count: %w", err)
		}
		if _, err := tx.Exec(`
			INSERT INTO transactions (user_id, type, amount, balance_before, balance_after, remark, related_id, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, businessUser.ID, model.TransactionTypeConsume, -refundAmount, businessUser.Balance, nextBalance, "申诉回滚：撤销举报退款", claim.ID, now); err != nil {
			return fmt.Errorf("insert business reversal transaction: %w", err)
		}

		if _, err := tx.Exec(`
			UPDATE tasks
			SET frozen_amount = frozen_amount + ?,
				updated_at = ?
			WHERE id = ?
		`, refundAmount, now, task.ID); err != nil {
			return fmt.Errorf("restore task frozen amount: %w", err)
		}
		if err := businessRepo.RestoreTaskForAppeal(tx, task.ID, reviewDeadline); err != nil {
			return fmt.Errorf("restore task for appeal: %w", err)
		}

		if err := updateClaim(); err != nil {
			return fmt.Errorf("restore claim state: %w", err)
		}

		return nil
	default:
		return fmt.Errorf("unsupported review result: %d", reviewResult)
	}
}

// CreateAppeal handles creating a new appeal
// POST /api/v1/appeals
func CreateAppeal(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, AppealResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	var req model.CreateAppealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AppealResponse{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	claimID := req.ClaimID
	if claimID <= 0 {
		c.JSON(http.StatusBadRequest, AppealResponse{
			Code:    40001,
			Message: "参数错误: 缺少 claim_id",
			Data:    nil,
		})
		return
	}

	repo := getAppealAdminRepo()
	if repo == nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "获取申诉对象失败",
			Data:    nil,
		})
		return
	}

	claim, err := repo.GetClaimByID(claimID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "获取申诉对象失败",
			Data:    nil,
		})
		return
	}
	if claim == nil {
		c.JSON(http.StatusNotFound, AppealResponse{
			Code:    40401,
			Message: "申诉对象不存在",
			Data:    nil,
		})
		return
	}
	if claim.CreatorID != userID {
		c.JSON(http.StatusForbidden, AppealResponse{
			Code:    40301,
			Message: "无权申诉该作品",
			Data:    nil,
		})
		return
	}
	reviewResult := 0
	if claim.ReviewResult != nil {
		reviewResult = *claim.ReviewResult
	}
	if claim.Status != model.ClaimStatusPending || (reviewResult != int(model.ReviewResultReturn) && reviewResult != int(model.ReviewResultReport)) {
		c.JSON(http.StatusBadRequest, AppealResponse{
			Code:    40002,
			Message: "当前状态不允许申诉",
			Data:    nil,
		})
		return
	}

	appeal := &model.Appeal{
		UserID:    userID,
		Type:      model.AppealType(req.Type),
		ClaimID:   claimID,
		TargetID:  claimID,
		Reason:    req.Reason,
		Evidence:  req.Evidence,
		Status:    model.AppealStatusPending,
		CreatedAt: time.Now(),
	}

	if err := appealRepo.CreateAppeal(appeal); err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "创建申诉失败",
			Data:    nil,
		})
		return
	}

	if notificationSvc != nil {
		notificationSvc.NotifyAppealCreated(userID, appeal.ID)
	}

	c.JSON(http.StatusOK, AppealResponse{
		Code:    0,
		Message: "申诉已提交",
		Data: gin.H{
			"id":         appeal.ID,
			"type":       appeal.Type,
			"claim_id":   appeal.ClaimID,
			"target_id":  appeal.TargetID,
			"task_id":    claim.TaskID,
			"reason":     appeal.Reason,
			"evidence":   resolveAppealEvidenceURLs(c, appeal.Evidence),
			"status":     appeal.Status,
			"created_at": appeal.CreatedAt.Format(time.RFC3339),
		},
	})
}

// ListAppeals handles listing user's appeals
// GET /api/v1/appeals
func ListAppeals(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, AppealResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	// Parse pagination
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

	appeals, total, err := appealRepo.ListAppealsByUserID(userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "获取申诉列表失败",
			Data:    nil,
		})
		return
	}

	var formattedAppeals []gin.H
	for _, appeal := range appeals {
		taskID := resolveAppealTaskID(appeal)
		typeStr := "作品申诉"
		statusStr := "待处理"
		if appeal.Status == model.AppealStatusResolved {
			statusStr = "已处理"
		}
		formattedAppeals = append(formattedAppeals, gin.H{
			"id":         appeal.ID,
			"type":       appeal.Type,
			"type_str":   typeStr,
			"claim_id":   appeal.ClaimID,
			"target_id":  appeal.TargetID,
			"task_id":    taskID,
			"reason":     appeal.Reason,
			"status":     appeal.Status,
			"status_str": statusStr,
			"result":     appeal.Result,
			"created_at": appeal.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, AppealResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"appeals": formattedAppeals,
			"total":   total,
		},
	})
}

// GetAppeal handles getting a single appeal
// GET /api/v1/appeals/:id
func GetAppeal(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, AppealResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, AppealResponse{
			Code:    40001,
			Message: "无效的申诉ID",
			Data:    nil,
		})
		return
	}

	appeal, err := appealRepo.GetAppealByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "获取申诉详情失败",
			Data:    nil,
		})
		return
	}

	if appeal == nil {
		c.JSON(http.StatusNotFound, AppealResponse{
			Code:    40401,
			Message: "申诉不存在",
			Data:    nil,
		})
		return
	}

	// Check ownership
	if appeal.UserID != userID {
		// Check if user is admin
		isAdmin, _ := middleware.GetIsAdminFromContext(c)
		if !isAdmin {
			c.JSON(http.StatusForbidden, AppealResponse{
				Code:    40301,
				Message: "无权查看此申诉",
				Data:    nil,
			})
			return
		}
	}

	typeStr := "作品申诉"
	statusStr := "待处理"
	if appeal.Status == model.AppealStatusResolved {
		statusStr = "已处理"
	}
	taskID := resolveAppealTaskID(appeal)

	c.JSON(http.StatusOK, AppealResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"id":         appeal.ID,
			"user_id":    appeal.UserID,
			"type":       appeal.Type,
			"type_str":   typeStr,
			"claim_id":   appeal.ClaimID,
			"target_id":  appeal.TargetID,
			"task_id":    taskID,
			"reason":     appeal.Reason,
			"evidence":   resolveAppealEvidenceURLs(c, appeal.Evidence),
			"status":     appeal.Status,
			"status_str": statusStr,
			"result":     appeal.Result,
			"created_at": appeal.CreatedAt.Format(time.RFC3339),
		},
	})
}

// ResolveAppeal handles resolving an appeal (admin only)
// PUT /api/v1/appeals/:id
func ResolveAppeal(c *gin.Context) {
	isAdmin, ok := middleware.GetIsAdminFromContext(c)
	if !ok || !isAdmin {
		c.JSON(http.StatusForbidden, AppealResponse{
			Code:    40301,
			Message: "需要管理员权限",
			Data:    nil,
		})
		return
	}
	adminID, _ := middleware.GetUserIDFromContext(c)

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, AppealResponse{
			Code:    40001,
			Message: "无效的申诉ID",
			Data:    nil,
		})
		return
	}

	var req model.ResolveAppealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AppealResponse{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	appeal, err := appealRepo.GetAppealByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "获取申诉详情失败",
			Data:    nil,
		})
		return
	}

	if appeal == nil {
		c.JSON(http.StatusNotFound, AppealResponse{
			Code:    40401,
			Message: "申诉不存在",
			Data:    nil,
		})
		return
	}

	var claim *model.Claim
	var task *model.Task
	var creator *model.User
	var businessUser *model.User
	if req.Accepted {
		claimID := appeal.ClaimID
		if claimID <= 0 {
			c.JSON(http.StatusNotFound, AppealResponse{
				Code:    40401,
				Message: "未找到对应的认领记录",
				Data:    nil,
			})
			return
		}
		claim, err = adminRepo.GetClaimByID(claimID)
		if err != nil || claim == nil {
			c.JSON(http.StatusNotFound, AppealResponse{
				Code:    40401,
				Message: "未找到对应的认领记录",
				Data:    nil,
			})
			return
		}
		task, err = adminRepo.GetTaskByID(claim.TaskID)
		if err != nil || task == nil {
			c.JSON(http.StatusNotFound, AppealResponse{
				Code:    40401,
				Message: "未找到对应的任务",
				Data:    nil,
			})
			return
		}
		creator, err = adminRepo.GetUserByID(claim.CreatorID)
		if err != nil || creator == nil {
			c.JSON(http.StatusNotFound, AppealResponse{
				Code:    40401,
				Message: "未找到对应的创作者",
				Data:    nil,
			})
			return
		}
		businessUser, err = adminRepo.GetUserByID(task.BusinessID)
		if err != nil || businessUser == nil {
			c.JSON(http.StatusNotFound, AppealResponse{
				Code:    40401,
				Message: "未找到对应的商家",
				Data:    nil,
			})
			return
		}
	}

	if err := handleAppealResolutionTx(adminRepo, appeal, req, adminID, claim, task, creator, businessUser); err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "处理申诉失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, AppealResponse{
		Code:    0,
		Message: "申诉已处理",
		Data: gin.H{
			"id":       appeal.ID,
			"claim_id": appeal.ClaimID,
			"task_id":  resolveAppealTaskID(appeal),
			"status":   2,
			"result":   req.Result,
		},
	})
}

// ListBusinessAppeals handles listing appeals for business
// GET /api/v1/business/appeals
func ListBusinessAppeals(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, AppealResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	// Parse pagination
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

	// Get all task IDs owned by this business, then resolve claim IDs
	taskIDs, err := adminRepo.GetTaskIDsByBusinessID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "获取申诉列表失败",
			Data:    nil,
		})
		return
	}

	if len(taskIDs) == 0 {
		c.JSON(http.StatusOK, AppealResponse{
			Code:    0,
			Message: "success",
			Data: gin.H{
				"appeals": []interface{}{},
				"total":   0,
			},
		})
		return
	}

	claimIDSet := make(map[int64]struct{})
	for _, taskID := range taskIDs {
		claims, err := adminRepo.GetClaimsByTaskID(taskID, 100, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, AppealResponse{
				Code:    50001,
				Message: "获取申诉列表失败",
				Data:    nil,
			})
			return
		}
		for _, claim := range claims {
			if claim != nil {
				claimIDSet[claim.ID] = struct{}{}
			}
		}
	}
	claimIDs := make([]int64, 0, len(claimIDSet))
	for id := range claimIDSet {
		claimIDs = append(claimIDs, id)
	}

	// Get appeals for these claims
	appeals, total, err := adminRepo.GetAppealsByClaimIDs(claimIDs, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "获取申诉列表失败",
			Data:    nil,
		})
		return
	}

	var formattedAppeals []gin.H
	for _, appeal := range appeals {
		typeStr := "作品申诉"
		statusStr := "待处理"
		if appeal.Status == model.AppealStatusResolved {
			statusStr = "已处理"
		}
		formattedAppeals = append(formattedAppeals, gin.H{
			"id":         appeal.ID,
			"user_id":    appeal.UserID,
			"type":       appeal.Type,
			"type_str":   typeStr,
			"claim_id":   appeal.ClaimID,
			"target_id":  appeal.TargetID,
			"reason":     appeal.Reason,
			"evidence":   resolveAppealEvidenceURLs(c, appeal.Evidence),
			"status":     appeal.Status,
			"status_str": statusStr,
			"result":     appeal.Result,
			"created_at": appeal.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, AppealResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"appeals": formattedAppeals,
			"total":   total,
		},
	})
}

func resolveAppealEvidenceURLs(c *gin.Context, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	provider, err := GetStorageProvider()
	if err != nil || provider == nil {
		return raw
	}
	cfg := config.Load()
	bucket := configuredStorageBucket(cfg)
	parts := strings.Split(raw, ",")
	resolved := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		url, err := storage.ResolveDisplayURL(c.Request.Context(), provider, bucket, part, 2*time.Hour)
		if err != nil || url == "" {
			resolved = append(resolved, part)
			continue
		}
		resolved = append(resolved, url)
	}
	return strings.Join(resolved, ",")
}

// HandleBusinessAppeal handles resolving an appeal by business
// PUT /api/v1/business/appeals/:id/handle
func HandleBusinessAppeal(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, AppealResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, AppealResponse{
			Code:    40001,
			Message: "无效的申诉ID",
			Data:    nil,
		})
		return
	}

	// Verify ownership - check if the appeal belongs to a task owned by this business
	appeal, err := appealRepo.GetAppealByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "获取申诉详情失败",
			Data:    nil,
		})
		return
	}

	if appeal == nil {
		c.JSON(http.StatusNotFound, AppealResponse{
			Code:    40401,
			Message: "申诉不存在",
			Data:    nil,
		})
		return
	}

	claimID := appeal.ClaimID
	if claimID <= 0 {
		c.JSON(http.StatusNotFound, AppealResponse{
			Code:    40401,
			Message: "未找到对应的认领记录",
			Data:    nil,
		})
		return
	}

	claim, err := adminRepo.GetClaimByID(claimID)
	if err != nil || claim == nil {
		c.JSON(http.StatusNotFound, AppealResponse{
			Code:    40401,
			Message: "未找到对应的认领记录",
			Data:    nil,
		})
		return
	}

	task, err := adminRepo.GetTaskByID(claim.TaskID)
	if err != nil || task == nil || task.BusinessID != userID {
		c.JSON(http.StatusForbidden, AppealResponse{
			Code:    40301,
			Message: "无权处理此申诉",
			Data:    nil,
		})
		return
	}

	var creator *model.User
	var businessUser *model.User
	var req model.ResolveAppealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AppealResponse{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	if req.Accepted {
		creator, err = adminRepo.GetUserByID(claim.CreatorID)
		if err != nil || creator == nil {
			c.JSON(http.StatusNotFound, AppealResponse{
				Code:    40401,
				Message: "未找到对应的创作者",
				Data:    nil,
			})
			return
		}
		businessUser, err = adminRepo.GetUserByID(task.BusinessID)
		if err != nil || businessUser == nil {
			c.JSON(http.StatusNotFound, AppealResponse{
				Code:    40401,
				Message: "未找到对应的商家",
				Data:    nil,
			})
			return
		}
	}

	if err := handleAppealResolutionTx(adminRepo, appeal, req, 0, claim, task, creator, businessUser); err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "处理申诉失败",
			Data:    nil,
		})
		return
	}

	// Send notification to user
	notificationService.NotifyAppealHandled(appeal.UserID, appeal.ID, req.Result)

	c.JSON(http.StatusOK, AppealResponse{
		Code:    0,
		Message: "申诉已处理",
		Data: gin.H{
			"id":       appeal.ID,
			"claim_id": appeal.ClaimID,
			"task_id":  claim.TaskID,
			"status":   2,
			"result":   req.Result,
		},
	})
}
