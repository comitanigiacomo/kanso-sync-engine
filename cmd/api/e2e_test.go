package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adapterHTTP "github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http/middleware"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/repository"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/workers"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type idResponse struct {
	ID string `json:"id"`
}

type listEntryResponse []struct {
	ID    string `json:"id"`
	Value int    `json:"value"`
}

type loginResponse struct {
	Token string `json:"token"`
	User  struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

func setupTestDB(t *testing.T) *sqlx.DB {
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "kanso_user"
	}
	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		dbPass = "secret"
	}
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "kanso_db"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, dbHost, dbPort, dbName)

	db, err := sqlx.Connect("pgx", dsn)
	require.NoError(t, err, "Failed to connect to test database")

	_, err = db.Exec("DROP TABLE IF EXISTS habit_entries, habits, users CASCADE")
	require.NoError(t, err, "Failed to drop tables")

	schema := `
    CREATE TABLE users (
        id TEXT PRIMARY KEY,
        email TEXT UNIQUE NOT NULL,
        password_hash TEXT NOT NULL,
        created_at TIMESTAMP WITH TIME ZONE NOT NULL,
        updated_at TIMESTAMP WITH TIME ZONE NOT NULL
    );

    CREATE TABLE habits (
        id TEXT PRIMARY KEY,
        user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        title TEXT NOT NULL,
        description TEXT,
        color TEXT,
        icon TEXT,
        type TEXT NOT NULL,
        reminder_time TEXT,
        unit TEXT,
        target_value INTEGER,
        interval INTEGER,
        weekdays TEXT, -- JSON TEXT
        frequency_type TEXT,
        
        start_date TIMESTAMP WITH TIME ZONE,
        end_date TIMESTAMP WITH TIME ZONE,
        archived_at TIMESTAMP WITH TIME ZONE,

        created_at TIMESTAMP WITH TIME ZONE NOT NULL,
        updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
        deleted_at TIMESTAMP WITH TIME ZONE,
        version INTEGER DEFAULT 1,
        sort_order INTEGER DEFAULT 0,
        current_streak INTEGER DEFAULT 0,
        longest_streak INTEGER DEFAULT 0
    );

    CREATE TABLE habit_entries (
        id TEXT PRIMARY KEY,
        habit_id TEXT NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
        user_id TEXT NOT NULL,
        value INTEGER NOT NULL,
        notes TEXT,
        completion_date TIMESTAMP WITH TIME ZONE NOT NULL,
        created_at TIMESTAMP WITH TIME ZONE NOT NULL,
        updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
        deleted_at TIMESTAMP WITH TIME ZONE,
        version INTEGER DEFAULT 1
    );
    `
	_, err = db.Exec(schema)
	require.NoError(t, err, "Failed to initialize test database schema")

	return db
}

func setupTestRedis(t *testing.T) *redis.Client {
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("Failed to connect to Redis during setup: %v", err)
	}

	return rdb
}

