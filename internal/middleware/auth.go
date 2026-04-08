package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/tans/miao/internal/config"
)

var jwtSecret []byte

func init() {
	cfg := config.Load()
	jwtSecret = []byte(cfg.JWT.Secret)
}

// Claims represents JWT claims
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
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
		role, _ := claims["role"].(string)

		c.Set("user_id", int64(userID))
		c.Set("username", username)
		c.Set("role", role)

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

// GetRoleFromContext extracts role from gin context
func GetRoleFromContext(c *gin.Context) (string, bool) {
	role, exists := c.Get("role")
	if !exists {
		return "", false
	}
	r, ok := role.(string)
	return r, ok
}

// RequireRole checks if the user has the required role
// Supports multi-role format like "business,creator"
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, ok := GetRoleFromContext(c)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    40301,
				"message": "Access denied - no role in context",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// Split user roles by comma to support multi-role format
		userRoles := strings.Split(userRole, ",")

		for _, role := range roles {
			for _, ur := range userRoles {
				if strings.TrimSpace(ur) == role {
					c.Next()
					return
				}
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"code":    40302,
			"message": "Insufficient permissions - user role: " + userRole + ", required: " + strings.Join(roles, ","),
			"data":    nil,
		})
		c.Abort()
	}
}
