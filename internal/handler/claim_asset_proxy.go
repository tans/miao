package handler

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/storage"
)

func DownloadClaimAsset(c *gin.Context) {
	cfg := config.Load()
	rawPath, err := storage.VerifyProxyDownloadURL(
		cfg.JWT.Secret,
		c.Query("asset"),
		c.Query("expires"),
		c.Query("signature"),
	)
	if err != nil {
		c.JSON(http.StatusForbidden, ErrorResponse(CodeForbidden, "访问链接已失效"))
		return
	}

	key := storage.ExtractObjectKey(rawPath, configuredStorageBucket(cfg))
	if !storage.IsClaimAssetKey(key) {
		c.JSON(http.StatusForbidden, ErrorResponse(CodeForbidden, "不支持的资源"))
		return
	}

	provider, err := GetStorageProvider()
	if err != nil || provider == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(CodeServiceError, "存储服务不可用"))
		return
	}

	downloader, ok := provider.(storage.ObjectDownloader)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(CodeServiceError, "当前存储不支持代理下载"))
		return
	}

	result, err := downloader.DownloadObject(c.Request.Context(), key, c.GetHeader("Range"))
	if err != nil || result == nil || result.Body == nil {
		c.JSON(http.StatusBadGateway, ErrorResponse(CodeServiceError, "资源读取失败"))
		return
	}
	defer result.Body.Close()

	if contentType := strings.TrimSpace(result.ContentType); contentType != "" {
		c.Header("Content-Type", contentType)
	}
	if result.ContentLen >= 0 {
		c.Header("Content-Length", strconv.FormatInt(result.ContentLen, 10))
	}
	if contentRange := strings.TrimSpace(result.ContentRange); contentRange != "" {
		c.Header("Content-Range", contentRange)
	}
	if acceptRanges := strings.TrimSpace(result.AcceptRanges); acceptRanges != "" {
		c.Header("Accept-Ranges", acceptRanges)
	} else {
		c.Header("Accept-Ranges", "bytes")
	}
	if etag := strings.TrimSpace(result.ETag); etag != "" {
		c.Header("ETag", etag)
	}
	c.Header("Cache-Control", "private, max-age=300")

	statusCode := result.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	c.Status(statusCode)
	if c.Request.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(c.Writer, result.Body)
}
