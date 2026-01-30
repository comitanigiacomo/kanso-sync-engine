package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidEntry = errors.New("invalid habit entry data")
)

type HabitEntry struct {
	ID      string `json:"id" db:"id"`
	HabitID string `json:"habit_id" db:"habit_id"`
	UserID  string `json:"user_id" db:"user_id"`

	CompletionDate time.Time `json:"completion_date" db:"completion_date"`
	Value          int       `json:"value" db:"value"`
	Notes          string    `json:"notes" db:"notes"`

	Version   int        `json:"version" db:"version"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

func NewHabitEntry(habitID, userID string, date time.Time, value int) *HabitEntry {
	now := time.Now().UTC()

	return &HabitEntry{
		HabitID:        habitID,
		UserID:         userID,
		CompletionDate: date.UTC(),
		Value:          value,

		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (e *HabitEntry) Validate() error {
	if strings.TrimSpace(e.HabitID) == "" {
		return errors.New("habit_id is required")
	}
	if strings.TrimSpace(e.UserID) == "" {
		return errors.New("user_id is required")
	}
	if e.Value < 0 {
		return errors.New("value cannot be negative")
	}
	if e.CompletionDate.IsZero() {
		return errors.New("completion_date is required")
	}
	return nil
}
