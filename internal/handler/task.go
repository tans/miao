package handler

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
)

// Response represents the standard API response
type TaskResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// GetTask handles getting a single task by ID
// GET /api/v1/tasks/:id
func GetTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, TaskResponse{
			Code:    40001,
			Message: "无效的任务ID",
			Data:    nil,
		})
		return
	}

	db := GetDB()
	taskRepo := repository.NewTaskRepository(db)
	userRepo := repository.NewUserRepository(db)
	creatorRepo := repository.NewCreatorRepository(db)

	task, err := taskRepo.GetTaskByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, TaskResponse{
			Code:    50001,
			Message: "获取任务失败",
			Data:    nil,
		})
		return
	}

	if task == nil {
		c.JSON(http.StatusNotFound, TaskResponse{
			Code:    40401,
			Message: "任务不存在",
			Data:    nil,
		})
		return
	}

	// Get business (publisher) info
	business, _ := userRepo.GetUserByID(task.BusinessID)
	businessName := ""
	businessAvatar := ""
	if business != nil {
		if business.Nickname != "" {
			businessName = business.Nickname
		} else {
			businessName = business.Username
		}
	}
	if business != nil {
		businessAvatar = resolveAvatarURL(business.Avatar, task.BusinessID)
	} else {
		businessAvatar = defaultAvatarURLByID(task.BusinessID)
	}

	// Check if current user has claimed this task
	var creatorClaim *model.Claim
	var creatorMaterials []*model.ClaimMaterial
	var submissions []gin.H
	submissionCount := 0
	userID, hasAuth := middleware.GetUserIDFromContext(c)
	if hasAuth {
		creatorClaim, err = creatorRepo.GetClaimByTaskIDAndCreatorID(task.ID, userID)
		if err != nil {
			log.Printf("Failed to get claim for task %d and user %d: %v", task.ID, userID, err)
		}
		if creatorClaim != nil {
			creatorMaterials, err = creatorRepo.GetClaimMaterials(creatorClaim.ID)
			if err != nil {
				log.Printf("Failed to get claim materials for claim %d: %v", creatorClaim.ID, err)
			}
		}
	}

	allClaims, err := taskRepo.GetTaskClaims(task.ID)
	if err != nil {
		log.Printf("Failed to get claims for task %d: %v", task.ID, err)
	} else {
		submissions = make([]gin.H, 0, len(allClaims))
		for _, claim := range allClaims {
			if !isVisibleTaskSubmission(claim) {
				continue
			}
			submissionCount++
			if !task.Public {
				continue
			}

			creator, creatorErr := userRepo.GetUserByID(claim.CreatorID)
			if creatorErr != nil {
				log.Printf("Failed to get creator %d for task %d: %v", claim.CreatorID, task.ID, creatorErr)
			}

			materials, materialsErr := creatorRepo.GetClaimMaterials(claim.ID)
			if materialsErr != nil {
				log.Printf("Failed to get claim materials for submission %d: %v", claim.ID, materialsErr)
				materials = []*model.ClaimMaterial{}
			}

			submissions = append(submissions, formatTaskSubmission(claim, creator, materials))
		}
	}

	c.JSON(http.StatusOK, TaskResponse{
		Code:    0,
		Message: "success",
		Data:    formatTaskDetail(task, businessName, businessAvatar, creatorClaim, creatorMaterials, submissions, submissionCount),
	})
}

// formatTask converts a Task model to a gin.H map
func formatTask(task *model.Task) gin.H {
	return formatTaskDetail(task, "", "", nil, nil, nil, 0)
}

