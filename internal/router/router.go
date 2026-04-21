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
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/handler"
	"github.com/tans/miao/internal/middleware"
)

func SetupRouter() *gin.Engine {
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

	r.Use(func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				errorLogger.Printf("[PANIC] %v | %s %s | client_ip=%s | user_id=%v", rec, c.Request.Method, c.Request.URL.Path, c.ClientIP(), getUserID(c))
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    50000,
					"message": "服务器内部错误",
					"data":    nil,
				})
			}
		}()
		c.Next()
	})

	r.Use(func(c *gin.Context) {
		c.Next()
		if c.Writer.Status() >= 500 {
			errorLogger.Printf("[500] status=%d | %s %s | client_ip=%s | user_id=%v | errors=%v", c.Writer.Status(), c.Request.Method, c.Request.URL.Path, c.ClientIP(), getUserID(c), c.Errors.String())
		}
	})

	templatesDir := filepath.Join(getWorkDir(), "web", "templates")
	allFiles, _ := filepath.Glob(filepath.Join(templatesDir, "*.html"))
	subDirs := []string{"auth", "business", "creator", "mobile", "user", "admin", "help"}
	for _, dir := range subDirs {
		files, _ := filepath.Glob(filepath.Join(templatesDir, dir, "*.html"))
		allFiles = append(allFiles, files...)
	}
	mobileNested, _ := filepath.Glob(filepath.Join(templatesDir, "mobile", "components", "*.html"))
	allFiles = append(allFiles, mobileNested...)

	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"iterate": func(count int) []int {
			result := make([]int, 0, count)
			for i := 0; i < count; i++ {
				result = append(result, i)
			}
			return result
		},
		"getStatusColor": func(status int) string {
			colors := map[int]string{1: "#FF9800", 2: "#2196F3", 3: "#4CAF50", 4: "#9E9E9E", 5: "#F44336"}
			if c, ok := colors[status]; ok {
				return c
			}
			return "#9E9E9E"
		},
		"getStatusBg": func(status int) string {
			bgs := map[int]string{1: "rgba(255,152,0,0.1)", 2: "rgba(33,150,243,0.1)", 3: "rgba(76,175,80,0.1)", 4: "rgba(158,158,158,0.1)", 5: "rgba(244,67,54,0.1)"}
			if bg, ok := bgs[status]; ok {
				return bg
			}
			return "rgba(158,158,158,0.1)"
		},
		"getStatusText": func(status int) string {
			texts := map[int]string{1: "待提交", 2: "待审核", 3: "已通过", 4: "已取消", 5: "已超时"}
			if t, ok := texts[status]; ok {
				return t
			}
			return "未知"
		},
	}).ParseFiles(allFiles...))
	r.SetHTMLTemplate(tmpl)

	staticDir := filepath.Join(getWorkDir(), "web", "static")
	uploadDir := filepath.Join(getWorkDir(), "web", "static", "uploads")
	docsDir := filepath.Join(getWorkDir(), "docs")
	r.Static("/static", staticDir)
	r.Static("/uploads", uploadDir)
	r.Static("/docs", docsDir)

	r.Use(func(c *gin.Context) {
		c.Next()
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
			if strings.HasPrefix(c.Request.URL.Path, "/static/") || strings.HasPrefix(c.Request.URL.Path, "/uploads/") {
				c.Header("Cache-Control", "public, max-age=604800, immutable")
				c.Header("X-Content-Type-Options", "nosniff")
			}
		}
	})

	r.Use(gzip.Gzip(gzip.DefaultCompression))
	cfg := config.Load()
	r.Use(corsMiddleware(cfg))

	if os.Getenv("DISABLE_RATE_LIMIT") != "1" {
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

	r.Use(middleware.AuditMiddlewareSensitive())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/", func(c *gin.Context) { c.HTML(http.StatusOK, "index.html", nil) })
	r.GET("/tasks.html", func(c *gin.Context) { c.HTML(http.StatusOK, "tasks.html", nil) })
	r.GET("/auth/login.html", func(c *gin.Context) { c.HTML(http.StatusOK, "auth/login.html", nil) })
	r.GET("/auth/register.html", func(c *gin.Context) { c.HTML(http.StatusOK, "auth/register.html", nil) })

	businessPages := []string{"dashboard.html", "task_create.html", "task_list.html", "task_detail.html", "claim_review.html", "recharge.html", "transactions.html", "appeal.html", "appeal_list.html", "notifications.html"}
	for _, page := range businessPages {
		registerHTMLPage(r, "/business/"+page, "business/"+page)
	}

	creatorPages := []string{"dashboard.html", "task_hall.html", "task_detail.html", "claim_list.html", "wallet.html", "transactions.html", "appeal.html", "appeal_list.html", "notifications.html"}
	for _, page := range creatorPages {
		registerHTMLPage(r, "/creator/"+page, "creator/"+page)
	}

	adminPages := []string{"dashboard.html", "user_list.html", "task_list.html", "task_review.html", "appeal_list.html", "appeals.html", "users.html", "tasks.html", "finance.html", "database.html", "login.html", "works.html", "inspirations.html", "user_detail.html", "task_detail.html", "settings.html"}
	for _, page := range adminPages {
		registerHTMLPage(r, "/admin/"+page, "admin/"+page)
	}

	helpPages := []string{"index.html", "faq.html", "tutorial.html"}
	for _, page := range helpPages {
		registerHTMLPage(r, "/help/"+page, "help/"+page)
	}

	userPages := []string{"profile.html", "password.html"}
	for _, page := range userPages {
		registerHTMLPage(r, "/user/"+page, "user/"+page)
	}

	mobile := r.Group("/mobile")
	{
		mobile.GET("/", handler.MobileIndex)
		mobile.GET("/works", handler.MobileWorks)
		mobile.GET("/work/:id", handler.MobileWorkDetail)
		mobile.GET("/mine", middleware.MobilePageAuthMiddleware(), handler.MobileMine)
		mobile.GET("/task/:id", handler.MobileTaskDetail)
		mobile.GET("/task/:id/claims", middleware.MobilePageAuthMiddleware(), handler.MobileTaskClaims)
		mobile.GET("/login", func(c *gin.Context) {
			c.HTML(http.StatusOK, "mobile/login.html", gin.H{"Title": "登录"})
		})
		mobile.GET("/register", func(c *gin.Context) {
			c.HTML(http.StatusOK, "mobile/register.html", gin.H{"Title": "注册"})
		})
		mobile.GET("/wallet", middleware.MobilePageAuthMiddleware(), func(c *gin.Context) {
			c.HTML(http.StatusOK, "mobile/wallet.html", gin.H{"Title": "钱包", "ActiveTab": "mine"})
		})
		mobile.GET("/my-claims", middleware.MobilePageAuthMiddleware(), func(c *gin.Context) {
			c.HTML(http.StatusOK, "mobile/my_claims.html", gin.H{"Title": "我领取的任务", "ActiveTab": "mine"})
		})
		mobile.GET("/my-tasks", middleware.MobilePageAuthMiddleware(), func(c *gin.Context) {
			c.HTML(http.StatusOK, "mobile/my_tasks.html", gin.H{"Title": "我发布的任务", "ActiveTab": "mine"})
		})
		mobile.GET("/transactions", middleware.MobilePageAuthMiddleware(), func(c *gin.Context) {
			c.HTML(http.StatusOK, "mobile/transactions.html", gin.H{"Title": "收益明细", "ActiveTab": "mine"})
		})
		mobile.GET("/settings", middleware.MobilePageAuthMiddleware(), func(c *gin.Context) {
			c.HTML(http.StatusOK, "mobile/settings.html", gin.H{"Title": "设置", "ActiveTab": "mine"})
		})
	}

	v1 := r.Group("/api/v1")
	{
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", handler.Register)
			authGroup.POST("/login", handler.Login)
			authGroup.POST("/wechat-mini-login", handler.WechatMiniLogin)
			authGroup.GET("/csrf-token", middleware.CSRFTokenHandler)
		}

		v1.POST("/admin/login", handler.AdminLogin)
		v1.POST("/admin/register", handler.AdminRegister)

		v1.GET("/tasks", handler.ListAvailableTasks)
		v1.GET("/tasks/:id", handler.GetTask)
		v1.GET("/works", handler.ListApprovedWorks)
		v1.GET("/works/:id", handler.GetWork)
		v1.GET("/inspirations", handler.ListInspirations)
		v1.GET("/inspirations/:id", handler.GetInspiration)
		v1.POST("/upload", middleware.AuthMiddleware(), handler.UploadFile)

		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware())
		protected.Use(middleware.CSRFProtection())
		{
			protected.GET("/inspirations/:id/like-status", handler.GetInspirationLikeStatus)
			protected.POST("/inspirations/:id/like", handler.LikeInspiration)
			protected.DELETE("/inspirations/:id/like", handler.UnlikeInspiration)

			protected.GET("/works/:id/like-status", handler.GetWorkLikeStatus)
			protected.POST("/works/:id/like", handler.LikeWork)
			protected.DELETE("/works/:id/like", handler.UnlikeWork)

			userGroup := protected.Group("/users")
			{
				userGroup.GET("/me", handler.GetCurrentUser)
				userGroup.PUT("/me", handler.UpdateProfile)
				userGroup.GET("/credits", handler.GetUserCredits)
			}

			v1User := protected.Group("/user")
			{
				v1User.GET("/profile", handler.GetUserProfile)
				v1User.PUT("/profile", handler.UpdateUserProfile)
				v1User.PUT("/password", handler.ChangePassword)
			}

			notificationGroup := protected.Group("/notifications")
			{
				notificationGroup.GET("", handler.GetNotifications)
				notificationGroup.PUT("/:id/read", handler.MarkNotificationAsRead)
				notificationGroup.GET("/unread-count", handler.GetUnreadNotificationCount)
				notificationGroup.PUT("/read-all", handler.MarkAllNotificationsAsRead)
			}

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
				creatorGroup.POST("/withdraw", handler.Withdraw)
				creatorGroup.GET("/transactions", handler.GetTransactions)
				creatorGroup.GET("/stats", handler.GetCreatorStats)
				creatorGroup.GET("/chart/income", handler.GetCreatorIncomeChart)
			}

			protected.GET("/wallet", handler.GetWallet)

			businessGroup := protected.Group("/business")
			{
				businessGroup.POST("/tasks", handler.CreateTask)
				businessGroup.DELETE("/tasks/:id", handler.CancelTask)
				businessGroup.GET("/tasks", handler.ListMyTasks)
				businessGroup.GET("/inspirations", handler.ListBusinessInspirations)
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
				businessGroup.POST("/tasks/ai-write", handler.AIWriteTaskDescription)
			}

			appealGroup := protected.Group("/appeals")
			{
				appealGroup.POST("", handler.CreateAppeal)
				appealGroup.GET("", handler.ListAppeals)
				appealGroup.GET("/:id", handler.GetAppeal)
			}

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
				adminGroup.GET("/inspirations", handler.ListInspirationsAdmin)
				adminGroup.GET("/inspirations/:id", handler.GetInspirationAdmin)
				adminGroup.POST("/inspirations", handler.CreateInspirationAdmin)
				adminGroup.PUT("/inspirations/:id", handler.UpdateInspirationAdmin)
				adminGroup.DELETE("/inspirations/:id", handler.DeleteInspirationAdmin)
				adminGroup.GET("/appeals", handler.ListAppealsAdmin)
				adminGroup.GET("/appeals/:id", handler.GetAppealAdmin)
				adminGroup.PUT("/appeals/:id/handle", handler.HandleAppeal)
				adminGroup.GET("/tables", handler.ListTables)
				adminGroup.GET("/tables/:table/schema", handler.GetTableSchema)
				adminGroup.POST("/tables/:table", handler.InsertRecord)
				adminGroup.PUT("/tables/:table/:id", handler.UpdateRecord)
				adminGroup.DELETE("/tables/:table/:id", handler.DeleteRecord)
				adminGroup.POST("/query", handler.ExecuteQuery)
				adminGroup.GET("/finance/stats", handler.GetFinanceStats)
				adminGroup.GET("/finance/transactions", handler.ListFinanceTransactions)
				adminGroup.GET("/finance/transactions/:id", handler.GetFinanceTransactionDetail)
				adminGroup.GET("/settings", handler.GetSettings)
				adminGroup.PUT("/settings", handler.UpdateSettings)
			}
		}
	}

	return r
}

func corsMiddleware(cfg *config.Config) gin.HandlerFunc {
	allowedOrigins := cfg.Server.CORSAllowedOrigins
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowOrigin := "*"

		// 如果配置了允许的 origins，使用动态检查
		if allowedOrigins != "" {
			origins := strings.Split(allowedOrigins, ",")
			for _, allowed := range origins {
				allowed = strings.TrimSpace(allowed)
				if allowed == "*" || allowed == origin {
					allowOrigin = origin
					break
				}
			}
			// 如果没有匹配，且不是 *，则不设置 Allow-Origin
			if allowOrigin == "*" && allowedOrigins != "*" {
				// 不允许，但先设为 * ，后面再处理
			}
		}

		c.Header("Access-Control-Allow-Origin", allowOrigin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

		if c.Request.Method == http.MethodOptions {
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

func registerHTMLPage(r *gin.Engine, routePath, templateName string) {
	r.GET(routePath, func(c *gin.Context) {
		c.HTML(http.StatusOK, templateName, gin.H{})
	})
}

func getUserID(c *gin.Context) interface{} {
	if id, exists := c.Get("userID"); exists {
		return id
	}
	return nil
}
