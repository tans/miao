package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/middleware"
	"github.com/tans/miao/internal/model"

	"github.com/tans/miao/internal/repository"
	"github.com/tans/miao/internal/service"
)

var authService *service.AuthService
var errorLog *log.Logger

func initAuthService() (*service.AuthService, error) {
	cfg := config.Load()
	db, err := database.InitDB(cfg.Database.Path)
	if err != nil {
		return nil, err
	}
	userRepo := repository.NewUserRepository(db)
	return service.NewAuthService(userRepo, cfg), nil
}

func getWorkDir() string {
	dir, _ := filepath.Abs(filepath.Dir("."))
	return dir
}

func init() {
	var err error
	authService, err = initAuthService()
	if err != nil {
		log.Fatalf("failed to initialize auth service: %v", err)
	}

	// Setup dedicated error log file
	logDir := filepath.Join(getWorkDir(), "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("Warning: failed to create logs dir: %v", err)
	}
	errorLogFile, err := os.OpenFile(filepath.Join(logDir, "error.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Warning: failed to open error.log: %v", err)
	} else {
		errorLog = log.New(errorLogFile, "", log.LstdFlags|log.Lshortfile)
	}
}

// Response represents the standard API response
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func buildAuthUserData(user *model.User) gin.H {
	return gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"phone":      user.Phone,
		"is_admin":   user.IsAdmin,
		"status":     user.Status,
		"balance":    user.Balance,
		"created_at": user.CreatedAt.Format(time.RFC3339),

		// Creator fields (all users)
		"level":              user.Level,
		"level_name":         user.GetLevelName(),
		"adopted_count":      user.AdoptedCount,
		"commission_rate":    user.GetCommission(),
		"daily_claim_count":  user.DailyClaimCount,
		"margin_frozen":      user.MarginFrozen,

		// Business fields (all users)
		"business_verified":  user.BusinessVerified,
		"publish_count":      user.PublishCount,
	}
}

