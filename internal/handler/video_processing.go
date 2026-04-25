package handler

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/model"
	"net/http"
	"strings"
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
	cfg := config.Load()
	cdn := cfg.Static.CDN
	if cdn == "" {
		cdn = cfg.Static.Host
	}

	result := make([]*model.ClaimMaterial, 0, len(materials))
	for _, material := range materials {
		if material == nil {
			continue
		}
		item := *material
		item.SourceFilePath = ""

		status := strings.TrimSpace(item.ProcessStatus)
		if item.FileType == "video" && status == "" {
			status = model.VideoProcessStatusDone
		}
		item.ProcessStatus = status

		if item.FileType == "video" {
			displayPath := strings.TrimSpace(item.ProcessedFilePath)
			if displayPath == "" && status == model.VideoProcessStatusDone {
				displayPath = strings.TrimSpace(item.FilePath)
			}
			item.ProcessedFilePath = normalizeClaimAssetURL(displayPath, cdn)
			item.FilePath = item.ProcessedFilePath
			if status != model.VideoProcessStatusDone {
				item.FilePath = ""
			}
		} else {
			item.FilePath = normalizeClaimAssetURL(item.FilePath, cdn)
			item.ProcessedFilePath = normalizeClaimAssetURL(item.ProcessedFilePath, cdn)
		}

		item.ThumbnailPath = normalizeClaimAssetURL(item.ThumbnailPath, cdn)
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
