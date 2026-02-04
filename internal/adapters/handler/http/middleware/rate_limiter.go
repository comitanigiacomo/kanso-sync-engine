package middleware

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func RateLimiterMiddleware(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		key := fmt.Sprintf("rate_limit:%s", clientIP)

		count, err := rdb.Incr(c.Request.Context(), key).Result()
		if err != nil {
			log.Printf("Redis error (Rate Limiter skipped): %v", err)
			c.Next()
			return
		}

		if count == 1 {
			if err := rdb.Expire(c.Request.Context(), key, window).Err(); err != nil {
				log.Printf("Redis expire error: %v. Deleting key to avoid zombie.", err)
				rdb.Del(c.Request.Context(), key)
				c.Next()
				return
			}
		}

		ttl, err := rdb.TTL(c.Request.Context(), key).Result()
		if err != nil {
			ttl = window
		}

		resetTime := time.Now().Add(ttl).Unix()
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, int64(limit)-count)))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", resetTime))

		if count > int64(limit) {

			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"status":     "error",
				"message":    "Too many requests. Slow down!",
				"retry_in_s": int(ttl.Seconds()),
			})
			return
		}

		c.Next()
	}
}
