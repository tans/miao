package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

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
	ReviewResultPass   ReviewResult = 1 // 通过/采纳
	ReviewResultReturn ReviewResult = 2 // 退回/拒绝
	ReviewResultReport ReviewResult = 3 // 举报（不合格）
)

// Claim 认领表
type Claim struct {
	ID            int64       `json:"id" db:"id"`
	TaskID        int64       `json:"task_id" db:"task_id"`
	TaskTitle     string      `json:"task_title" db:"task_title"` // 任务标题（通过JOIN获取）
	TaskStatus    TaskStatus  `json:"task_status" db:"task_status"`
	TaskEndAt     *time.Time  `json:"end_at,omitempty" db:"task_end_at"`
	UnitPrice     float64     `json:"unit_price" db:"unit_price"`   // 对应 tasks.unit_price
	AwardPrice    float64     `json:"award_price" db:"award_price"` // 对应 tasks.award_price
	CreatorID     int64       `json:"creator_id" db:"creator_id"`
	Status        ClaimStatus `json:"status" db:"status"`                         // 1=已认领, 2=已提交, 3=已验收, 4=已取消, 5=超时
	Content       string      `json:"content" db:"content"`                       // 交付内容
	SubmitAt      *time.Time  `json:"submit_at,omitempty" db:"submit_at"`         // 提交时间
	ExpiresAt     time.Time   `json:"expires_at" db:"expires_at"`                 // 超时时间 (认领+24h)
	ReviewAt      *time.Time  `json:"review_at,omitempty" db:"review_at"`         // 验收时间
	ReviewResult  *int        `json:"review_result,omitempty" db:"review_result"` // 1=通过, 2=退回
	ReviewComment string      `json:"review_comment" db:"review_comment"`         // 验收意见
	Likes         int64       `json:"likes" db:"likes"`                           // 点赞数

	// 资金
	CreatorReward  float64 `json:"creator_reward" db:"creator_reward"`   // 创作者收益 (85%)
	PlatformFee    float64 `json:"platform_fee" db:"platform_fee"`       // 平台抽成 (15%)
	MarginReturned float64 `json:"margin_returned" db:"margin_returned"` // 保证金退还 (10元)

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ClaimCreate 认领请求
type ClaimCreate struct {
	TaskID int64 `json:"task_id" binding:"required,gt=0"`
}

func (c *ClaimCreate) UnmarshalJSON(data []byte) error {
	type claimCreateJSON struct {
		TaskID json.RawMessage `json:"task_id"`
	}

	var raw claimCreateJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	taskID, err := parseFlexibleInt64(raw.TaskID)
	if err != nil {
		return fmt.Errorf("task_id %w", err)
	}

	c.TaskID = taskID
	return nil
}

func parseFlexibleInt64(raw json.RawMessage) (int64, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return 0, nil
	}

	var number int64
	if err := json.Unmarshal(raw, &number); err == nil {
		return number, nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0, fmt.Errorf("must be an integer")
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return 0, nil
	}

	number, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("must be an integer")
	}

	return number, nil
}

// ClaimSubmit 提交交付请求
type ClaimSubmit struct {
	Content   string               `json:"content" binding:"required"`
	Materials []ClaimMaterialInput `json:"materials"`
}

// ClaimReview 验收请求
type ClaimReview struct {
	Result  int    `json:"result" binding:"required,oneof=1 2 3"` // 1=通过/采纳, 2=退回/拒绝, 3=举报/不合格
	Comment string `json:"comment"`
}

// ClaimQuery 认领查询
type ClaimQuery struct {
	Status   *int `form:"status"`
	Page     int  `form:"page,default=1"`
	PageSize int  `form:"page_size,default=20"`
}

// ClaimMaterial 认领媒体文件
type ClaimMaterial struct {
	ID                int64     `json:"id" db:"id"`
	ClaimID           int64     `json:"claim_id" db:"claim_id"`
	FileName          string    `json:"file_name" db:"file_name"`
	FilePath          string    `json:"file_path" db:"file_path"`
	SourceFilePath    string    `json:"source_file_path,omitempty" db:"source_file_path"`
	ProcessedFilePath string    `json:"processed_file_path,omitempty" db:"processed_file_path"`
	FileSize          int64     `json:"file_size" db:"file_size"`
	FileType          string    `json:"file_type" db:"file_type"`
	ThumbnailPath     string    `json:"thumbnail_path,omitempty" db:"thumbnail_path"`
	ProcessStatus     string    `json:"process_status,omitempty" db:"process_status"`
	ProcessError      string    `json:"process_error,omitempty" db:"process_error"`
	ProcessJobID      string    `json:"process_job_id,omitempty" db:"process_job_id"`
	ProcessRetryCount int       `json:"process_retry_count,omitempty" db:"process_retry_count"`
	WatermarkApplied  bool      `json:"watermark_applied" db:"watermark_applied"`
	Compressed        bool      `json:"compressed" db:"compressed"`
	Duration          float64   `json:"duration,omitempty" db:"duration"`
	Width             int       `json:"width,omitempty" db:"width"`
	Height            int       `json:"height,omitempty" db:"height"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

// ClaimMaterialInput 提交时的媒体输入
type ClaimMaterialInput struct {
	FileName      string `json:"file_name" binding:"required"`
	FilePath      string `json:"file_path" binding:"required"`
	FileSize      int64  `json:"file_size"`
	FileType      string `json:"file_type" binding:"required"`
	ThumbnailPath string `json:"thumbnail_path"`
}
