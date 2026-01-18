package domain

import (
	"context"
	"errors"
)

var ErrHabitNotFound = errors.New("habit not found")

type HabitRepository interface {
	// Create persists a new habit definition in the storage.
	Create(ctx context.Context, habit *Habit) error

	// GetByID retrieves a habit by its unique identifier.
	// Returns (nil, ErrHabitNotFound) if the habit does not exist.
	GetByID(ctx context.Context, id string) (*Habit, error)

	// ListByUserID retrieves all habits associated with a specific user.
	// IMPLEMENTATION NOTE: Results must be ordered by 'sort_order' ascending.
	ListByUserID(ctx context.Context, userID string) ([]*Habit, error)

	// Update modifies the state of an existing habit.
	// Returns ErrHabitNotFound if the habit with the given ID does not exist.
	Update(ctx context.Context, habit *Habit) error

	// Delete permanently removes a habit from the system (Hard Delete).
	Delete(ctx context.Context, id string) error
}
