package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adapterHTTP "github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/repository"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type createResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
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

func TestEndToEnd_HabitLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec("TRUNCATE TABLE habits CASCADE")
	require.NoError(t, err, "Failed to truncate habits table")

	repo := repository.NewPostgresHabitRepository(db)
	svc := services.NewHabitService(repo)
	handler := adapterHTTP.NewHabitHandler(svc)

	router := gin.Default()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	var habitID string

	t.Run("1. Create Habit", func(t *testing.T) {
		habitPayload := `{
			"title": "Morning Run",
			"type": "boolean",
			"frequency_type": "daily",
			"start_date": "2023-10-27T08:00:00Z"
		}`

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/habits", bytes.NewBuffer([]byte(habitPayload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", "e2e-tester-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var resp createResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
		habitID = resp.ID
	})

	t.Run("2. Update Habit", func(t *testing.T) {
		require.NotEmpty(t, habitID, "Create step failed, cannot update")

		updatePayload := `{
			"title": "Evening Run", 
			"type": "boolean"
		}`

		req, _ := http.NewRequest(http.MethodPut, "/api/v1/habits/"+habitID, bytes.NewBuffer([]byte(updatePayload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", "e2e-tester-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("3. Verify Update", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/habits?user_id=e2e-tester-1", nil)
		req.Header.Set("X-User-ID", "e2e-tester-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Evening Run")
	})

	t.Run("4. Delete Habit", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/habits/"+habitID, nil)
		req.Header.Set("X-User-ID", "e2e-tester-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("5. Verify Delete", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/habits?user_id=e2e-tester-1", nil)
		req.Header.Set("X-User-ID", "e2e-tester-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), habitID)
	})

	t.Run("6. Validation Error", func(t *testing.T) {
		invalidPayload := `{"type": "boolean"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/habits", bytes.NewBuffer([]byte(invalidPayload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", "e2e-tester-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("7. Auth Error", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/habits", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
