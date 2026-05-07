package handler

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
	"github.com/tans/miao/internal/service"
	"github.com/tans/miao/internal/storage"
)

var (
	merchantAuthCreditCodeRegex = regexp.MustCompile(`^[0-9A-Z]{18}$`)
	merchantAuthPhoneRegex      = regexp.MustCompile(`^1[3-9]\d{9}$|^0\d{2,3}-?\d{7,8}$`)
)

func deriveMerchantAuthState(userVerified bool, app *model.MerchantAuthApplication) (string, int) {
	if userVerified {
		return "certified", 2
	}
	if app != nil {
		switch app.Status {
		case int(model.MerchantAuthStatusApproved):
			return "certified", 2
		case int(model.MerchantAuthStatusRejected):
			return "rejected", 3
		case int(model.MerchantAuthStatusPending):
			return "pending", 1
		}
	}
	return "uncertified", 0
}

func buildMerchantAuthResponse(user *model.User, app *model.MerchantAuthApplication) gin.H {
	status, statusCode := deriveMerchantAuthState(user != nil && user.BusinessVerified, app)
	resp := gin.H{
		"status":              status,
		"status_code":         statusCode,
		"business_verified":   user != nil && user.BusinessVerified,
		"company_name":        "",
		"credit_code":         "",
		"contact_name":        "",
		"contact_phone":       "",
		"license_url":         "",
		"license_preview_url": "",
		"review_comment":      "",
		"reviewed_at":         "",
		"created_at":          "",
		"updated_at":          "",
	}

	if app == nil {
		return resp
	}

	resp["company_name"] = app.CompanyName
	resp["credit_code"] = app.CreditCode
	resp["contact_name"] = app.ContactName
	resp["contact_phone"] = app.ContactPhone
	resp["license_url"] = app.LicenseURL
	resp["license_preview_url"] = resolveSignedStoredAssetURL(app.LicenseURL)
	resp["review_comment"] = app.ReviewComment
	if !app.ReviewedAt.IsZero() {
		resp["reviewed_at"] = app.ReviewedAt.Format(time.RFC3339)
	}
	if !app.CreatedAt.IsZero() {
		resp["created_at"] = app.CreatedAt.Format(time.RFC3339)
	}
	if !app.UpdatedAt.IsZero() {
		resp["updated_at"] = app.UpdatedAt.Format(time.RFC3339)
	}
	return resp
}

func formatMerchantAuthStatus(status int, reviewedAt time.Time) string {
	switch status {
	case int(model.MerchantAuthStatusApproved):
		return "已认证"
	case int(model.MerchantAuthStatusRejected):
		return "已拒绝"
	case int(model.MerchantAuthStatusPending):
		if reviewedAt.IsZero() {
			return "待审核"
		}
		return "审核中"
	default:
		return "未知"
	}
}

// ListMerchantAuthApplications returns merchant auth applications for admin.
// GET /api/v1/admin/merchant-auth
func ListMerchantAuthApplications(c *gin.Context) {
	_, ok := middleware.GetIsAdminFromContext(c)
	if !ok {
		c.JSON(http.StatusForbidden, Response{
			Code:    40301,
			Message: "需要管理员权限",
			Data:    nil,
		})
		return
	}

	page := parseInt(c.DefaultQuery("page", "1"), 1)
	if page < 1 {
		page = 1
	}
	pageSize := parseInt(c.DefaultQuery("page_size", "20"), 20)
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	search := strings.TrimSpace(c.Query("search"))
	var status *int
	switch strings.TrimSpace(c.Query("status")) {
	case "pending":
		val := int(model.MerchantAuthStatusPending)
		status = &val
	case "approved":
		val := int(model.MerchantAuthStatusApproved)
		status = &val
	case "rejected":
		val := int(model.MerchantAuthStatusRejected)
		status = &val
	}

	appRepo := repository.NewMerchantAuthRepository(GetDB())
	items, total, err := appRepo.ListApplications(search, status, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取商家认证列表失败: " + err.Error(),
			Data:    nil,
		})
		return
	}

	var formatted []gin.H
	now := time.Now()
	for _, item := range items {
		autoApproveAt := item.CreatedAt.Add(30 * time.Minute)
		minutesLeft := int(autoApproveAt.Sub(now).Minutes())
		if minutesLeft < 0 {
			minutesLeft = 0
		}
		reviewedAt := ""
		if !item.ReviewedAt.IsZero() {
			reviewedAt = item.ReviewedAt.Format(time.RFC3339)
		}
		formatted = append(formatted, gin.H{
			"id":                      item.ID,
			"user_id":                 item.UserID,
			"username":                item.Username,
			"phone":                   item.Phone,
			"company_name":            item.CompanyName,
			"credit_code":             item.CreditCode,
			"contact_name":            item.ContactName,
			"contact_phone":           item.ContactPhone,
			"license_url":             item.LicenseURL,
			"license_preview_url":     resolveSignedStoredAssetURL(item.LicenseURL),
			"status":                  item.Status,
			"status_text":             formatMerchantAuthStatus(item.Status, item.ReviewedAt),
			"review_comment":          item.ReviewComment,
			"reviewed_at":             reviewedAt,
			"created_at":              item.CreatedAt.Format(time.RFC3339),
			"updated_at":              item.UpdatedAt.Format(time.RFC3339),
			"business_verified":       item.BusinessVerified,
			"auto_approve_at":         autoApproveAt.Format(time.RFC3339),
			"auto_approve_in_minutes": minutesLeft,
		})
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"items":     formatted,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetMerchantAuthStatus returns the current merchant auth state.
// GET /api/v1/business/merchant/auth/status
func GetMerchantAuthStatus(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	db := GetDB()
	userRepo := repository.NewUserRepository(db)
	appRepo := repository.NewMerchantAuthRepository(db)

	user, err := userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}

	app, err := appRepo.GetByUserID(userID)
	if err != nil && err != repository.ErrNotFound {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取认证状态失败",
			Data:    nil,
		})
		return
	}
	if err == repository.ErrNotFound {
		app = nil
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    buildMerchantAuthResponse(user, app),
	})
}

