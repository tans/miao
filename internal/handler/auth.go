package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/middleware"

	"github.com/tans/miao/internal/repository"
	"github.com/tans/miao/internal/service"
)

var authService *service.AuthService

func initAuthService() (*service.AuthService, error) {
	cfg := config.Load()
	db, err := database.InitDB(cfg.Database.Path)
	if err != nil {
		return nil, err
	}
	userRepo := repository.NewUserRepository(db)
	return service.NewAuthService(userRepo, cfg), nil
}

func init() {
	var err error
	authService, err = initAuthService()
	if err != nil {
		panic("failed to initialize auth service: " + err.Error())
	}
}

// Response represents the standard API response
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// Register handles user registration
// POST /api/v1/auth/register
func Register(c *gin.Context) {
	var req struct {
		Username    string `json:"username" binding:"required,min=3,max=50"`
		Password    string `json:"password" binding:"required,min=6,max=50"`
		Phone       string `json:"phone" binding:"required"`
		IsAdmin     bool   `json:"is_admin"` // 是否为管理员
		RealName    string `json:"real_name"`
		CompanyName string `json:"company_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	user, err := authService.Register(req.Username, req.Password, req.Phone, req.IsAdmin, req.RealName, req.CompanyName)
	if err != nil {
		if err == service.ErrUserExists {
			c.JSON(http.StatusConflict, ErrorResponse(CodeUsernameExists))
			return
		}
		if err == service.ErrPhoneExists {
			c.JSON(http.StatusConflict, ErrorResponse(CodePhoneExists))
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "注册失败："+err.Error()))
		return
	}

	// Build response - all users have both business and creator capabilities
	userData := gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"phone":      user.Phone,
		"is_admin":   user.IsAdmin,
		"created_at": user.CreatedAt.Format(time.RFC3339),

		// Creator fields (all users)
		"level":             user.Level,
		"level_name":        user.GetLevelName(),
		"total_score":       user.TotalScore,
		"daily_claim_count": user.DailyClaimCount,

		// Business fields (all users)
		"business_verified": user.BusinessVerified,
		"publish_count":     user.PublishCount,
	}

	c.JSON(http.StatusOK, SuccessResponse(userData))
}

// Login handles user authentication
// POST /api/v1/auth/login
func Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	token, user, err := authService.Login(req.Username, req.Password)
	if err != nil {
		if err == service.ErrInvalidUsername {
			c.JSON(http.StatusUnauthorized, ErrorResponse(CodeInvalidPassword, "用户名或密码错误"))
			return
		}
		if err == service.ErrInvalidPassword {
			c.JSON(http.StatusUnauthorized, ErrorResponse(CodeInvalidPassword))
			return
		}
		if err == service.ErrUserDisabled {
			c.JSON(http.StatusForbidden, ErrorResponse(CodeForbidden, "账户已被禁用"))
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "登录失败："+err.Error()))
		return
	}

	// Build response - all users have both business and creator capabilities
	userData := gin.H{
		"id":           user.ID,
		"username":     user.Username,
		"phone":        user.Phone,
		"is_admin":     user.IsAdmin,
		"status":       user.Status,
		"balance":      user.Balance,
		"created_at":   user.CreatedAt.Format(time.RFC3339),

		// Creator fields (all users)
		"level":             user.Level,
		"level_name":        user.GetLevelName(),
		"total_score":       user.TotalScore,
		"behavior_score":    user.BehaviorScore,
		"trade_score":       user.TradeScore,
		"daily_claim_count": user.DailyClaimCount,
		"margin_frozen":     user.MarginFrozen,

		// Business fields (all users)
		"business_verified": user.BusinessVerified,
		"publish_count":     user.PublishCount,
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"token": token,
		"user":  userData,
	}))
}

// GetCurrentUser returns the current authenticated user
// GET /api/v1/users/me
func GetCurrentUser(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	user, err := authService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}

	// Build response - all users have both business and creator capabilities
	userData := gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"phone":      user.Phone,
		"is_admin":   user.IsAdmin,
		"status":     user.Status,
		"created_at": user.CreatedAt.Format(time.RFC3339),
		"level":      user.Level,
		"level_name": user.GetLevelName(),
		"total_score": user.TotalScore,
		"behavior_score": user.BehaviorScore,
		"trade_score": user.TradeScore,
		"daily_claim_count": user.DailyClaimCount,
		"margin_frozen": user.MarginFrozen,
		"daily_limit": user.GetDailyLimit(),
		"business_verified": user.BusinessVerified,
		"publish_count": user.PublishCount,
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    userData,
	})
}

// UpdateProfile updates the current user's profile
// PUT /api/v1/users/me
func UpdateProfile(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	var req struct {
		Nickname string `json:"nickname"`
		Avatar   string `json:"avatar"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	user, err := authService.UpdateProfile(userID, req.Nickname, req.Avatar)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "更新失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "更新成功",
		Data: gin.H{
			"id":       user.ID,
			"username": user.Username,
			"phone":    user.Phone,
			"is_admin": user.IsAdmin,
			"status":   user.Status,
			"nickname": user.Nickname,
			"avatar":   user.Avatar,
		},
	})
}

// Helper function to convert string to int64
func parseInt64(s string, defaultVal int64) int64 {
	if val, err := strconv.ParseInt(s, 10, 64); err == nil {
		return val
	}
	return defaultVal
}
