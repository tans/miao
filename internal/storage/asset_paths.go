package storage

import (
	"fmt"
	"path"
	"strings"
)

const (
	BizTypeAvatar          = "avatar"
	BizTypeTaskMaterial    = "task_material"
	BizTypeMerchantLicense = "merchant_license"
	BizTypeAppealEvidence  = "appeal_evidence"
	BizTypeClaimSource     = "claim_source"
)

func BuildUploadObjectKey(fileType, bizType, bizID, jobID, ext, fallbackFilename string, userID int64) string {
	fileType = strings.TrimSpace(strings.ToLower(fileType))
	bizType = strings.TrimSpace(strings.ToLower(bizType))
	bizID = strings.TrimSpace(bizID)
	jobID = strings.TrimSpace(jobID)
	ext = normalizeExt(ext)
	fallbackFilename = strings.TrimSpace(fallbackFilename)

	switch bizType {
	case BizTypeAvatar:
		return path.Join("public", "avatars", ownerSegment(userID, bizID), jobOrFilename(jobID, ext, fallbackFilename))
	case BizTypeTaskMaterial:
		return path.Join("public", "task-materials", ownerSegment(userID, bizID), jobOrFilename(jobID, ext, fallbackFilename))
	case BizTypeMerchantLicense:
		return path.Join("private", "license", ownerSegment(userID, bizID), jobOrFilename(jobID, ext, fallbackFilename))
	case BizTypeAppealEvidence:
		return path.Join("private", "evidence", ownerSegment(userID, bizID), jobOrFilename(jobID, ext, fallbackFilename))
	case BizTypeClaimSource:
		if fileType == "video" && bizID != "" && jobID != "" {
			return path.Join("private", "source", bizID, jobID+ext)
		}
	}

	if fileType == "image" {
		return path.Join("public", "images", fallbackFilename)
	}
	if fileType == "video" {
		return path.Join("private", "video", fallbackFilename)
	}
	return path.Join("public", "uploads", fallbackFilename)
}

func BuildWatermarkedVideoKey(claimID int64, jobID, ext string) string {
	ext = normalizeExt(ext)
	return path.Join("public", "watermarked", fmt.Sprintf("%d", claimID), strings.TrimSpace(jobID)+ext)
}

func BuildThumbnailKey(claimID int64, jobID, ext string) string {
	ext = normalizeExt(ext)
	return path.Join("public", "thumbnails", fmt.Sprintf("%d", claimID), strings.TrimSpace(jobID)+ext)
}

func IsPrivateObjectKey(key string) bool {
	key = strings.TrimLeft(strings.TrimSpace(key), "/")
	return strings.HasPrefix(key, "private/")
}

func IsProtectedObjectKey(key string) bool {
	key = strings.TrimLeft(strings.TrimSpace(key), "/")
	switch {
	case strings.HasPrefix(key, "private/"),
		strings.HasPrefix(key, "image/"),
		strings.HasPrefix(key, "video/"),
		strings.HasPrefix(key, "claim-source/"),
		strings.HasPrefix(key, "claim-processed/"):
		return true
	default:
		return false
	}
}

func normalizeExt(ext string) string {
	ext = strings.TrimSpace(strings.ToLower(ext))
	if ext == "" {
		return ""
	}
	if strings.HasPrefix(ext, ".") {
		return ext
	}
	return "." + ext
}

func ownerSegment(userID int64, bizID string) string {
	if bizID = strings.TrimSpace(bizID); bizID != "" {
		return bizID
	}
	if userID > 0 {
		return fmt.Sprintf("%d", userID)
	}
	return "anonymous"
}

func jobOrFilename(jobID, ext, fallbackFilename string) string {
	jobID = strings.TrimSpace(jobID)
	if jobID != "" {
		return jobID + normalizeExt(ext)
	}
	return fallbackFilename
}
