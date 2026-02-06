package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	adapterHTTP "github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http/middleware"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/workers"
)

func getTestWorker() *workers.StreakWorker {
	return workers.NewStreakWorker(nil, nil)
}

type MockEntryRepo struct {
	store map[string]*domain.HabitEntry
}

func NewMockEntryRepo() *MockEntryRepo {
	return &MockEntryRepo{store: make(map[string]*domain.HabitEntry)}
}

func (m *MockEntryRepo) Create(ctx context.Context, e *domain.HabitEntry) error {
	if e.Version == 0 {
		e.Version = 1
	}
	m.store[e.ID] = e
	return nil
}

func (m *MockEntryRepo) Update(ctx context.Context, e *domain.HabitEntry) error {
	existing, ok := m.store[e.ID]
	if !ok {
		return domain.ErrEntryNotFound
	}
	if e.Version != existing.Version {
		return domain.ErrEntryConflict
	}
	e.Version++
	m.store[e.ID] = e
	return nil
}

func (m *MockEntryRepo) GetByID(ctx context.Context, id string) (*domain.HabitEntry, error) {
	e, ok := m.store[id]
	if !ok {
		return nil, domain.ErrEntryNotFound
	}
	return e, nil
}

func (m *MockEntryRepo) Delete(ctx context.Context, id string, userID string) error {
	e, ok := m.store[id]
	if !ok {
		return domain.ErrEntryNotFound
	}
	if e.UserID != userID {
		return domain.ErrEntryNotFound
	}
	delete(m.store, id)
	return nil
}

func (m *MockEntryRepo) ListByHabitID(ctx context.Context, habitID string) ([]*domain.HabitEntry, error) {
	var list []*domain.HabitEntry
	for _, e := range m.store {
		if e.HabitID == habitID {
			list = append(list, e)
		}
	}
	return list, nil
}

func (m *MockEntryRepo) ListByHabitIDWithRange(ctx context.Context, habitID string, from, to time.Time) ([]*domain.HabitEntry, error) {
	var list []*domain.HabitEntry
	for _, e := range m.store {
		if e.HabitID == habitID {
			if (e.CompletionDate.After(from) || e.CompletionDate.Equal(from)) &&
				(e.CompletionDate.Before(to) || e.CompletionDate.Equal(to)) {
				list = append(list, e)
			}
		}
	}
	return list, nil
}

func (m *MockEntryRepo) GetChanges(ctx context.Context, userID string, since time.Time) ([]*domain.HabitEntry, error) {
	var changes []*domain.HabitEntry
	for _, e := range m.store {
		if e.UserID == userID && e.UpdatedAt.After(since) {
			changes = append(changes, e)
		}
	}
	return changes, nil
}

func (m *MockEntryRepo) ListByUserIDAndDateRange(ctx context.Context, userID string, startDate, endDate time.Time) ([]domain.HabitEntry, error) {
	var list []domain.HabitEntry
	for _, e := range m.store {
		if e.UserID == userID {
			if (e.CompletionDate.After(startDate) || e.CompletionDate.Equal(startDate)) &&
				(e.CompletionDate.Before(endDate) || e.CompletionDate.Equal(endDate)) {
				list = append(list, *e)
			}
		}
	}
	return list, nil
}

type MockHabitRepoForEntry struct {
	habits map[string]*domain.Habit
}

func NewMockHabitRepo() *MockHabitRepoForEntry {
	return &MockHabitRepoForEntry{habits: make(map[string]*domain.Habit)}
}

func (m *MockHabitRepoForEntry) GetByID(ctx context.Context, id string) (*domain.Habit, error) {
	h, ok := m.habits[id]
	if !ok {
		return nil, domain.ErrHabitNotFound
	}
	return h, nil
}

func (m *MockHabitRepoForEntry) Create(ctx context.Context, h *domain.Habit) error {
	m.habits[h.ID] = h
	return nil
}
func (m *MockHabitRepoForEntry) ListByUserID(ctx context.Context, u string) ([]*domain.Habit, error) {
	return nil, nil
}
func (m *MockHabitRepoForEntry) Update(ctx context.Context, h *domain.Habit) error { return nil }
func (m *MockHabitRepoForEntry) Delete(ctx context.Context, id string) error       { return nil }
func (m *MockHabitRepoForEntry) GetChanges(ctx context.Context, u string, t time.Time) ([]*domain.Habit, error) {
	return nil, nil
}

func setupEntryRouter() (*gin.Engine, *MockEntryRepo, *MockHabitRepoForEntry) {
	gin.SetMode(gin.TestMode)
	entryRepo := NewMockEntryRepo()
	habitRepo := NewMockHabitRepo()
	worker := getTestWorker()

	svc := services.NewEntryService(entryRepo, habitRepo, worker)
	handler := adapterHTTP.NewEntryHandler(svc)

	r := gin.New()

	r.Use(func(c *gin.Context) {
		if userID := c.GetHeader("X-User-ID"); userID != "" {
			c.Set(middleware.ContextUserIDKey, userID)
		}
		c.Next()
	})

	api := r.Group("/api/v1")
	handler.RegisterRoutes(api)
	return r, entryRepo, habitRepo
}

