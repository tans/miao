package model

import "time"

// UserLevel 创作者等级 Lv0-Lv5
type UserLevel int

const (
	LevelTrial     UserLevel = 0 // 试用创作者
	LevelNewbie    UserLevel = 1 // 新手创作者
	LevelActive    UserLevel = 2 // 活跃创作者
	LevelQuality   UserLevel = 3 // 优质创作者
	LevelGold      UserLevel = 4 // 金牌创作者
	LevelExclusive UserLevel = 5 // 特约创作者
)

// User 用户表
type User struct {
	ID           int64  `json:"id" db:"id"`
	Username     string `json:"username" db:"username"`
	PasswordHash string `json:"-" db:"password_hash"`   // 不返回密码
	IsAdmin      bool   `json:"is_admin" db:"is_admin"` // 是否为管理员
	Phone        string `json:"phone" db:"phone"`
	Nickname     string `json:"nickname" db:"nickname"`
	Avatar       string `json:"avatar" db:"avatar"`
	WechatOpenID string `json:"wechat_openid" db:"wechat_openid"` // 微信小程序openid

	// 账户资金
	Balance      float64 `json:"balance" db:"balance"`             // 账户余额
	FrozenAmount float64 `json:"frozen_amount" db:"frozen_amount"` // 冻结金额

	// 实名认证
	RealNameVerified bool `json:"real_name_verified" db:"real_name_verified"` // 是否已通过实名认证

	// 创作者专属
	Level           UserLevel `json:"level" db:"level"`                         // 0-5
	AdoptedCount    int       `json:"adopted_count" db:"adopted_count"`         // 累计采纳数
	MarginFrozen    float64   `json:"margin_frozen" db:"margin_frozen"`         // 冻结保证金（保留字段，新体系无需）
	DailyClaimCount int       `json:"daily_claim_count" db:"daily_claim_count"` // 今日投稿数
	DailyClaimReset time.Time `json:"daily_claim_reset" db:"daily_claim_reset"` // 重置时间
	ReportCount     int       `json:"report_count" db:"report_count"`           // 被举报次数（超过5次无法提交作品）

	// 商家专属
	BusinessVerified bool `json:"business_verified" db:"business_verified"` // 企业实名认证
	PublishCount     int  `json:"publish_count" db:"publish_count"`         // 已发布任务数

	// 统计计数（通过子查询填充，不存储）
	CreatedTasksCount   int `json:"created_tasks_count" db:"-"`   // 创建任务数
	ClaimedTasksCount   int `json:"claimed_tasks_count" db:"-"`   // 领取任务数（参与的任务）
	SubmittedWorksCount int `json:"submitted_works_count" db:"-"` // 提交作品数

	Status    int       `json:"status" db:"status"` // 1=正常, 0=禁用
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// GetLevelName 获取等级名称
func (u *User) GetLevelName() string {
	names := []string{"试用创作者", "新手创作者", "活跃创作者", "优质创作者", "金牌创作者", "特约创作者"}
	if u.Level < 0 || u.Level > 5 {
		return "试用创作者"
	}
	return names[u.Level]
}

// GetCommission 获取平台抽成比例
func (u *User) GetCommission() float64 {
	// Lv0-Lv3: 10%, Lv4: 5%, Lv5: 3%
	commissions := []float64{0.10, 0.10, 0.10, 0.10, 0.05, 0.03}
	if u.Level < 0 || int(u.Level) >= len(commissions) {
		return 0.10
	}
	return commissions[u.Level]
}

// GetDailyLimit 获取每日投稿上限
func (u *User) GetDailyLimit() int {
	// 3, 8, 15, 30, 50, 999(无上限)
	limits := []int{3, 8, 15, 30, 50, 999}
	if u.Level < 0 || u.Level > 5 {
		return 3
	}
	return limits[u.Level]
}

// CanClaim 是否可以认领/投稿任务
func (u *User) CanClaim() bool {
	return true // Lv0起即可投稿
}

// CanSubmitWork 是否可以提交作品（被举报超过5次则无法提交）
func (u *User) CanSubmitWork() bool {
	return u.ReportCount < 5
}

// NeedMargin 是否需要保证金（新体系无需保证金）
func (u *User) NeedMargin() bool {
	return false
}

// UserRegister 注册请求
type UserRegister struct {
	Username    string `json:"username" binding:"required,min=3,max=50"`
	Password    string `json:"password" binding:"required,min=6,max=50"`
	Phone       string `json:"phone" binding:"required"`
	IsAdmin     bool   `json:"is_admin"` // 是否为管理员（注册时可选）
	RealName    string `json:"real_name"`
	CompanyName string `json:"company_name"`
}

// UserLogin 登录请求
type UserLogin struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// UserProfileUpdate 更新个人资料
type UserProfileUpdate struct {
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// UserListQuery 用户列表查询
type UserListQuery struct {
	IsAdmin  *bool  `form:"is_admin"`
	Status   *int   `form:"status"`
	Keyword  string `form:"keyword"`
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"page_size,default=20"`
}

// UserWallet 创作者钱包信息
type UserWallet struct {
	Balance          float64 `json:"balance"`            // 账户余额
	FrozenAmount     float64 `json:"frozen_amount"`      // 冻结金额
	MarginFrozen     float64 `json:"margin_frozen"`      // 冻结保证金
	Level            int     `json:"level"`              // 等级 0-5
	LevelName        string  `json:"level_name"`         // 等级名称
	RealNameVerified bool    `json:"real_name_verified"` // 是否已实名认证
}
