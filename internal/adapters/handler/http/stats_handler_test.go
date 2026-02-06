package http_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	adapterHTTP "github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http/middleware"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
)

type MockHabitRepoForStats struct {
	mock.Mock
}

func (m *MockHabitRepoForStats) ListByUserID(ctx context.Context, userID string) ([]*domain.Habit, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Habit), args.Error(1)
}

func (m *MockHabitRepoForStats) Create(ctx context.Context, h *domain.Habit) error { return nil }
func (m *MockHabitRepoForStats) Update(ctx context.Context, h *domain.Habit) error { return nil }
func (m *MockHabitRepoForStats) Delete(ctx context.Context, id string) error       { return nil }
func (m *MockHabitRepoForStats) GetByID(ctx context.Context, id string) (*domain.Habit, error) {
	return nil, nil
}
func (m *MockHabitRepoForStats) GetChanges(ctx context.Context, u string, t time.Time) ([]*domain.Habit, error) {
	return nil, nil
}

func setupStatsRouter() (*gin.Engine, *MockHabitRepoForStats, *MockEntryRepo) {
	gin.SetMode(gin.TestMode)

	habitRepo := new(MockHabitRepoForStats)
	entryRepo := NewMockEntryRepo()

	svc := services.NewStatsService(habitRepo, entryRepo)
	handler := adapterHTTP.NewStatsHandler(svc)

	r := gin.New()

	r.Use(func(c *gin.Context) {
		if userID := c.GetHeader("X-User-ID"); userID != "" {
			c.Set(middleware.ContextUserIDKey, userID)
		}
		c.Next()
	})

	api := r.Group("/api/v1")
	handler.RegisterRoutes(api)

	return r, habitRepo, entryRepo
}

func TestGetWeeklyStats(t *testing.T) {
	t.Run("Success: Returns 200 with valid params", func(t *testing.T) {
		r, habitRepo, _ := setupStatsRouter()

		userID := "user-1"
		habitRepo.On("ListByUserID", mock.Anything, userID).Return([]*domain.Habit{}, nil)

		req, _ := http.NewRequest("GET", "/api/v1/stats/weekly?start_date=2024-01-01&end_date=2024-01-07", nil)
		req.Header.Set("X-User-ID", userID)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "total_habits")
		assert.Contains(t, w.Body.String(), "overall_completion_rate")
	})

	t.Run("Success: Returns 200 with Smart Defaults (No dates provided)", func(t *testing.T) {
		r, habitRepo, _ := setupStatsRouter()
		userID := "user-1"

		habitRepo.On("ListByUserID", mock.Anything, userID).Return([]*domain.Habit{}, nil)

		req, _ := http.NewRequest("GET", "/api/v1/stats/weekly", nil)
		req.Header.Set("X-User-ID", userID)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Security: 400 Bad Request on DoS Attempt (Range too big)", func(t *testing.T) {
		r, _, _ := setupStatsRouter()

		req, _ := http.NewRequest("GET", "/api/v1/stats/weekly?start_date=2022-01-01&end_date=2024-01-01", nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "range too large")
	})

	t.Run("Validation: 400 Bad Request on Invalid Dates (Start > End)", func(t *testing.T) {
		r, _, _ := setupStatsRouter()

		req, _ := http.NewRequest("GET", "/api/v1/stats/weekly?start_date=2024-01-10&end_date=2024-01-01", nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "start_date cannot be after end_date")
	})

	t.Run("Validation: 400 Bad Request on Malformed Date", func(t *testing.T) {
		r, _, _ := setupStatsRouter()

		req, _ := http.NewRequest("GET", "/api/v1/stats/weekly?start_date=not-a-date", nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Security: 401 Unauthorized if no User ID", func(t *testing.T) {
		r, _, _ := setupStatsRouter()

		req, _ := http.NewRequest("GET", "/api/v1/stats/weekly", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Failure: 500 Internal Server Error on DB Fail", func(t *testing.T) {
		r, habitRepo, _ := setupStatsRouter()
		userID := "user-1"

		habitRepo.On("ListByUserID", mock.Anything, userID).Return(nil, errors.New("db boom"))

		req, _ := http.NewRequest("GET", "/api/v1/stats/weekly", nil)
		req.Header.Set("X-User-ID", userID)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
