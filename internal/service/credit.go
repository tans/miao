package service

import "github.com/tans/miao/internal/model"

// CreditService 信用等级服务
type CreditService struct{}

func NewCreditService() *CreditService {
	return &CreditService{}
}

// CalculateLevel 根据完成任务数计算等级
func (s *CreditService) CalculateLevel(completedTasks int) model.UserLevel {
	if completedTasks >= 200 {
		return model.LevelDiamond // 钻石
	} else if completedTasks >= 50 {
		return model.LevelGold // 黄金
	} else if completedTasks >= 10 {
		return model.LevelSilver // 白银
	}
	return model.LevelBronze // 青铜
}

// GetCommissionRate 根据等级返回抽成比例
func (s *CreditService) GetCommissionRate(level model.UserLevel) float64 {
	switch level {
	case model.LevelDiamond:
		return 0.10 // 10%
	case model.LevelGold:
		return 0.12 // 12%
	case model.LevelSilver:
		return 0.15 // 15%
	case model.LevelBronze:
		return 0.20 // 20%
	default:
		return 0.20
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
