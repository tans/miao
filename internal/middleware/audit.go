package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	Timestamp   time.Time `json:"timestamp"`
	UserID      int64     `json:"user_id,omitempty"`
	Username    string    `json:"username,omitempty"`
	ClientIP   string    `json:"client_ip"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Query      string    `json:"query,omitempty"`
	Body       string    `json:"body,omitempty"`
	StatusCode int       `json:"status_code"`
	Latency    string    `json:"latency"`
	UserAgent  string    `json:"user_agent"`
	IsAdmin    bool      `json:"is_admin"`
}

// AuditLogger logs audit information
var AuditLogger = log.New(log.Writer(), "[AUDIT] ", log.LstdFlags)

// AuditMiddleware returns a middleware that logs all requests
func AuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// Get request body
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			if bodyBytes != nil {
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get user info from context
		userID, _ := GetUserIDFromContext(c)
		username, _ := GetUsernameFromContext(c)
		isAdmin, _ := GetIsAdminFromContext(c)

		// Create audit log entry
		auditLog := AuditLog{
			Timestamp:   start,
			UserID:      userID,
			Username:    username,
			ClientIP:    c.ClientIP(),
			Method:      c.Request.Method,
			Path:        c.Request.URL.Path,
			Query:       c.Request.URL.RawQuery,
			StatusCode:  c.Writer.Status(),
			Latency:     latency.String(),
			UserAgent:   c.Request.UserAgent(),
			IsAdmin:     isAdmin,
		}

		// Only log body for non-GET requests and sensitive endpoints
		if c.Request.Method != "GET" && len(bodyBytes) > 0 {
			// Truncate body if too large (max 1KB)
			if len(bodyBytes) > 1024 {
				bodyBytes = bodyBytes[:1024]
			}
			auditLog.Body = string(bodyBytes)
		}

		// Log as JSON
		logBytes, _ := json.Marshal(auditLog)
		AuditLogger.Println(string(logBytes))
	}
}

// SensitivePathCheck checks if a path is sensitive and should be audited
func SensitivePathCheck(path string) bool {
	sensitivePaths := []string{
		"/api/v1/admin",
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/api/v1/user/password",
		"/api/v1/business",
		"/api/v1/creator",
		"/api/v1/transactions",
	}

	for _, p := range sensitivePaths {
		if len(path) >= len(p) && path[:len(p)] == p {
			return true
		}
	}
	return false
}

// AuditMiddlewareSensitive returns a middleware that only audits sensitive endpoints
func AuditMiddlewareSensitive() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if this is a sensitive path
		if SensitivePathCheck(c.Request.URL.Path) {
			// Start timer
			start := time.Now()

			// Get request body
			var bodyBytes []byte
			if c.Request.Body != nil {
				bodyBytes, _ = io.ReadAll(c.Request.Body)
				if bodyBytes != nil {
					c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				}
			}

			// Process request
			c.Next()

			// Calculate latency
			latency := time.Since(start)

			// Get user info from context
			userID, _ := GetUserIDFromContext(c)
			username, _ := GetUsernameFromContext(c)
			isAdmin, _ := GetIsAdminFromContext(c)

			// Create audit log entry
			auditLog := AuditLog{
				Timestamp:   start,
				UserID:      userID,
				Username:    username,
				ClientIP:    c.ClientIP(),
				Method:      c.Request.Method,
				Path:        c.Request.URL.Path,
				Query:       c.Request.URL.RawQuery,
				StatusCode:  c.Writer.Status(),
				Latency:     latency.String(),
				UserAgent:   c.Request.UserAgent(),
				IsAdmin:     isAdmin,
			}

			// Only log body for non-GET requests
			if c.Request.Method != "GET" && len(bodyBytes) > 0 {
				if len(bodyBytes) > 1024 {
					bodyBytes = bodyBytes[:1024]
				}
				auditLog.Body = string(bodyBytes)
			}

			// Log as JSON
			logBytes, _ := json.Marshal(auditLog)
			AuditLogger.Println(string(logBytes))
		} else {
			c.Next()
		}
	}
}
