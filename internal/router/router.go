package router

import (
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
	r.POST("/internal/video-processing/callback", handler.VideoProcessingCallback)

	// Serve docs directory
	docsDir := filepath.Join(getWorkDir(), "docs")
	if _, err := os.Stat(docsDir); err == nil {
		r.Static("/docs", docsDir)
	}

	v1 := r.Group("/api/v1")
	{
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", handler.Register)
			authGroup.POST("/login", handler.Login)
			authGroup.POST("/wechat-mini-login", handler.WechatMiniLogin)
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
		v1.GET("/cos/credential", middleware.AuthMiddleware(), handler.GetCOSCredential)

		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware())
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

			// Payment routes
			v1.POST("/payment/callback", handler.PaymentCallback)
			v1.POST("/account/recharge", middleware.AuthMiddleware(), handler.CreateRechargeOrder)
			v1.GET("/account/recharge/:order_no", middleware.AuthMiddleware(), handler.QueryRechargeOrder)

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
				adminGroup.GET("/users/:id/token", handler.GenerateUserToken)
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

		if allowedOrigins != "" {
			origins := strings.Split(allowedOrigins, ",")
			for _, allowed := range origins {
				allowed = strings.TrimSpace(allowed)
				if allowed == "*" || allowed == origin {
					allowOrigin = origin
					break
				}
			}
			if allowOrigin == "*" && allowedOrigins != "*" {
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

func getUserID(c *gin.Context) interface{} {
	if id, exists := c.Get("userID"); exists {
		return id
	}
	return nil
}