// formatTaskDetail converts a Task model to a gin.H map with full details
func formatTaskDetail(task *model.Task, businessName, businessAvatar string, creatorClaim *model.Claim, creatorMaterials []*model.ClaimMaterial, submissions []gin.H, submissionCount int) gin.H {
	h := gin.H{
		"id":                   task.ID,
		"business_id":          task.BusinessID,
		"title":                task.Title,
		"description":          task.Description,
		"category":             task.Category,
		"unit_price":           task.UnitPrice,
		"total_count":          task.TotalCount,
		"remaining_count":      task.RemainingCount,
		"status":               task.Status,
		"is_available":         task.IsAvailable(),
		"total_budget":         task.TotalBudget,
		"service_fee_rate":     task.ServiceFeeRate,
		"service_fee_amount":   task.ServiceFeeAmount,
		"public":               task.Public,
		"frozen_amount":        task.FrozenAmount,
		"paid_amount":          task.PaidAmount,
		"pending_review_count": task.PendingReviewCount,
		"created_at":           task.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":           task.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"submission_count":     submissionCount,
	}

	// Business (publisher) info
	if businessName != "" {
		h["business_name"] = businessName
	}
	if businessAvatar != "" {
		h["business_avatar"] = businessAvatar
	} else {
		h["business_avatar"] = defaultAvatarURLByID(task.BusinessID)
	}

	if task.PublishAt != nil {
		h["publish_at"] = task.PublishAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if task.EndAt != nil {
		h["end_at"] = task.EndAt.Format("2006-01-02T15:04:05Z07:00")
		// 兼容小程序 camelCase 字段
		h["endAt"] = h["end_at"]
	}
	if task.ReviewAt != nil {
		h["review_at"] = task.ReviewAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if task.ReviewDeadlineAt != nil {
		h["review_deadline_at"] = task.ReviewDeadlineAt.Format("2006-01-02T15:04:05Z07:00")
	}

	// 小程序需要的字段：是否已报名、是否可以提交
	if creatorClaim != nil {
		h["hasSignedUp"] = true
		h["canSubmit"] = canCreatorSubmitClaim(creatorClaim)
	} else {
		h["hasSignedUp"] = false
		h["canSubmit"] = false
	}

	// V1 fields
	if task.Industries != "" {
		h["industries"] = task.Industries
	}
	if task.VideoDuration != "" {
		h["video_duration"] = task.VideoDuration
	}
	if task.VideoAspect != "" {
		h["video_aspect"] = task.VideoAspect
	}
	if task.VideoResolution != "" {
		h["video_resolution"] = task.VideoResolution
	}
	if task.Styles != "" {
		h["styles"] = task.Styles
	}
	if task.AwardPrice > 0 {
		h["award_price"] = task.AwardPrice
	}

	// 即梦合拍字段
	if task.JimengLink != "" || task.JimengCode != "" {
		h["jimeng_enabled"] = true
	} else {
		h["jimeng_enabled"] = false
	}
	if task.JimengLink != "" {
		h["jimeng_link"] = task.JimengLink
	}
	if task.JimengCode != "" {
		h["jimeng_code"] = task.JimengCode
	}

	// Task materials (reference materials from task publisher)
	if len(task.Materials) > 0 {
		h["materials"] = formatMaterials(task.Materials)
	} else {
		h["materials"] = []model.TaskMaterial{}
	}

	// Creator's claim info (if current user has claimed this task)
	if creatorClaim != nil {
		h["claim"] = formatClaim(creatorClaim)
		// Creator's submitted materials
		if len(creatorMaterials) > 0 {
			h["claim_materials"] = formatClaimMaterials(creatorMaterials)
		} else {
			h["claim_materials"] = []*model.ClaimMaterial{}
		}
	}
	if task.Public {
		h["submissions"] = submissions
	} else {
		h["submissions"] = []gin.H{}
	}

	return h
}

func canCreatorSubmitClaim(claim *model.Claim) bool {
	if claim == nil {
		return false
	}
	if claim.Status != model.ClaimStatusPending {
		return false
	}
	if claim.SubmitAt != nil {
		return false
	}
	return claim.ReviewResult == nil || *claim.ReviewResult == 0
}

func isVisibleTaskSubmission(claim *model.Claim) bool {
	if claim == nil {
		return false
	}
	if claim.Status >= model.ClaimStatusSubmitted {
		return true
	}
	if claim.SubmitAt != nil {
		return true
	}
	return claim.ReviewResult != nil
}

func formatTaskSubmission(claim *model.Claim, creator *model.User, materials []*model.ClaimMaterial) gin.H {
	creatorName := ""
	creatorAvatar := ""
	creatorLevel := 0
	if creator != nil {
		if creator.Nickname != "" {
			creatorName = creator.Nickname
		} else {
			creatorName = creator.Username
		}
		creatorAvatar = resolveAvatarURL(creator.Avatar, creator.ID)
		creatorLevel = int(creator.Level)
	}

	result := gin.H{
		"id":             claim.ID,
		"task_id":        claim.TaskID,
		"creator_id":     claim.CreatorID,
		"creator_name":   creatorName,
		"creator_avatar": creatorAvatar,
		"creator_level":  creatorLevel,
		"status":         claim.Status,
		"content":        claim.Content,
		"materials":      formatClaimMaterials(materials),
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
	return result
}

// formatClaim converts a Claim model to gin.H
func formatClaim(claim *model.Claim) gin.H {
	h := gin.H{
		"id":         claim.ID,
		"task_id":    claim.TaskID,
		"creator_id": claim.CreatorID,
		"status":     claim.Status,
		"content":    claim.Content,
		"created_at": claim.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if claim.SubmitAt != nil {
		h["submit_at"] = claim.SubmitAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if claim.ReviewAt != nil {
		h["review_at"] = claim.ReviewAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if claim.ReviewResult != nil {
		h["review_result"] = *claim.ReviewResult
	}
	if claim.ReviewComment != "" {
		h["review_comment"] = claim.ReviewComment
	}
	return h
}

// formatClaimMaterials converts claim materials and prefixes their URLs with CDN
func formatClaimMaterials(materials []*model.ClaimMaterial) []*model.ClaimMaterial {
	return formatVisibleClaimMaterials(materials)
}

// formatMaterials converts materials and prefixes their URLs with CDN
func formatMaterials(materials []model.TaskMaterial) []model.TaskMaterial {
	cfg := config.Load()
	cdn := cfg.Static.CDN
	if cdn == "" {
		cdn = cfg.Static.Host
	}
	result := make([]model.TaskMaterial, len(materials))
	for i, m := range materials {
		result[i] = m
		if result[i].FilePath != "" && !strings.HasPrefix(result[i].FilePath, "http") {
			result[i].FilePath = cdn + result[i].FilePath
		}
	}
	return result
}

// formatTaskList converts a slice of Task models to the API list format.
// Parses industries from comma-separated string to []string, includes materials.
func formatTaskList(tasks []*model.Task) []gin.H {
	result := make([]gin.H, 0, len(tasks))
	for _, t := range tasks {
		h := formatTask(t)
		// Parse industries string to array
		if ind, ok := h["industries"].(string); ok && ind != "" {
			h["industries"] = strings.Split(ind, ",")
		} else {
			h["industries"] = []string{}
		}
		result = append(result, h)
	}
	return result
}
