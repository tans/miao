package router

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/handler"
	"github.com/tans/miao/internal/middleware"
)

func SetupRouter() *gin.Engine {
	// Setup error log file before gin.Default() so recovery can use it
	logDir := filepath.Join(getWorkDir(), "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("Warning: failed to create logs dir: %v", err)
	}
	errorLogFile, err := os.OpenFile(filepath.Join(logDir, "errors.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Warning: failed to open errors.log: %v", err)
	}
	errorLogger := log.New(errorLogFile, "", log.LstdFlags|log.Lshortfile)

	r := gin.New()

	// Custom recovery that logs panics to error.log
	r.Use(func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				errorLogger.Printf("[PANIC] %v | %s %s | client_ip=%s | user_id=%v",
					r, c.Request.Method, c.Request.URL.Path, c.ClientIP(), getUserID(c))
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    50000,
					"message": "服务器内部错误",
					"data":    nil,
				})
			}
		}()
		c.Next()
	})

	// Error logging middleware for all 500 responses
	r.Use(func(c *gin.Context) {
		c.Next()
		if c.Writer.Status() >= 500 {
			errorLogger.Printf("[500] status=%d | %s %s | client_ip=%s | user_id=%v | errors=%v",
				c.Writer.Status(), c.Request.Method, c.Request.URL.Path, c.ClientIP(), getUserID(c), c.Errors.String())
		}
	})

	// Load HTML templates
	templatesDir := filepath.Join(getWorkDir(), "web", "templates")

	// Collect template files from all directories (admin templates are now standalone HTML)
	allFiles, _ := filepath.Glob(filepath.Join(templatesDir, "*.html"))
	subDirs := []string{"auth", "business", "creator", "mobile", "user", "admin"}
	for _, dir := range subDirs {
		files, _ := filepath.Glob(filepath.Join(templatesDir, dir, "*.html"))
		allFiles = append(allFiles, files...)
	}
	// Mobile nested components
	mobileNested, _ := filepath.Glob(filepath.Join(templatesDir, "mobile", "components", "*.html"))
	allFiles = append(allFiles, mobileNested...)

	// Parse all templates with custom functions
	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"iterate": func(count int) []int {
			var result []int
			for i := 0; i < count; i++ {
				result = append(result, i)
			}
			return result
		},
	}).ParseFiles(allFiles...))
	r.SetHTMLTemplate(tmpl)

	// Serve static files
	staticDir := filepath.Join(getWorkDir(), "web", "static")
	r.Static("/static", staticDir)

	// Add cache headers middleware for static assets
	r.Use(func(c *gin.Context) {
		c.Next()
		// Add cache headers after request is processed
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" {
			if strings.HasPrefix(c.Request.URL.Path, "/static/") {
				c.Header("Cache-Control", "public, max-age=604800, immutable")
				c.Header("X-Content-Type-Options", "nosniff")
			}
		}
	})

	// Serve docs (OpenAPI spec)
	docsDir := filepath.Join(getWorkDir(), "docs")
	r.Static("/docs", docsDir)

	// Gzip compression middleware for responses
	r.Use(gzip.Gzip(gzip.DefaultCompression))

	// CORS middleware
	r.Use(corsMiddleware())

	// Rate limiting middleware (disabled when DISABLE_RATE_LIMIT=1)
	if os.Getenv("DISABLE_RATE_LIMIT") != "1" {
		// Use higher limit for test environments (default 100/min, test 500/min)
		if limit := os.Getenv("RATE_LIMIT"); limit != "" {
			if l, err := strconv.Atoi(limit); err == nil {
				r.Use(middleware.IPRateLimitByEndpoint(l, time.Minute))
			} else {
				r.Use(middleware.RateLimitMiddleware())
			}
		} else {
			r.Use(middleware.RateLimitMiddleware())
		}
	}

	// Audit middleware for sensitive endpoints
	r.Use(middleware.AuditMiddlewareSensitive())

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
		c.HTML(http.StatusOK, "auth/login.html", nil)
	})
	r.GET("/auth/register.html", func(c *gin.Context) {
		c.HTML(http.StatusOK, "auth/register.html", nil)
	})

	// 商家端页面（公开访问，由前端 JS 处理认证）
	businessPages := []string{"dashboard.html", "task_create.html", "task_list.html", "task_detail.html", "claim_review.html", "recharge.html", "transactions.html", "appeal.html", "appeal_list.html", "notifications.html"}
	for _, page := range businessPages {
		r.GET("/business/"+page, func(page string) gin.HandlerFunc {
			return func(c *gin.Context) {
				c.HTML(http.StatusOK, "business/"+page, nil)
			}
		}(page))
	}

	// 创作者端页面（公开访问，由前端 JS 处理认证）
	creatorPages := []string{"dashboard.html", "task_hall.html", "task_detail.html", "claim_list.html", "wallet.html", "transactions.html", "appeal.html", "appeal_list.html", "notifications.html"}
	for _, page := range creatorPages {
		r.GET("/creator/"+page, func(page string) gin.HandlerFunc {
			return func(c *gin.Context) {
				c.HTML(http.StatusOK, "creator/"+page, nil)
			}
		}(page))
	}

	// 管理端页面
	adminPages := []string{"dashboard.html", "user_list.html", "task_list.html", "task_review.html", "appeal_list.html", "appeals.html", "users.html", "tasks.html", "finance.html", "database.html", "login.html", "works.html", "user_detail.html", "task_detail.html"}
	for _, page := range adminPages {
		r.GET("/admin/"+page, func(page string) gin.HandlerFunc {
			return func(c *gin.Context) {
				c.HTML(http.StatusOK, "admin/"+page, nil)
			}
		}(page))
	}
	helpPages := []string{"index.html", "faq.html", "tutorial.html"}
	for _, page := range helpPages {
		r.GET("/help/"+page, func(page string) gin.HandlerFunc {
			return func(c *gin.Context) {
				c.HTML(http.StatusOK, "help/"+page, nil)
			}
		}(page))
	}

	// 用户中心页面
	userPages := []string{"profile.html", "password.html"}
	for _, page := range userPages {
		r.GET("/user/"+page, func(page string) gin.HandlerFunc {
			return func(c *gin.Context) {
				c.HTML(http.StatusOK, "user/"+page, nil)
			}
		}(page))
	}

	// 移动端页面
	mobile := r.Group("/mobile")
	{
		mobile.GET("/", handler.MobileIndex)
		mobile.GET("/mine", middleware.MobilePageAuthMiddleware(), handler.MobileMine)
		mobile.GET("/task/:id", handler.MobileTaskDetail)
		mobile.GET("/login", func(c *gin.Context) {
			c.HTML(http.StatusOK, "mobile/login.html", gin.H{
				"Title": "登录",
			})
		})
		mobile.GET("/register", func(c *gin.Context) {
			c.HTML(http.StatusOK, "mobile/register.html", gin.H{
				"Title": "注册",
			})
		})
		mobile.GET("/wallet", middleware.MobilePageAuthMiddleware(), func(c *gin.Context) {
			c.HTML(http.StatusOK, "mobile/wallet.html", gin.H{
				"Title":     "钱包",
				"ActiveTab": "mine",
			})
		})
		mobile.GET("/my-claims", middleware.MobilePageAuthMiddleware(), func(c *gin.Context) {
			c.HTML(http.StatusOK, "mobile/my_claims.html", gin.H{
				"Title":     "我领取的任务",
				"ActiveTab": "mine",
			})
		})
		mobile.GET("/my-tasks", middleware.MobilePageAuthMiddleware(), func(c *gin.Context) {
			c.HTML(http.StatusOK, "mobile/my_tasks.html", gin.H{
				"Title":     "我发布的任务",
				"ActiveTab": "mine",
			})
		})
		mobile.GET("/transactions", middleware.MobilePageAuthMiddleware(), func(c *gin.Context) {
			c.HTML(http.StatusOK, "mobile/transactions.html", gin.H{
				"Title":     "收益明细",
				"ActiveTab": "mine",
			})
		})
		mobile.GET("/settings", middleware.MobilePageAuthMiddleware(), func(c *gin.Context) {
			c.HTML(http.StatusOK, "mobile/settings.html", gin.H{
				"Title":     "设置",
				"ActiveTab": "mine",
			})
		})
	}

	// API v1
	v1 := r.Group("/api/v1")
	{
		// ===== 公开 API =====
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", handler.Register)
			authGroup.POST("/login", handler.Login)
			authGroup.POST("/wechat-mini-login", handler.WechatMiniLogin)
		}

		// 管理端登录（独立于用户登录）
		v1.POST("/admin/login", handler.AdminLogin)
		v1.POST("/admin/register", handler.AdminRegister)

		// 创作者任务大厅（公开）
		v1.GET("/tasks", handler.ListAvailableTasks)
		v1.GET("/tasks/:id", handler.GetTask)

		// 已完成作品列表（公开，供小程序看灵感用，基于 claims 表）
		v1.GET("/works", handler.ListApprovedWorks)
		v1.GET("/works/:id", handler.GetWork)

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

			// 通知
			notificationGroup := protected.Group("/notifications")
			{
				notificationGroup.GET("", handler.GetNotifications)
				notificationGroup.PUT("/:id/read", handler.MarkNotificationAsRead)
				notificationGroup.GET("/unread-count", handler.GetUnreadNotificationCount)
				notificationGroup.PUT("/read-all", handler.MarkAllNotificationsAsRead)
			}

			// 创作者端 API - 移除角色校验，所有用户都可访问
			creatorGroup := protected.Group("/creator")
			{
				creatorGroup.GET("/tasks", handler.ListAvailableTasks)
				creatorGroup.POST("/claim", handler.ClaimTask)
				creatorGroup.GET("/claims", handler.ListMyClaims)
				creatorGroup.PUT("/claim/:id/submit", handler.SubmitClaim)
				creatorGroup.DELETE("/claim/:id", handler.CancelClaim)
				creatorGroup.GET("/claim/:id", handler.GetClaimByID)
				creatorGroup.GET("/claim/by-task/:taskId", handler.GetClaimByTaskID)
				creatorGroup.GET("/wallet", handler.GetWallet)
				creatorGroup.GET("/transactions", handler.GetTransactions)
				creatorGroup.GET("/stats", handler.GetCreatorStats)
				creatorGroup.GET("/chart/income", handler.GetCreatorIncomeChart)
			}

			// 统一钱包端点（/creator/wallet 的别名，供小程序和Web共用）
			protected.GET("/wallet", handler.GetWallet)

			// 商家端 API - 移除角色校验，所有用户都可访问
			businessGroup := protected.Group("/business")
			{
				businessGroup.POST("/tasks", handler.CreateTask)
				businessGroup.DELETE("/tasks/:id", handler.CancelTask)
				businessGroup.GET("/tasks", handler.ListMyTasks)
				businessGroup.GET("/tasks/:id/claims", handler.GetTaskClaims)
				businessGroup.GET("/claims", handler.GetAllClaims)
				businessGroup.GET("/claim/:id", handler.GetClaim)
				businessGroup.PUT("/claim/:id/review", handler.ReviewClaim)
				businessGroup.POST("/recharge", handler.Recharge)
				businessGroup.GET("/transactions", handler.GetTransactions)
				businessGroup.GET("/stats", handler.GetBusinessStats)
				businessGroup.GET("/chart/expense", handler.GetBusinessExpenseChart)
				businessGroup.GET("/appeals", handler.ListBusinessAppeals)
				businessGroup.PUT("/appeals/:id/handle", handler.HandleBusinessAppeal)
			}

			// 申诉 API
			appealGroup := protected.Group("/appeals")
			{
				appealGroup.POST("", handler.CreateAppeal)
				appealGroup.GET("", handler.ListAppeals)
				appealGroup.GET("/:id", handler.GetAppeal)
			}

			// 管理端 API - 仅管理员可访问
			adminGroup := protected.Group("/admin")
			adminGroup.Use(middleware.RequireAdmin())
			{
				adminGroup.GET("/dashboard", handler.GetDashboard)
				adminGroup.GET("/stats", handler.GetStats)
				adminGroup.GET("/users", handler.ListUsers)
				adminGroup.GET("/users/:id", handler.GetUserDetail)
				adminGroup.PUT("/users/:id/status", handler.UpdateUserStatus)
				adminGroup.PUT("/users/:id/credit", handler.UpdateUserCredit)
				adminGroup.PUT("/users/:id/balance", handler.UpdateUserBalance)
				adminGroup.GET("/users/:id/transactions", handler.GetUserTransactionsAdmin)
				adminGroup.GET("/tasks", handler.ListTasksAdmin)
				adminGroup.GET("/tasks/:id", handler.GetTaskAdmin)
				adminGroup.PUT("/tasks/:id", handler.UpdateTaskAdmin)
				adminGroup.PUT("/task/:id/review", handler.ReviewTask)
				adminGroup.GET("/claims", handler.ListClaimsAdmin)
				adminGroup.GET("/works", handler.ListWorksAdmin)
				adminGroup.GET("/works/:id", handler.GetWorkAdmin)
				adminGroup.PUT("/works/:id", handler.UpdateWorkAdmin)
				adminGroup.DELETE("/works/:id", handler.DeleteWorkAdmin)
				adminGroup.GET("/appeals", handler.ListAppealsAdmin)
				adminGroup.GET("/appeals/:id", handler.GetAppealAdmin)
				adminGroup.PUT("/appeals/:id/handle", handler.HandleAppeal)
				adminGroup.GET("/tables", handler.ListTables)
				adminGroup.POST("/query", handler.ExecuteQuery)
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

func getUserID(c *gin.Context) interface{} {
	if id, exists := c.Get("userID"); exists {
		return id
	}
	return nil
}
