package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/model"
)

// Response represents the standard API response
type TaskResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// GetTask handles getting a single task by ID
// GET /api/v1/tasks/:id
func GetTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, TaskResponse{
			Code:    40001,
			Message: "无效的任务ID",
			Data:    nil,
		})
		return
	}

	task, err := GetTaskRepo().GetTaskByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, TaskResponse{
			Code:    50001,
			Message: "获取任务失败",
			Data:    nil,
		})
		return
	}

	if task == nil {
		c.JSON(http.StatusNotFound, TaskResponse{
			Code:    40401,
			Message: "任务不存在",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, TaskResponse{
		Code:    0,
		Message: "success",
		Data:    formatTask(task),
	})
}

// formatTask converts a Task model to a gin.H map
func formatTask(task *model.Task) gin.H {
	h := gin.H{
		"id":              task.ID,
		"business_id":     task.BusinessID,
		"title":           task.Title,
		"description":     task.Description,
		"category":        task.Category,
		"unit_price":      task.UnitPrice,
		"total_count":     task.TotalCount,
		"remaining_count": task.RemainingCount,
		"status":          task.Status,
		"total_budget":    task.TotalBudget,
		"frozen_amount":   task.FrozenAmount,
		"paid_amount":     task.PaidAmount,
		"created_at":      task.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":      task.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if task.PublishAt != nil {
		h["publish_at"] = task.PublishAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if task.EndAt != nil {
		h["end_at"] = task.EndAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if task.ReviewAt != nil {
		h["review_at"] = task.ReviewAt.Format("2006-01-02T15:04:05Z07:00")
	}

	// V1 fields
	if task.Industries != "" {
		h["industries"] = task.Industries
	}
	if task.VideoDuration != "" {
		h["video_duration"] = task.VideoDuration
	}
	if task.VideoAspect != "" {
		h["video_aspect"] = task.VideoAspect
	}
	if task.VideoResolution != "" {
		h["video_resolution"] = task.VideoResolution
	}
	if task.CreativeStyle != "" {
		h["creative_style"] = task.CreativeStyle
	}
	if task.AwardPrice > 0 {
		h["award_price"] = task.AwardPrice
	}
	if task.AwardCount > 0 {
		h["award_count"] = task.AwardCount
	}

	// Materials
	if len(task.Materials) > 0 {
		h["materials"] = task.Materials
	} else {
		h["materials"] = []model.TaskMaterial{}
	}

	return h
}

// formatTaskList converts a slice of Task models to the API list format.
// Parses industries from comma-separated string to []string, includes materials.
func formatTaskList(tasks []*model.Task) []gin.H {
	result := make([]gin.H, 0, len(tasks))
	for _, t := range tasks {
		h := formatTask(t)
		// Parse industries string to array
		if ind, ok := h["industries"].(string); ok && ind != "" {
			h["industries"] = strings.Split(ind, ",")
		} else {
			h["industries"] = []string{}
		}
		result = append(result, h)
	}
	return result
}
