package http

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http/middleware"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
)

type RouterDependencies struct {
	AuthHandler  *AuthHandler
	HabitHandler *HabitHandler
	EntryHandler *EntryHandler
	TokenService *services.TokenService
	DB           *sqlx.DB
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

	router.GET("/health", func(c *gin.Context) {
		if err := deps.DB.Ping(); err != nil {
			c.JSON(503, gin.H{"status": "error", "database": "unreachable"})
			return
		}
		c.JSON(200, gin.H{
			"status":   "ok",
			"database": "connected",
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
