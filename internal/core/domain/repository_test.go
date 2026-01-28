package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type InMemoryHabitRepository struct {
	habits map[string]*domain.Habit
}

func NewInMemoryHabitRepository() *InMemoryHabitRepository {
	return &InMemoryHabitRepository{
		habits: make(map[string]*domain.Habit),
	}
}

func (r *InMemoryHabitRepository) Create(ctx context.Context, habit *domain.Habit) error {
	if habit.Version == 0 {
		habit.Version = 1
	}
	r.habits[habit.ID] = habit
	return nil
}

func (r *InMemoryHabitRepository) GetByID(ctx context.Context, id string) (*domain.Habit, error) {
	habit, exists := r.habits[id]
	if !exists {
		return nil, domain.ErrHabitNotFound
	}
	if habit.DeletedAt != nil {
		return nil, domain.ErrHabitNotFound
	}
	copied := *habit
	return &copied, nil
}

func (r *InMemoryHabitRepository) ListByUserID(ctx context.Context, userID string) ([]*domain.Habit, error) {
	var list []*domain.Habit
	for _, h := range r.habits {
		if h.UserID == userID && h.DeletedAt == nil {
			val := *h
			list = append(list, &val)
		}
	}
	return list, nil
}

func (r *InMemoryHabitRepository) Update(ctx context.Context, habit *domain.Habit) error {
	existing, exists := r.habits[habit.ID]
	if !exists {
		return domain.ErrHabitNotFound
	}
	if existing.DeletedAt != nil {
		return domain.ErrHabitNotFound
	}

	if habit.Version != existing.Version {
		return domain.ErrHabitConflict
	}

	habit.Version++
	habit.UpdatedAt = time.Now().UTC()

	r.habits[habit.ID] = habit
	return nil
}

func (r *InMemoryHabitRepository) Delete(ctx context.Context, id string) error {
	habit, exists := r.habits[id]
	if !exists {
		return domain.ErrHabitNotFound
	}
	if habit.DeletedAt != nil {
		return domain.ErrHabitNotFound
	}

	now := time.Now().UTC()
	habit.DeletedAt = &now
	habit.UpdatedAt = now
	habit.Version++

	return nil
}

func (r *InMemoryHabitRepository) GetChanges(ctx context.Context, userID string, since time.Time) ([]*domain.Habit, error) {
	var changes []*domain.Habit
	for _, h := range r.habits {
		if h.UserID == userID && h.UpdatedAt.After(since) {
			val := *h
			changes = append(changes, &val)
		}
	}
	return changes, nil
}

func TestHabitRepository_Contract_60k(t *testing.T) {
	repo := NewInMemoryHabitRepository()
	ctx := context.Background()

	habit, _ := domain.NewHabit("Drink Water", "user-123")
	habit.FrequencyType = "daily"
	habit.Interval = 1
	habit.TargetValue = 1

	t.Run("Create & Get", func(t *testing.T) {
		err := repo.Create(ctx, habit)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, habit.ID)
		require.NoError(t, err)
		assert.Equal(t, "Drink Water", found.Title)
		assert.Equal(t, 1, found.Version)
	})

	t.Run("Optimistic Locking (Conflict)", func(t *testing.T) {
		v1, err := repo.GetByID(ctx, habit.ID)
		require.NoError(t, err)

		v1_cloneA := *v1
		v1_cloneA.Title = "Title by Device A"
		err = repo.Update(ctx, &v1_cloneA)
		require.NoError(t, err)

		v1_cloneB := *v1
		v1_cloneB.Title = "Title by Device B"

		err = repo.Update(ctx, &v1_cloneB)

		assert.Error(t, err)
		assert.Equal(t, domain.ErrHabitConflict, err, "In-memory repository must simulate version checking")
	})

	t.Run("Delta Sync (GetChanges)", func(t *testing.T) {
		deltaRepo := NewInMemoryHabitRepository()

		h1, _ := domain.NewHabit("Old Habit", "user-sync")
		deltaRepo.Create(ctx, h1)

		time.Sleep(10 * time.Millisecond)
		lastSync := time.Now().UTC()
		time.Sleep(10 * time.Millisecond)

		h2, _ := domain.NewHabit("New Habit", "user-sync")
		deltaRepo.Create(ctx, h2)

		h1_update, _ := deltaRepo.GetByID(ctx, h1.ID)
		h1_update.Title = "Updated Habit"
		deltaRepo.Update(ctx, h1_update)

		changes, err := deltaRepo.GetChanges(ctx, "user-sync", lastSync)
		require.NoError(t, err)

		assert.Len(t, changes, 2)
	})

	t.Run("Soft Delete Logic", func(t *testing.T) {
		err := repo.Delete(ctx, habit.ID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, habit.ID)
		assert.Equal(t, domain.ErrHabitNotFound, err)

		changes, err := repo.GetChanges(ctx, "user-123", time.Time{})
		require.NoError(t, err)

		var deletedHabit *domain.Habit
		for _, c := range changes {
			if c.ID == habit.ID {
				deletedHabit = c
				break
			}
		}

		require.NotNil(t, deletedHabit, "GetChanges must return the deleted item")
		assert.NotNil(t, deletedHabit.DeletedAt, "DeletedAt must be set")
	})
}
