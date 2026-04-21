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

// MobileWorks - 过审作品列表
func MobileWorks(c *gin.Context) {
	db := GetDB()
	creatorRepo := repository.NewCreatorRepository(db)
	userRepo := repository.NewUserRepository(db)
	taskRepo := repository.NewTaskRepository(db)

	claims, _, err := creatorRepo.ListClaimsByStatus(model.ClaimStatusApproved, 20, 0)
	if err != nil {
		log.Printf("Failed to load initial works: %v", err)
		claims = []*model.Claim{}
	}

	works := make([]gin.H, 0, len(claims))
	for _, claim := range claims {
		creator, _ := userRepo.GetUserByID(claim.CreatorID)
		creatorName := "匿名"
		creatorAvatar := "/static/images/avatar-default.jpg"
		if creator != nil {
			if creator.Nickname != "" {
				creatorName = creator.Nickname
			} else if creator.Username != "" {
				creatorName = creator.Username
			}
			if creator.Avatar != "" {
				creatorAvatar = creator.Avatar
			}
		}

		if task, err := taskRepo.GetTaskByID(claim.TaskID); err == nil && task != nil && task.Title != "" {
			works = append(works, gin.H{
				"ID":            claim.ID,
				"Content":       claim.Content,
				"CreatorName":   creatorName,
				"CreatorAvatar": creatorAvatar,
				"TaskTitle":     task.Title,
			})
			continue
		}

		works = append(works, gin.H{
			"ID":            claim.ID,
			"Content":       claim.Content,
			"CreatorName":   creatorName,
			"CreatorAvatar": creatorAvatar,
		})
	}

	c.HTML(http.StatusOK, "mobile/works.html", gin.H{
		"Title":     "过审作品",
		"ActiveTab": "works",
		"Works":     works,
	})
}

