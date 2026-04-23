package model

import "time"

const (
	VideoProcessStatusPending    = "pending"
	VideoProcessStatusProcessing = "processing"
	VideoProcessStatusDone       = "done"
	VideoProcessStatusFailed     = "failed"
)

type VideoProcessingJob struct {
	ID                int64      `json:"id" db:"id"`
	JobID             string     `json:"job_id" db:"job_id"`
	MaterialID        int64      `json:"material_id" db:"material_id"`
	BizType           string     `json:"biz_type" db:"biz_type"`
	BizID             int64      `json:"biz_id" db:"biz_id"`
	SourceURL         string     `json:"source_url" db:"source_url"`
	Status            string     `json:"status" db:"status"`
	ProcessedURL      string     `json:"processed_url" db:"processed_url"`
	ThumbnailURL      string     `json:"thumbnail_url" db:"thumbnail_url"`
	WatermarkTemplate string     `json:"watermark_template" db:"watermark_template"`
	TargetFormat      string     `json:"target_format" db:"target_format"`
	TargetResolution  string     `json:"target_resolution" db:"target_resolution"`
	ErrorMessage      string     `json:"error_message" db:"error_message"`
	Duration          float64    `json:"duration" db:"duration"`
	Width             int        `json:"width" db:"width"`
	Height            int        `json:"height" db:"height"`
	WatermarkApplied  bool       `json:"watermark_applied" db:"watermark_applied"`
	Compressed        bool       `json:"compressed" db:"compressed"`
	CompletedAt       *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	LastCallbackAt    *time.Time `json:"last_callback_at,omitempty" db:"last_callback_at"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}

type VideoProcessingJobRequest struct {
	JobID             string `json:"job_id"`
	SourceURL         string `json:"source_url"`
	BizType           string `json:"biz_type"`
	BizID             int64  `json:"biz_id"`
	WatermarkTemplate string `json:"watermark_template"`
	TargetFormat      string `json:"target_format"`
	TargetResolution  string `json:"target_resolution"`
	CallbackURL       string `json:"callback_url"`
}

type VideoProcessingCallback struct {
	JobID            string  `json:"job_id" binding:"required"`
	Status           string  `json:"status" binding:"required"`
	ProcessedURL     string  `json:"processed_url"`
	ThumbnailURL     string  `json:"thumbnail_url"`
	Duration         float64 `json:"duration"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	ErrorMessage     string  `json:"error_message"`
	WatermarkApplied bool    `json:"watermark_applied"`
	Compressed       bool    `json:"compressed"`
}
