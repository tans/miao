package model

import "time"

// SubmissionStatus 投稿状态
type SubmissionStatus int

const (
	SubmissionPending  SubmissionStatus = 1 // 待审核
	SubmissionPassed   SubmissionStatus = 2 // 已通过
	SubmissionRejected SubmissionStatus = 3 // 已驳回
)

// AwardLevel 获奖等级
type AwardLevel int

const (
	AwardNone   AwardLevel = 0 // 未获奖
	Award1      AwardLevel = 1 // 一等奖
	Award2      AwardLevel = 2 // 二等奖
	Award3      AwardLevel = 3 // 三等奖
	AwardGood   AwardLevel = 4 // 优秀奖
	AwardBase   AwardLevel = 5 // 基础参与奖
)

// Submission 投稿表
type Submission struct {
	ID            int64            `json:"id"`
	TaskID        int64            `json:"task_id"`
	TaskTitle     string           `json:"task_title"`
	CreatorID     int64            `json:"creator_id"`
	CreatorName   string           `json:"creator_name"`
	CreatorAvatar string           `json:"creator_avatar"`
	Content       string           `json:"content"`           // 作品描述
	Status        SubmissionStatus `json:"status"`             // 审核状态
	AwardLevel    AwardLevel       `json:"award_level"`         // 获奖等级
	Score         int              `json:"score"`               // 商家评分 1-5
	ReviewComment string           `json:"review_comment"`      // 审核/评价
	RewardAmount  float64          `json:"reward_amount"`       // 奖励金额
	IsUsed        bool             `json:"is_used"`             // 是否已使用
	IsTop         bool             `json:"is_top"`             // 是否爆款标注
	CreatedAt     time.Time        `json:"created_at"`
	ReviewedAt    *time.Time       `json:"reviewed_at,omitempty"`
}

// SubmissionCreate 创建投稿
type SubmissionCreate struct {
	TaskID  int64  `json:"task_id" binding:"required"`
	Content string `json:"content"`
}

// SubmissionReview 审核投稿
type SubmissionReview struct {
	Status        SubmissionStatus `json:"status" binding:"required,oneof=2 3"`
	ReviewComment string           `json:"review_comment"`
}

// SubmissionAward 评选获奖
type SubmissionAward struct {
	AwardLevel AwardLevel `json:"award_level" binding:"required"`
	RewardAmount float64   `json:"reward_amount"`
}

// SubmissionScore 评分
type SubmissionScore struct {
	Score         int    `json:"score" binding:"required,min=1,max=5"`
	ReviewComment string `json:"review_comment"`
}

// SubmissionQuery 投稿查询
type SubmissionQuery struct {
	TaskID   *int64             `form:"task_id"`
	Status   *SubmissionStatus  `form:"status"`
	Page     int                `form:"page,default=1"`
	PageSize int                `form:"page_size,default=20"`
}

// SubmissionMaterial 投稿素材
type SubmissionMaterial struct {
	ID            int64     `json:"id"`
	SubmissionID  int64     `json:"submission_id"`
	FileName      string    `json:"file_name"`
	FilePath      string    `json:"file_path"`
	FileSize      int64     `json:"file_size"`
	FileType      string    `json:"file_type"`
	ThumbnailPath string    `json:"thumbnail_path"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreatorSubmissionQuery 创作者查询自己的投稿
type CreatorSubmissionQuery struct {
	Status *SubmissionStatus `form:"status"`
	Page   int               `form:"page,default=1"`
	PageSize int             `form:"page_size,default=20"`
}
