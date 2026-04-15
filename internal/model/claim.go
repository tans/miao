package model

import "time"

// ClaimStatus 认领状态
type ClaimStatus int

const (
	ClaimStatusPending   ClaimStatus = 1 // 已认领(待提交)
	ClaimStatusSubmitted ClaimStatus = 2 // 已提交(待验收)
	ClaimStatusApproved  ClaimStatus = 3 // 已验收(已完成)
	ClaimStatusCancelled ClaimStatus = 4 // 已取消
	ClaimStatusExpired   ClaimStatus = 5 // 已超时
)

// ReviewResult 验收结果
type ReviewResult int

const (
	ReviewResultPass   ReviewResult = 1 // 通过
	ReviewResultReturn ReviewResult = 2 // 退回
)

// Claim 认领表
type Claim struct {
	ID           int64        `json:"id" db:"id"`
	TaskID       int64        `json:"task_id" db:"task_id"`
	TaskTitle    string       `json:"task_title" db:"task_title"` // 任务标题（通过JOIN获取）
	CreatorID    int64        `json:"creator_id" db:"creator_id"`
	Status       ClaimStatus  `json:"status" db:"status"`        // 1=已认领, 2=已提交, 3=已验收, 4=已取消, 5=超时
	Content      string       `json:"content" db:"content"`        // 交付内容
	SubmitAt     *time.Time   `json:"submit_at,omitempty" db:"submit_at"` // 提交时间
	ExpiresAt    time.Time    `json:"expires_at" db:"expires_at"`    // 超时时间 (认领+24h)
	ReviewAt     *time.Time   `json:"review_at,omitempty" db:"review_at"` // 验收时间
	ReviewResult *int         `json:"review_result,omitempty" db:"review_result"` // 1=通过, 2=退回
	ReviewComment string      `json:"review_comment" db:"review_comment"` // 验收意见

	// 资金
	CreatorReward float64     `json:"creator_reward" db:"creator_reward"`  // 创作者收益 (85%)
	PlatformFee   float64     `json:"platform_fee" db:"platform_fee"`    // 平台抽成 (15%)
	MarginReturned float64    `json:"margin_returned" db:"margin_returned"` // 保证金退还 (10元)

	CreatedAt    time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at" db:"updated_at"`
}

// ClaimCreate 认领请求
type ClaimCreate struct {
	TaskID int64 `json:"task_id" binding:"required"`
}

// ClaimSubmit 提交交付请求
type ClaimSubmit struct {
	Content   string               `json:"content" binding:"required"`
	Materials []ClaimMaterialInput `json:"materials"`
}

// ClaimReview 验收请求
type ClaimReview struct {
	Result  int    `json:"result" binding:"required,oneof=1 2"` // 1=通过, 2=退回
	Comment string `json:"comment"`
}

// ClaimQuery 认领查询
type ClaimQuery struct {
	Status *int `form:"status"`
	Page   int  `form:"page,default=1"`
	PageSize int `form:"page_size,default=20"`
}

// ClaimMaterial 认领媒体文件
type ClaimMaterial struct {
	ID            int64     `json:"id" db:"id"`
	ClaimID       int64     `json:"claim_id" db:"claim_id"`
	FileName      string    `json:"file_name" db:"file_name"`
	FilePath      string    `json:"file_path" db:"file_path"`
	FileSize      int64     `json:"file_size" db:"file_size"`
	FileType      string    `json:"file_type" db:"file_type"`
	ThumbnailPath string    `json:"thumbnail_path,omitempty" db:"thumbnail_path"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// ClaimMaterialInput 提交时的媒体输入
type ClaimMaterialInput struct {
	FileName      string `json:"file_name" binding:"required"`
	FilePath      string `json:"file_path" binding:"required"`
	FileSize      int64  `json:"file_size"`
	FileType      string `json:"file_type" binding:"required"`
	ThumbnailPath string `json:"thumbnail_path"`
}
