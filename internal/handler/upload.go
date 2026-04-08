package handler

import (
	"fmt"
	"net/http"
	"os"
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

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}

	if !allowedExts[ext] {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "只支持图片格式 (jpg, jpeg, png, gif, webp)",
			Data:    nil,
		})
		return
	}

	// 检查文件大小 (最大 5MB)
	if file.Size > 5*1024*1024 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40003,
			Message: "文件大小不能超过 5MB",
			Data:    nil,
		})
		return
	}

	// 生成唯一文件名
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("%d_%s", timestamp, file.Filename)

	// 确保上传目录存在
	uploadDir := filepath.Join("web", "static", "uploads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "创建上传目录失败",
			Data:    nil,
		})
		return
	}

	// 保存文件
	filePath := filepath.Join(uploadDir, filename)
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50002,
			Message: "保存文件失败",
			Data:    nil,
		})
		return
	}

	// 返回文件 URL
	fileURL := fmt.Sprintf("/static/uploads/%s", filename)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "上传成功",
		Data: gin.H{
			"url": fileURL,
		},
	})
}
