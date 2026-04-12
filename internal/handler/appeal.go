package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
)

// AppealResponse represents the standard API response
type AppealResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

var appealRepo *repository.AppealRepository

func initAppealRepo() error {
	if err := initDB(); err != nil {
		return err
	}
	appealRepo = repository.NewAppealRepository(db)
	return nil
}

func init() {
	if err := initAppealRepo(); err != nil {
		panic("failed to initialize appeal repository: " + err.Error())
	}
}

// CreateAppeal handles creating a new appeal
// POST /api/v1/appeals
func CreateAppeal(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, AppealResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	var req model.CreateAppealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AppealResponse{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	appeal := &model.Appeal{
		UserID:    userID,
		Type:      model.AppealType(req.Type),
		TargetID:  req.TargetID,
		Reason:    req.Reason,
		Evidence:  req.Evidence,
		Status:    model.AppealStatusPending,
		CreatedAt: time.Now(),
	}

	if err := appealRepo.CreateAppeal(appeal); err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "创建申诉失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, AppealResponse{
		Code:    0,
		Message: "申诉已提交",
		Data: gin.H{
			"id":         appeal.ID,
			"type":       appeal.Type,
			"target_id":  appeal.TargetID,
			"reason":     appeal.Reason,
			"evidence":   appeal.Evidence,
			"status":     appeal.Status,
			"created_at": appeal.CreatedAt.Format(time.RFC3339),
		},
	})
}

// ListAppeals handles listing user's appeals
// GET /api/v1/appeals
func ListAppeals(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, AppealResponse{
			Code:    40101,
			Message: "未登录",
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

	appeals, total, err := appealRepo.ListAppealsByUserID(userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "获取申诉列表失败",
			Data:    nil,
		})
		return
	}

	var formattedAppeals []gin.H
	for _, appeal := range appeals {
		typeStr := "任务申诉"
		statusStr := "待处理"
		if appeal.Status == model.AppealStatusResolved {
			statusStr = "已处理"
		}
		formattedAppeals = append(formattedAppeals, gin.H{
			"id":         appeal.ID,
			"type":       appeal.Type,
			"type_str":   typeStr,
			"target_id":  appeal.TargetID,
			"reason":     appeal.Reason,
			"status":     appeal.Status,
			"status_str": statusStr,
			"result":     appeal.Result,
			"created_at": appeal.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, AppealResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"appeals": formattedAppeals,
			"total":   total,
		},
	})
}

// GetAppeal handles getting a single appeal
// GET /api/v1/appeals/:id
func GetAppeal(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, AppealResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, AppealResponse{
			Code:    40001,
			Message: "无效的申诉ID",
			Data:    nil,
		})
		return
	}

	appeal, err := appealRepo.GetAppealByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "获取申诉详情失败",
			Data:    nil,
		})
		return
	}

	if appeal == nil {
		c.JSON(http.StatusNotFound, AppealResponse{
			Code:    40401,
			Message: "申诉不存在",
			Data:    nil,
		})
		return
	}

	// Check ownership
	if appeal.UserID != userID {
		// Check if user is admin
		isAdmin, _ := middleware.GetIsAdminFromContext(c)
		if !isAdmin {
			c.JSON(http.StatusForbidden, AppealResponse{
				Code:    40301,
				Message: "无权查看此申诉",
				Data:    nil,
			})
			return
		}
	}

	typeStr := "任务申诉"
	statusStr := "待处理"
	if appeal.Status == model.AppealStatusResolved {
		statusStr = "已处理"
	}

	c.JSON(http.StatusOK, AppealResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"id":         appeal.ID,
			"user_id":    appeal.UserID,
			"type":       appeal.Type,
			"type_str":   typeStr,
			"target_id":  appeal.TargetID,
			"reason":     appeal.Reason,
			"status":     appeal.Status,
			"status_str": statusStr,
			"result":     appeal.Result,
			"created_at": appeal.CreatedAt.Format(time.RFC3339),
		},
	})
}