// Register handles user registration
// POST /api/v1/auth/register
func Register(c *gin.Context) {
	var req struct {
		Username    string `json:"username" binding:"required,min=3,max=50"`
		Password    string `json:"password" binding:"required,min=6,max=50"`
		Phone       string `json:"phone" binding:"required"`
		IsAdmin     bool   `json:"is_admin"` // 是否为管理员
		RealName    string `json:"real_name"`
		CompanyName string `json:"company_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	if _, err := authService.Register(req.Username, req.Password, req.Phone, req.IsAdmin, req.RealName, req.CompanyName); err != nil {
		if err == service.ErrUserExists {
			c.JSON(http.StatusConflict, ErrorResponse(CodeUsernameExists))
			return
		}
		if err == service.ErrPhoneExists {
			c.JSON(http.StatusConflict, ErrorResponse(CodePhoneExists))
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "注册失败："+err.Error()))
		return
	}

	token, loginUser, err := authService.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "注册成功但自动登录失败："+err.Error()))
		return
	}

	// Generate CSRF token and set cookie for web clients
	csrfToken, _ := middleware.GenerateCSRFToken()
	if csrfToken != "" {
		middleware.SetCSRFCookie(c, csrfToken)
		c.Set("csrf_token", csrfToken)
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"token":      token,
		"user":       buildAuthUserData(loginUser),
		"csrf_token": csrfToken,
	}))
}

// Login handles user authentication
// POST /api/v1/auth/login
func Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	token, user, err := authService.Login(req.Username, req.Password)
	if err != nil {
		if err == service.ErrInvalidUsername {
			c.JSON(http.StatusNotFound, ErrorResponse(CodeUserNotFound, "用户名不存在"))
			return
		}
		if err == service.ErrInvalidPassword {
			c.JSON(http.StatusUnauthorized, ErrorResponse(CodeInvalidPassword, "密码错误"))
			return
		}
		if err == service.ErrUserDisabled {
			c.JSON(http.StatusForbidden, ErrorResponse(CodeForbidden, "账户已被禁用"))
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "登录失败："+err.Error()))
		return
	}

	// Generate CSRF token and set cookie for web clients
	csrfToken, err := middleware.GenerateCSRFToken()
	if err == nil {
		middleware.SetCSRFCookie(c, csrfToken)
		c.Set("csrf_token", csrfToken)
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"token":      token,
		"user":       buildAuthUserData(user),
		"csrf_token": csrfToken,
	}))
}

// WechatMiniLogin handles Wechat Mini Program login
// POST /api/v1/auth/wechat-mini-login
func WechatMiniLogin(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"` // 小程序wx.login返回的code
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	cfg := config.Load()

	// 调用微信接口用code换openid
	openid, err := getWechatOpenID(req.Code, cfg.WechatMini.AppID, cfg.WechatMini.AppSecret)
	if err != nil {
		errorLog.Printf("[wechat-mini-login] code=%s appid=%s err=%v | %s %s | client_ip=%s",
			req.Code, cfg.WechatMini.AppID, err, c.Request.Method, c.Request.URL.Path, c.ClientIP())
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "微信登录失败："+err.Error()))
		return
	}

	// 查找是否已存在该openid的用户
	user, err := authService.GetUserByWechatOpenID(openid)
	if err != nil {
		errorLog.Printf("[wechat-mini-login] openid=%s err=%v | %s %s | client_ip=%s",
			openid[:16], err, c.Request.Method, c.Request.URL.Path, c.ClientIP())
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "查询用户失败："+err.Error()))
		return
	}

	// 安全截取 openid，避免短 openid 导致 panic
	safeOpenid := openid
	if len(safeOpenid) > 16 {
		safeOpenid = safeOpenid[:16]
	}
	safeOpenid8 := safeOpenid
	if len(safeOpenid8) > 8 {
		safeOpenid8 = safeOpenid8[:8]
	}

	// 已存在用户，直接登录
	if user != nil {
		if user.Status == 0 {
			c.JSON(http.StatusForbidden, ErrorResponse(CodeForbidden, "账户已被禁用"))
			return
		}
		token, err := middleware.GenerateToken(user.ID, user.Username, user.IsAdmin)
		if err != nil {
			errorLog.Printf("[wechat-mini-login] user_id=%d openid=%s err=generate_token_failed | %s %s | client_ip=%s",
				user.ID, safeOpenid, c.Request.Method, c.Request.URL.Path, c.ClientIP())
			c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "生成令牌失败"))
			return
		}
		// Generate CSRF token and set cookie for web clients
		csrfToken, _ := middleware.GenerateCSRFToken()
		if csrfToken != "" {
			middleware.SetCSRFCookie(c, csrfToken)
			c.Set("csrf_token", csrfToken)
		}
		c.JSON(http.StatusOK, SuccessResponse(gin.H{
			"token":      token,
			"user":       buildAuthUserData(user),
			"is_new":     false,
			"csrf_token": csrfToken,
		}))
		return
	}

	// 新用户：自动创建账户（用户名基于openid，密码随机）
	username := fmt.Sprintf("wechat_%s", safeOpenid)
	password := fmt.Sprintf("%d", time.Now().UnixNano())

	// 检查用户名是否已存在
	exists, err := authService.UserRepo.ExistsByUsername(username)
	if err == nil && exists {
		// openid已绑定但用户名被占用，尝试登录
		c.JSON(http.StatusConflict, ErrorResponse(CodeUsernameExists, "微信账户已存在但无法自动登录"))
		return
	}

	// 创建新用户（无手机号）
	newUser, err := authService.Register(username, password, "", false, "", "")
	if err != nil {
		// 可能用户名冲突，生成唯一用户名
		username = fmt.Sprintf("wechat_%s_%d", safeOpenid8, time.Now().UnixNano()%100000)
		newUser, err = authService.Register(username, password, "", false, "", "")
		if err != nil {
			errorLog.Printf("[wechat-mini-login] user_id=%d openid=%s username=%s err=register_failed %v | %s %s | client_ip=%s",
				newUser.ID, safeOpenid, username, err, c.Request.Method, c.Request.URL.Path, c.ClientIP())
			c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "创建微信账户失败："+err.Error()))
			return
		}
	}

	// 更新openid
	err = authService.UserRepo.UpdateWechatOpenID(newUser.ID, openid)
	if err != nil {
		errorLog.Printf("[wechat-mini-login] user_id=%d openid=%s err=update_openid %v | %s %s | client_ip=%s",
			newUser.ID, safeOpenid, err, c.Request.Method, c.Request.URL.Path, c.ClientIP())
	}

	token, err := middleware.GenerateToken(newUser.ID, newUser.Username, newUser.IsAdmin)
	if err != nil {
		errorLog.Printf("[wechat-mini-login] user_id=%d openid=%s err=generate_token_failed %v | %s %s | client_ip=%s",
			newUser.ID, safeOpenid, err, c.Request.Method, c.Request.URL.Path, c.ClientIP())
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "生成令牌失败"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"token": token,
		"user":  buildAuthUserData(newUser),
		"is_new": true,
	}))
}

