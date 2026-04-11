package handler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/repository"
)

// ListApprovedWorks returns paginated list of approved submissions (works)
func ListApprovedWorks(c *gin.Context) {
	db := GetDB()
	submissionRepo := repository.NewSubmissionRepository(db)

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	sort := c.DefaultQuery("sort", "created_at")
	// Validate sort parameter
	allowedSorts := map[string]bool{
		"created_at": true,
		"likes":      true,
		"views":      true,
	}
	if !allowedSorts[sort] {
		sort = "created_at"
	}

	offset := (page - 1) * limit

	// Fetch approved submissions
	works, total, err := submissionRepo.ListApprovedSubmissions(limit, offset, sort)
	if err != nil {
		log.Printf("Failed to list approved works: %v", err)
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取作品列表失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
			"data":  works,
		},
	})
}

// GetWork returns a single approved submission (work) by ID
func GetWork(c *gin.Context) {
	db := GetDB()
	submissionRepo := repository.NewSubmissionRepository(db)

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的作品ID",
		})
		return
	}

	work, err := submissionRepo.GetApprovedWork(id)
	if err != nil {
		log.Printf("Failed to get work: %v", err)
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取作品详情失败",
		})
		return
	}

	if work == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "作品不存在或未通过审核",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    work,
	})
}
