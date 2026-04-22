package service

import "github.com/tans/miao/internal/model"

// CreditService 信用等级服务（新版创作者积分体系）
type CreditService struct {
	cfg *CreditServiceConfig
}

type CreditServiceConfig struct {
	DiamondRate float64 // 钻石创作者抽成
	GoldRate    float64 // 黄金创作者抽成
	SilverRate  float64 // 白银创作者抽成
	BronzeRate  float64 // 青铜创作者抽成
}

// NewCreditService creates a new CreditService
func NewCreditService() *CreditService {
	return &CreditService{}
}

// NewCreditServiceWithConfig creates a new CreditService with config
func NewCreditServiceWithConfig(cfg *CreditServiceConfig) *CreditService {
	return &CreditService{cfg: cfg}
}

// CalculateLevel 根据累计采纳数计算等级 Lv0-Lv5
// 升级条件：0/3/10/30/80/200
func (s *CreditService) CalculateLevel(adoptedCount int) model.UserLevel {
	if adoptedCount >= 200 {
		return model.LevelExclusive // 特约创作者 Lv5
	} else if adoptedCount >= 80 {
		return model.LevelGold // 金牌创作者 Lv4
	} else if adoptedCount >= 30 {
		return model.LevelQuality // 优质创作者 Lv3
	} else if adoptedCount >= 10 {
		return model.LevelActive // 活跃创作者 Lv2
	} else if adoptedCount >= 3 {
		return model.LevelNewbie // 新手创作者 Lv1
	}
	return model.LevelTrial // 试用创作者 Lv0
}

// GetCommissionRate 根据等级返回抽成比例
// Lv0-Lv3: 10%, Lv4: 5%, Lv5: 3%
func (s *CreditService) GetCommissionRate(level model.UserLevel) float64 {
	if s.cfg != nil {
		switch level {
		case model.LevelExclusive:
			return s.cfg.DiamondRate
		case model.LevelGold:
			return s.cfg.GoldRate
		default:
			return s.cfg.BronzeRate
		}
	}
	switch level {
	case model.LevelExclusive:
		return 0.03 // 3%
	case model.LevelGold:
		return 0.05 // 5%
	default:
		return 0.10 // 10%
	}
}

// GetCreatorRewardRate 获取创作者收益比例
func (s *CreditService) GetCreatorRewardRate(level model.UserLevel) float64 {
	return 1.0 - s.GetCommissionRate(level)
}

// CalculateReward 计算创作者奖励和平台抽成
func (s *CreditService) CalculateReward(unitPrice float64, level model.UserLevel) (creatorReward, platformFee float64) {
	commissionRate := s.GetCommissionRate(level)
	platformFee = unitPrice * commissionRate
	creatorReward = unitPrice - platformFee
	return
}

// GetDailyLimit 获取每日投稿上限
func (s *CreditService) GetDailyLimit(level model.UserLevel) int {
	limits := []int{3, 8, 15, 30, 50, 999}
	if level < 0 || level > 5 {
		return 3
	}
	return limits[level]
}