// SubmitMerchantAuth submits or updates merchant auth application.
// POST /api/v1/business/merchant/auth
func SubmitMerchantAuth(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	var req struct {
		CompanyName             string `json:"company_name"`
		CreditCode              string `json:"credit_code"`
		SocialCreditCode        string `json:"social_credit_code"`
		UnifiedSocialCreditCode string `json:"unified_social_credit_code"`
		ContactName             string `json:"contact_name"`
		ContactPhone            string `json:"contact_phone"`
		LicenseURL              string `json:"license_url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	companyName := strings.TrimSpace(req.CompanyName)
	creditCode := strings.ToUpper(strings.TrimSpace(req.CreditCode))
	if creditCode == "" {
		creditCode = strings.ToUpper(strings.TrimSpace(req.SocialCreditCode))
	}
	if creditCode == "" {
		creditCode = strings.ToUpper(strings.TrimSpace(req.UnifiedSocialCreditCode))
	}
	contactName := strings.TrimSpace(req.ContactName)
	contactPhone := strings.TrimSpace(req.ContactPhone)
	licenseURL := strings.TrimSpace(req.LicenseURL)

	if companyName == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "请输入企业名称",
			Data:    nil,
		})
		return
	}
	if creditCode == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "请输入统一社会信用代码",
			Data:    nil,
		})
		return
	}
	if !merchantAuthCreditCodeRegex.MatchString(creditCode) {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "请输入正确的统一社会信用代码",
			Data:    nil,
		})
		return
	}
	if contactName == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "请输入联系人",
			Data:    nil,
		})
		return
	}
	if contactPhone == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "请输入联系电话",
			Data:    nil,
		})
		return
	}
	if !merchantAuthPhoneRegex.MatchString(contactPhone) {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "请输入正确的联系电话",
			Data:    nil,
		})
		return
	}
	if licenseURL == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "请上传营业执照",
			Data:    nil,
		})
		return
	}

	db := GetDB()
	userRepo := repository.NewUserRepository(db)
	appRepo := repository.NewMerchantAuthRepository(db)

	user, err := userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}

	if user.BusinessVerified {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "您已通过商家认证",
			Data:    nil,
		})
		return
	}

	app, err := appRepo.SaveSubmission(userID, &model.MerchantAuthApplication{
		CompanyName:  companyName,
		CreditCode:   creditCode,
		ContactName:  contactName,
		ContactPhone: contactPhone,
		LicenseURL:   licenseURL,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "提交认证失败",
			Data:    nil,
		})
		return
	}

	user.BusinessVerified = false
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "提交成功，等待审核",
		Data:    buildMerchantAuthResponse(user, app),
	})
}

// RecognizeMerchantAuthLicense extracts merchant auth fields from an uploaded license image.
// POST /api/v1/business/merchant/auth/ocr
func RecognizeMerchantAuthLicense(c *gin.Context) {
	_, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	var req struct {
		Key string `json:"key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	key := strings.TrimSpace(req.Key)
	if key == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "缺少上传文件标识",
			Data:    nil,
		})
		return
	}

	provider, err := GetStorageProvider()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "存储初始化失败: " + err.Error(),
			Data:    nil,
		})
		return
	}

	cfg := config.Load()
	imageURL, err := storage.GetDownloadURL(c.Request.Context(), provider, configuredStorageBucket(cfg), key, 2*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取营业执照地址失败: " + err.Error(),
			Data:    nil,
		})
		return
	}

	if merchantAuthOCRService == nil {
		c.JSON(http.StatusServiceUnavailable, Response{
			Code:    50301,
			Message: "OCR服务未初始化",
			Data:    nil,
		})
		return
	}

	result, err := merchantAuthOCRService.RecognizeBusinessLicense(c.Request.Context(), imageURL)
	if err != nil {
		statusCode := http.StatusInternalServerError
		message := "营业执照识别失败"
		switch {
		case errors.Is(err, service.ErrOCRNotConfigured), errors.Is(err, service.ErrOCRDisabled):
			statusCode = http.StatusServiceUnavailable
			message = "OCR服务未配置"
		case strings.Contains(strings.ToLower(err.Error()), "nopermission"), strings.Contains(strings.ToLower(err.Error()), "not authorized"):
			statusCode = http.StatusServiceUnavailable
			message = "OCR凭证无权限或服务未开通"
		case errors.Is(err, service.ErrOCRImageURLRequired):
			statusCode = http.StatusBadRequest
			message = "缺少营业执照图片地址"
		case errors.Is(err, service.ErrOCRNoUsefulData):
			statusCode = http.StatusUnprocessableEntity
			message = "未识别到营业执照信息"
		}
		c.JSON(statusCode, Response{
			Code:    statusCode*100 + 1,
			Message: message,
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"company_name": result.CompanyName,
			"credit_code":  result.CreditCode,
			"legal_person": result.LegalPerson,
			"key":          key,
		},
	})
}

