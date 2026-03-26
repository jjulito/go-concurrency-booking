package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// userIDKey is the gin context key under which the authenticated user's UUID is stored.
const userIDKey = "userID"

// AuthMiddleware reads the X-User-ID header and injects the parsed UUID into the
// gin context. In production this header is set by the API gateway after verifying
// the JWT — the service itself never handles raw tokens.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("X-User-ID")
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing X-User-ID header"})
			return
		}
		userID, err := uuid.Parse(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid X-User-ID header: must be a UUID"})
			return
		}
		c.Set(userIDKey, userID)
		c.Next()
	}
}

// rateLimitScript atomically increments the request counter and sets a 1-minute
// TTL only on the first request. Using a Lua script ensures the INCR and
// conditional EXPIRE execute as a single Redis operation — no race window where
// the key could exist without a TTL and block an IP address permanently.
var rateLimitScript = redis.NewScript(`
	local count = redis.call('INCR', KEYS[1])
	if count == 1 then
		redis.call('EXPIRE', KEYS[1], 60)
	end
	return count
`)

// RateLimiterMiddleware limits each IP to 60 requests per minute.
func RateLimiterMiddleware(redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("rate_limit:%s", c.ClientIP())

		count, err := rateLimitScript.Run(c.Request.Context(), redisClient, []string{key}).Int64()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal Error"})
			return
		}

		if count > 60 {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
			return
		}

		c.Next()
	}
}
