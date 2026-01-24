package http_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	adapterHTTP "github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http"
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
	m.store[h.ID] = h
	return nil
}

func (m *MockRepo) GetByID(ctx context.Context, id string) (*domain.Habit, error) {
	h, ok := m.store[id]
	if !ok {
		return nil, domain.ErrHabitNotFound
	}
	return h, nil
}

func (m *MockRepo) ListByUserID(ctx context.Context, userID string) ([]*domain.Habit, error) {
	var list []*domain.Habit
	for _, h := range m.store {
		if h.UserID == userID {
			list = append(list, h)
		}
	}
	return list, nil
}

func (m *MockRepo) Update(ctx context.Context, h *domain.Habit) error {
	if _, ok := m.store[h.ID]; !ok {
		return domain.ErrHabitNotFound
	}
	m.store[h.ID] = h
	return nil
}

func (m *MockRepo) Delete(ctx context.Context, id string) error {
	delete(m.store, id)
	return nil
}

func setupRouter() (*gin.Engine, *MockRepo) {
	gin.SetMode(gin.TestMode)

	repo := NewMockRepo()
	svc := services.NewHabitService(repo)
	handler := adapterHTTP.NewHabitHandler(svc)

	r := gin.New()
	handler.RegisterRoutes(r.Group("/api/v1"))
	return r, repo
}

func TestCreateHabit(t *testing.T) {
	t.Run("Success: 201 Created", func(t *testing.T) {
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

	t.Run("Fail: 401 Unauthorized (Missing Header)", func(t *testing.T) {
		router, _ := setupRouter()
		body := `{"title": "Gym"}`

		req, _ := http.NewRequest("POST", "/api/v1/habits", bytes.NewBufferString(body))

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Fail: 400 Bad Request (Invalid JSON/Validation)", func(t *testing.T) {
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

		h1, _ := domain.NewHabit("Run", "user-1")
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

		h, _ := domain.NewHabit("Old", "user-1")
		h.Update("Old", "", "#FFFFFF", "book", "boolean", "", "", 1, 1, []int{1})
		repo.Create(context.Background(), h)

		body := `{
            "title": "New", 
            "type": "boolean", 
            "color": "#00FF00", 
            "weekdays": [1, 2],
            "target_value": 1,
            "interval": 1
        }`

		req, _ := http.NewRequest("PUT", "/api/v1/habits/"+h.ID, bytes.NewBufferString(body))
		req.Header.Set("X-User-ID", "user-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		updated, _ := repo.GetByID(context.Background(), h.ID)
		assert.Equal(t, "New", updated.Title)
		assert.Equal(t, "#00FF00", updated.Color)
	})

	t.Run("Success: 200 OK Partial Update", func(t *testing.T) {
		router, repo := setupRouter()

		h, _ := domain.NewHabit("Old Title", "user-1")
		h.Update("Old Title", "Desc", "#FF0000", "icon", "timer", "", "", 10, 1, []int{1})
		repo.Create(context.Background(), h)

		body := `{"title": "Updated Title"}`

		req, _ := http.NewRequest("PUT", "/api/v1/habits/"+h.ID, bytes.NewBufferString(body))
		req.Header.Set("X-User-ID", "user-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		updated, _ := repo.GetByID(context.Background(), h.ID)
		assert.Equal(t, "Updated Title", updated.Title)
		assert.Equal(t, "#FF0000", updated.Color)
		assert.Equal(t, "timer", updated.Type)
	})

	t.Run("Fail: 404 Not Found (IDOR Protection)", func(t *testing.T) {
		router, repo := setupRouter()
		h, _ := domain.NewHabit("Secret", "user-1")
		repo.Create(context.Background(), h)

		body := `{"title": "Hacked"}`
		req, _ := http.NewRequest("PUT", "/api/v1/habits/"+h.ID, bytes.NewBufferString(body))
		req.Header.Set("X-User-ID", "user-2")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestDeleteHabit(t *testing.T) {
	t.Run("Success: 204 No Content", func(t *testing.T) {
		router, repo := setupRouter()
		h, _ := domain.NewHabit("To Delete", "user-1")
		repo.Create(context.Background(), h)

		req, _ := http.NewRequest("DELETE", "/api/v1/habits/"+h.ID, nil)
		req.Header.Set("X-User-ID", "user-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Body.String())
	})

	t.Run("Fail: 404 Not Found (IDOR Protection)", func(t *testing.T) {
		router, repo := setupRouter()
		h, _ := domain.NewHabit("Secret", "user-1")
		repo.Create(context.Background(), h)

		req, _ := http.NewRequest("DELETE", "/api/v1/habits/"+h.ID, nil)
		req.Header.Set("X-User-ID", "user-2")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Fail: 401 Unauthorized", func(t *testing.T) {
		router, _ := setupRouter()
		req, _ := http.NewRequest("DELETE", "/api/v1/habits/123", nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
