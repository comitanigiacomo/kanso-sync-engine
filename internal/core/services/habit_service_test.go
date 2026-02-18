package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
	"github.com/stretchr/testify/assert"
)

func ptr[T any](v T) *T {
	return &v
}

func newTestService(repo domain.HabitRepository) *services.HabitService {
	return services.NewHabitService(repo)
}

type MockRepo struct {
	store         map[string]*domain.Habit
	simulateError error
}

func NewMockRepo() *MockRepo {
	return &MockRepo{
		store: make(map[string]*domain.Habit),
	}
}

func (m *MockRepo) Create(ctx context.Context, habit *domain.Habit) error {
	if m.simulateError != nil {
		return m.simulateError
	}

	if _, exists := m.store[habit.ID]; exists {
		return errors.New("duplicate key value violates unique constraint")
	}

	if habit.Version == 0 {
		habit.Version = 1
	}
	clone := *habit
	m.store[habit.ID] = &clone
	return nil
}

func (m *MockRepo) GetByID(ctx context.Context, id string) (*domain.Habit, error) {
	if m.simulateError != nil {
		return nil, m.simulateError
	}
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
	if m.simulateError != nil {
		return nil, m.simulateError
	}
	var list []*domain.Habit
	for _, h := range m.store {
		if h.UserID == userID && h.DeletedAt == nil {
			clone := *h
			list = append(list, &clone)
		}
	}
	return list, nil
}

func (m *MockRepo) Update(ctx context.Context, habit *domain.Habit) error {
	if m.simulateError != nil {
		return m.simulateError
	}

	if _, ok := m.store[habit.ID]; !ok {
		return domain.ErrHabitNotFound
	}

	clone := *habit
	m.store[habit.ID] = &clone
	return nil
}

func (m *MockRepo) Delete(ctx context.Context, id string) error {
	if m.simulateError != nil {
		return m.simulateError
	}
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
	if m.simulateError != nil {
		return m.simulateError
	}
	h, ok := m.store[id]
	if !ok {
		return domain.ErrHabitNotFound
	}
	h.CurrentStreak = current
	h.LongestStreak = longest
	h.UpdatedAt = time.Now().UTC()
	return nil
}

func TestHabitService_Create(t *testing.T) {
	t.Run("Success: Should create and persist a valid habit (Auto-ID)", func(t *testing.T) {
		repo := NewMockRepo()
		svc := newTestService(repo)
		ctx := context.Background()

		input := services.CreateHabitInput{
			UserID: "user-1",
			Title:  "Read Book",
			Type:   domain.HabitTypeBoolean,
		}

		created, err := svc.Create(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, created)
		assert.Equal(t, "Read Book", created.Title)
		assert.Equal(t, 1, created.Version)
		assert.NotEmpty(t, created.ID)

		stored, _ := repo.GetByID(ctx, created.ID)
		assert.NotNil(t, stored)
		assert.Equal(t, created.ID, stored.ID)
	})

	t.Run("Success: Should create habit with PROVIDED ID (Offline Sync)", func(t *testing.T) {
		repo := NewMockRepo()
		svc := newTestService(repo)
		ctx := context.Background()

		customID := "custom-uuid-123"
		input := services.CreateHabitInput{
			ID:     customID,
			UserID: "user-1",
			Title:  "Offline Habit",
			Type:   domain.HabitTypeBoolean,
		}

		created, err := svc.Create(ctx, input)

		assert.NoError(t, err)
		assert.Equal(t, customID, created.ID)

		stored, _ := repo.GetByID(ctx, customID)
		assert.NotNil(t, stored)
		assert.Equal(t, customID, stored.ID)
	})

	t.Run("Idempotency: Should return existing habit if ID exists (Sync Retry)", func(t *testing.T) {
		repo := NewMockRepo()
		svc := newTestService(repo)
		ctx := context.Background()

		input := services.CreateHabitInput{
			ID:     "retry-id",
			UserID: "user-1",
			Title:  "Retry Habit",
		}
		first, _ := svc.Create(ctx, input)

		second, err := svc.Create(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, second)
		assert.Equal(t, first.ID, second.ID)
		assert.Equal(t, first.CreatedAt, second.CreatedAt)
	})

	t.Run("Resurrection: Should revive soft-deleted habit (Zombie Fix)", func(t *testing.T) {
		repo := NewMockRepo()
		svc := newTestService(repo)
		ctx := context.Background()

		deletedID := "zombie-id"
		deletedHabit, _ := domain.NewHabit(deletedID, "I was deleted", "user-1")
		now := time.Now()
		deletedHabit.DeletedAt = &now
		repo.store[deletedID] = deletedHabit

		input := services.CreateHabitInput{
			ID:     deletedID,
			UserID: "user-1",
			Title:  "I am back",
		}

		revived, err := svc.Create(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, revived)
		assert.Nil(t, revived.DeletedAt)

		stored, err := repo.GetByID(ctx, deletedID)
		assert.NoError(t, err)
		assert.Nil(t, stored.DeletedAt)
	})

	t.Run("Fail: Domain Validation Error (Blocked BEFORE DB)", func(t *testing.T) {
		repo := NewMockRepo()
		svc := newTestService(repo)

		input := services.CreateHabitInput{
			UserID: "user-1",
			Title:  "",
		}

		_, err := svc.Create(context.Background(), input)

		assert.ErrorIs(t, err, domain.ErrHabitTitleEmpty)
		assert.Empty(t, repo.store)
	})
}