// getWechatOpenID 通过小程序code获取openid
func getWechatOpenID(code, appID, appSecret string) (string, error) {
	if appID == "" || appSecret == "" {
		// 测试模式：直接返回模拟openid
		return fmt.Sprintf("test_openid_%s", code), nil
	}

	url := fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		appID, appSecret, code)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("请求微信接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if errMsg, ok := result["errmsg"]; ok {
		return "", fmt.Errorf("微信错误: %v", errMsg)
	}

	openid, ok := result["openid"].(string)
	if !ok {
		return "", fmt.Errorf("未获取到openid")
	}

	return openid, nil
}

// GetCurrentUser returns the current authenticated user
// GET /api/v1/users/me
func GetCurrentUser(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	user, err := authService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取用户信息失败",
			Data:    nil,
		})
		return
	}

	// Build response - all users have both business and creator capabilities
	userData := gin.H{
		"id":                  user.ID,
		"username":            user.Username,
		"nickname":            user.Nickname,
		"phone":               user.Phone,
		"avatar":              user.Avatar,
		"is_admin":            user.IsAdmin,
		"status":              user.Status,
		"created_at":          user.CreatedAt.Format(time.RFC3339),
		"role":                "creator", // 所有用户都有创作者能力
		"level":               user.Level,
		"level_name":          user.GetLevelName(),
		"adopted_count":       user.AdoptedCount,
		"daily_claim_count":   user.DailyClaimCount,
		"margin_frozen":       user.MarginFrozen,
		"daily_limit":         user.GetDailyLimit(),
		"business_verified":    user.BusinessVerified,
		"publish_count":       user.PublishCount,
		"report_count":        user.ReportCount,
		"real_name_verified":   user.RealNameVerified,
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    userData,
	})
}

// UpdateProfile updates the current user's profile
// PUT /api/v1/users/me
func UpdateProfile(c *gin.Context) {
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
		Nickname string `json:"nickname"`
		Avatar   string `json:"avatar"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	user, err := authService.UpdateProfile(userID, req.Nickname, req.Avatar)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "更新失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "更新成功",
		Data: gin.H{
			"id":       user.ID,
			"username": user.Username,
			"phone":    user.Phone,
			"is_admin": user.IsAdmin,
			"status":   user.Status,
			"nickname": user.Nickname,
			"avatar":   user.Avatar,
		},
	})
}

// Helper function to convert string to int64
func parseInt64(s string, defaultVal int64) int64 {
	if val, err := strconv.ParseInt(s, 10, 64); err == nil {
		return val
	}
	return defaultVal
}
