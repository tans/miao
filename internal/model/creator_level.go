package model

// CreatorLevelRule 创作者等级规则
type CreatorLevelRule struct {
	Level           UserLevel `json:"level"`
	Name            string    `json:"name"`
	Condition       string    `json:"condition"`
	MinAdoptedCount int       `json:"min_adopted_count"`
	DailyLimit      int       `json:"daily_limit"`
	DailyLimitText  string    `json:"daily_limit_text"`
	CommissionRate  float64   `json:"commission_rate"`
	CommissionText  string    `json:"commission_text"`
	Privileges      []string  `json:"privileges"`
}

var creatorLevelRules = []CreatorLevelRule{
	{
		Level:           LevelTrial,
		Name:            "试用创作者",
		Condition:       "0",
		MinAdoptedCount: 0,
		DailyLimit:      3,
		DailyLimitText:  "3条",
		CommissionRate:  0.10,
		CommissionText:  "10%",
		Privileges:      []string{"每日投稿3条"},
	},
	{
		Level:           LevelNewbie,
		Name:            "新手创作者",
		Condition:       "≥3",
		MinAdoptedCount: 3,
		DailyLimit:      8,
		DailyLimitText:  "8条",
		CommissionRate:  0.10,
		CommissionText:  "10%",
		Privileges:      []string{"每日投稿8条", "升级后解锁更多权益"},
	},
	{
		Level:           LevelActive,
		Name:            "活跃创作者",
		Condition:       "≥10",
		MinAdoptedCount: 10,
		DailyLimit:      15,
		DailyLimitText:  "15条",
		CommissionRate:  0.10,
		CommissionText:  "10%",
		Privileges:      []string{"每日投稿15条", "享有平台推荐优先权"},
	},
	{
		Level:           LevelQuality,
		Name:            "优质创作者",
		Condition:       "≥30",
		MinAdoptedCount: 30,
		DailyLimit:      30,
		DailyLimitText:  "30条",
		CommissionRate:  0.10,
		CommissionText:  "10%",
		Privileges:      []string{"每日投稿30条", "高质量任务优先推送"},
	},
	{
		Level:           LevelGold,
		Name:            "金牌创作者",
		Condition:       "≥80",
		MinAdoptedCount: 80,
		DailyLimit:      50,
		DailyLimitText:  "50条",
		CommissionRate:  0.05,
		CommissionText:  "5%",
		Privileges:      []string{"每日投稿50条", "高佣金低抽成", "专属高价任务"},
	},
	{
		Level:           LevelExclusive,
		Name:            "特约创作者",
		Condition:       "≥200",
		MinAdoptedCount: 200,
		DailyLimit:      999,
		DailyLimitText:  "无上限",
		CommissionRate:  0.03,
		CommissionText:  "3%",
		Privileges:      []string{"投稿无上限", "最低佣金3%", "专属客服通道", "定向约稿特权"},
	},
}

// CreatorLevelSummary 创作者等级概览
type CreatorLevelSummary struct {
	Level          UserLevel          `json:"level"`
	LevelName      string             `json:"level_name"`
	AdoptedCount   int                `json:"adopted_count"`
	DailyLimit     int                `json:"daily_limit"`
	DailyLimitText string             `json:"daily_limit_text"`
	CommissionRate float64            `json:"commission_rate"`
	CommissionText string             `json:"commission_text"`
	NextLevelName  string             `json:"next_level_name"`
	NeedCount      int                `json:"need_count"`
	LevelRules     []CreatorLevelRule `json:"level_rules"`
}

// CalculateCreatorLevel 根据累计采纳数计算创作者等级
func CalculateCreatorLevel(adoptedCount int) UserLevel {
	for i := len(creatorLevelRules) - 1; i >= 0; i-- {
		if adoptedCount >= creatorLevelRules[i].MinAdoptedCount {
			return creatorLevelRules[i].Level
		}
	}
	return LevelTrial
}

// CreatorLevelRules 返回全部等级规则
func CreatorLevelRules() []CreatorLevelRule {
	rules := make([]CreatorLevelRule, len(creatorLevelRules))
	for i, rule := range creatorLevelRules {
		rules[i] = rule
		if len(rule.Privileges) > 0 {
			rules[i].Privileges = append([]string(nil), rule.Privileges...)
		}
	}
	return rules
}

// CreatorLevelRuleFor 返回指定等级的规则
func CreatorLevelRuleFor(level UserLevel) CreatorLevelRule {
	if level < 0 || int(level) >= len(creatorLevelRules) {
		return creatorLevelRules[0]
	}
	rule := creatorLevelRules[level]
	if len(rule.Privileges) > 0 {
		rule.Privileges = append([]string(nil), rule.Privileges...)
	}
	return rule
}

// CreatorLevelSummaryFor 根据累计采纳数生成等级概览
func CreatorLevelSummaryFor(adoptedCount int) CreatorLevelSummary {
	level := CalculateCreatorLevel(adoptedCount)
	current := CreatorLevelRuleFor(level)
	rules := CreatorLevelRules()

	summary := CreatorLevelSummary{
		Level:          level,
		LevelName:      current.Name,
		AdoptedCount:   adoptedCount,
		DailyLimit:     current.DailyLimit,
		DailyLimitText: current.DailyLimitText,
		CommissionRate: current.CommissionRate,
		CommissionText: current.CommissionText,
		LevelRules:     rules,
	}

	nextIdx := int(level) + 1
	if nextIdx >= 0 && nextIdx < len(rules) {
		next := rules[nextIdx]
		summary.NextLevelName = next.Name
		summary.NeedCount = next.MinAdoptedCount - adoptedCount
		if summary.NeedCount < 0 {
			summary.NeedCount = 0
		}
	}

	return summary
}

// RefreshCreatorLevel 根据累计采纳数同步等级字段
func (u *User) RefreshCreatorLevel() {
	if u == nil {
		return
	}
	u.Level = CalculateCreatorLevel(u.AdoptedCount)
}
