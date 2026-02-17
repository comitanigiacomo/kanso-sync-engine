package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	adapterHTTP "github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http/middleware"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
)

type MockRepo struct {
	store map[string]*domain.Habit
}

func NewMockRepo() *MockRepo {
	return &MockRepo{store: make(map[string]*domain.Habit)}
}

func (m *MockRepo) Create(ctx context.Context, h *domain.Habit) error {
	if h.Version == 0 {
		h.Version = 1
	}
	clone := *h
	m.store[h.ID] = &clone
	return nil
}

func (m *MockRepo) GetByID(ctx context.Context, id string) (*domain.Habit, error) {
	h, ok := m.store[id]
	if !ok {
		return nil, domain.ErrHabitNotFound
	}
	if h.DeletedAt != nil {
		return nil, domain.ErrHabitNotFound
	}
	clone := *h
	return &clone, nil
}

func (m *MockRepo) ListByUserID(ctx context.Context, userID string) ([]*domain.Habit, error) {
	var list []*domain.Habit
	for _, h := range m.store {
		if h.UserID == userID && h.DeletedAt == nil {
			clone := *h
			list = append(list, &clone)
		}
	}
	return list, nil
}

func (m *MockRepo) Update(ctx context.Context, h *domain.Habit) error {
	_, ok := m.store[h.ID]
	if !ok {
		return domain.ErrHabitNotFound
	}

	clone := *h
	m.store[h.ID] = &clone
	return nil
}

func (m *MockRepo) Delete(ctx context.Context, id string) error {
	h, ok := m.store[id]
	if !ok {
		return domain.ErrHabitNotFound
	}
	now := time.Now().UTC()
	h.DeletedAt = &now
	h.Version++
	h.UpdatedAt = now
	return nil
}

func (m *MockRepo) GetChanges(ctx context.Context, userID string, since time.Time) ([]*domain.Habit, error) {
	var changes []*domain.Habit
	for _, h := range m.store {
		if h.UserID == userID && h.UpdatedAt.After(since) {
			clone := *h
			changes = append(changes, &clone)
		}
	}
	return changes, nil
}

func (m *MockRepo) UpdateStreaks(ctx context.Context, id string, current, longest int) error {
	h, ok := m.store[id]
	if !ok {
		return domain.ErrHabitNotFound
	}
	h.CurrentStreak = current
	h.LongestStreak = longest
	h.UpdatedAt = time.Now().UTC()
	return nil
}

func setupRouter() (*gin.Engine, *MockRepo) {
	gin.SetMode(gin.TestMode)

	repo := NewMockRepo()

	svc := services.NewHabitService(repo)
	handler := adapterHTTP.NewHabitHandler(svc)

	r := gin.New()

	r.Use(func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID != "" {
			c.Set(middleware.ContextUserIDKey, userID)
		}
		c.Next()
	})

	api := r.Group("/api/v1")
	handler.RegisterRoutes(api)

	return r, repo
}

