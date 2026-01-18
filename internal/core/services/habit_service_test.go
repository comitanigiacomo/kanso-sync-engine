package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
	"github.com/stretchr/testify/assert"
)

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
	m.store[habit.ID] = habit
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
	return h, nil
}

func (m *MockRepo) ListByUserID(ctx context.Context, userID string) ([]*domain.Habit, error) {
	if m.simulateError != nil {
		return nil, m.simulateError
	}
	var list []*domain.Habit
	for _, h := range m.store {
		if h.UserID == userID {
			list = append(list, h)
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
	m.store[habit.ID] = habit
	return nil
}

func (m *MockRepo) Delete(ctx context.Context, id string) error {
	if m.simulateError != nil {
		return m.simulateError
	}
	delete(m.store, id)
	return nil
}

func TestHabitService_Create(t *testing.T) {
	t.Run("Success: Should create and persist a valid habit", func(t *testing.T) {
		repo := NewMockRepo()
		svc := services.NewHabitService(repo)
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

		stored, _ := repo.GetByID(ctx, created.ID)
		assert.NotNil(t, stored)
		assert.Equal(t, created.ID, stored.ID)
	})

	t.Run("Fail: Domain Validation Error (Blocked BEFORE DB)", func(t *testing.T) {
		repo := NewMockRepo()
		svc := services.NewHabitService(repo)

		input := services.CreateHabitInput{
			UserID: "user-1",
			Title:  "",
		}

		_, err := svc.Create(context.Background(), input)

		assert.ErrorIs(t, err, domain.ErrHabitTitleEmpty)
		assert.Empty(t, repo.store)
	})

	t.Run("Fail: Repository Error (Database Down)", func(t *testing.T) {
		repo := NewMockRepo()
		repo.simulateError = errors.New("db connection lost")

		svc := services.NewHabitService(repo)

		input := services.CreateHabitInput{
			UserID: "user-1",
			Title:  "Valid Title",
			Type:   domain.HabitTypeBoolean,
		}

		_, err := svc.Create(context.Background(), input)

		assert.EqualError(t, err, "db connection lost")
	})
}

func TestHabitService_Update(t *testing.T) {
	t.Run("Success: Should update existing habit (Owner)", func(t *testing.T) {
		repo := NewMockRepo()
		svc := services.NewHabitService(repo)

		existing, _ := domain.NewHabit("Old Title", "user-1")
		repo.Create(context.Background(), existing)

		updateInput := services.UpdateHabitInput{
			ID:          existing.ID,
			UserID:      "user-1",
			Title:       "New Title",
			Description: "Updated desc",
			Color:       "#FFFFFF",
			Type:        domain.HabitTypeBoolean,
			TargetValue: 1,
			Interval:    1,
		}

		err := svc.Update(context.Background(), updateInput)

		assert.NoError(t, err)

		updated, _ := repo.GetByID(context.Background(), existing.ID)
		assert.Equal(t, "New Title", updated.Title)
		assert.Equal(t, "#FFFFFF", updated.Color)
	})

	t.Run("Fail: Security - Cannot update other user's habit (IDOR)", func(t *testing.T) {
		repo := NewMockRepo()
		svc := services.NewHabitService(repo)

		existing, _ := domain.NewHabit("Secret Habit", "user-1")
		repo.Create(context.Background(), existing)

		updateInput := services.UpdateHabitInput{
			ID:     existing.ID,
			UserID: "user-2",
			Title:  "Hacked Title",
		}

		err := svc.Update(context.Background(), updateInput)

		assert.ErrorIs(t, err, domain.ErrHabitNotFound)

		unchanged, _ := repo.GetByID(context.Background(), existing.ID)
		assert.Equal(t, "Secret Habit", unchanged.Title)
	})

	t.Run("Fail: Habit Not Found", func(t *testing.T) {
		repo := NewMockRepo()
		svc := services.NewHabitService(repo)

		input := services.UpdateHabitInput{
			ID:     "ghost-id",
			UserID: "user-1",
			Title:  "New Title",
		}

		err := svc.Update(context.Background(), input)

		assert.ErrorIs(t, err, domain.ErrHabitNotFound)
	})

	t.Run("Fail: Domain Validation during Update", func(t *testing.T) {
		repo := NewMockRepo()
		svc := services.NewHabitService(repo)

		existing, _ := domain.NewHabit("Valid", "u1")
		repo.Create(context.Background(), existing)

		input := services.UpdateHabitInput{
			ID:     existing.ID,
			UserID: "u1",
			Title:  "Valid",
			Color:  "INVALID-COLOR",
		}

		err := svc.Update(context.Background(), input)

		assert.ErrorIs(t, err, domain.ErrInvalidColor)
	})
}

func TestHabitService_Delete(t *testing.T) {
	t.Run("Success: Should delete own habit", func(t *testing.T) {
		repo := NewMockRepo()
		svc := services.NewHabitService(repo)

		h, _ := domain.NewHabit("To Delete", "user-1")
		repo.Create(context.Background(), h)

		err := svc.Delete(context.Background(), h.ID, "user-1")

		assert.NoError(t, err)

		_, err = repo.GetByID(context.Background(), h.ID)
		assert.Equal(t, domain.ErrHabitNotFound, err)
	})

	t.Run("Fail: Security - Cannot delete other user's habit (IDOR)", func(t *testing.T) {
		repo := NewMockRepo()
		svc := services.NewHabitService(repo)

		h, _ := domain.NewHabit("Don't Touch", "user-1")
		repo.Create(context.Background(), h)

		err := svc.Delete(context.Background(), h.ID, "user-2")

		assert.ErrorIs(t, err, domain.ErrHabitNotFound)

		found, _ := repo.GetByID(context.Background(), h.ID)
		assert.NotNil(t, found)
	})

	t.Run("Fail: Delete non-existent habit", func(t *testing.T) {
		repo := NewMockRepo()
		svc := services.NewHabitService(repo)

		err := svc.Delete(context.Background(), "ghost-id", "user-1")

		assert.ErrorIs(t, err, domain.ErrHabitNotFound)
	})
}

func TestHabitService_ListAndGet(t *testing.T) {
	repo := NewMockRepo()
	svc := services.NewHabitService(repo)

	h1, _ := domain.NewHabit("H1", "user-1")
	h2, _ := domain.NewHabit("H2", "user-1")
	h3, _ := domain.NewHabit("H3", "user-2")

	repo.Create(context.Background(), h1)
	repo.Create(context.Background(), h2)
	repo.Create(context.Background(), h3)

	t.Run("ListByUserID returns only user's habits", func(t *testing.T) {
		list, err := svc.ListByUserID(context.Background(), "user-1")

		assert.NoError(t, err)
		assert.Len(t, list, 2)
		ids := []string{list[0].ID, list[1].ID}
		assert.Contains(t, ids, h1.ID)
		assert.Contains(t, ids, h2.ID)
	})

	t.Run("ListByUserID returns empty for new user", func(t *testing.T) {
		list, err := svc.ListByUserID(context.Background(), "user-999")
		assert.NoError(t, err)
		assert.Len(t, list, 0)
	})
}
