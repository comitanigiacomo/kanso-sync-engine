package http

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http/middleware"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
)

type RouterDependencies struct {
	AuthHandler  *AuthHandler
	HabitHandler *HabitHandler
	EntryHandler *EntryHandler
	TokenService *services.TokenService
	DB           *sqlx.DB
	Redis        *redis.Client
	StartTime    time.Time
}

func NewRouter(deps RouterDependencies) *gin.Engine {
	router := gin.Default()

	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	if deps.Redis != nil {
		router.Use(middleware.RateLimiterMiddleware(deps.Redis, 100, 1*time.Minute))
	}

	router.GET("/health", func(c *gin.Context) {
		dbStatus := "connected"
		if err := deps.DB.Ping(); err != nil {
			dbStatus = "unreachable"
		}

		redisStatus := "connected"
		if deps.Redis == nil || deps.Redis.Ping(c.Request.Context()).Err() != nil {
			redisStatus = "unreachable"
		}

		statusCode := 200
		if dbStatus == "unreachable" || redisStatus == "unreachable" {
			statusCode = 503
		}

		c.JSON(statusCode, gin.H{
			"status":   "ok",
			"database": dbStatus,
			"redis":    redisStatus,
			"uptime":   time.Since(deps.StartTime).String(),
		})
	})

	apiV1 := router.Group("/api/v1")

	deps.AuthHandler.RegisterRoutes(apiV1)

	protected := apiV1.Group("")
	protected.Use(middleware.AuthMiddleware(deps.TokenService))
	{
		deps.HabitHandler.RegisterRoutes(protected)
		deps.EntryHandler.RegisterRoutes(protected)
	}

	return router
}
