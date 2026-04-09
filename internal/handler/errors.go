package handler

// 错误码定义
const (
	// 成功
	CodeSuccess = 0

	// 客户端错误 (40xxx)
	CodeBadRequest       = 40001 // 请求参数错误
	CodeInvalidOperation = 40002 // 无效操作
	CodeInvalidStatus    = 40003 // 状态错误

	// 认证错误 (401xx)
	CodeAuthRequired    = 40101 // 需要认证
	CodeInvalidToken    = 40102 // 无效的 token
	CodeTokenExpired    = 40103 // token 已过期
	CodeInvalidPassword = 40104 // 密码错误

	// 权限错误 (403xx)
	CodeForbidden           = 40301 // 无权限
	CodeInsufficientBalance = 40302 // 余额不足
	CodeTaskNotClaimable    = 40303 // 任务不可认领

	// 资源不存在 (404xx)
	CodeNotFound     = 40401 // 资源不存在
	CodeUserNotFound = 40402 // 用户不存在
	CodeTaskNotFound = 40403 // 任务不存在

	// 冲突错误 (409xx)
	CodeConflict       = 40901 // 资源冲突
	CodeUsernameExists = 40902 // 用户名已存在
	CodePhoneExists    = 40903 // 手机号已存在
	CodeAlreadyClaimed = 40904 // 已认领过该任务

	// 服务器错误 (50xxx)
	CodeInternalError = 50001 // 服务器内部错误
	CodeDatabaseError = 50002 // 数据库错误
	CodeServiceError  = 50003 // 服务错误
)

// 错误消息映射
var errorMessages = map[int]string{
	CodeSuccess: "成功",

	// 客户端错误
	CodeBadRequest:       "请求参数错误",
	CodeInvalidOperation: "无效操作",
	CodeInvalidStatus:    "状态错误",

	// 认证错误
	CodeAuthRequired:    "需要登录",
	CodeInvalidToken:    "登录已失效，请重新登录",
	CodeTokenExpired:    "登录已过期，请重新登录",
	CodeInvalidPassword: "密码错误",

	// 权限错误
	CodeForbidden:           "无权限访问",
	CodeInsufficientBalance: "余额不足",
	CodeTaskNotClaimable:    "任务不可认领",

	// 资源不存在
	CodeNotFound:     "资源不存在",
	CodeUserNotFound: "用户不存在",
	CodeTaskNotFound: "任务不存在",

	// 冲突错误
	CodeConflict:       "资源冲突",
	CodeUsernameExists: "用户名已被使用",
	CodePhoneExists:    "手机号已被注册",
	CodeAlreadyClaimed: "您已认领过该任务",

	// 服务器错误
	CodeInternalError: "服务器内部错误，请稍后重试",
	CodeDatabaseError: "数据库错误，请稍后重试",
	CodeServiceError:  "服务错误，请稍后重试",
}

// GetErrorMessage 获取错误消息
func GetErrorMessage(code int) string {
	if msg, ok := errorMessages[code]; ok {
		return msg
	}
	return "未知错误"
}

// ErrorResponse 返回错误响应
func ErrorResponse(code int, customMessage ...string) Response {
	message := GetErrorMessage(code)
	if len(customMessage) > 0 && customMessage[0] != "" {
		message = customMessage[0]
	}
	return Response{
		Code:    code,
		Message: message,
		Data:    nil,
	}
}

// SuccessResponse 返回成功响应
func SuccessResponse(data interface{}) Response {
	return Response{
		Code:    CodeSuccess,
		Message: "成功",
		Data:    data,
	}
}
