package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	MaxConcurrentUsers = 100 // Example limit
	QueueKey           = "waiting_room:queue"
)

// VirtualQueueMiddleware limits concurrent access by checking the total number of active users.
// If the limit is reached, it returns a ServiceUnavailable status.
func VirtualQueueMiddleware(redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		
		// Check current system load using a global counter
		activeUsers, err := redisClient.Get(ctx, "active_users").Int()
		if err != nil && err != redis.Nil {
			// Fail securely on Redis error
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Service busy"})
			return
		}

		if activeUsers >= MaxConcurrentUsers {
			// Redirect to waiting room or return 503
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "Server is full. Please try again later.",
				"waiting_room_url": "/queue",
			})
			return
		}

		// Increment active user count
		redisClient.Incr(ctx, "active_users")

		c.Next()
		
		// Decrement count when request completes
		go redisClient.Decr(context.Background(), "active_users")
	}
}

// Simple Rate Limiter Middleware
func RateLimiterMiddleware(redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := fmt.Sprintf("rate_limit:%s", ip)
		
		// Allow 10 requests per minute
		count, err := redisClient.Incr(c.Request.Context(), key).Result()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal Error"})
			return
		}
		
		if count == 1 {
			redisClient.Expire(c.Request.Context(), key, 1*time.Minute)
		}
		
		if count > 60 {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
			return
		}
		
		c.Next()
	}
}
