package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/repository"
)

var userRepo *repository.UserRepository

func initUserRepo() error {
	if err := initDB(); err != nil {
		return err
	}
	userRepo = repository.NewUserRepository(db)
	return nil
}

func init() {
	if err := initUserRepo(); err != nil {
		panic("failed to initialize user repository: " + err.Error())
	}
}

// GetUserProfile 获取个人资料
// GET /api/v1/user/profile
func GetUserProfile(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	user, err := userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"id":                user.ID,
			"username":          user.Username,
			"nickname":          user.Nickname,
			"phone":             user.Phone,
			"avatar":            user.Avatar,
			"role":              user.Role,
			"level":             user.Level,
			"balance":           user.Balance,
			"frozen_amount":     user.FrozenAmount,
			"behavior_score":    user.BehaviorScore,
			"trade_score":       user.TradeScore,
			"total_score":       user.TotalScore,
			"business_verified": user.BusinessVerified,
			"status":            user.Status,
			"created_at":        user.CreatedAt,
		},
	})
}

// UpdateUserProfile 更新个人资料
// PUT /api/v1/user/profile
func UpdateUserProfile(c *gin.Context) {
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
		Phone    string `json:"phone"`
		Avatar   string `json:"avatar"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// Update user profile
	if err := userRepo.UpdateProfile(userID, req.Nickname, req.Phone, req.Avatar); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "更新资料失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "资料已更新",
		Data:    nil,
	})
}

// ChangePassword 修改密码
// PUT /api/v1/user/password
func ChangePassword(c *gin.Context) {
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
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// Get user
	user, err := userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "原密码错误",
			Data:    nil,
		})
		return
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50002,
			Message: "密码加密失败",
			Data:    nil,
		})
		return
	}

	// Update password
	if err := userRepo.UpdatePassword(userID, string(hashedPassword)); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50003,
			Message: "修改密码失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "密码已修改",
		Data:    nil,
	})
}
