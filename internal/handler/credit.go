package handler

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
)

// CreditResponse represents the standard API response
type CreditResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

var creditRepo *repository.CreditRepository

func initCreditRepo() error {
	if err := initDB(); err != nil {
		return err
	}
	creditRepo = repository.NewCreditRepository(db)
	return nil
}

func init() {
	if err := initCreditRepo(); err != nil {
		log.Fatalf("failed to initialize credit repository: %v", err)
	}
}

// GetUserCredits handles credit score and records query
// GET /api/v1/users/credits
func GetUserCredits(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, CreditResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	// Get user's credit score from user record
	user, err := getUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CreditResponse{
			Code:    50001,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, CreditResponse{
			Code:    40401,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	// Parse pagination
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 20
	}
	offset, _ := strconv.Atoi(offsetStr)
	if offset < 0 {
		offset = 0
	}

	// Get credit logs
	records, total, err := creditRepo.GetCreditLogsByUserID(userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CreditResponse{
			Code:    50001,
			Message: "获取信用记录失败",
			Data:    nil,
		})
		return
	}

	var formattedRecords []gin.H
	for _, record := range records {
		typeStr := "奖励"
		if record.Type == model.CreditLogTypePunish {
			typeStr = "处罚"
		}
		formattedRecords = append(formattedRecords, gin.H{
			"id":         record.ID,
			"type":       record.Type,
			"type_str":   typeStr,
			"change":     record.Change,
			"reason":     record.Reason,
			"created_at": record.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, CreditResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"level":           user.GetEffectiveLevel(),
			"level_name":      user.GetLevelName(),
			"adopted_count":   user.AdoptedCount,
			"commission_rate": user.GetCommission(),
			"daily_limit":     user.GetDailyLimit(),
			"records":         formattedRecords,
			"total":           total,
		},
	})
}

// getUserByID is a helper to get user from repository
func getUserByID(userID int64) (*model.User, error) {
	userRepo := repository.NewUserRepository(db)
	return userRepo.GetUserByID(userID)
}
