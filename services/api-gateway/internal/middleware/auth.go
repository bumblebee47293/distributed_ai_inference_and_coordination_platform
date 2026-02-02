package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Auth middleware validates JWT tokens or API keys
func Auth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		// Support both "Bearer <token>" and direct API key
		parts := strings.Split(authHeader, " ")
		var token string
		
		if len(parts) == 2 && parts[0] == "Bearer" {
			token = parts[1]
		} else {
			token = authHeader
		}

		// For demo purposes, accept "demo-token" as valid
		if token == "demo-token" {
			c.Set("user_id", "demo-user")
			c.Next()
			return
		}

		// Validate JWT
		claims := jwt.MapClaims{}
		parsedToken, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !parsedToken.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		// Extract user ID from claims
		if userID, ok := claims["user_id"].(string); ok {
			c.Set("user_id", userID)
		}

		c.Next()
	}
}
