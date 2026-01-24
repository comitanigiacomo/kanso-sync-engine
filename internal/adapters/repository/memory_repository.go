package repository

import (
	"context"
	"sort"
	"sync"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
)

type InMemoryHabitRepository struct {
	store map[string]*domain.Habit

	mu sync.RWMutex
}

func NewInMemoryHabitRepository() *InMemoryHabitRepository {
	return &InMemoryHabitRepository{
		store: make(map[string]*domain.Habit),
	}
}

func (r *InMemoryHabitRepository) Create(ctx context.Context, habit *domain.Habit) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.store[habit.ID] = habit
	return nil
}

func (r *InMemoryHabitRepository) GetByID(ctx context.Context, id string) (*domain.Habit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	habit, ok := r.store[id]
	if !ok {
		return nil, domain.ErrHabitNotFound
	}
	return habit, nil
}

func (r *InMemoryHabitRepository) ListByUserID(ctx context.Context, userID string) ([]*domain.Habit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var habits []*domain.Habit
	for _, h := range r.store {
		if h.UserID == userID {
			habits = append(habits, h)
		}
	}

	sort.Slice(habits, func(i, j int) bool {
		return habits[i].SortOrder < habits[j].SortOrder
	})

	return habits, nil
}

func (r *InMemoryHabitRepository) Update(ctx context.Context, habit *domain.Habit) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.store[habit.ID]; !ok {
		return domain.ErrHabitNotFound
	}

	r.store[habit.ID] = habit
	return nil
}

func (r *InMemoryHabitRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.store[id]; !ok {
		return domain.ErrHabitNotFound
	}

	delete(r.store, id)
	return nil
}
