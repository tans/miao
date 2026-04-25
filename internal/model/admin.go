package model

import "time"

// AdminStats 平台统计数据
type AdminStats struct {
	TotalUsers        int     `json:"total_users"`
	TotalAdmins       int     `json:"total_admins"`
	TotalTasks        int     `json:"total_tasks"`
	PendingTasks      int     `json:"pending_tasks"`
	TotalClaims       int     `json:"total_claims"`
	TotalAppeals      int     `json:"total_appeals"`
	PendingAppeals    int     `json:"pending_appeals"`
	TotalAmount       float64 `json:"total_amount"`
	TotalTransactions int     `json:"total_transactions"`
	TodayNewUsers     int     `json:"today_new_users"`
	TodayNewTasks     int     `json:"today_new_tasks"`
	TodayNewClaims    int     `json:"today_new_claims"`
	TodayNewAppeals   int     `json:"today_new_appeals"`
}

// AdminActionLog 管理员操作日志
type AdminActionLog struct {
	ID         int64     `json:"id"`
	AdminID    int64     `json:"admin_id"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"` // user, task, appeal, etc.
	TargetID   int64     `json:"target_id"`
	Detail     string    `json:"detail"`
	IP         string    `json:"ip"`
	CreatedAt  time.Time `json:"created_at"`
}

// AdminLoginRequest 管理员登录请求
type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AdminRegisterRequest 管理员注册请求
type AdminRegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6,max=50"`
	Phone    string `json:"phone" binding:"required"`
	RealName string `json:"real_name"`
}

// AdminPasswordChangeRequest 管理员修改密码请求
type AdminPasswordChangeRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// SystemSettings 系统设置
type SystemSettings struct {
	ReviewDays    int     `json:"review_days"`     // 商家审核天数
	SubmitDays    int     `json:"submit_days"`     // 任务默认提交期限
	GraceDays     int     `json:"grace_days"`      // 默认审核宽限期
	ReportAction  int     `json:"report_action"`   // 举报处理方式: 1=强制采纳, 2=强制淘汰, 3=举报成立
	MinUnitPrice  float64 `json:"min_unit_price"`  // 最低参与奖励
	MinAwardPrice float64 `json:"min_award_price"` // 最低采纳奖励
}
