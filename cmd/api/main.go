package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	adapterHTTP "github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http/middleware"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/repository"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
)

func main() {
	startTime := time.Now()

	dbUser := getEnv("DB_USER", "kanso_user")
	dbPass := getEnv("DB_PASSWORD", "secret")
	dbName := getEnv("DB_NAME", "kanso_db")
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, dbHost, dbPort, dbName)

	log.Println("Connecting to database...")

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		log.Fatalf("Critical: Failed to connect to database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Println("Database connected successfully.")

	jwtSecret := getEnv("JWT_SECRET", "change-me-in-production-super-secret-key")
	if jwtSecret == "change-me-in-production-super-secret-key" {
		log.Println("WARNING: Using default unsafe JWT secret")
	}
	jwtIssuer := getEnv("JWT_ISSUER", "kanso-api")
	jwtExpStr := getEnv("JWT_EXPIRATION", "24h")

	tokenDuration, err := time.ParseDuration(jwtExpStr)
	if err != nil {
		log.Fatalf("Critical: Invalid JWT_EXPIRATION format: %v", err)
	}

	habitRepo := repository.NewPostgresHabitRepository(db)
	entryRepo := repository.NewPostgresEntryRepository(db)
	userRepo := repository.NewPostgresUserRepository(db.DB)

	tokenService := services.NewTokenService(jwtSecret, jwtIssuer, tokenDuration)

	habitService := services.NewHabitService(habitRepo)
	entryService := services.NewEntryService(entryRepo, habitRepo)
	authService := services.NewAuthService(userRepo, tokenService)

	habitHandler := adapterHTTP.NewHabitHandler(habitService)
	entryHandler := adapterHTTP.NewEntryHandler(entryService)
	authHandler := adapterHTTP.NewAuthHandler(authService)

	serverPort := getEnv("PORT", "8080")

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
		if err := db.Ping(); err != nil {
			log.Printf("Health check failed: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "database": "unreachable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"database": "connected",
			"uptime":   time.Since(startTime).String(),
		})
	})

	apiV1 := router.Group("/api/v1")

	authHandler.RegisterRoutes(apiV1)

	protected := apiV1.Group("")
	protected.Use(middleware.AuthMiddleware(tokenService))
	{
		habitHandler.RegisterRoutes(protected)
		entryHandler.RegisterRoutes(protected)
	}

	srv := &http.Server{
		Addr:         ":" + serverPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Kanso Sync Engine running on port %s", serverPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Critical server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Stop signal received. Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Forced shutdown error:", err)
	}
	log.Println("Server stopped gracefully.")
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
