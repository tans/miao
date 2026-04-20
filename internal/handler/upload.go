package handler

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// UploadFile 上传文件
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

	// 获取文件类型参数（默认为 image）
	fileType := c.DefaultQuery("type", "image")

	// 检查文件类型和大小
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
		maxSize = 500 * 1024 * 1024 // 500MB
		errorMsg = "只支持视频格式 (mp4, mov, avi, wmv, flv, mkv, webm)"
	case "image":
		allowedExts = map[string]bool{
			".jpg":  true,
			".jpeg": true,
			".png":  true,
			".gif":  true,
			".webp": true,
		}
		maxSize = 5 * 1024 * 1024 // 5MB
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

	// 检查文件大小
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

	// 打开文件
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

	// 生成唯一文件名
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("%d_%s", timestamp, file.Filename)
	key := filepath.Join(fileType, filename)

	// 获取文件内容用于上传
	data, err := io.ReadAll(src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50004,
			Message: "读取文件内容失败",
			Data:    nil,
		})
		return
	}

	// 确定 Content-Type
	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 使用存储提供上传
	provider := GetStorageProvider()
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
			"size":     file.Size,
			"type":     fileType,
		},
	})
}
