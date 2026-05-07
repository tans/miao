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

func formatAppealDisplayText(text string, fallback string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return fallback
	}
	return text
}

func buildAppealMerchantResult(claim *model.Claim) string {
	if claim == nil {
		return ""
	}
	if claim.ReviewResult != nil {
		switch *claim.ReviewResult {
		case int(model.ReviewResultReport):
			return formatAppealDisplayText(claim.ReviewComment, "作品被举报")
		case int(model.ReviewResultReturn):
			return formatAppealDisplayText(claim.ReviewComment, "作品被退回")
		default:
			return formatAppealDisplayText(claim.ReviewComment, "")
		}
	}
	return formatAppealDisplayText(claim.ReviewComment, "")
}

func buildAppealDecisionText(claim *model.Claim) string {
	if claim == nil {
		return ""
	}
	if claim.Status == model.ClaimStatusSubmitted && claim.ReviewResult == nil {
		return "通过申诉"
	}
	if claim.Status == model.ClaimStatusPending && claim.ReviewResult != nil {
		switch *claim.ReviewResult {
		case int(model.ReviewResultReturn), int(model.ReviewResultReport):
			return "拒绝申诉"
		}
	}
	return ""
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

	if err := resolveAppealWithinTx(tx, appeal.ID, req.Result, adminID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
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
		taskTitle := ""
		merchantResult := ""
		decisionText := ""
		appealResult := formatAppealDisplayText(appeal.Result, "平台处理中")
		claimID := appeal.ClaimID
		if claimID > 0 {
			if claim, err := adminRepo.GetClaimByID(claimID); err == nil && claim != nil {
				if task, err := adminRepo.GetTaskByID(claim.TaskID); err == nil && task != nil {
					taskTitle = task.Title
				}
				merchantResult = buildAppealMerchantResult(claim)
				decisionText = buildAppealDecisionText(claim)
			}
		}
		typeStr := "作品申诉"
		statusStr := "待处理"
		if appeal.Status == model.AppealStatusResolved {
			statusStr = "已处理"
		}
		formattedAppeals = append(formattedAppeals, gin.H{
			"id":              appeal.ID,
			"type":            appeal.Type,
			"type_str":        typeStr,
			"claim_id":        appeal.ClaimID,
			"target_id":       appeal.TargetID,
			"task_id":         taskID,
			"task_title":      taskTitle,
			"reason":          appeal.Reason,
			"status":          appeal.Status,
			"status_str":      statusStr,
			"result":          appealResult,
			"merchant_result": merchantResult,
			"decision_text":   decisionText,
			"created_at":      appeal.CreatedAt.Format(time.RFC3339),
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
	taskTitle := ""
	merchantResult := ""
	decisionText := ""
	claimID := appeal.ClaimID
	if claimID > 0 {
		if claim, err := adminRepo.GetClaimByID(claimID); err == nil && claim != nil {
			if task, err := adminRepo.GetTaskByID(claim.TaskID); err == nil && task != nil {
				taskTitle = task.Title
			}
			merchantResult = buildAppealMerchantResult(claim)
			decisionText = buildAppealDecisionText(claim)
		}
	}

	c.JSON(http.StatusOK, AppealResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"id":              appeal.ID,
			"user_id":         appeal.UserID,
			"type":            appeal.Type,
			"type_str":        typeStr,
			"claim_id":        appeal.ClaimID,
			"target_id":       appeal.TargetID,
			"task_id":         taskID,
			"task_title":      taskTitle,
			"reason":          appeal.Reason,
			"evidence":        resolveAppealEvidenceURLs(c, appeal.Evidence),
			"status":          appeal.Status,
			"status_str":      statusStr,
			"result":          formatAppealDisplayText(appeal.Result, "平台处理中"),
			"merchant_result": merchantResult,
			"decision_text":   decisionText,
			"created_at":      appeal.CreatedAt.Format(time.RFC3339),
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
		taskID := int64(0)
		taskTitle := ""
		merchantResult := ""
		decisionText := ""
		if claimID := appeal.ClaimID; claimID > 0 {
			if claim, err := adminRepo.GetClaimByID(claimID); err == nil && claim != nil {
				taskID = claim.TaskID
				merchantResult = buildAppealMerchantResult(claim)
				decisionText = buildAppealDecisionText(claim)
				if task, err := adminRepo.GetTaskByID(claim.TaskID); err == nil && task != nil {
					taskTitle = task.Title
				}
			}
		}
		formattedAppeals = append(formattedAppeals, gin.H{
			"id":              appeal.ID,
			"user_id":         appeal.UserID,
			"type":            appeal.Type,
			"type_str":        typeStr,
			"claim_id":        appeal.ClaimID,
			"task_id":         taskID,
			"task_title":      taskTitle,
			"target_id":       appeal.TargetID,
			"reason":          appeal.Reason,
			"evidence":        resolveAppealEvidenceURLs(c, appeal.Evidence),
			"status":          appeal.Status,
			"status_str":      statusStr,
			"result":          formatAppealDisplayText(appeal.Result, "平台处理中"),
			"merchant_result": merchantResult,
			"decision_text":   decisionText,
			"created_at":      appeal.CreatedAt.Format(time.RFC3339),
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
