package handler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
)

// ListApprovedWorks returns paginated list of approved claims (status=3) as works
// for the mini-program inspiration feed.
func ListApprovedWorks(c *gin.Context) {
	db := GetDB()
	creatorRepo := repository.NewCreatorRepository(db)
	userRepo := repository.NewUserRepository(db)
	taskRepo := repository.NewTaskRepository(db)

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		log.Printf("Invalid page parameter: %v", err)
	}
	if page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil {
		log.Printf("Invalid limit parameter: %v", err)
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	claims, total, err := creatorRepo.ListClaimsByStatus(model.ClaimStatusApproved, limit, offset)
	if err != nil {
		log.Printf("Failed to list approved works: %v", err)
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取作品列表失败",
		})
		return
	}

	works := make([]gin.H, 0, len(claims))
	for _, claim := range claims {
		// Get creator info
		creator, err := userRepo.GetUserByID(claim.CreatorID)
		if err != nil {
			log.Printf("Failed to get creator %d: %v", claim.CreatorID, err)
		}
		creatorName := ""
		creatorAvatar := ""
		if creator != nil {
			if creator.Nickname != "" {
				creatorName = creator.Nickname
			} else {
				creatorName = creator.Username
			}
			creatorAvatar = creator.Avatar
		}

		// Get task info
		var taskTitle string
		var taskCategory int
		task, err := taskRepo.GetTaskByID(claim.TaskID)
		if err != nil {
			log.Printf("Failed to get task %d: %v", claim.TaskID, err)
		}
		if task != nil {
			taskTitle = task.Title
			taskCategory = int(task.Category)
		}

		materials, err := creatorRepo.GetClaimMaterials(claim.ID)
		if err != nil {
			log.Printf("Failed to get materials for claim %d: %v", claim.ID, err)
		}
		if materials == nil {
			materials = []*model.ClaimMaterial{}
		}

		// Use first material's file_path as cover_url for the mini-program
		coverURL := ""
		if len(materials) > 0 {
			coverURL = materials[0].FilePath
		}

		works = append(works, gin.H{
			"id":             claim.ID,
			"task_id":        claim.TaskID,
			"task_title":     taskTitle,
			"task_category":  taskCategory,
			"creator_id":     claim.CreatorID,
			"creator_name":   creatorName,
			"creator_avatar": creatorAvatar,
			"content":        claim.Content,
			"reward":         claim.CreatorReward,
			"cover_url":      coverURL,
			"submit_at":      claim.SubmitAt,
			"review_at":      claim.ReviewAt,
			"materials":      materials,
		})
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

// GetWork returns a single approved claim (work) by claim ID.
func GetWork(c *gin.Context) {
	db := GetDB()
	creatorRepo := repository.NewCreatorRepository(db)
	userRepo := repository.NewUserRepository(db)
	taskRepo := repository.NewTaskRepository(db)

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的作品ID",
		})
		return
	}

	claim, err := creatorRepo.GetClaimByID(id)
	if err != nil || claim == nil || claim.Status != model.ClaimStatusApproved {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "作品不存在或未通过审核",
		})
		return
	}

	// Creator info
	creatorName := ""
	creatorAvatar := ""
	creator, err := userRepo.GetUserByID(claim.CreatorID)
	if err != nil {
		log.Printf("Failed to get creator %d: %v", claim.CreatorID, err)
	}
	if creator != nil {
		if creator.Nickname != "" {
			creatorName = creator.Nickname
		} else {
			creatorName = creator.Username
		}
		creatorAvatar = creator.Avatar
	}

	// Task info
	var taskTitle string
	var taskCategory int
	task, err := taskRepo.GetTaskByID(claim.TaskID)
	if err != nil {
		log.Printf("Failed to get task %d: %v", claim.TaskID, err)
	}
	if task != nil {
		taskTitle = task.Title
		taskCategory = int(task.Category)
	}

	materials, err := creatorRepo.GetClaimMaterials(claim.ID)
	if err != nil {
		log.Printf("Failed to get materials for claim %d: %v", claim.ID, err)
	}
	if materials == nil {
		materials = []*model.ClaimMaterial{}
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"id":             claim.ID,
			"task_id":        claim.TaskID,
			"task_title":     taskTitle,
			"task_category":  taskCategory,
			"creator_id":     claim.CreatorID,
			"creator_name":   creatorName,
			"creator_avatar": creatorAvatar,
			"content":        claim.Content,
			"reward":         claim.CreatorReward,
			"submit_at":      claim.SubmitAt,
			"review_at":      claim.ReviewAt,
			"materials":      materials,
		},
	})
}

// GetWorkLikeStatus 获取作品点赞状态
func GetWorkLikeStatus(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse(CodeAuthRequired))
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "无效的作品ID"))
		return
	}

	db := GetDB()
	creatorRepo := repository.NewCreatorRepository(db)

	claim, err := creatorRepo.GetClaimByID(id)
	if err != nil || claim == nil || claim.Status != model.ClaimStatusApproved {
		c.JSON(http.StatusNotFound, ErrorResponse(CodeNotFound, "作品不存在"))
		return
	}

	liked, err := creatorRepo.HasWorkLiked(id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "获取点赞状态失败"))
		return
	}

	likeCount, _ := creatorRepo.GetWorkLikeCount(id)

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"id":       id,
		"is_liked": liked,
		"likes":    likeCount,
	}))
}

// LikeWork 点赞作品
func LikeWork(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse(CodeAuthRequired))
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "无效的作品ID"))
		return
	}

	db := GetDB()
	creatorRepo := repository.NewCreatorRepository(db)

	claim, err := creatorRepo.GetClaimByID(id)
	if err != nil || claim == nil || claim.Status != model.ClaimStatusApproved {
		c.JSON(http.StatusNotFound, ErrorResponse(CodeNotFound, "作品不存在"))
		return
	}

	_, err = creatorRepo.AddWorkLike(id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "点赞失败"))
		return
	}

	likeCount, _ := creatorRepo.GetWorkLikeCount(id)

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"id":       id,
		"is_liked": true,
		"likes":    likeCount,
	}))
}

// UnlikeWork 取消点赞作品
func UnlikeWork(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse(CodeAuthRequired))
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "无效的作品ID"))
		return
	}

	db := GetDB()
	creatorRepo := repository.NewCreatorRepository(db)

	claim, err := creatorRepo.GetClaimByID(id)
	if err != nil || claim == nil || claim.Status != model.ClaimStatusApproved {
		c.JSON(http.StatusNotFound, ErrorResponse(CodeNotFound, "作品不存在"))
		return
	}

	_, err = creatorRepo.RemoveWorkLike(id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "取消点赞失败"))
		return
	}

	likeCount, _ := creatorRepo.GetWorkLikeCount(id)

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"id":       id,
		"is_liked": false,
		"likes":    likeCount,
	}))
}
