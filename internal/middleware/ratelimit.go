package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/config"
)

// RateLimiter implements a sliding window rate limiter
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
	stopCh   chan struct{}
	stopped  bool
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
		stopCh:   make(chan struct{}),
	}
	// Start cleanup goroutine
	go rl.cleanup()
	return rl
}

// Allow checks if a request from the given key is allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	// Get existing requests for this key
	requests := rl.requests[key]

	// Filter out requests outside the window
	var validRequests []time.Time
	for _, t := range requests {
		if t.After(windowStart) {
			validRequests = append(validRequests, t)
		}
	}

	// Check if we're at the limit
	if len(validRequests) >= rl.limit {
		rl.requests[key] = validRequests
		return false
	}

	// Add the new request
	rl.requests[key] = append(validRequests, now)
	return true
}

// cleanup removes old entries periodically
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			windowStart := now.Add(-rl.window)
			for key, requests := range rl.requests {
				var validRequests []time.Time
				for _, t := range requests {
					if t.After(windowStart) {
						validRequests = append(validRequests, t)
					}
				}
				if len(validRequests) == 0 {
					delete(rl.requests, key)
				} else {
					rl.requests[key] = validRequests
				}
			}
			rl.mu.Unlock()
		}
	}
}

// Stop stops the cleanup goroutine
func (rl *RateLimiter) Stop() {
	rl.mu.Lock()
	if rl.stopped {
		rl.mu.Unlock()
		return
	}
	rl.stopped = true
	rl.mu.Unlock()
	close(rl.stopCh)
}

var (
	// DefaultRateLimiter is the default rate limiter
	DefaultRateLimiter *RateLimiter

	// StrictRateLimiter is a stricter rate limiter
	StrictRateLimiter *RateLimiter
)

// InitRateLimiters initializes the global rate limiters from config
func InitRateLimiters(cfg *config.RateLimitConfig) {
	if DefaultRateLimiter != nil {
		DefaultRateLimiter.Stop()
	}
	if StrictRateLimiter != nil {
		StrictRateLimiter.Stop()
	}
	DefaultRateLimiter = NewRateLimiter(cfg.DefaultLimit, cfg.DefaultWindow)
	StrictRateLimiter = NewRateLimiter(cfg.StrictLimit, cfg.StrictWindow)
}

// RateLimitMiddleware returns a middleware that rate limits by IP
func RateLimitMiddleware() gin.HandlerFunc {
	return RateLimitMiddlewareWithLimiter(DefaultRateLimiter)
}

// RateLimitMiddlewareWithLimiter returns a middleware with a custom rate limiter
func RateLimitMiddlewareWithLimiter(limiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if shouldBypassRateLimit(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Use IP address as the key
		key := c.ClientIP()

		if !limiter.Allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    42901,
				"message": "请求过于频繁，请稍后再试",
				"data":    nil,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func shouldBypassRateLimit(path string) bool {
	return strings.HasPrefix(path, "/api/v1/admin")
}

// StrictRateLimitMiddleware returns a stricter rate limiting middleware
func StrictRateLimitMiddleware() gin.HandlerFunc {
	if StrictRateLimiter == nil {
		InitRateLimiters(&config.RateLimitConfig{DefaultLimit: 100, DefaultWindow: time.Minute, StrictLimit: 20, StrictWindow: time.Minute})
	}
	return RateLimitMiddlewareWithLimiter(StrictRateLimiter)
}

// IPRateLimitByEndpoint returns a rate limiter for a specific endpoint
func IPRateLimitByEndpoint(limit int, window time.Duration) gin.HandlerFunc {
	limiter := NewRateLimiter(limit, window)
	return RateLimitMiddlewareWithLimiter(limiter)
}
