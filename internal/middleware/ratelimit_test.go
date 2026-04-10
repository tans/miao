package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRateLimiter_Allow(t *testing.T) {
	tests := []struct {
		name          string
		limit         int
		window        time.Duration
		requests      int
		expectedAllow int // number of allowed requests
	}{
		{
			name:          "容量为2的桶",
			limit:         2,
			window:        time.Minute,
			requests:      3,
			expectedAllow: 2,
		},
		{
			name:          "容量为5的桶",
			limit:         5,
			window:        time.Minute,
			requests:      7,
			expectedAllow: 5,
		},
		{
			name:          "容量为10的桶",
			limit:         10,
			window:        time.Minute,
			requests:      15,
			expectedAllow: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewRateLimiter(tt.limit, tt.window)

			allowed := 0
			for i := 0; i < tt.requests; i++ {
				if limiter.Allow("test_ip") {
					allowed++
				}
			}

			assert.Equal(t, tt.expectedAllow, allowed)
		})
	}
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)

	// IP1: should get 2 successful requests
	assert.True(t, limiter.Allow("192.168.1.1"))
	assert.True(t, limiter.Allow("192.168.1.1"))
	assert.False(t, limiter.Allow("192.168.1.1")) // 3rd should be blocked

	// IP2: should also get 2 successful requests (different key)
	assert.True(t, limiter.Allow("192.168.1.2"))
	assert.True(t, limiter.Allow("192.168.1.2"))
	assert.False(t, limiter.Allow("192.168.1.2")) // 3rd should be blocked
}

func TestRateLimitMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		requestCount   int
		expectedStatus int
	}{
		{
			name:           "前10个请求应该成功",
			requestCount:   10,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "超出限制的请求应该被限流",
			requestCount:   150,
			expectedStatus: http.StatusTooManyRequests,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)

			// Create a fresh limiter for each test
			limiter := NewRateLimiter(100, time.Minute)

			router := gin.New()
			router.Use(RateLimitMiddlewareWithLimiter(limiter))
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			// Make requests and check the last one
			for i := 0; i < tt.requestCount-1; i++ {
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "192.168.1.1:12345"
				router.ServeHTTP(w, req)
			}

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestRateLimitMiddleware_DifferentIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := NewRateLimiter(5, time.Minute)

	router := gin.New()
	router.Use(RateLimitMiddlewareWithLimiter(limiter))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// IP 1: should get 5 successful requests
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "IP1 request %d should succeed", i+1)
	}

	// 6th request from IP1 should be blocked
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// IP 2: should also get 5 successful requests (different IP)
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.2:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "IP2 request %d should succeed", i+1)
	}
}

func TestStrictRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(StrictRateLimitMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First 20 requests should succeed
	for i := 0; i < 20; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
	}

	// 21st request should be blocked
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestIPRateLimitByEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create endpoint-specific limiter with 3 req/min
	router := gin.New()
	router.Use(IPRateLimitByEndpoint(3, time.Minute))
	router.POST("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// 4th request should be blocked
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestRateLimitResponseFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := NewRateLimiter(1, time.Minute)

	router := gin.New()
	router.Use(RateLimitMiddlewareWithLimiter(limiter))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First request succeeds
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w, req)

	// Second request is rate limited
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusTooManyRequests, w2.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w2.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(42901), response["code"])
	assert.Equal(t, "请求过于频繁，请稍后再试", response["message"])
}
