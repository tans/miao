package model

import "time"

// UserLevel 创作者等级
type UserLevel int // 1=青铜, 2=白银, 3=黄金, 4=钻石

const (
	LevelBronze  UserLevel = 1 // 青铜
	LevelSilver UserLevel = 2 // 白银
	LevelGold   UserLevel = 3 // 黄金
	LevelDiamond UserLevel = 4 // 钻石
)

// User 用户表
type User struct {
	ID           int64     `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	PasswordHash string    `json:"-" db:"password_hash"` // 不返回密码
	IsAdmin      bool      `json:"is_admin" db:"is_admin"` // 是否为管理员
	Phone        string    `json:"phone" db:"phone"`
	Nickname     string    `json:"nickname" db:"nickname"`
	Avatar       string    `json:"avatar" db:"avatar"`
	WechatOpenID string    `json:"wechat_openid" db:"wechat_openid"` // 微信小程序openid

	// 账户资金
	Balance      float64   `json:"balance" db:"balance"`      // 账户余额
	FrozenAmount float64   `json:"frozen_amount" db:"frozen_amount"` // 冻结金额

	// 创作者专属
	Level           UserLevel `json:"level" db:"level"`            // 1-4
	BehaviorScore   int       `json:"behavior_score" db:"behavior_score"`   // 行为分 -1000~+2000
	TradeScore      float64   `json:"trade_score" db:"trade_score"`      // 交易分 0~500
	TotalScore      int       `json:"total_score" db:"total_score"`      // 总积分 = BehaviorScore + TradeScore
	MarginFrozen    float64   `json:"margin_frozen" db:"margin_frozen"`     // 冻结保证金
	DailyClaimCount int       `json:"daily_claim_count" db:"daily_claim_count"` // 今日认领数
	DailyClaimReset time.Time `json:"daily_claim_reset" db:"daily_claim_reset"` // 重置时间

	// 商家专属
	BusinessVerified bool `json:"business_verified" db:"business_verified"` // 企业实名认证
	PublishCount     int  `json:"publish_count" db:"publish_count"`     // 已发布任务数

	// 统计计数（通过子查询填充，不存储）
	CreatedTasksCount   int `json:"created_tasks_count" db:"-"`   // 创建任务数
	ClaimedTasksCount   int `json:"claimed_tasks_count" db:"-"`   // 领取任务数（参与的任务）
	SubmittedWorksCount int `json:"submitted_works_count" db:"-"` // 提交作品数

	Status    int       `json:"status" db:"status"`    // 1=正常, 0=禁用
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// GetLevelName 获取等级名称
func (u *User) GetLevelName() string {
	names := []string{"", "青铜", "白银", "黄金", "钻石"}
	if u.Level < 1 || u.Level > 4 {
		return "未知"
	}
	return names[u.Level]
}

// GetCommission 获取平台抽成比例
func (u *User) GetCommission() float64 {
	commissions := []float64{0, 0.20, 0.15, 0.12, 0.10} // 索引1-4
	if u.Level < 1 || u.Level > 4 {
		return 0.20
	}
	return commissions[u.Level]
}

// GetDailyLimit 获取每日认领上限
func (u *User) GetDailyLimit() int {
	limits := []int{0, 3, 10, 20, 50}
	if u.Level < 1 || u.Level > 4 {
		return 0
	}
	return limits[u.Level]
}

// CanClaim 是否可以认领任务
func (u *User) CanClaim() bool {
	return u.Level >= 2 // 白银及以上直接认领
}

// NeedMargin 是否需要保证金
func (u *User) NeedMargin() bool {
	return u.Level == 1 // 只有青铜需要保证金
}

// CalcTotalScore 计算总积分
func (u *User) CalcTotalScore() int {
	u.TotalScore = u.BehaviorScore + int(u.TradeScore)
	return u.TotalScore
}


// UserRegister 注册请求
type UserRegister struct {
	Username   string `json:"username" binding:"required,min=3,max=50"`
	Password   string `json:"password" binding:"required,min=6,max=50"`
	Phone      string `json:"phone" binding:"required"`
	IsAdmin    bool   `json:"is_admin"` // 是否为管理员（注册时可选）
	RealName   string `json:"real_name"`
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
	IsAdmin  *bool   `form:"is_admin"`
	Status   *int    `form:"status"`
	Keyword  string `form:"keyword"`
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"page_size,default=20"`
}

// UserWallet 创作者钱包信息
type UserWallet struct {
	Balance       float64 `json:"balance"`        // 账户余额
	FrozenAmount  float64 `json:"frozen_amount"`  // 冻结金额
	MarginFrozen  float64 `json:"margin_frozen"`  // 冻结保证金
	TotalScore    int     `json:"total_score"`    // 总积分
	BehaviorScore int     `json:"behavior_score"` // 行为分
	TradeScore    float64 `json:"trade_score"`    // 交易分
	Level         int     `json:"level"`          // 等级
	LevelName     string  `json:"level_name"`     // 等级名称
}
