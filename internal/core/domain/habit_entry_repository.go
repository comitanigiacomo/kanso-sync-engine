package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrEntryNotFound = errors.New("habit entry not found")
	ErrEntryConflict = errors.New("habit entry version conflict")
	ErrUnauthorized  = errors.New("unauthorized action")
)

type HabitEntryRepository interface {
	// Create persists a new entry to the storage.
	Create(ctx context.Context, entry *HabitEntry) error

	// Update modifies an existing entry.
	Update(ctx context.Context, entry *HabitEntry) error

	// Delete performs a Soft Delete on the entry.
	Delete(ctx context.Context, id string, userID string) error

	// GetByID retrieves a single active (non-deleted) entry by its ID.
	GetByID(ctx context.Context, id string) (*HabitEntry, error)

	ListByHabitID(ctx context.Context, habitID string) ([]*HabitEntry, error)

	ListByHabitIDWithRange(ctx context.Context, habitID string, from, to time.Time) ([]*HabitEntry, error)

	// GetChanges returns changes for sync.
	GetChanges(ctx context.Context, userID string, since time.Time) ([]*HabitEntry, error)
}
