package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
	"github.com/tans/miao/internal/service"
)

var notificationRepo *repository.NotificationRepository
var notificationSvc *service.NotificationService

func initNotificationRepo() error {
	if err := initDB(); err != nil {
		return err
	}
	notificationRepo = repository.NewNotificationRepository(db)
	notificationSvc = service.NewNotificationServiceWithNotification(db)
	return nil
}

func init() {
	if err := initNotificationRepo(); err != nil {
		panic("failed to initialize notification repository: " + err.Error())
	}
}

// NotificationResponse represents the standard API response
type NotificationResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// GetNotifications 获取通知列表
// GET /api/v1/notifications
func GetNotifications(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, NotificationResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	notifType := c.DefaultQuery("type", "")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var notifications []model.Notification
	var total int64
	var err error

	if notifType != "" {
		notifications, total, err = notificationSvc.GetNotificationsByType(uint(userID), notifType, page, limit)
	} else {
		notifications, total, err = notificationSvc.GetNotifications(uint(userID), page, limit)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, NotificationResponse{
			Code:    50001,
			Message: "获取通知列表失败",
			Data:    nil,
		})
		return
	}

	var formattedNotifications []gin.H
	for _, n := range notifications {
		formattedNotifications = append(formattedNotifications, gin.H{
			"id":         n.ID,
			"user_id":    n.UserID,
			"type":       n.Type,
			"type_str":   n.GetTypeStr(),
			"title":      n.Title,
			"content":    n.Content,
			"is_read":    n.IsRead,
			"related_id": n.RelatedID,
			"created_at": n.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, NotificationResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"notifications": formattedNotifications,
			"total":         total,
			"page":          page,
			"limit":         limit,
		},
	})
}

// MarkNotificationAsRead 标记通知已读
// PUT /api/v1/notifications/:id/read
func MarkNotificationAsRead(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, NotificationResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, NotificationResponse{
			Code:    40001,
			Message: "无效的通知ID",
			Data:    nil,
		})
		return
	}

	// Verify the notification belongs to the user
	notif, err := notificationRepo.GetNotificationByID(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, NotificationResponse{
			Code:    50001,
			Message: "获取通知失败",
			Data:    nil,
		})
		return
	}

	if notif == nil {
		c.JSON(http.StatusNotFound, NotificationResponse{
			Code:    40401,
			Message: "通知不存在",
			Data:    nil,
		})
		return
	}

	if notif.UserID != uint(userID) {
		c.JSON(http.StatusForbidden, NotificationResponse{
			Code:    40301,
			Message: "无权操作此通知",
			Data:    nil,
		})
		return
	}

	if err := notificationSvc.MarkAsRead(uint(id), uint(userID)); err != nil {
		c.JSON(http.StatusInternalServerError, NotificationResponse{
			Code:    50001,
			Message: "标记已读失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, NotificationResponse{
		Code:    0,
		Message: "已标记为已读",
		Data:    nil,
	})
}

// GetUnreadNotificationCount 获取未读通知数量
// GET /api/v1/notifications/unread-count
func GetUnreadNotificationCount(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, NotificationResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	count, err := notificationSvc.GetUnreadCount(uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, NotificationResponse{
			Code:    50001,
			Message: "获取未读数量失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, NotificationResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"count": count,
		},
	})
}

// MarkAllNotificationsAsRead 标记全部已读
// PUT /api/v1/notifications/read-all
func MarkAllNotificationsAsRead(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, NotificationResponse{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	if err := notificationRepo.MarkAllAsRead(uint(userID)); err != nil {
		c.JSON(http.StatusInternalServerError, NotificationResponse{
			Code:    50001,
			Message: "标记全部已读失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, NotificationResponse{
		Code:    0,
		Message: "已标记全部为已读",
		Data:    nil,
	})
}
