package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func setupTestRedis(t *testing.T) *redis.Client {
	_ = godotenv.Load("../../../../../.env")

	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}
	pass := os.Getenv("REDIS_PASSWORD")
	if pass == "" {
		pass = "secret_redis_pass_local"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: pass,
		DB:       1,
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("Skipping integration test (Redis down): %v", err)
	}

	rdb.FlushDB(ctx)
	return rdb
}

func TestRateLimiterMiddleware_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rdb := setupTestRedis(t)
	defer rdb.Close()

	ctx := context.Background()

	t.Run("Allow Requests under limit", func(t *testing.T) {
		rdb.FlushDB(ctx)

		limit := 5
		router := gin.New()
		router.Use(RateLimiterMiddleware(rdb, limit, 1*time.Minute))
		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		for i := 1; i <= limit; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Forwarded-For", "192.168.1.100")

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, fmt.Sprintf("%d", limit), w.Header().Get("X-RateLimit-Limit"))
			assert.Equal(t, fmt.Sprintf("%d", limit-i), w.Header().Get("X-RateLimit-Remaining"))
		}
	})

	t.Run("Block Requests over limit", func(t *testing.T) {
		rdb.FlushDB(ctx)

		limit := 2
		router := gin.New()
		router.Use(RateLimiterMiddleware(rdb, limit, 1*time.Minute))
		router.GET("/test-block", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		ip := "192.168.1.101"

		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest("GET", "/test-block", nil)
		req1.Header.Set("X-Forwarded-For", ip)
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code, "Request 1 should pass")

		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("GET", "/test-block", nil)
		req2.Header.Set("X-Forwarded-For", ip)
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code, "Request 2 should pass")

		w3 := httptest.NewRecorder()
		req3, _ := http.NewRequest("GET", "/test-block", nil)
		req3.Header.Set("X-Forwarded-For", ip)
		router.ServeHTTP(w3, req3)

		assert.Equal(t, http.StatusTooManyRequests, w3.Code, "Request 3 should be blocked")
		assert.Contains(t, w3.Body.String(), "Too many requests")
	})

	t.Run("Fail Open (Redis Down)", func(t *testing.T) {
		badRdb := redis.NewClient(&redis.Options{
			Addr: "localhost:9999",
		})

		router := gin.New()
		router.Use(RateLimiterMiddleware(badRdb, 5, 1*time.Minute))
		router.GET("/test-fail-open", func(c *gin.Context) {
			c.String(http.StatusOK, "passed")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test-fail-open", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "passed", w.Body.String())
	})
}
