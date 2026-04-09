package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
)

// MobileIndex - 任务大厅首页
func MobileIndex(c *gin.Context) {
	db := GetDB()
	taskRepo := repository.NewTaskRepository(db)

	// Fetch initial tasks for first screen (20 tasks)
	tasks, _, err := taskRepo.ListTasksWithPagination(0, "", "created_at", 20, 0)
	if err != nil {
		log.Printf("Failed to load initial tasks: %v", err)
		tasks = []*model.Task{} // Continue with empty array for graceful degradation
	}

	c.HTML(http.StatusOK, "mobile/index.html", gin.H{
		"Title":     "任务大厅",
		"ActiveTab": "tasks",
		"Tasks":     tasks,
	})
}

// MobileWorks - 过审作品页
func MobileWorks(c *gin.Context) {
	db := GetDB()
	submissionRepo := repository.NewSubmissionRepository(db)

	// Fetch initial approved works (20 works for first screen)
	works, _, err := submissionRepo.ListApprovedSubmissions(20, 0, "created_at")
	if err != nil {
		log.Printf("Failed to load initial works: %v", err)
		works = []*model.Submission{} // Continue with empty array for graceful degradation
	}

	c.HTML(http.StatusOK, "mobile/works.html", gin.H{
		"Title":     "过审作品",
		"ActiveTab": "works",
		"Works":     works,
	})
}

// MobileMine - 我的页面
func MobileMine(c *gin.Context) {
	// Get user ID from context (middleware already verified auth)
	userID, _ := middleware.GetUserIDFromContext(c)

	db := GetDB()
	userRepo := repository.NewUserRepository(db)

	// Get user info
	user, err := userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		log.Printf("Failed to load user info: %v", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"Message": "获取用户信息失败",
		})
		return
	}

	// Get wallet balance
	balance := user.Balance

	c.HTML(http.StatusOK, "mobile/mine.html", gin.H{
		"Title":     "我的",
		"ActiveTab": "mine",
		"User":      user,
		"Balance":   balance,
	})
}

// MobileTaskDetail - 任务详情
func MobileTaskDetail(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID := parseInt64(taskIDStr, 0)

	if taskID == 0 {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"Message": "无效的任务ID",
		})
		return
	}

	db := GetDB()
	taskRepo := repository.NewTaskRepository(db)
	userRepo := repository.NewUserRepository(db)

	// Get task details
	task, err := taskRepo.GetTaskByID(taskID)
	if err != nil || task == nil {
		log.Printf("Failed to load task %d: %v", taskID, err)
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"Message": "任务不存在",
		})
		return
	}

	// Get business info
	business, err := userRepo.GetUserByID(task.BusinessID)
	if err != nil || business == nil {
		log.Printf("Failed to load business info for task %d: %v", taskID, err)
		business = &model.User{
			Username: "商家",
			Avatar:   "/static/images/avatar-default.svg",
		}
	}

	// Check if user already claimed this task (if logged in)
	var alreadyClaimed bool
	var claimID int64
	var claimStatus model.ClaimStatus
	userID, hasAuth := middleware.GetUserIDFromContext(c)
	if hasAuth {
		creatorRepo := repository.NewCreatorRepository(db)
		claims, _ := creatorRepo.ListClaimsByCreatorID(userID)
		for _, claim := range claims {
			if claim.TaskID == taskID && (claim.Status == model.ClaimStatusPending || claim.Status == model.ClaimStatusSubmitted) {
				alreadyClaimed = true
				claimID = claim.ID
				claimStatus = claim.Status
				break
			}
		}
	}

	c.HTML(http.StatusOK, "mobile/task_detail.html", gin.H{
		"Title":           task.Title,
		"Task":            task,
		"Business":        business,
		"AlreadyClaimed":  alreadyClaimed,
		"ClaimID":         claimID,
		"ClaimStatus":     claimStatus,
		"IsLoggedIn":      hasAuth,
		"TaskAvailable":    task.IsAvailable(),
		"ActiveTab":       "tasks",
	})
}

// MobileWorkDetail - 作品详情
func MobileWorkDetail(c *gin.Context) {
	workIDStr := c.Param("id")
	workID := parseInt64(workIDStr, 0)

	if workID == 0 {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"Message": "无效的作品ID",
		})
		return
	}

	db := GetDB()
	submissionRepo := repository.NewSubmissionRepository(db)
	userRepo := repository.NewUserRepository(db)

	// Get submission details
	submission, err := submissionRepo.GetSubmissionByID(workID)
	if err != nil || submission == nil {
		log.Printf("Failed to load submission %d: %v", workID, err)
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"Message": "作品不存在",
		})
		return
	}

	// Only show approved submissions
	if submission.Status != model.SubmissionPassed {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"Message": "作品不存在",
		})
		return
	}

	// Get creator info
	creator, err := userRepo.GetUserByID(submission.CreatorID)
	if err != nil || creator == nil {
		log.Printf("Failed to load creator info for submission %d: %v", workID, err)
		creator = &model.User{
			Nickname: "匿名创作者",
			Avatar:   "/static/images/avatar-default.png",
		}
	}

	// Get submission materials (images/videos)
	materials, err := submissionRepo.GetSubmissionMaterials(workID)
	if err != nil {
		log.Printf("Failed to load materials for submission %d: %v", workID, err)
		materials = []*model.SubmissionMaterial{}
	}

	// Get creator's total work count
	workCount, err := submissionRepo.CountSubmissionsByCreatorID(submission.CreatorID)
	if err != nil {
		log.Printf("Failed to count creator works: %v", err)
		workCount = 0
	}

	// Increment view count (async, don't block on error)
	go func() {
		if err := submissionRepo.IncrementViewCount(workID); err != nil {
			log.Printf("Failed to increment view count for submission %d: %v", workID, err)
		}
	}()

	// Check if user liked this work (if logged in)
	var isLiked bool
	userID, hasAuth := middleware.GetUserIDFromContext(c)
	if hasAuth {
		// TODO: Implement like tracking when like feature is added
		_ = userID
		isLiked = false
	}

	// Use score field as view count temporarily (until dedicated view_count column is added)
	viewCount := submission.Score
	if viewCount < 0 {
		viewCount = 0
	}

	c.HTML(http.StatusOK, "mobile/work_detail.html", gin.H{
		"Title":      submission.Content,
		"Work":       submission,
		"Creator":    creator,
		"Materials":  materials,
		"WorkCount":  workCount,
		"ViewCount":  viewCount,
		"LikeCount":  0, // TODO: Implement like count
		"IsLiked":    isLiked,
		"IsLoggedIn": hasAuth,
		"ActiveTab":  "works",
	})
}