func TestHabitService_Update(t *testing.T) {
	t.Run("Success: Should update existing habit (Owner)", func(t *testing.T) {
		repo := NewMockRepo()
		svc := newTestService(repo)

		existing, _ := domain.NewHabit("", "Old Title", "user-1")
		existing.Version = 1
		repo.Create(context.Background(), existing)

		updateInput := services.UpdateHabitInput{
			ID:          existing.ID,
			UserID:      "user-1",
			Title:       ptr("New Title"),
			Description: ptr("Updated desc"),
			Color:       ptr("#FFFFFF"),
			Type:        ptr(domain.HabitTypeBoolean),
			TargetValue: ptr(1),
			Interval:    ptr(1),
			Version:     1,
		}

		updated, err := svc.Update(context.Background(), updateInput)

		assert.NoError(t, err)
		assert.NotNil(t, updated)
		assert.Equal(t, "New Title", updated.Title)
		assert.Equal(t, "#FFFFFF", updated.Color)
		assert.Equal(t, 2, updated.Version)
	})

	t.Run("Upsert: Should CREATE habit if not found (Missing Parent)", func(t *testing.T) {
		repo := NewMockRepo()
		svc := newTestService(repo)
		ctx := context.Background()

		ghostID := "ghost-id"

		updateInput := services.UpdateHabitInput{
			ID:     ghostID,
			UserID: "user-1",
			Title:  ptr("Ghost Habit"),
		}

		updated, err := svc.Update(ctx, updateInput)

		assert.NoError(t, err)
		assert.NotNil(t, updated)
		assert.Equal(t, ghostID, updated.ID)
		assert.Equal(t, "Ghost Habit", updated.Title)
		assert.Equal(t, 1, updated.Version)

		stored, _ := repo.GetByID(ctx, ghostID)
		assert.NotNil(t, stored)
	})

	t.Run("Fail: Habit Not Found (No Title for Upsert)", func(t *testing.T) {
		repo := NewMockRepo()
		svc := newTestService(repo)

		input := services.UpdateHabitInput{
			ID:          "ghost-id",
			UserID:      "user-1",
			Description: ptr("Just description"),
		}

		_, err := svc.Update(context.Background(), input)

		assert.ErrorIs(t, err, domain.ErrHabitNotFound)
	})

	t.Run("Fail: Security - Cannot update other user's habit (IDOR)", func(t *testing.T) {
		repo := NewMockRepo()
		svc := newTestService(repo)

		existing, _ := domain.NewHabit("", "Secret Habit", "user-1")
		repo.Create(context.Background(), existing)

		updateInput := services.UpdateHabitInput{
			ID:     existing.ID,
			UserID: "user-2",
			Title:  ptr("Hacked Title"),
		}

		_, err := svc.Update(context.Background(), updateInput)

		assert.ErrorIs(t, err, domain.ErrHabitNotFound)
	})

	t.Run("Optimistic Locking: Should fail if client has old version", func(t *testing.T) {
		repo := NewMockRepo()
		svc := newTestService(repo)

		existing, _ := domain.NewHabit("", "V2 Habit", "user-1")
		existing.Version = 2
		repo.Create(context.Background(), existing)

		input := services.UpdateHabitInput{
			ID:      existing.ID,
			UserID:  "user-1",
			Title:   ptr("Override attempt"),
			Version: 1,
		}

		_, err := svc.Update(context.Background(), input)

		assert.ErrorIs(t, err, domain.ErrHabitConflict)
	})
}

