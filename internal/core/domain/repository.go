package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrHabitNotFound = errors.New("habit not found")
)

type HabitRepository interface {
	// Create persists a new habit definition in the storage.
	Create(ctx context.Context, habit *Habit) error

	// GetByID retrieves a habit by its unique identifier.
	GetByID(ctx context.Context, id string) (*Habit, error)

	// ListByUserID retrieves all habits associated with a specific user.
	ListByUserID(ctx context.Context, userID string) ([]*Habit, error)

	// Update modifies the state of an existing habit.
	Update(ctx context.Context, habit *Habit) error

	// Delete permanently removes a habit from the system.
	Delete(ctx context.Context, id string) error

	// GetChanges [SYNC] Returns only the deltas (changes) occurring after a specific date.
	GetChanges(ctx context.Context, userID string, since time.Time) ([]*Habit, error)

	UpdateStreaks(ctx context.Context, id string, current, longest int) error
}
