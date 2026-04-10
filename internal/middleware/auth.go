package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/tans/miao/internal/config"
)

var jwtSecret []byte

func init() {
	cfg := config.Load()
	jwtSecret = []byte(cfg.JWT.Secret)
}

// GenerateToken creates a JWT token for the given user
func GenerateToken(userID int64, username string, isAdmin bool) (string, error) {
	cfg := config.Load()
	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"is_admin": isAdmin,
		"exp":      time.Now().Add(cfg.JWT.ExpireTime).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// Claims represents JWT claims
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

// AuthMiddleware validates JWT tokens
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    40101,
				"message": "Authorization header required",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// Check Bearer prefix
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    40101,
				"message": "Invalid authorization format. Use: Bearer <token>",
				"data":    nil,
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Parse and validate token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    40102,
				"message": "Invalid or expired token",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// Extract claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    40102,
				"message": "Invalid token claims",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// Set user info in context
		userID, ok := claims["user_id"].(float64)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    40102,
				"message": "Invalid user_id in token",
				"data":    nil,
			})
			c.Abort()
			return
		}

		username, _ := claims["username"].(string)
		isAdmin, _ := claims["is_admin"].(bool)

		c.Set("user_id", int64(userID))
		c.Set("username", username)
		c.Set("is_admin", isAdmin)

		c.Next()
	}
}

// GetUserIDFromContext extracts user_id from gin context
func GetUserIDFromContext(c *gin.Context) (int64, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	id, ok := userID.(int64)
	return id, ok
}

// GetUsernameFromContext extracts username from gin context
func GetUsernameFromContext(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		return "", false
	}
	name, ok := username.(string)
	return name, ok
}

// GetIsAdminFromContext extracts is_admin from gin context
func GetIsAdminFromContext(c *gin.Context) (bool, bool) {
	isAdmin, exists := c.Get("is_admin")
	if !exists {
		return false, false
	}
	admin, ok := isAdmin.(bool)
	return admin, ok
}

// RequireAdmin checks if the user is an admin
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, ok := GetIsAdminFromContext(c)
		if !ok || !isAdmin {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    40301,
				"message": "Access denied - admin only",
				"data":    nil,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// MobilePageAuthMiddleware handles authentication for mobile HTML pages
// If not authenticated, redirects to mobile login page instead of returning JSON
func MobilePageAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// If no Authorization header, try to get token from cookie
		if authHeader == "" {
			if cookie, err := c.Cookie("token"); err == nil && cookie != "" {
				authHeader = "Bearer " + cookie
			}
		}

		if authHeader == "" {
			// Check if this is an HTML page request (browser navigation)
			accept := c.GetHeader("Accept")
			if strings.Contains(accept, "text/html") || c.Request.URL.Path != "/api/v1/" {
				// Redirect to mobile login with return URL
				c.Redirect(http.StatusFound, "/mobile/login")
				c.Abort()
				return
			}
			// API request without auth - return JSON error
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    40101,
				"message": "Authorization header required",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// Check Bearer prefix
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			if strings.Contains(c.GetHeader("Accept"), "text/html") {
				c.Redirect(http.StatusFound, "/mobile/login")
				c.Abort()
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    40101,
				"message": "Invalid authorization format. Use: Bearer <token>",
				"data":    nil,
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Parse and validate token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			if strings.Contains(c.GetHeader("Accept"), "text/html") {
				c.Redirect(http.StatusFound, "/mobile/login")
				c.Abort()
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    40102,
				"message": "Invalid or expired token",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// Extract claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    40102,
				"message": "Invalid token claims",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// Set user info in context
		userID, ok := claims["user_id"].(float64)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    40102,
				"message": "Invalid user_id in token",
				"data":    nil,
			})
			c.Abort()
			return
		}

		username, _ := claims["username"].(string)
		isAdmin, _ := claims["is_admin"].(bool)

		c.Set("user_id", int64(userID))
		c.Set("username", username)
		c.Set("is_admin", isAdmin)

		c.Next()
	}
}
