package router

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/handler"
	"github.com/tans/miao/internal/middleware"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	// Load HTML templates - need to manually include both root and subdirectory files
	templatesDir := filepath.Join(getWorkDir(), "web", "templates")

	// Collect all template files
	allFiles, _ := filepath.Glob(filepath.Join(templatesDir, "*.html"))
	subFiles, _ := filepath.Glob(filepath.Join(templatesDir, "**", "*.html"))
	allFiles = append(allFiles, subFiles...)

	// Parse all templates into a single template set
	tmpl := template.Must(template.ParseFiles(allFiles...))
	r.SetHTMLTemplate(tmpl)

	// Serve static files
	r.Static("/static", filepath.Join(getWorkDir(), "web", "static"))

	// CORS middleware
	r.Use(corsMiddleware())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// ===== 公开页面 =====
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})
	r.GET("/tasks.html", func(c *gin.Context) {
		c.HTML(http.StatusOK, "tasks.html", nil)
	})
	r.GET("/auth/login.html", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", nil)
	})
	r.GET("/auth/register.html", func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.html", nil)
	})

	// 商家端页面（公开访问，由前端 JS 处理认证）
	businessPages := []string{"dashboard.html", "task_create.html", "task_list.html", "task_detail.html", "claim_review.html", "recharge.html", "transactions.html", "appeal.html", "appeal_list.html"}
	for _, page := range businessPages {
		r.GET("/business/"+page, func(page string) gin.HandlerFunc {
			return func(c *gin.Context) {
				c.HTML(http.StatusOK, page, nil)
			}
		}(page))
	}

	// 创作者端页面（公开访问，由前端 JS 处理认证）
	creatorPages := []string{"dashboard.html", "task_hall.html", "task_detail.html", "claim_list.html", "my_submissions.html", "delivery.html", "wallet.html", "transactions.html", "appeal.html", "appeal_list.html"}
	for _, page := range creatorPages {
		r.GET("/creator/"+page, func(page string) gin.HandlerFunc {
			return func(c *gin.Context) {
				c.HTML(http.StatusOK, page, nil)
			}
		}(page))
	}

	// 管理端页面（公开访问，由前端 JS 处理认证）
	adminPages := []string{"dashboard.html", "user_list.html", "task_list.html", "task_review.html", "appeal_list.html"}
	for _, page := range adminPages {
		r.GET("/admin/"+page, func(page string) gin.HandlerFunc {
			return func(c *gin.Context) {
				c.HTML(http.StatusOK, page, nil)
			}
		}(page))
	}

	// API v1
	v1 := r.Group("/api/v1")
	{
		// ===== 公开 API =====
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", handler.Register)
			authGroup.POST("/login", handler.Login)
		}

		// 创作者任务大厅（公开）
		v1.GET("/tasks", handler.ListAvailableTasks)

		// 文件上传（需要认证）
		v1.POST("/upload", middleware.AuthMiddleware(), handler.UploadFile)

		// ===== 需要认证的 API =====
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware())
		{
			// 用户信息
			userGroup := protected.Group("/users")
			{
				userGroup.GET("/me", handler.GetCurrentUser)
				userGroup.PUT("/me", handler.UpdateProfile)
				userGroup.GET("/credits", handler.GetUserCredits)
			}

			// 用户个人中心
			v1User := protected.Group("/user")
			{
				v1User.GET("/profile", handler.GetUserProfile)
				v1User.PUT("/profile", handler.UpdateUserProfile)
				v1User.PUT("/password", handler.ChangePassword)
			}

			// 消息通知
			messageGroup := protected.Group("/messages")
			{
				messageGroup.GET("", handler.GetMessages)
				messageGroup.GET("/unread-count", handler.GetUnreadCount)
				messageGroup.POST("/:id/read", handler.MarkMessageAsRead)
				messageGroup.POST("/read-all", handler.MarkAllAsRead)
				messageGroup.DELETE("/:id", handler.DeleteMessage)
			}

			// 创作者端 API
			creatorGroup := protected.Group("/creator")
			creatorGroup.Use(middleware.RequireRole("creator"))
			{
				creatorGroup.GET("/tasks", handler.ListAvailableTasks)
				creatorGroup.POST("/claim", handler.ClaimTask)
				creatorGroup.GET("/claims", handler.ListMyClaims)
				creatorGroup.PUT("/claim/:id/submit", handler.SubmitClaim)
				creatorGroup.GET("/wallet", handler.GetWallet)
				creatorGroup.GET("/transactions", handler.GetTransactions)
				creatorGroup.GET("/stats", handler.GetCreatorStats)
				creatorGroup.GET("/chart/income", handler.GetCreatorIncomeChart)
			}

			// 商家端 API
			businessGroup := protected.Group("/business")
			businessGroup.Use(middleware.RequireRole("business"))
			{
				businessGroup.POST("/tasks", handler.CreateTask)
				businessGroup.DELETE("/tasks/:id", handler.CancelTask)
				businessGroup.GET("/tasks", handler.ListMyTasks)
				businessGroup.GET("/tasks/:id/claims", handler.GetTaskClaims)
				businessGroup.GET("/claims", handler.GetAllClaims)
				businessGroup.GET("/claim/:id", handler.GetClaim)
				businessGroup.PUT("/claim/:id/review", handler.ReviewClaim)
				businessGroup.GET("/balance", handler.GetBalance)
				businessGroup.POST("/recharge", handler.Recharge)
				businessGroup.GET("/transactions", handler.GetTransactions)
				businessGroup.GET("/stats", handler.GetBusinessStats)
				businessGroup.GET("/chart/expense", handler.GetBusinessExpenseChart)
			}

			// 申诉 API
			appealGroup := protected.Group("/appeals")
			{
				appealGroup.POST("", handler.CreateAppeal)
				appealGroup.GET("", handler.ListAppeals)
				appealGroup.GET("/:id", handler.GetAppeal)
			}

			// 管理端 API
			adminGroup := protected.Group("/admin")
			adminGroup.Use(handler.RequireAdmin())
			{
				adminGroup.GET("/dashboard", handler.GetDashboard)
				adminGroup.GET("/users", handler.ListUsers)
				adminGroup.PUT("/users/:id/status", handler.UpdateUserStatus)
				adminGroup.PUT("/users/:id/credit", handler.UpdateUserCredit)
				adminGroup.GET("/tasks", handler.ListTasksAdmin)
				adminGroup.PUT("/task/:id/review", handler.ReviewTask)
				adminGroup.GET("/claims", handler.ListClaimsAdmin)
				adminGroup.GET("/appeals", handler.ListAppealsAdmin)
				adminGroup.GET("/appeals/:id", handler.GetAppealAdmin)
				adminGroup.PUT("/appeals/:id/handle", handler.HandleAppeal)
			}
		}
	}

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func getWorkDir() string {
	dir, _ := filepath.Abs(filepath.Dir("."))
	return dir
}
