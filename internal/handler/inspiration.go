package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
)

func ListInspirations(c *gin.Context) {
	db := GetDB()
	repo := repository.NewInspirationRepository(db)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	items, total, err := repo.ListPublic(c.DefaultQuery("sort", "latest"), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "获取灵感列表失败"))
		return
	}

	for _, item := range items {
		materials, _ := repo.GetMaterials(item.ID)
		item.Materials = materials
		applyInspirationFallbacks(item)
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"total": total,
		"page":  page,
		"limit": limit,
		"data":  items,
	}))
}

func GetInspiration(c *gin.Context) {
	id, ok := parseInspirationID(c)
	if !ok {
		return
	}

	db := GetDB()
	repo := repository.NewInspirationRepository(db)
	item, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "获取灵感详情失败"))
		return
	}
	if item == nil || item.Status != model.InspirationStatusPublished {
		c.JSON(http.StatusNotFound, ErrorResponse(CodeNotFound, "灵感不存在"))
		return
	}

	materials, _ := repo.GetMaterials(item.ID)
	item.Materials = materials
	applyInspirationFallbacks(item)
	_ = repo.IncrementViews(item.ID)
	item.Views++

	c.JSON(http.StatusOK, SuccessResponse(item))
}

func GetInspirationLikeStatus(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse(CodeAuthRequired))
		return
	}
	id, ok := parseInspirationID(c)
	if !ok {
		return
	}

	db := GetDB()
	repo := repository.NewInspirationRepository(db)
	item, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "获取点赞状态失败"))
		return
	}
	if item == nil || item.Status != model.InspirationStatusPublished {
		c.JSON(http.StatusNotFound, ErrorResponse(CodeNotFound, "灵感不存在"))
		return
	}

	liked, err := repo.HasLiked(id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "获取点赞状态失败"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"id":       id,
		"is_liked": liked,
		"likes":    item.Likes,
	}))
}

func LikeInspiration(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse(CodeAuthRequired))
		return
	}
	id, ok := parseInspirationID(c)
	if !ok {
		return
	}

	db := GetDB()
	repo := repository.NewInspirationRepository(db)
	item, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "点赞失败"))
		return
	}
	if item == nil || item.Status != model.InspirationStatusPublished {
		c.JSON(http.StatusNotFound, ErrorResponse(CodeNotFound, "灵感不存在"))
		return
	}

	changed, err := repo.AddLike(id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "点赞失败"))
		return
	}
	if changed {
		item.Likes++
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"id":       id,
		"is_liked": true,
		"likes":    item.Likes,
	}))
}

func UnlikeInspiration(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse(CodeAuthRequired))
		return
	}
	id, ok := parseInspirationID(c)
	if !ok {
		return
	}

	db := GetDB()
	repo := repository.NewInspirationRepository(db)
	item, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "取消点赞失败"))
		return
	}
	if item == nil || item.Status != model.InspirationStatusPublished {
		c.JSON(http.StatusNotFound, ErrorResponse(CodeNotFound, "灵感不存在"))
		return
	}

	changed, err := repo.RemoveLike(id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "取消点赞失败"))
		return
	}
	if changed && item.Likes > 0 {
		item.Likes--
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"id":       id,
		"is_liked": false,
		"likes":    item.Likes,
	}))
}

func ListInspirationsAdmin(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse(CodeAuthRequired))
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "15"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 15
	}
	offset := (page - 1) * pageSize

	var status *int
	if statusStr := strings.TrimSpace(c.Query("status")); statusStr != "" {
		parsed, err := strconv.Atoi(statusStr)
		if err == nil {
			status = &parsed
		}
	}

	db := GetDB()
	repo := repository.NewInspirationRepository(db)
	items, total, err := repo.ListAdmin(strings.TrimSpace(c.Query("keyword")), status, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "获取灵感列表失败"))
		return
	}
	for _, item := range items {
		materials, _ := repo.GetMaterials(item.ID)
		item.Materials = materials
		applyInspirationFallbacks(item)
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}))
}

func GetInspirationAdmin(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse(CodeAuthRequired))
		return
	}
	id, ok := parseInspirationID(c)
	if !ok {
		return
	}

	db := GetDB()
	repo := repository.NewInspirationRepository(db)
	item, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "获取灵感详情失败"))
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, ErrorResponse(CodeNotFound, "灵感不存在"))
		return
	}
	materials, _ := repo.GetMaterials(item.ID)
	item.Materials = materials
	applyInspirationFallbacks(item)

	c.JSON(http.StatusOK, SuccessResponse(item))
}

