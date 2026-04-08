package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/repository"
)

var messageRepo *repository.MessageRepository

func initMessageRepo() error {
	if err := initDB(); err != nil {
		return err
	}
	messageRepo = repository.NewMessageRepository(db)
	return nil
}

func init() {
	if err := initMessageRepo(); err != nil {
		panic("failed to initialize message repository: " + err.Error())
	}
}

// GetMessages 获取消息列表
// GET /api/v1/messages?page=1&limit=20
func GetMessages(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	messages, total, err := messageRepo.GetMessages(userID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取消息列表失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"messages": messages,
			"total":    total,
			"page":     page,
			"limit":    limit,
		},
	})
}

// GetMessageDetail 获取单条消息详情
// GET /api/v1/messages/:id
func GetMessageDetail(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	messageID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的消息ID",
			Data:    nil,
		})
		return
	}

	messages, _, err := messageRepo.GetMessages(userID, 1, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取消息详情失败",
			Data:    nil,
		})
		return
	}

	for _, msg := range messages {
		if msg.ID == messageID {
			c.JSON(http.StatusOK, Response{
				Code:    0,
				Message: "success",
				Data:    msg,
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, Response{
		Code:    40401,
		Message: "消息不存在",
		Data:    nil,
	})
}

// GetUnreadCount 获取未读消息数
// GET /api/v1/messages/unread-count
func GetUnreadCount(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	count, err := messageRepo.GetUnreadCount(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取未读消息数失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"count": count,
		},
	})
}

// MarkMessageAsRead 标记消息已读
// POST /api/v1/messages/:id/read
func MarkMessageAsRead(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	messageID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的消息ID",
			Data:    nil,
		})
		return
	}

	if err := messageRepo.MarkAsRead(messageID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "标记已读失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "已标记为已读",
		Data:    nil,
	})
}

// MarkAllAsRead 标记全部已读
// POST /api/v1/messages/read-all
func MarkAllAsRead(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	if err := messageRepo.MarkAllAsRead(userID); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "标记全部已读失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "已标记全部为已读",
		Data:    nil,
	})
}

// DeleteMessage 删除消息
// DELETE /api/v1/messages/:id
func DeleteMessage(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	messageID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的消息ID",
			Data:    nil,
		})
		return
	}

	if err := messageRepo.DeleteMessage(messageID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "删除消息失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "消息已删除",
		Data:    nil,
	})
}