func TestEndToEnd_FullSystemLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	defer db.Close()

	rdb := setupTestRedis(t)
	defer rdb.Close()

	_, err := db.Exec("TRUNCATE TABLE habit_entries, habits, users CASCADE")
	require.NoError(t, err, "Failed to truncate tables")

	rdb.FlushDB(context.Background())

	habitRepoPostgres := repository.NewPostgresHabitRepository(db)
	habitRepoCached := repository.NewCachedHabitRepository(habitRepoPostgres, rdb)

	entryRepo := repository.NewPostgresEntryRepository(db)
	userRepo := repository.NewPostgresUserRepository(db.DB)

	streakWorker := workers.NewStreakWorker(habitRepoCached, entryRepo)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	streakWorker.Start(workerCtx)

	tokenService := services.NewTokenService("test-secret-e2e", "kanso-e2e", 24*time.Hour, userRepo)

	habitSvc := services.NewHabitService(habitRepoCached)
	entrySvc := services.NewEntryService(entryRepo, habitRepoCached, streakWorker)
	authSvc := services.NewAuthService(userRepo, tokenService)

	habitHandler := adapterHTTP.NewHabitHandler(habitSvc)
	entryHandler := adapterHTTP.NewEntryHandler(entrySvc)
	authHandler := adapterHTTP.NewAuthHandler(authSvc)

	router := gin.Default()

	authMiddleware := middleware.AuthMiddleware(tokenService)

	api := router.Group("/api/v1")
	authHandler.RegisterRoutes(api, authMiddleware)

	protected := api.Group("")
	protected.Use(authMiddleware)
	{
		habitHandler.RegisterRoutes(protected)
		entryHandler.RegisterRoutes(protected)
	}

	router.GET("/health", func(c *gin.Context) {
		if err := db.Ping(); err != nil {
			c.JSON(503, gin.H{"status": "error"})
			return
		}
		c.JSON(200, gin.H{"status": "ok", "uptime": "testing"})
	})

	var habitID string
	var entryID string
	var authToken string

	t.Run("0. Health Check", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("1. Register New User", func(t *testing.T) {
		payload := `{"email": "e2e@kanso.app", "password": "PasswordSicura123!"}`
		req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("1b. Login & Get Token", func(t *testing.T) {
		payload := `{"email": "e2e@kanso.app", "password": "PasswordSicura123!"}`
		req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp loginResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.NotEmpty(t, resp.Token)

		authToken = resp.Token
	})

	t.Run("2. Create Habit", func(t *testing.T) {
		payload := `{
            "title": "Drink Water",
            "type": "numeric",
            "target_value": 2000,
            "unit": "ml",
            "frequency_type": "daily"
        }`
		req, _ := http.NewRequest("POST", "/api/v1/habits", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)
		var resp idResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		habitID = resp.ID
		require.NotEmpty(t, habitID)
	})

	t.Run("3. Create Entry (Triggers Worker)", func(t *testing.T) {
		require.NotEmpty(t, habitID)
		payload := fmt.Sprintf(`{
            "habit_id": "%s",
            "completion_date": "%s",
            "value": 500,
            "notes": "Morning glass"
        }`, habitID, time.Now().Format(time.RFC3339))

		req, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)
		var resp idResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		entryID = resp.ID

		time.Sleep(100 * time.Millisecond)
	})

	t.Run("3b. Validation Error (Bad JSON)", func(t *testing.T) {
		fakeHabitID := "fake-habit-id"
		payload := fmt.Sprintf(`{"habit_id": "%s", "value": "non-numeric"}`, fakeHabitID)

		req, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("4. Update Entry", func(t *testing.T) {
		payload := `{"value": 600, "notes": "Updated", "version": 1}`
		req, _ := http.NewRequest("PUT", "/api/v1/entries/"+entryID, bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("4b. Optimistic Locking Conflict", func(t *testing.T) {
		payload := `{"value": 700, "version": 1}`
		req, _ := http.NewRequest("PUT", "/api/v1/entries/"+entryID, bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("5. List Entries", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/entries?habit_id="+habitID, nil)
		req.Header.Set("Authorization", "Bearer "+authToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var list listEntryResponse
		json.Unmarshal(w.Body.Bytes(), &list)
		require.Len(t, list, 1)
		assert.Equal(t, 600, list[0].Value)
	})

	t.Run("6. Sync Logic", func(t *testing.T) {
		since := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
		safeSince := url.QueryEscape(since)

		req, _ := http.NewRequest("GET", "/api/v1/entries/sync?since="+safeSince, nil)
		req.Header.Set("Authorization", "Bearer "+authToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), entryID)
	})

	t.Run("7. Security: IDOR Check (Attacker)", func(t *testing.T) {
		attackerPayload := `{"email": "attacker@kanso.app", "password": "PasswordAttacker1!"}`
		reqReg, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(attackerPayload))
		reqReg.Header.Set("Content-Type", "application/json")
		wReg := httptest.NewRecorder()
		router.ServeHTTP(wReg, reqReg)
		require.Equal(t, http.StatusCreated, wReg.Code)

		reqLogin, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(attackerPayload))
		reqLogin.Header.Set("Content-Type", "application/json")
		wLogin := httptest.NewRecorder()
		router.ServeHTTP(wLogin, reqLogin)
		require.Equal(t, http.StatusOK, wLogin.Code)

		var loginResp loginResponse
		json.Unmarshal(wLogin.Body.Bytes(), &loginResp)
		attackerToken := loginResp.Token

		req, _ := http.NewRequest("DELETE", "/api/v1/entries/"+entryID, nil)
		req.Header.Set("Authorization", "Bearer "+attackerToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("8. Delete Entry", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/v1/entries/"+entryID, nil)
		req.Header.Set("Authorization", "Bearer "+authToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNoContent, w.Code)

		time.Sleep(1 * time.Second)
	})

	t.Run("9. Verify Entry Deletion", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/entries?habit_id="+habitID, nil)
		req.Header.Set("Authorization", "Bearer "+authToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var list listEntryResponse
		json.Unmarshal(w.Body.Bytes(), &list)
		assert.Len(t, list, 0)
	})

	t.Run("10. Delete Habit (Full Cleanup)", func(t *testing.T) {
		//rdb.FlushDB(context.Background())

		req, _ := http.NewRequest("DELETE", "/api/v1/habits/"+habitID, nil)
		req.Header.Set("Authorization", "Bearer "+authToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Logf("Step 10 Failed. Body: %s", w.Body.String())
		}

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("11. Delete Account", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/v1/auth/user", nil)
		req.Header.Set("Authorization", "Bearer "+authToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "account deleted")
	})

	t.Run("12. Verify Account Deletion", func(t *testing.T) {
		payload := `{"email": "e2e@kanso.app", "password": "PasswordSicura123!"}`
		req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
