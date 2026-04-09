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
	// Check if user is logged in
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		// Not logged in, redirect to login
		c.Redirect(http.StatusFound, "/auth/login.html")
		return
	}

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
	taskID := c.Param("id")
	c.HTML(http.StatusOK, "mobile/task_detail.html", gin.H{
		"Title":     "任务详情",
		"TaskID":    taskID,
		"ActiveTab": "tasks",
	})
}

// MobileWorkDetail - 作品详情
func MobileWorkDetail(c *gin.Context) {
	workID := c.Param("id")
	c.HTML(http.StatusOK, "mobile/work_detail.html", gin.H{
		"Title":     "作品详情",
		"WorkID":    workID,
		"ActiveTab": "works",
	})
}