func TestHabitService_Delete(t *testing.T) {
	t.Run("Success: Should soft-delete via Update", func(t *testing.T) {
		repo := NewMockRepo()
		svc := newTestService(repo)

		h, _ := domain.NewHabit("", "To Delete", "user-1")
		repo.Create(context.Background(), h)

		err := svc.Delete(context.Background(), h.ID, "user-1")

		assert.NoError(t, err)

		_, err = repo.GetByID(context.Background(), h.ID)
		assert.Equal(t, domain.ErrHabitNotFound, err)

		rawH := repo.store[h.ID]
		assert.NotNil(t, rawH.DeletedAt)
	})

	t.Run("Fail: Security - Cannot delete other user's habit (IDOR)", func(t *testing.T) {
		repo := NewMockRepo()
		svc := newTestService(repo)

		h, _ := domain.NewHabit("", "Don't Touch", "user-1")
		repo.Create(context.Background(), h)

		err := svc.Delete(context.Background(), h.ID, "user-2")

		assert.ErrorIs(t, err, domain.ErrHabitNotFound)
	})

	t.Run("Fail: Delete non-existent habit", func(t *testing.T) {
		repo := NewMockRepo()
		svc := newTestService(repo)

		err := svc.Delete(context.Background(), "ghost-id", "user-1")

		assert.ErrorIs(t, err, domain.ErrHabitNotFound)
	})
}

func TestHabitService_ListAndGet(t *testing.T) {
	repo := NewMockRepo()
	svc := newTestService(repo)

	h1, _ := domain.NewHabit("", "H1", "user-1")
	h2, _ := domain.NewHabit("", "H2", "user-1")
	h3, _ := domain.NewHabit("", "H3", "user-2")

	repo.Create(context.Background(), h1)
	repo.Create(context.Background(), h2)
	repo.Create(context.Background(), h3)

	t.Run("ListByUserID returns only user's habits", func(t *testing.T) {
		list, err := svc.ListByUserID(context.Background(), "user-1")

		assert.NoError(t, err)
		assert.Len(t, list, 2)
		foundIDs := make(map[string]bool)
		for _, h := range list {
			foundIDs[h.ID] = true
		}
		assert.True(t, foundIDs[h1.ID])
		assert.True(t, foundIDs[h2.ID])
		assert.False(t, foundIDs[h3.ID])
	})

	t.Run("ListByUserID returns empty for new user", func(t *testing.T) {
		list, err := svc.ListByUserID(context.Background(), "user-999")
		assert.NoError(t, err)
		assert.Len(t, list, 0)
	})
}

func TestHabitService_SyncLogic(t *testing.T) {
	t.Run("GetDelta: Should return only changed items", func(t *testing.T) {
		repo := NewMockRepo()
		svc := newTestService(repo)
		ctx := context.Background()

		h1, _ := domain.NewHabit("", "Old", "user-1")
		h1.UpdatedAt = time.Now().Add(-1 * time.Hour)
		repo.Create(ctx, h1)

		lastSync := time.Now()
		time.Sleep(1 * time.Millisecond)

		h2, _ := domain.NewHabit("", "New", "user-1")
		h2.UpdatedAt = time.Now().Add(1 * time.Minute)
		repo.Create(ctx, h2)

		deltas, err := svc.GetDelta(ctx, "user-1", lastSync)

		assert.NoError(t, err)
		assert.Len(t, deltas, 1)
		assert.Equal(t, h2.ID, deltas[0].ID)
	})
}
