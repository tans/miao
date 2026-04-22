package model

import "time"

type InspirationStatus int

const (
	InspirationStatusDraft     InspirationStatus = 0
	InspirationStatusPublished InspirationStatus = 1
)

type Inspiration struct {
	ID            int64                  `json:"id" db:"id"`
	Title         string                 `json:"title" db:"title"`
	Content       string                 `json:"content" db:"content"`
	Tags          string                 `json:"-" db:"tags"`
	TagList       []string               `json:"tags,omitempty" db:"-"`
	CreatorName   string                 `json:"creator_name" db:"creator_name"`
	CreatorAvatar string                 `json:"creator_avatar" db:"creator_avatar"`
	CoverURL        string                 `json:"cover_url" db:"cover_url"`
	CoverWidth      int                    `json:"cover_width" db:"cover_width"`
	CoverHeight     int                    `json:"cover_height" db:"cover_height"`
	CoverType       string                 `json:"cover_type" db:"cover_type"`
	PreviewVideoSrc string                 `json:"previewVideoSrc,omitempty" db:"-"`
	DisplayCover    string                 `json:"displayCover,omitempty" db:"-"`
	VideoURL        string                 `json:"video_url,omitempty" db:"-"`
	IsVideo         bool                   `json:"isVideo" db:"-"`
	Status          InspirationStatus      `json:"status" db:"status"`
	Views           int64                  `json:"views" db:"views"`
	Likes           int64                  `json:"likes" db:"likes"`
	SortOrder     int                    `json:"sort_order" db:"sort_order"`
	CreatedBy     int64                  `json:"created_by" db:"created_by"`
	SourceClaimID *int64                 `json:"source_claim_id,omitempty" db:"source_claim_id"`
	PublishedAt   *time.Time             `json:"published_at,omitempty" db:"published_at"`
	CreatedAt     time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at" db:"updated_at"`
	Materials     []*InspirationMaterial `json:"materials,omitempty" db:"-"`
}

type InspirationMaterial struct {
	ID            int64     `json:"id" db:"id"`
	InspirationID int64     `json:"inspiration_id" db:"inspiration_id"`
	FileName      string    `json:"file_name" db:"file_name"`
	FilePath      string    `json:"file_path" db:"file_path"`
	FileSize      int64     `json:"file_size" db:"file_size"`
	FileType      string    `json:"file_type" db:"file_type"`
	ThumbnailPath string    `json:"thumbnail_path,omitempty" db:"thumbnail_path"`
	SortOrder     int       `json:"sort_order" db:"sort_order"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

type InspirationMaterialInput struct {
	FileName      string `json:"file_name" binding:"required"`
	FilePath      string `json:"file_path" binding:"required"`
	FileSize      int64  `json:"file_size"`
	FileType      string `json:"file_type" binding:"required"`
	ThumbnailPath string `json:"thumbnail_path"`
	SortOrder     int    `json:"sort_order"`
}

type InspirationCreateRequest struct {
	Title         string                     `json:"title" binding:"required"`
	Content       string                     `json:"content"`
	Tags          []string                   `json:"tags"`
	CreatorName   string                     `json:"creator_name"`
	CreatorAvatar string                     `json:"creator_avatar"`
	CoverURL      string                     `json:"cover_url"`
	CoverWidth    int                        `json:"cover_width"`
	CoverHeight   int                        `json:"cover_height"`
	CoverType     string                     `json:"cover_type"`
	SortOrder     int                        `json:"sort_order"`
	Materials     []InspirationMaterialInput `json:"materials" binding:"required"`
}

type InspirationUpdateRequest struct {
	Title         string                     `json:"title" binding:"required"`
	Content       string                     `json:"content"`
	Tags          []string                   `json:"tags"`
	CreatorName   string                     `json:"creator_name"`
	CreatorAvatar string                     `json:"creator_avatar"`
	CoverURL      string                     `json:"cover_url"`
	CoverWidth    int                        `json:"cover_width"`
	CoverHeight   int                        `json:"cover_height"`
	CoverType     string                     `json:"cover_type"`
	SortOrder     int                        `json:"sort_order"`
	Status        InspirationStatus          `json:"status"`
	Materials     []InspirationMaterialInput `json:"materials" binding:"required"`
}
