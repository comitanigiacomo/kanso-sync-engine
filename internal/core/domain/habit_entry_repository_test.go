package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
)

type InMemoryEntryRepository struct {
	entries map[string]*domain.HabitEntry
}

func NewInMemoryEntryRepository() *InMemoryEntryRepository {
	return &InMemoryEntryRepository{
		entries: make(map[string]*domain.HabitEntry),
	}
}

func (r *InMemoryEntryRepository) Create(ctx context.Context, entry *domain.HabitEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	if entry.Version == 0 {
		entry.Version = 1
	}
	r.entries[entry.ID] = entry
	return nil
}

func (r *InMemoryEntryRepository) GetByID(ctx context.Context, id string) (*domain.HabitEntry, error) {
	entry, exists := r.entries[id]
	if !exists {
		return nil, domain.ErrEntryNotFound
	}
	if entry.DeletedAt != nil {
		return nil, domain.ErrEntryNotFound
	}
	copied := *entry
	return &copied, nil
}

func (r *InMemoryEntryRepository) Update(ctx context.Context, entry *domain.HabitEntry) error {
	existing, exists := r.entries[entry.ID]
	if !exists {
		return domain.ErrEntryNotFound
	}

	if entry.Version != existing.Version {
		return domain.ErrEntryConflict
	}

	entry.Version++
	entry.UpdatedAt = time.Now().UTC()
	r.entries[entry.ID] = entry
	return nil
}

func (r *InMemoryEntryRepository) Delete(ctx context.Context, id string, userID string) error {
	entry, exists := r.entries[id]
	if !exists {
		return domain.ErrEntryNotFound
	}
	if entry.UserID != userID {
		return domain.ErrEntryNotFound
	}

	now := time.Now().UTC()
	entry.DeletedAt = &now
	entry.UpdatedAt = now
	entry.Version++

	return nil
}

func (r *InMemoryEntryRepository) ListByHabitID(ctx context.Context, habitID string, from, to time.Time) ([]*domain.HabitEntry, error) {
	var list []*domain.HabitEntry
	for _, e := range r.entries {
		if e.HabitID == habitID && e.DeletedAt == nil {
			if (e.CompletionDate.Equal(from) || e.CompletionDate.After(from)) &&
				(e.CompletionDate.Equal(to) || e.CompletionDate.Before(to)) {
				val := *e
				list = append(list, &val)
			}
		}
	}
	return list, nil
}

func (r *InMemoryEntryRepository) GetChanges(ctx context.Context, userID string, since time.Time) ([]*domain.HabitEntry, error) {
	var changes []*domain.HabitEntry
	for _, e := range r.entries {
		if e.UserID == userID && e.UpdatedAt.After(since) {
			val := *e
			changes = append(changes, &val)
		}
	}
	return changes, nil
}

func TestHabitEntryRepository_Contract_60k(t *testing.T) {
	repo := NewInMemoryEntryRepository()
	ctx := context.Background()

	habitID := "habit-abc"
	userID := "user-xyz"
	today := time.Now().UTC()

	entry := domain.NewHabitEntry(habitID, userID, today, 500)

	t.Run("Create & Get", func(t *testing.T) {
		err := repo.Create(ctx, entry)
		require.NoError(t, err)
		assert.NotEmpty(t, entry.ID)

		found, err := repo.GetByID(ctx, entry.ID)
		require.NoError(t, err)
		assert.Equal(t, 500, found.Value)
		assert.Equal(t, 1, found.Version)
	})

	t.Run("Optimistic Locking (Conflict)", func(t *testing.T) {
		v1, _ := repo.GetByID(ctx, entry.ID)

		v1_cloneA := *v1
		v1_cloneA.Value = 600
		err := repo.Update(ctx, &v1_cloneA)
		require.NoError(t, err)

		v1_cloneB := *v1
		v1_cloneB.Value = 700
		err = repo.Update(ctx, &v1_cloneB)

		assert.Error(t, err)
		assert.Equal(t, domain.ErrEntryConflict, err, "Repository must prevent overwrites of newer versions")
	})

	t.Run("ListByHabitID (Calendar Filter)", func(t *testing.T) {
		cleanRepo := NewInMemoryEntryRepository()

		d1 := domain.NewHabitEntry(habitID, userID, today.Add(-48*time.Hour), 10)
		d2 := domain.NewHabitEntry(habitID, userID, today, 20)
		d3 := domain.NewHabitEntry(habitID, userID, today.Add(48*time.Hour), 30)

		cleanRepo.Create(ctx, d1)
		cleanRepo.Create(ctx, d2)
		cleanRepo.Create(ctx, d3)

		start := today.Add(-24 * time.Hour)
		end := today.Add(24 * time.Hour)

		list, err := cleanRepo.ListByHabitID(ctx, habitID, start, end)
		require.NoError(t, err)

		assert.Len(t, list, 1)
		assert.Equal(t, 20, list[0].Value, "Should return only the entry within date range")
	})

	t.Run("Delta Sync & Soft Delete", func(t *testing.T) {
		syncRepo := NewInMemoryEntryRepository()

		e1 := domain.NewHabitEntry("h-1", "user-sync", today, 1)
		syncRepo.Create(ctx, e1)

		time.Sleep(10 * time.Millisecond)
		lastSync := time.Now().UTC()
		time.Sleep(10 * time.Millisecond)

		e2 := domain.NewHabitEntry("h-1", "user-sync", today, 2)
		syncRepo.Create(ctx, e2)

		syncRepo.Delete(ctx, e1.ID, "user-sync")

		changes, err := syncRepo.GetChanges(ctx, "user-sync", lastSync)
		require.NoError(t, err)

		assert.Len(t, changes, 2)

		var deletedItem *domain.HabitEntry
		for _, c := range changes {
			if c.ID == e1.ID {
				deletedItem = c
			}
		}
		require.NotNil(t, deletedItem, "Deleted item must be in sync changes")
		assert.NotNil(t, deletedItem.DeletedAt, "DeletedAt must be set for sync actions")
	})
}
