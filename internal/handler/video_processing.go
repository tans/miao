package handler

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/storage"
	"net/http"
	"strings"
	"time"
)

func VideoProcessingCallback(c *gin.Context) {
	if videoProcessingService == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(CodeServiceError, "视频处理服务未初始化"))
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "读取回调内容失败"))
		return
	}

	signature := c.GetHeader("X-Miao-Signature")
	if !videoProcessingService.VerifySignature(body, signature) {
		c.JSON(http.StatusUnauthorized, ErrorResponse(CodeForbidden, "回调签名校验失败"))
		return
	}

	var req model.VideoProcessingCallback
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(CodeBadRequest, "回调参数错误"))
		return
	}

	if err := videoProcessingService.HandleCallback(&req); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(CodeInternalError, "处理回调失败"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{"job_id": req.JobID}))
}

func formatVisibleClaimMaterials(materials []*model.ClaimMaterial) []*model.ClaimMaterial {
	return formatClaimMaterialsForViewer(materials, false)
}

func formatCreatorVisibleClaimMaterials(materials []*model.ClaimMaterial) []*model.ClaimMaterial {
	return formatClaimMaterialsForViewer(materials, true)
}

func formatClaimMaterialsForViewer(materials []*model.ClaimMaterial, includeSourceVideo bool) []*model.ClaimMaterial {
	cfg := config.Load()
	cdn := cfg.Static.CDN
	if cdn == "" {
		cdn = cfg.Static.Host
	}
	provider, _ := GetStorageProvider()
	storageBucket := configuredStorageBucket(cfg)

	result := make([]*model.ClaimMaterial, 0, len(materials))
	for _, material := range materials {
		if material == nil {
			continue
		}
		item := *material

		status := strings.TrimSpace(item.ProcessStatus)
		if item.FileType == "video" && status == "" {
			status = model.VideoProcessStatusDone
		}
		item.ProcessStatus = status

		if item.FileType == "video" {
			sourcePath := readableClaimAssetURL(provider, storageBucket, item.SourceFilePath, cdn)
			processedPath := strings.TrimSpace(item.ProcessedFilePath)
			if processedPath == "" && status == model.VideoProcessStatusDone {
				processedPath = strings.TrimSpace(item.FilePath)
			}
			item.ProcessedFilePath = readableClaimAssetURL(provider, storageBucket, processedPath, cdn)
			item.FilePath = item.ProcessedFilePath
			if item.FilePath == "" && includeSourceVideo {
				item.FilePath = sourcePath
			}
			if includeSourceVideo {
				item.SourceFilePath = sourcePath
			} else {
				item.SourceFilePath = ""
			}
		} else {
			item.FilePath = normalizeClaimAssetURL(item.FilePath, cdn)
			item.ProcessedFilePath = normalizeClaimAssetURL(item.ProcessedFilePath, cdn)
			item.SourceFilePath = ""
		}

		item.ThumbnailPath = readableClaimAssetURL(provider, storageBucket, item.ThumbnailPath, cdn)
		result = append(result, &item)
	}
	return result
}

func summarizeMaterialProcessing(materials []*model.ClaimMaterial) gin.H {
	summary := gin.H{
		"total":      len(materials),
		"pending":    0,
		"processing": 0,
		"done":       0,
		"failed":     0,
	}

	for _, material := range materials {
		if material == nil {
			continue
		}
		status := strings.TrimSpace(material.ProcessStatus)
		if material.FileType != "video" && status == "" {
			status = model.VideoProcessStatusDone
		}
		switch status {
		case model.VideoProcessStatusPending:
			summary["pending"] = summary["pending"].(int) + 1
		case model.VideoProcessStatusProcessing:
			summary["processing"] = summary["processing"].(int) + 1
		case model.VideoProcessStatusFailed:
			summary["failed"] = summary["failed"].(int) + 1
		default:
			summary["done"] = summary["done"].(int) + 1
		}
	}
	return summary
}

func normalizeClaimAssetURL(path, cdn string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(cdn, "/") + path
}

func readableClaimAssetURL(provider storage.StorageProvider, bucket, path, cdn string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if provider != nil {
		if signedURL, err := storage.GetDownloadURL(context.Background(), provider, bucket, path, 2*time.Hour); err == nil && signedURL != "" {
			return signedURL
		}
	}
	return normalizeClaimAssetURL(path, cdn)
}

func configuredStorageBucket(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Storage.Provider)) {
	case "cos":
		if cfg.Storage.COS.AppID != "" && !strings.Contains(cfg.Storage.COS.Bucket, "-") {
			return cfg.Storage.COS.Bucket + "-" + cfg.Storage.COS.AppID
		}
		return cfg.Storage.COS.Bucket
	case "s3":
		return cfg.Storage.S3.Bucket
	case "oss":
		return cfg.Storage.OSS.Bucket
	case "rustfs":
		return cfg.Storage.RustFS.Bucket
	default:
		return ""
	}
}
