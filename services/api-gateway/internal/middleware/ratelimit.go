package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimit implements token bucket rate limiting using Redis
func RateLimit(redisClient *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by Auth middleware)
		userID, exists := c.Get("user_id")
		if !exists {
			userID = c.ClientIP() // Fallback to IP if no user ID
		}

		key := fmt.Sprintf("ratelimit:%v", userID)
		ctx := context.Background()

		// Increment counter
		count, err := redisClient.Incr(ctx, key).Result()
		if err != nil {
			// If Redis is down, allow the request (fail open)
			c.Next()
			return
		}

		// Set expiry on first request
		if count == 1 {
			redisClient.Expire(ctx, key, window)
		}

		// Check if limit exceeded
		if count > int64(limit) {
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(window).Unix()))
			
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
				"retry_after": window.Seconds(),
			})
			c.Abort()
			return
		}

		// Set rate limit headers
		remaining := limit - int(count)
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		c.Next()
	}
}