func CreateInspirationAdmin(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse(CodeAuthRequired))
		return
	}

	var req model.InspirationCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "参数错误: "+err.Error()))
		return
	}
	if len(req.Materials) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "请至少上传一个素材"))
		return
	}

	item := &model.Inspiration{
		Title:         strings.TrimSpace(req.Title),
		Content:       strings.TrimSpace(req.Content),
		CreatorName:   strings.TrimSpace(req.CreatorName),
		CreatorAvatar: strings.TrimSpace(req.CreatorAvatar),
		CoverURL:      strings.TrimSpace(req.CoverURL),
		CoverType:     strings.TrimSpace(req.CoverType),
		SortOrder:     req.SortOrder,
		Status:        model.InspirationStatusPublished,
		CreatedBy:     userID,
	}
	applyMaterialDefaults(item, req.Materials)
	publishedAt := time.Now()
	item.PublishedAt = &publishedAt

	db := GetDB()
	repo := repository.NewInspirationRepository(db)
	if err := repo.Create(item); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "创建灵感失败"))
		return
	}
	if err := repo.ReplaceMaterials(item.ID, req.Materials); err != nil {
		_ = repo.Delete(item.ID)
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "保存灵感素材失败"))
		return
	}

	materials, _ := repo.GetMaterials(item.ID)
	item.Materials = materials
	applyInspirationFallbacks(item)
	c.JSON(http.StatusOK, SuccessResponse(item))
}

func UpdateInspirationAdmin(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse(CodeAuthRequired))
		return
	}
	id, ok := parseInspirationID(c)
	if !ok {
		return
	}

	var req model.InspirationUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "参数错误: "+err.Error()))
		return
	}
	if len(req.Materials) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "请至少保留一个素材"))
		return
	}

	db := GetDB()
	repo := repository.NewInspirationRepository(db)
	item, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "获取灵感失败"))
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, ErrorResponse(CodeNotFound, "灵感不存在"))
		return
	}

	item.Title = strings.TrimSpace(req.Title)
	item.Content = strings.TrimSpace(req.Content)
	item.CreatorName = strings.TrimSpace(req.CreatorName)
	item.CreatorAvatar = strings.TrimSpace(req.CreatorAvatar)
	item.CoverURL = strings.TrimSpace(req.CoverURL)
	item.CoverType = strings.TrimSpace(req.CoverType)
	item.SortOrder = req.SortOrder
	item.Status = req.Status
	applyMaterialDefaults(item, req.Materials)
	if item.Status == model.InspirationStatusPublished {
		now := time.Now()
		if item.PublishedAt == nil {
			item.PublishedAt = &now
		}
	} else {
		item.PublishedAt = nil
	}

	if err := repo.Update(item); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "更新灵感失败"))
		return
	}
	if err := repo.ReplaceMaterials(item.ID, req.Materials); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "更新灵感素材失败"))
		return
	}
	materials, _ := repo.GetMaterials(item.ID)
	item.Materials = materials
	applyInspirationFallbacks(item)
	c.JSON(http.StatusOK, SuccessResponse(item))
}

func DeleteInspirationAdmin(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse(CodeAuthRequired))
		return
	}
	id, ok := parseInspirationID(c)
	if !ok {
		return
	}

	db := GetDB()
	repo := repository.NewInspirationRepository(db)
	if err := repo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "删除灵感失败"))
		return
	}
	c.JSON(http.StatusOK, SuccessResponse(gin.H{"id": id}))
}

func parseInspirationID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "无效的灵感ID"))
		return 0, false
	}
	return id, true
}

func applyMaterialDefaults(item *model.Inspiration, materials []model.InspirationMaterialInput) {
	if item.CoverURL == "" && len(materials) > 0 {
		item.CoverURL = materials[0].ThumbnailPath
		if item.CoverURL == "" {
			item.CoverURL = materials[0].FilePath
		}
	}
	if item.CoverType == "" && len(materials) > 0 {
		item.CoverType = materials[0].FileType
	}
	if item.CreatorName == "" {
		item.CreatorName = "创意喵"
	}
}

func applyInspirationFallbacks(item *model.Inspiration) {
	if item.CoverURL == "" && len(item.Materials) > 0 {
		item.CoverURL = item.Materials[0].ThumbnailPath
		if item.CoverURL == "" {
			item.CoverURL = item.Materials[0].FilePath
		}
	}
	if item.CoverType == "" {
		if len(item.Materials) > 0 {
			item.CoverType = item.Materials[0].FileType
		} else {
			item.CoverType = "image"
		}
	}
	if item.CreatorName == "" {
		item.CreatorName = "创意喵"
	}
}
