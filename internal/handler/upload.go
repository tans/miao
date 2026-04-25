package handler

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// UploadFile handles image and video uploads.
// POST /api/v1/upload
func UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "未选择文件",
			Data:    nil,
		})
		return
	}

	fileType := c.DefaultQuery("type", "image")
	ext := strings.ToLower(filepath.Ext(file.Filename))

	var allowedExts map[string]bool
	var maxSize int64
	var errorMsg string

	switch fileType {
	case "video":
		allowedExts = map[string]bool{
			".mp4":  true,
			".mov":  true,
			".avi":  true,
			".wmv":  true,
			".flv":  true,
			".mkv":  true,
			".webm": true,
		}
		maxSize = 500 * 1024 * 1024
		errorMsg = "只支持视频格式 (mp4, mov, avi, wmv, flv, mkv, webm)"
	case "image":
		allowedExts = map[string]bool{
			".jpg":  true,
			".jpeg": true,
			".png":  true,
			".gif":  true,
			".webp": true,
		}
		maxSize = 5 * 1024 * 1024
		errorMsg = "只支持图片格式 (jpg, jpeg, png, gif, webp)"
	default:
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "不支持的文件类型参数",
			Data:    nil,
		})
		return
	}

	if !allowedExts[ext] {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: errorMsg,
			Data:    nil,
		})
		return
	}

	if file.Size > maxSize {
		sizeLimit := "5MB"
		if fileType == "video" {
			sizeLimit = "500MB"
		}
		c.JSON(http.StatusBadRequest, Response{
			Code:    40003,
			Message: fmt.Sprintf("文件大小不能超过 %s", sizeLimit),
			Data:    nil,
		})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50003,
			Message: "打开文件失败",
			Data:    nil,
		})
		return
	}
	defer src.Close()

	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("%d_%s", timestamp, file.Filename)
	jobID := strings.TrimSpace(c.Query("job_id"))
	bizType := strings.TrimSpace(c.Query("biz_type"))
	bizID := strings.TrimSpace(c.Query("biz_id"))
	key := buildUploadKey(fileType, bizType, bizID, jobID, ext, filename)

	data, err := io.ReadAll(src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50004,
			Message: "读取文件内容失败",
			Data:    nil,
		})
		return
	}

	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	provider, err := GetStorageProvider()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "存储初始化失败: " + err.Error(),
			Data:    nil,
		})
		return
	}
	fileURL, err := provider.Upload(c.Request.Context(), key, bytes.NewReader(data), file.Size, contentType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50002,
			Message: "上传文件失败: " + err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "上传成功",
		Data: gin.H{
			"url":      fileURL,
			"filename": filename,
			"key":      key,
			"job_id":   jobID,
			"size":     file.Size,
			"type":     fileType,
		},
	})
}

func buildUploadKey(fileType, bizType, bizID, jobID, ext, fallbackFilename string) string {
	if fileType == "video" && bizType == "claim_source" && bizID != "" && jobID != "" {
		return path.Join("claim-source", bizID, jobID+ext)
	}
	return path.Join(fileType, fallbackFilename)
}

// GetCOSCredential returns a presigned URL for direct COS upload.
// GET /api/v1/cos/credential
func GetCOSCredential(c *gin.Context) {
	fileType := c.DefaultQuery("type", "image")
	ext := strings.TrimSpace(c.Query("ext"))
	jobID := strings.TrimSpace(c.Query("job_id"))
	bizType := strings.TrimSpace(c.Query("biz_type"))
	bizID := strings.TrimSpace(c.Query("biz_id"))

	// Validate file type
	if fileType != "image" && fileType != "video" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "不支持的文件类型",
			Data:    nil,
		})
		return
	}

	// Validate extension
	var allowedExts map[string]bool
	var contentType string
	switch fileType {
	case "video":
		allowedExts = map[string]bool{
			".mp4": true, ".mov": true, ".avi": true, ".wmv": true,
			".flv": true, ".mkv": true, ".webm": true,
		}
		contentType = "video/mp4"
	case "image":
		allowedExts = map[string]bool{
			".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
		}
		contentType = "image/jpeg"
	}
	if ext == "" || !allowedExts[ext] {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "无效的文件扩展名",
			Data:    nil,
		})
		return
	}

	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("%d_%s", timestamp, "upload"+ext)
	key := buildUploadKey(fileType, bizType, bizID, jobID, ext, filename)

	provider, err := GetStorageProvider()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "存储初始化失败: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// Generate presigned PUT URL, valid for 30 minutes
	signedURL, err := provider.GetUploadSignedURL(c.Request.Context(), key, contentType, 1800)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50002,
			Message: "生成上传凭证失败: " + err.Error(),
			Data:    nil,
		})
		return
	}

	fileURL, _ := provider.GetURL(c.Request.Context(), key)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"upload_url": signedURL,
			"key":        key,
			"file_url":   fileURL,
			"expires_in": 1800,
			"type":       fileType,
		},
	})
}
