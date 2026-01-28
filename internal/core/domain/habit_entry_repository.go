package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrEntryNotFound = errors.New("habit entry not found")
	ErrEntryConflict = errors.New("habit entry version conflict")
)

type HabitEntryRepository interface {
	// Create persists a new entry to the storage.
	Create(ctx context.Context, entry *HabitEntry) error

	// Update modifies an existing entry.
	// Implementations must handle Optimistic Locking (version check) to prevent data races.
	Update(ctx context.Context, entry *HabitEntry) error

	// Delete performs a Soft Delete on the entry.
	// It requires userID to ensure the user actually owns the entry being deleted.
	Delete(ctx context.Context, id string, userID string) error

	// GetByID retrieves a single active (non-deleted) entry by its ID.
	GetByID(ctx context.Context, id string) (*HabitEntry, error)

	// ListByHabitID retrieves entries for a specific habit within a given date range.
	// This is optimized for UI views like calendars or charts.
	ListByHabitID(ctx context.Context, habitID string, from, to time.Time) ([]*HabitEntry, error)

	// GetChanges [SYNC ENGINE] Returns all changes (creations, updates, soft-deletes)
	// that occurred after the 'since' timestamp. Crucial for offline-first synchronization.
	GetChanges(ctx context.Context, userID string, since time.Time) ([]*HabitEntry, error)
}
