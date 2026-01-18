package domain_test

import (
	"context"
	"testing"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
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
	r.habits[habit.ID] = habit
	return nil
}

func (r *InMemoryHabitRepository) GetByID(ctx context.Context, id string) (*domain.Habit, error) {
	habit, exists := r.habits[id]
	if !exists {
		return nil, domain.ErrHabitNotFound
	}
	return habit, nil
}

func (r *InMemoryHabitRepository) ListByUserID(ctx context.Context, userID string) ([]*domain.Habit, error) {
	return nil, nil
}
func (r *InMemoryHabitRepository) Update(ctx context.Context, habit *domain.Habit) error { return nil }
func (r *InMemoryHabitRepository) Delete(ctx context.Context, id string) error           { return nil }

func TestHabitRepository_Contract(t *testing.T) {
	repo := NewInMemoryHabitRepository()
	ctx := context.Background()

	habit, err := domain.NewHabit("Drink Water", "user-123")
	if err != nil {
		t.Fatalf("failed to create habit: %v", err)
	}

	t.Run("Should save and retrieve a habit", func(t *testing.T) {
		err := repo.Create(ctx, habit)
		if err != nil {
			t.Fatalf("expected no error on Create, got %v", err)
		}

		found, err := repo.GetByID(ctx, habit.ID)
		if err != nil {
			t.Fatalf("expected no error on GetByID, got %v", err)
		}

		if found.Title != "Drink Water" {
			t.Errorf("expected title 'Drink Water', got '%s'", found.Title)
		}
	})

	t.Run("Should return ErrHabitNotFound for missing ID", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "non-existent-id")

		if err != domain.ErrHabitNotFound {
			t.Errorf("expected error '%v', got '%v'", domain.ErrHabitNotFound, err)
		}
	})
}
