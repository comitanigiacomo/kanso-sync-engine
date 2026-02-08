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

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/cache"
	adapterHTTP "github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/repository"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/workers"
)

// @title           Kanso Sync Engine API
// @version         1.0
// @description     Backend for the Kanso Habit Tracker application.
// @description     Features include offline-first sync, streak calculation, and JWT authentication.

// @contact.name   Giacomo Comitani
// @contact.url    https://github.com/comitanigiacomo

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and the JWT token.
func main() {
	startTime := time.Now()

	dbUser := getEnv("DB_USER", "kanso_user")
	dbPass := getEnv("DB_PASSWORD", "secret")
	dbName := getEnv("DB_NAME", "kanso_db")
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	serverPort := getEnv("PORT", "8080")

	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	redisPass := getEnv("REDIS_PASSWORD", "secret_redis_pass_local")

	jwtSecret := getEnv("JWT_SECRET", "change-me-in-production-super-secret-key")
	jwtIssuer := getEnv("JWT_ISSUER", "kanso-api")
	jwtExpStr := getEnv("JWT_EXPIRATION", "24h")

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

	log.Println("Connecting to Redis...")
	rdb, err := cache.NewRedisClient(redisHost, redisPort, redisPass, 0)
	if err != nil {
		log.Fatalf("Critical: Failed to connect to Redis: %v", err)
	}
	defer rdb.Close()
	log.Println("Redis connected successfully.")

	tokenDuration, err := time.ParseDuration(jwtExpStr)
	if err != nil {
		log.Fatalf("Invalid JWT duration: %v", err)
	}

	habitRepoPostgres := repository.NewPostgresHabitRepository(db)
	entryRepo := repository.NewPostgresEntryRepository(db)
	userRepo := repository.NewPostgresUserRepository(db.DB)

	habitRepoCached := repository.NewCachedHabitRepository(habitRepoPostgres, rdb)

	streakWorker := workers.NewStreakWorker(habitRepoCached, entryRepo)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	streakWorker.Start(workerCtx)

	tokenService := services.NewTokenService(jwtSecret, jwtIssuer, tokenDuration)

	habitService := services.NewHabitService(habitRepoCached)

	authService := services.NewAuthService(userRepo, tokenService)

	entryService := services.NewEntryService(entryRepo, habitRepoCached, streakWorker)
	statsService := services.NewStatsService(habitRepoCached, entryRepo)

	habitHandler := adapterHTTP.NewHabitHandler(habitService)
	entryHandler := adapterHTTP.NewEntryHandler(entryService)
	authHandler := adapterHTTP.NewAuthHandler(authService)
	statsHandler := adapterHTTP.NewStatsHandler(statsService)

	router := adapterHTTP.NewRouter(adapterHTTP.RouterDependencies{
		AuthHandler:  authHandler,
		HabitHandler: habitHandler,
		EntryHandler: entryHandler,
		StatsHandler: statsHandler,
		TokenService: tokenService,
		DB:           db,
		Redis:        rdb,
		StartTime:    startTime,
	})

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

	log.Println("Shutting down server...")

	workerCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	log.Println("Server exited properly.")
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