// ReviewMerchantAuth reviews a merchant auth application.
// PUT /api/v1/admin/merchant-auth/:id/review
func ReviewMerchantAuth(c *gin.Context) {
	_, ok := middleware.GetIsAdminFromContext(c)
	if !ok {
		c.JSON(http.StatusForbidden, Response{
			Code:    40301,
			Message: "需要管理员权限",
			Data:    nil,
		})
		return
	}

	userID := parseInt64(c.Param("id"), 0)
	if userID == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "无效的用户ID",
			Data:    nil,
		})
		return
	}

	var req struct {
		Approved *bool  `json:"approved"`
		Comment  string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}
	if req.Approved == nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "参数错误: approved 不能为空",
			Data:    nil,
		})
		return
	}

	db := GetDB()
	userRepo := repository.NewUserRepository(db)
	appRepo := repository.NewMerchantAuthRepository(db)

	user, err := userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    40401,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	app, err := appRepo.Review(userID, *req.Approved, req.Comment)
	if err != nil {
		if err == repository.ErrNotFound {
			c.JSON(http.StatusNotFound, Response{
				Code:    40402,
				Message: "认证申请不存在",
				Data:    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "审核失败",
			Data:    nil,
		})
		return
	}

	if refreshedUser, err := userRepo.GetUserByID(userID); err == nil && refreshedUser != nil {
		user = refreshedUser
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "审核完成",
		Data:    buildMerchantAuthResponse(user, app),
	})
}

// UpdateMerchantAuthStatus updates merchant auth application status directly.
// PUT /api/v1/admin/merchant-auth/:id/status
func UpdateMerchantAuthStatus(c *gin.Context) {
	_, ok := middleware.GetIsAdminFromContext(c)
	if !ok {
		c.JSON(http.StatusForbidden, Response{Code: 40301, Message: "需要管理员权限", Data: nil})
		return
	}

	userID := parseInt64(c.Param("id"), 0)
	if userID == 0 {
		c.JSON(http.StatusBadRequest, Response{Code: 40001, Message: "无效的用户ID", Data: nil})
		return
	}

	var req struct {
		Status  int    `json:"status"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 40001, Message: "参数错误: " + err.Error(), Data: nil})
		return
	}

	if req.Status < int(model.MerchantAuthStatusPending) || req.Status > int(model.MerchantAuthStatusRejected) {
		c.JSON(http.StatusBadRequest, Response{Code: 40001, Message: "无效的状态值", Data: nil})
		return
	}

	db := GetDB()
	userRepo := repository.NewUserRepository(db)
	appRepo := repository.NewMerchantAuthRepository(db)

	user, err := userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, Response{Code: 40401, Message: "用户不存在", Data: nil})
		return
	}

	app, err := appRepo.UpdateStatus(userID, req.Status, req.Comment)
	if err != nil {
		if err == repository.ErrNotFound {
			c.JSON(http.StatusNotFound, Response{Code: 40402, Message: "认证申请不存在", Data: nil})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 50001, Message: "更新状态失败", Data: nil})
		return
	}

	if refreshedUser, err := userRepo.GetUserByID(userID); err == nil && refreshedUser != nil {
		user = refreshedUser
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "状态已更新",
		Data:    buildMerchantAuthResponse(user, app),
	})
}

// DeleteMerchantAuth deletes merchant auth application.
// DELETE /api/v1/admin/merchant-auth/:id
func DeleteMerchantAuth(c *gin.Context) {
	_, ok := middleware.GetIsAdminFromContext(c)
	if !ok {
		c.JSON(http.StatusForbidden, Response{Code: 40301, Message: "需要管理员权限", Data: nil})
		return
	}

	userID := parseInt64(c.Param("id"), 0)
	if userID == 0 {
		c.JSON(http.StatusBadRequest, Response{Code: 40001, Message: "无效的用户ID", Data: nil})
		return
	}

	appRepo := repository.NewMerchantAuthRepository(GetDB())
	if err := appRepo.DeleteByUserID(userID); err != nil {
		if err == repository.ErrNotFound {
			c.JSON(http.StatusNotFound, Response{Code: 40402, Message: "认证申请不存在", Data: nil})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 50001, Message: "删除认证失败", Data: nil})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "删除成功",
		Data:    nil,
	})
}
