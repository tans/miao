package model

import "time"

type MerchantAuthStatus int

const (
	MerchantAuthStatusPending MerchantAuthStatus = iota
	MerchantAuthStatusApproved
	MerchantAuthStatusRejected
)

// MerchantAuthApplication 商家认证申请
type MerchantAuthApplication struct {
	ID            int64     `json:"id" db:"id"`
	UserID        int64     `json:"user_id" db:"user_id"`
	CompanyName   string    `json:"company_name" db:"company_name"`
	CreditCode    string    `json:"credit_code" db:"credit_code"`
	ContactName   string    `json:"contact_name" db:"contact_name"`
	ContactPhone  string    `json:"contact_phone" db:"contact_phone"`
	LicenseURL    string    `json:"license_url" db:"license_url"`
	Status        int       `json:"status" db:"status"`
	ReviewComment string    `json:"review_comment" db:"review_comment"`
	ReviewedAt    time.Time `json:"reviewed_at" db:"reviewed_at"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// MerchantAuthListItem 商家认证管理列表项
type MerchantAuthListItem struct {
	ID               int64     `json:"id"`
	UserID           int64     `json:"user_id"`
	Username         string    `json:"username"`
	Phone            string    `json:"phone"`
	CompanyName      string    `json:"company_name"`
	CreditCode       string    `json:"credit_code"`
	ContactName      string    `json:"contact_name"`
	ContactPhone     string    `json:"contact_phone"`
	LicenseURL       string    `json:"license_url"`
	LicensePreviewURL string   `json:"license_preview_url"`
	Status           int       `json:"status"`
	StatusText       string    `json:"status_text"`
	ReviewComment    string    `json:"review_comment"`
	ReviewedAt       time.Time `json:"reviewed_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	BusinessVerified bool      `json:"business_verified"`
}

func (s MerchantAuthStatus) Text() string {
	switch s {
	case MerchantAuthStatusPending:
		return "pending"
	case MerchantAuthStatusApproved:
		return "certified"
	default:
		return "uncertified"
	}
}