func TestCreateEntry(t *testing.T) {
	t.Run("Success: 201 Created", func(t *testing.T) {
		router, _, habitRepo := setupEntryRouter()
		h, _ := domain.NewHabit("Gym", "user-1")
		h.ID = "habit-1"
		habitRepo.Create(context.Background(), h)

		body := map[string]interface{}{
			"habit_id":        h.ID,
			"completion_date": time.Now().Format(time.RFC3339),
			"value":           10,
			"notes":           "Good workout",
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", "user-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), `"value":10`)
	})

	t.Run("Fail: 403 Forbidden (IDOR)", func(t *testing.T) {
		router, _, habitRepo := setupEntryRouter()
		h, _ := domain.NewHabit("Secret", "user-2")
		h.ID = "habit-secret"
		habitRepo.Create(context.Background(), h)

		body := map[string]interface{}{
			"habit_id":        h.ID,
			"completion_date": time.Now().Format(time.RFC3339),
			"value":           1,
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/v1/entries", bytes.NewBuffer(jsonBody))
		req.Header.Set("X-User-ID", "user-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestUpdateEntry(t *testing.T) {
	t.Run("Success: 200 OK", func(t *testing.T) {
		router, entryRepo, habitRepo := setupEntryRouter()
		h, _ := domain.NewHabit("Read", "user-1")
		h.ID = "habit-read"
		habitRepo.Create(context.Background(), h)

		e := domain.NewHabitEntry(h.ID, "user-1", time.Now(), 5)
		e.ID = "entry-1"
		e.Version = 1
		entryRepo.Create(context.Background(), e)

		body := map[string]interface{}{"value": 10, "notes": "Read more", "version": 1}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("PUT", "/api/v1/entries/"+e.ID, bytes.NewBuffer(jsonBody))
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"value":10`)
	})

	t.Run("Fail: 409 Conflict", func(t *testing.T) {
		router, entryRepo, habitRepo := setupEntryRouter()
		h, _ := domain.NewHabit("Read", "user-1")
		h.ID = "habit-read"
		habitRepo.Create(context.Background(), h)

		e := domain.NewHabitEntry(h.ID, "user-1", time.Now(), 5)
		e.ID = "entry-conflict"
		e.Version = 2
		entryRepo.Create(context.Background(), e)

		body := map[string]interface{}{"value": 10, "version": 1}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("PUT", "/api/v1/entries/"+e.ID, bytes.NewBuffer(jsonBody))
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})
}

func TestDeleteEntry(t *testing.T) {
	t.Run("Success: 204 No Content", func(t *testing.T) {
		router, entryRepo, habitRepo := setupEntryRouter()
		h, _ := domain.NewHabit("Run", "user-1")
		h.ID = "habit-run"
		habitRepo.Create(context.Background(), h)

		e := domain.NewHabitEntry(h.ID, "user-1", time.Now(), 1)
		e.ID = "entry-del"
		entryRepo.Create(context.Background(), e)

		req, _ := http.NewRequest("DELETE", "/api/v1/entries/"+e.ID, nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("Fail: 403 Forbidden (Delete other user's entry)", func(t *testing.T) {
		router, entryRepo, habitRepo := setupEntryRouter()
		h, _ := domain.NewHabit("Run", "user-2")
		h.ID = "habit-other"
		habitRepo.Create(context.Background(), h)

		e := domain.NewHabitEntry(h.ID, "user-2", time.Now(), 1)
		e.ID = "entry-other"
		entryRepo.Create(context.Background(), e)

		req, _ := http.NewRequest("DELETE", "/api/v1/entries/"+e.ID, nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Fail: 404 Not Found", func(t *testing.T) {
		router, _, _ := setupEntryRouter()
		req, _ := http.NewRequest("DELETE", "/api/v1/entries/non-existent-id", nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestListEntries(t *testing.T) {
	t.Run("Success: List entries for habit", func(t *testing.T) {
		router, entryRepo, habitRepo := setupEntryRouter()

		h, _ := domain.NewHabit("Yoga", "user-1")
		h.ID = "habit-yoga"
		habitRepo.Create(context.Background(), h)

		e1 := domain.NewHabitEntry(h.ID, "user-1", time.Now(), 1)
		e1.ID = "e1"
		e2 := domain.NewHabitEntry(h.ID, "user-1", time.Now().Add(-24*time.Hour), 1)
		e2.ID = "e2"
		entryRepo.Create(context.Background(), e1)
		entryRepo.Create(context.Background(), e2)

		url := fmt.Sprintf("/api/v1/entries?habit_id=%s", h.ID)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("X-User-ID", "user-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), e1.ID)
		assert.Contains(t, w.Body.String(), e2.ID)
	})

	t.Run("Fail: 400 Missing Habit ID", func(t *testing.T) {
		router, _, _ := setupEntryRouter()
		req, _ := http.NewRequest("GET", "/api/v1/entries", nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestSyncEntries(t *testing.T) {
	router, entryRepo, _ := setupEntryRouter()

	eOld := domain.NewHabitEntry("h1", "user-1", time.Now(), 1)
	eOld.ID = "old-1"
	eOld.UpdatedAt = time.Now().Add(-24 * time.Hour)
	entryRepo.Create(context.Background(), eOld)

	eNew := domain.NewHabitEntry("h1", "user-1", time.Now(), 1)
	eNew.ID = "new-1"
	eNew.UpdatedAt = time.Now()
	entryRepo.Create(context.Background(), eNew)

	since := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	safeSince := url.QueryEscape(since)

	t.Run("Success: Returns only new entries", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/entries/sync?since="+safeSince, nil)
		req.Header.Set("X-User-ID", "user-1")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), eNew.ID)
		assert.NotContains(t, w.Body.String(), eOld.ID)
	})
}