// MobileWorkDetail - 作品详情
func MobileWorkDetail(c *gin.Context) {
	workID := parseInt64(c.Param("id"), 0)
	if workID == 0 {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"Message": "无效的作品ID",
		})
		return
	}

	db := GetDB()
	creatorRepo := repository.NewCreatorRepository(db)
	userRepo := repository.NewUserRepository(db)

	work, err := creatorRepo.GetClaimByID(workID)
	if err != nil || work == nil || work.Status != model.ClaimStatusApproved {
		log.Printf("Failed to load work %d: %v", workID, err)
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"Message": "作品不存在",
		})
		return
	}

	creator, err := userRepo.GetUserByID(work.CreatorID)
	if err != nil || creator == nil {
		creator = &model.User{
			Nickname: "匿名创作者",
			Avatar:   "/static/images/avatar-default.jpg",
		}
	} else {
		if creator.Nickname == "" {
			creator.Nickname = creator.Username
		}
		if creator.Avatar == "" {
			creator.Avatar = "/static/images/avatar-default.jpg"
		}
	}

	materials, err := creatorRepo.GetClaimMaterials(work.ID)
	if err != nil || materials == nil {
		materials = []*model.ClaimMaterial{}
	}

	var workCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM claims WHERE creator_id = ? AND status = ?`,
		work.CreatorID, model.ClaimStatusApproved,
	).Scan(&workCount); err != nil {
		log.Printf("Failed to count works for creator %d: %v", work.CreatorID, err)
		workCount = 0
	}

	// 获取点赞数和点赞状态
	likeCount, _ := creatorRepo.GetWorkLikeCount(workID)
	isLiked := false
	_, hasToken := c.Cookie("token")
	if hasToken {
		userID, _ := middleware.GetUserIDFromContext(c)
		if userID > 0 {
			isLiked, _ = creatorRepo.HasWorkLiked(workID, userID)
		}
	}

	c.HTML(http.StatusOK, "mobile/work_detail.html", gin.H{
		"Title":      "作品详情",
		"Work":       work,
		"Creator":    creator,
		"Materials":  materials,
		"WorkCount":  workCount,
		"ViewCount":  0,
		"LikeCount":  likeCount,
		"IsLiked":    isLiked,
		"IsLoggedIn": hasToken,
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
			Avatar:   "/static/images/avatar-default.jpg",
		}
	}

	// Check if user already claimed this task (if logged in)
	var alreadyClaimed bool
	var claimID int64
	var claimStatus model.ClaimStatus
	var claimMaterials []*model.ClaimMaterial
	userID, hasAuth := middleware.GetUserIDFromContext(c)
	if hasAuth {
		creatorRepo := repository.NewCreatorRepository(db)
		claims, _ := creatorRepo.ListClaimsByCreatorID(userID)
		for _, claim := range claims {
			if claim.TaskID == taskID && (claim.Status == model.ClaimStatusPending || claim.Status == model.ClaimStatusSubmitted) {
				alreadyClaimed = true
				claimID = claim.ID
				claimStatus = claim.Status
				// Fetch claim materials
				claimMaterials, _ = creatorRepo.GetClaimMaterials(claim.ID)
				break
			}
		}
	}

	c.HTML(http.StatusOK, "mobile/task_detail.html", gin.H{
		"Title":          task.Title,
		"Task":           task,
		"Business":       business,
		"AlreadyClaimed": alreadyClaimed,
		"ClaimID":        claimID,
		"ClaimStatus":    claimStatus,
		"ClaimMaterials": claimMaterials,
		"IsLoggedIn":     hasAuth,
		"TaskAvailable":  task.IsAvailable(),
		"ActiveTab":      "tasks",
	})
}

// MobileTaskClaims - 任务投稿列表（商家查看创作者提交的作品）
// GET /mobile/task/:id/claims
func MobileTaskClaims(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID := parseInt64(taskIDStr, 0)

	if taskID == 0 {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"Message": "无效的任务ID",
		})
		return
	}

	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.HTML(http.StatusUnauthorized, "mobile/login.html", gin.H{
			"Title": "登录",
		})
		return
	}

	db := GetDB()
	taskRepo := repository.NewTaskRepository(db)
	userRepo := repository.NewUserRepository(db)
	creatorRepo := repository.NewCreatorRepository(db)

	// Get task
	task, err := taskRepo.GetTaskByID(taskID)
	if err != nil || task == nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"Message": "任务不存在",
		})
		return
	}

	// Verify ownership (only business can view)
	if task.BusinessID != userID {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"Message": "无权查看此任务的投稿",
		})
		return
	}

	// Get business info
	business, _ := userRepo.GetUserByID(task.BusinessID)

	// Get all claims for this task
	allClaims, err := taskRepo.GetTaskClaims(taskID)
	if err != nil {
		log.Printf("Failed to load claims for task %d: %v", taskID, err)
		allClaims = []*model.Claim{}
	}

	// For each claim, get creator info and materials
	type ClaimWithCreator struct {
		*model.Claim
		CreatorName   string
		CreatorAvatar string
		Materials     []*model.ClaimMaterial
	}

	claimsWithInfo := make([]ClaimWithCreator, 0, len(allClaims))
	for _, claim := range allClaims {
		creator, _ := userRepo.GetUserByID(claim.CreatorID)
		materials, _ := creatorRepo.GetClaimMaterials(claim.ID)

		cwc := ClaimWithCreator{
			Claim:         claim,
			CreatorName:   "",
			CreatorAvatar: "",
			Materials:     materials,
		}
		if creator != nil {
			if creator.Nickname != "" {
				cwc.CreatorName = creator.Nickname
			} else {
				cwc.CreatorName = creator.Username
			}
			cwc.CreatorAvatar = creator.Avatar
		}
		claimsWithInfo = append(claimsWithInfo, cwc)
	}

	c.HTML(http.StatusOK, "mobile/task_claims.html", gin.H{
		"Title":     task.Title + " - 投稿",
		"ActiveTab": "mine",
		"Task":      task,
		"Business":  business,
		"Claims":    claimsWithInfo,
	})
}
