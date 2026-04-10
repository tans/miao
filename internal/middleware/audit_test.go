package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAuditMiddlewareSensitive(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		path           string
		method         string
		authUserID     uint
		expectedCode   int
	}{
		{
			name:         "POST /api/v1/business/tasks 应该审计",
			path:         "/api/v1/business/tasks",
			method:       "POST",
			authUserID:   1,
			expectedCode: http.StatusOK,
		},
		{
			name:         "POST /api/v1/creator/claim 应该审计",
			path:         "/api/v1/creator/claim",
			method:       "POST",
			authUserID:   2,
			expectedCode: http.StatusOK,
		},
		{
			name:         "GET /api/v1/tasks 不应该审计",
			path:         "/api/v1/tasks",
			method:       "GET",
			authUserID:   0,
			expectedCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				if tt.authUserID > 0 {
					c.Set("user_id", tt.authUserID)
				}
				c.Next()
			})
			router.Use(AuditMiddlewareSensitive())
			router.POST("/api/v1/business/tasks", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})
			router.POST("/api/v1/creator/claim", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})
			router.GET("/api/v1/tasks", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBuffer([]byte(`{}`)))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)
		})
	}
}

func TestAuditLogJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(AuditMiddlewareSensitive())
	router.POST("/api/v1/business/tasks", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"task_id": 1})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/business/tasks", bytes.NewBuffer([]byte(`{"title":"test"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TestAgent")
	req.RemoteAddr = "192.168.1.1:12345"

	// Set user_id via context
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Next()
	})

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify response
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), response["task_id"])
}

func TestAuditMiddleware_AnonymousUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(AuditMiddlewareSensitive())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	// No user_id set in context
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuditMiddleware_NoUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	// No auth middleware, so no user_id
	router.Use(AuditMiddlewareSensitive())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