// ResolveAppeal handles resolving an appeal (admin only)
// PUT /api/v1/appeals/:id
func ResolveAppeal(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, AppealResponse{
			Code:    40001,
			Message: "无效的申诉ID",
			Data:    nil,
		})
		return
	}

	var req model.ResolveAppealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AppealResponse{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	appeal, err := appealRepo.GetAppealByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "获取申诉详情失败",
			Data:    nil,
		})
		return
	}

	if appeal == nil {
		c.JSON(http.StatusNotFound, AppealResponse{
			Code:    40401,
			Message: "申诉不存在",
			Data:    nil,
		})
		return
	}

	if err := appealRepo.UpdateAppealStatus(id, 2, req.Result); err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "处理申诉失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, AppealResponse{
		Code:    0,
		Message: "申诉已处理",
		Data: gin.H{
			"id":     appeal.ID,
			"status": 2,
			"result": req.Result,
		},
	})
}

// ListBusinessAppeals handles listing appeals for business
// GET /api/v1/business/appeals
func ListBusinessAppeals(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, AppealResponse{
			Code:    40101,
			Message: "未登录",
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

	// Get all task IDs owned by this business
	taskIDs, err := adminRepo.GetTaskIDsByBusinessID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "获取申诉列表失败",
			Data:    nil,
		})
		return
	}

	if len(taskIDs) == 0 {
		c.JSON(http.StatusOK, AppealResponse{
			Code:    0,
			Message: "success",
			Data: gin.H{
				"appeals": []interface{}{},
				"total":   0,
			},
		})
		return
	}

	// Get appeals for these tasks
	appeals, total, err := adminRepo.GetAppealsByTaskIDs(taskIDs, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "获取申诉列表失败",
			Data:    nil,
		})
		return
	}

	var formattedAppeals []gin.H
	for _, appeal := range appeals {
		typeStr := "任务申诉"
		statusStr := "待处理"
		if appeal.Status == model.AppealStatusResolved {
			statusStr = "已处理"
		}
		formattedAppeals = append(formattedAppeals, gin.H{
			"id":         appeal.ID,
			"user_id":    appeal.UserID,
			"type":       appeal.Type,
			"type_str":   typeStr,
			"target_id":  appeal.TargetID,
			"reason":     appeal.Reason,
			"evidence":   appeal.Evidence,
			"status":     appeal.Status,
			"status_str": statusStr,
			"result":     appeal.Result,
			"created_at": appeal.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, AppealResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"appeals": formattedAppeals,
			"total":   total,
		},
	})
}

// HandleBusinessAppeal handles resolving an appeal by business
// PUT /api/v1/business/appeals/:id/handle
func HandleBusinessAppeal(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, AppealResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, AppealResponse{
			Code:    40001,
			Message: "无效的申诉ID",
			Data:    nil,
		})
		return
	}

	// Verify ownership - check if the appeal's target belongs to this business
	appeal, err := appealRepo.GetAppealByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "获取申诉详情失败",
			Data:    nil,
		})
		return
	}

	if appeal == nil {
		c.JSON(http.StatusNotFound, AppealResponse{
			Code:    40401,
			Message: "申诉不存在",
			Data:    nil,
		})
		return
	}

	// For task appeals, verify business owns the task
	if appeal.Type == model.AppealTypeTask {
		task, _ := adminRepo.GetTaskByID(appeal.TargetID)
		if task == nil || task.BusinessID != userID {
			c.JSON(http.StatusForbidden, AppealResponse{
				Code:    40301,
				Message: "无权处理此申诉",
				Data:    nil,
			})
			return
		}
	}

	var req model.ResolveAppealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AppealResponse{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// Business resolves the appeal
	if err := adminRepo.UpdateAppealResult(id, req.Result); err != nil {
		c.JSON(http.StatusInternalServerError, AppealResponse{
			Code:    50001,
			Message: "处理申诉失败",
			Data:    nil,
		})
		return
	}

	// Send notification to user
	notificationService.NotifyAppealHandled(appeal.UserID, appeal.ID, req.Result)

	c.JSON(http.StatusOK, AppealResponse{
		Code:    0,
		Message: "申诉已处理",
		Data: gin.H{
			"id":     appeal.ID,
			"status": 2,
			"result": req.Result,
		},
	})
}
