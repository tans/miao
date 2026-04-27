package handler

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/storage"
)

// AssetPreview proxies stored assets through the API domain so mini programs
// can render images without whitelisting the COS host.
// GET /api/v1/assets/preview?raw=...
func AssetPreview(c *gin.Context) {
	raw := strings.TrimSpace(c.Query("raw"))
	if raw == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40001,
			Message: "缺少资源地址",
			Data:    nil,
		})
		return
	}

	cfg := config.Load()
	bucket := configuredStorageBucket(cfg)
	key := storage.ExtractObjectKey(raw, bucket)
	if key == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    40002,
			Message: "无效的资源地址",
			Data:    nil,
		})
		return
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

	signedURL, err := storage.GetDownloadURL(c.Request.Context(), provider, bucket, raw, 2*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50002,
			Message: "生成资源链接失败: " + err.Error(),
			Data:    nil,
		})
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, signedURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50003,
			Message: "创建请求失败: " + err.Error(),
			Data:    nil,
		})
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, Response{
			Code:    50201,
			Message: "拉取资源失败: " + err.Error(),
			Data:    nil,
		})
		return
	}
	defer resp.Body.Close()

	copyHeaders := []string{
		"Content-Type",
		"Content-Length",
		"Cache-Control",
		"ETag",
		"Last-Modified",
		"Content-Disposition",
	}
	for _, header := range copyHeaders {
		if value := resp.Header.Get(header); value != "" {
			c.Header(header, value)
		}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		c.Header("Cache-Control", "public, max-age=2592000, immutable")
	}

	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}
