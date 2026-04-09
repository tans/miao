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
