package main

import (
	"bytes"
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

	_, err := db.Exec("TRUNCATE TABLE habits")
	require.NoError(t, err, "Failed to truncate habits table")

	repo := repository.NewPostgresHabitRepository(db)
	svc := services.NewHabitService(repo)
	handler := adapterHTTP.NewHabitHandler(svc)

	router := gin.Default()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	t.Run("Should create and retrieve a habit successfully", func(t *testing.T) {
		habitPayload := `{
			"user_id": "e2e-tester-1",
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
		assert.Contains(t, w.Body.String(), `"title":"Morning Run"`)
		assert.Contains(t, w.Body.String(), `"id":`)

		reqGet, _ := http.NewRequest(http.MethodGet, "/api/v1/habits?user_id=e2e-tester-1", nil)
		reqGet.Header.Set("X-User-ID", "e2e-tester-1")

		wGet := httptest.NewRecorder()
		router.ServeHTTP(wGet, reqGet)

		assert.Equal(t, http.StatusOK, wGet.Code)
		assert.Contains(t, wGet.Body.String(), "Morning Run")
	})

	t.Run("Should return 400 when title is missing", func(t *testing.T) {
		invalidPayload := `{
			"user_id": "e2e-tester-1",
			"type": "boolean",
			"frequency_type": "daily",
			"start_date": "2023-10-27T08:00:00Z"
		}`

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/habits", bytes.NewBuffer([]byte(invalidPayload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", "e2e-tester-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "error")
	})

	t.Run("Should return 401 when X-User-ID header is missing", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/habits", nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "missing user id")
	})
}
