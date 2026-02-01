package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adapterHTTP "github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/repository"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type idResponse struct {
	ID string `json:"id"`
}

type listEntryResponse []struct {
	ID    string `json:"id"`
	Value int    `json:"value"`
}

type authResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
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
	return db
}

func TestEndToEnd_FullSystemLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec("TRUNCATE TABLE habit_entries, habits, users CASCADE")
	require.NoError(t, err, "Failed to truncate tables")

	habitRepo := repository.NewPostgresHabitRepository(db)
	entryRepo := repository.NewPostgresEntryRepository(db)
	userRepo := repository.NewPostgresUserRepository(db.DB)

	habitSvc := services.NewHabitService(habitRepo)
	entrySvc := services.NewEntryService(entryRepo, habitRepo)
	authSvc := services.NewAuthService(userRepo)

	habitHandler := adapterHTTP.NewHabitHandler(habitSvc)
	entryHandler := adapterHTTP.NewEntryHandler(entrySvc)
	authHandler := adapterHTTP.NewAuthHandler(authSvc)
	router := gin.Default()
	startTime := time.Now()

	router.GET("/health", func(c *gin.Context) {
		if err := db.Ping(); err != nil {
			c.JSON(503, gin.H{"status": "error"})
			return
		}
		c.JSON(200, gin.H{"status": "ok", "uptime": time.Since(startTime).String()})
	})

	api := router.Group("/api/v1")
	habitHandler.RegisterRoutes(api)
	entryHandler.RegisterRoutes(api)
	authHandler.RegisterRoutes(api)

	var habitID string
	var entryID string

	existingUserID := uuid.NewString()
	attackerID := uuid.NewString()

	t.Run("0. Health Check", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "uptime")
	})

	t.Run("1. Register New User (Public API)", func(t *testing.T) {
		payload := `{
			"email": "new_user_e2e@kanso.app",
			"password": "PasswordSicura123!"
		}`
		req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)

		var resp authResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "new_user_e2e@kanso.app", resp.Email)
		assert.NotEmpty(t, resp.ID)
		assert.NotContains(t, w.Body.String(), "password")
	})

	_, err = db.Exec(`
        INSERT INTO users (id, email, password_hash, created_at, updated_at) 
        VALUES ($1, $2, 'dummy_hash_for_e2e_test', NOW(), NOW())
        ON CONFLICT (id) DO NOTHING`,
		existingUserID, "tester@kanso.app")
	require.NoError(t, err, "Failed to create test user fixture.")

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
		req.Header.Set("X-User-ID", existingUserID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)

		var resp idResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		habitID = resp.ID
		require.NotEmpty(t, habitID)
	})

	t.Run("3. Create Entry", func(t *testing.T) {
		require.NotEmpty(t, habitID)

		payload := fmt.Sprintf(`{
            "habit_id": "%s",
            "completion_date": "%s",
            "value": 500,
            "notes": "Morning glass"
        }`, habitID, time.Now().Format(time.RFC3339))

		req, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", existingUserID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Logf("Create Entry Failed: %s", w.Body.String())
		}
		require.Equal(t, http.StatusCreated, w.Code)

		var resp idResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		entryID = resp.ID
		require.NotEmpty(t, entryID)
	})

	t.Run("3b. Validation Error (Bad JSON)", func(t *testing.T) {
		fakeHabitID := uuid.NewString()
		payload := fmt.Sprintf(`{"habit_id": "%s", "value": "non-numeric"}`, fakeHabitID)

		req, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", existingUserID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("4. Update Entry (Success)", func(t *testing.T) {
		require.NotEmpty(t, entryID)
		payload := `{"value": 600, "notes": "Updated", "version": 1}`

		req, _ := http.NewRequest("PUT", "/api/v1/entries/"+entryID, bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", existingUserID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"value":600`)
	})

	t.Run("4b. Optimistic Locking Conflict", func(t *testing.T) {
		payload := `{"value": 700, "version": 1}`

		req, _ := http.NewRequest("PUT", "/api/v1/entries/"+entryID, bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", existingUserID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("5. List Entries", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/entries?habit_id="+habitID, nil)
		req.Header.Set("X-User-ID", existingUserID)

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
		req.Header.Set("X-User-ID", existingUserID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), entryID)
	})

	t.Run("7. Security: IDOR Check", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/v1/entries/"+entryID, nil)
		req.Header.Set("X-User-ID", attackerID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("8. Delete Entry", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/v1/entries/"+entryID, nil)
		req.Header.Set("X-User-ID", existingUserID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("9. Verify Entry Deletion", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/entries?habit_id="+habitID, nil)
		req.Header.Set("X-User-ID", existingUserID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var list listEntryResponse
		json.Unmarshal(w.Body.Bytes(), &list)
		assert.Len(t, list, 0)
	})

	t.Run("10. Delete Habit (Full Cleanup)", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/v1/habits/"+habitID, nil)
		req.Header.Set("X-User-ID", existingUserID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}