func TestCreateHabit(t *testing.T) {
	t.Run("Success: 201 Created (Auto Generated ID)", func(t *testing.T) {
		router, _ := setupRouter()

		body := `{"title": "Gym", "type": "boolean", "weekdays": [1, 3, 5]}`

		req, _ := http.NewRequest("POST", "/api/v1/habits", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", "user-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), `"title":"Gym"`)
		assert.Contains(t, w.Body.String(), `"id":`)
	})

	t.Run("Success: 201 Created with Client-Side UUID", func(t *testing.T) {
		router, repo := setupRouter()

		clientUUID := "123e4567-e89b-12d3-a456-426614174000"
		body := `{"id": "` + clientUUID + `", "title": "Offline Habit", "type": "boolean"}`

		req, _ := http.NewRequest("POST", "/api/v1/habits", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", "user-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		assert.Contains(t, w.Body.String(), clientUUID)

		saved, err := repo.GetByID(context.Background(), clientUUID)
		assert.Nil(t, err)
		assert.NotNil(t, saved)
		assert.Equal(t, clientUUID, saved.ID)
		assert.Equal(t, "Offline Habit", saved.Title)
	})

	t.Run("Fail: 401 Unauthorized (Missing Header)", func(t *testing.T) {
		router, _ := setupRouter()
		body := `{"title": "Gym"}`
		req, _ := http.NewRequest("POST", "/api/v1/habits", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Contains(t, []int{http.StatusUnauthorized, http.StatusInternalServerError}, w.Code)
	})

	t.Run("Fail: 400 Bad Request", func(t *testing.T) {
		router, _ := setupRouter()
		body := `{"title": ""}`
		req, _ := http.NewRequest("POST", "/api/v1/habits", bytes.NewBufferString(body))
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestGetHabits(t *testing.T) {
	t.Run("Success: 200 OK with List", func(t *testing.T) {
		router, repo := setupRouter()
		h1, _ := domain.NewHabit("", "Run", "user-1")
		repo.Create(context.Background(), h1)

		req, _ := http.NewRequest("GET", "/api/v1/habits", nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Run")
	})
}

func TestUpdateHabit(t *testing.T) {
	t.Run("Success: 200 OK Full Update", func(t *testing.T) {
		router, repo := setupRouter()
		h, _ := domain.NewHabit("", "Old", "user-1")
		h.Version = 1
		repo.Create(context.Background(), h)

		body := `{
            "title": "New", 
            "type": "boolean", 
            "color": "#00FF00",
            "version": 1 
        }`

		req, _ := http.NewRequest("PUT", "/api/v1/habits/"+h.ID, bytes.NewBufferString(body))
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		updated, _ := repo.GetByID(context.Background(), h.ID)
		assert.Equal(t, "New", updated.Title)
		assert.Equal(t, "#00FF00", updated.Color)
		assert.Equal(t, 2, updated.Version)
	})

	t.Run("Success: Clear Description (JSON empty string)", func(t *testing.T) {
		router, repo := setupRouter()
		h, _ := domain.NewHabit("", "To Clear", "user-1")
		h.Description = "Must be deleted"
		h.Version = 1
		repo.Create(context.Background(), h)

		body := `{
            "description": "",
            "version": 1 
        }`

		req, _ := http.NewRequest("PUT", "/api/v1/habits/"+h.ID, bytes.NewBufferString(body))
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		updated, _ := repo.GetByID(context.Background(), h.ID)
		assert.Equal(t, "", updated.Description)
	})

	t.Run("Fail: 404 Not Found (IDOR Protection)", func(t *testing.T) {
		router, repo := setupRouter()
		h, _ := domain.NewHabit("", "Secret", "user-1")
		repo.Create(context.Background(), h)

		body := `{"title": "Hacked", "version": 1}`
		req, _ := http.NewRequest("PUT", "/api/v1/habits/"+h.ID, bytes.NewBufferString(body))
		req.Header.Set("X-User-ID", "user-2")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestUpdateHabit_Conflict(t *testing.T) {
	t.Run("Fail: 409 Conflict when updating old version", func(t *testing.T) {
		router, repo := setupRouter()

		h, _ := domain.NewHabit("", "V2", "user-1")
		h.Version = 2
		repo.Create(context.Background(), h)

		body := `{"title": "Overwrite", "version": 1}`

		req, _ := http.NewRequest("PUT", "/api/v1/habits/"+h.ID, bytes.NewBufferString(body))
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})
}

func TestSyncEndpoint(t *testing.T) {
	router, repo := setupRouter()
	ctx := context.Background()

	hOld, _ := domain.NewHabit("", "Old", "user-1")
	hOld.UpdatedAt = time.Now().UTC().Add(-24 * time.Hour)
	repo.Create(ctx, hOld)

	lastSyncTime := time.Now().UTC().Add(-1 * time.Hour)
	lastSyncStr := lastSyncTime.Format(time.RFC3339)

	time.Sleep(1 * time.Millisecond)

	hNew, _ := domain.NewHabit("", "New", "user-1")
	hNew.UpdatedAt = time.Now().UTC()
	repo.Create(ctx, hNew)

	t.Run("Sync returns only new items", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/habits/sync?last_sync="+lastSyncStr, nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &response)

		assert.Contains(t, w.Body.String(), hNew.ID)
		assert.NotContains(t, w.Body.String(), hOld.ID)
		assert.Contains(t, response, "timestamp")
	})
}
