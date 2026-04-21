package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

const (
	csrfCookieName = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
	csrfFormName   = "csrf_token"
	sessionCSRFKey = "csrf_token"
)

// CSRFConfig holds CSRF middleware configuration
type CSRFConfig struct {
	CookieName     string
	HeaderName     string
	FormName       string
	CookieHTTPOnly bool
	CookieSecure   bool
}

var defaultCSRFConfig = CSRFConfig{
	CookieName:     csrfCookieName,
	HeaderName:     csrfHeaderName,
	FormName:       csrfFormName,
	CookieHTTPOnly: false,
	CookieSecure:   false,
}

// CSRFTokenGenerator generates random CSRF tokens
type CSRFTokenGenerator struct {
	mu      sync.Mutex
	entropy int
}

var tokenGen = &CSRFTokenGenerator{entropy: 32}

// GenerateToken creates a new random CSRF token
func (g *CSRFTokenGenerator) GenerateToken() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	bytes := make([]byte, g.entropy)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateCSRFToken is a convenience function to generate a CSRF token
func GenerateCSRFToken() (string, error) {
	return tokenGen.GenerateToken()
}

// CSRFProtection returns a middleware that provides CSRF protection
// It generates tokens on GET requests and validates them on state-changing requests
func CSRFProtection(config ...CSRFConfig) gin.HandlerFunc {
	cfg := defaultCSRFConfig
	if len(config) > 0 {
		cfg = config[0]
	}

	return func(c *gin.Context) {
		// Only apply CSRF protection to state-changing methods
		method := c.Request.Method
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			// For GET requests, generate/refresh CSRF token if user is authenticated
			if _, exists := c.Get("user_id"); exists {
				token, err := GenerateCSRFToken()
				if err == nil {
					c.Set(sessionCSRFKey, token)
					c.SetCookie(
						cfg.CookieName,
						token,
						3600*24, // 24 hours
						"/",
						"",
						cfg.CookieSecure,
						cfg.CookieHTTPOnly,
					)
					// Set SameSite header via response
					c.Header("Set-Cookie", cfg.CookieName+"="+token+"; Path=/; Max-Age=86400; SameSite=Lax")
				}
			}
			c.Next()
			return
		}

		// For state-changing methods, validate CSRF token
		var token string

		// Check header first (preferred)
		if header := c.GetHeader(cfg.HeaderName); header != "" {
			token = header
		}

		// Fall back to form field
		if token == "" {
			token = c.PostForm(cfg.FormName)
		}

		// Fall back to cookie (double-submit cookie pattern)
		if token == "" {
			if cookie, err := c.Cookie(cfg.CookieName); err == nil {
				token = cookie
			}
		}

		// Get the expected token from context/session
		expectedToken, exists := c.Get(sessionCSRFKey)
		if !exists {
			// If no token in session, try to validate against cookie directly
			// This handles cases where session wasn't set properly
			expectedToken, _ = c.Cookie(cfg.CookieName)
		}

		if expectedToken == nil || token == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    40301,
				"message": "CSRF token missing",
				"data":    nil,
			})
			return
		}

		expectedTokenStr, ok := expectedToken.(string)
		if !ok || token != expectedTokenStr {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    40302,
				"message": "Invalid CSRF token",
				"data":    nil,
			})
			return
		}

		c.Next()
	}
}

// CSRFTokenHandler returns the current CSRF token as JSON
func CSRFTokenHandler(c *gin.Context) {
	token, exists := c.Get(sessionCSRFKey)
	if !exists {
		// Generate new token if none exists
		var err error
		token, err = GenerateCSRFToken()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    50001,
				"message": "Failed to generate CSRF token",
				"data":    nil,
			})
			return
		}
		c.Set(sessionCSRFKey, token)
	}

	tokenStr, ok := token.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    50001,
			"message": "Invalid CSRF token",
			"data":    nil,
		})
		return
	}

	// Set cookie with SameSite=Lax
	c.SetCookie(
		csrfCookieName,
		tokenStr,
		3600*24,
		"/",
		"",
		false,
		false,
	)
	c.Header("Set-Cookie", csrfCookieName+"="+tokenStr+"; Path=/; Max-Age=86400; SameSite=Lax")

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"csrf_token": tokenStr,
		},
	})
}

// SetCSRFCookie sets the CSRF cookie directly (for use after login)
func SetCSRFCookie(c *gin.Context, token string) {
	c.SetCookie(
		csrfCookieName,
		token,
		3600*24,
		"/",
		"",
		false,
		false,
	)
	c.Header("Set-Cookie", csrfCookieName+"="+token+"; Path=/; Max-Age=86400; SameSite=Lax")
}

// GetCSRFTokenFromCookie retrieves the CSRF token from cookie
func GetCSRFTokenFromCookie(c *gin.Context) string {
	if cookie, err := c.Cookie(csrfCookieName); err == nil {
		return cookie
	}
	return ""
}
