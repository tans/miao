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
		"Title":           task.Title,
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
