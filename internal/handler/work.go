package handler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
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
		creator, _ := userRepo.GetUserByID(claim.CreatorID)
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
		task, _ := taskRepo.GetTaskByID(claim.TaskID)
		if task != nil {
			taskTitle = task.Title
			taskCategory = int(task.Category)
		}

		materials, _ := creatorRepo.GetClaimMaterials(claim.ID)
		if materials == nil {
			materials = []*model.ClaimMaterial{}
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
	creator, _ := userRepo.GetUserByID(claim.CreatorID)
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
	task, _ := taskRepo.GetTaskByID(claim.TaskID)
	if task != nil {
		taskTitle = task.Title
		taskCategory = int(task.Category)
	}

	materials, _ := creatorRepo.GetClaimMaterials(claim.ID)
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
