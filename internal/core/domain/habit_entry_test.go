package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewHabitEntry(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Rome")
	if loc == nil {
		loc = time.UTC
	}

	inputDate := time.Date(2026, 1, 28, 10, 0, 0, 0, loc)
	habitID := "habit-123"
	userID := "user-456"
	value := 500

	entry := NewHabitEntry(habitID, userID, inputDate, value)

	t.Run("Should set core identity fields correctly", func(t *testing.T) {
		assert.Equal(t, habitID, entry.HabitID)
		assert.Equal(t, userID, entry.UserID)
		assert.Equal(t, value, entry.Value)
	})

	t.Run("Should initialize Sync Engine fields", func(t *testing.T) {
		assert.Equal(t, 1, entry.Version, "Version must always start at 1 for optimistic locking")
		assert.False(t, entry.CreatedAt.IsZero(), "CreatedAt must be set")
		assert.False(t, entry.UpdatedAt.IsZero(), "UpdatedAt must be set")
		assert.Nil(t, entry.DeletedAt, "DeletedAt must be nil on creation")
	})

	t.Run("Should force CompletionDate to UTC", func(t *testing.T) {
		assert.Equal(t, inputDate.UTC(), entry.CompletionDate, "Date must be converted to UTC automatically")
		assert.Equal(t, "UTC", entry.CompletionDate.Location().String())
	})
}

func TestHabitEntry_Validate(t *testing.T) {
	validDate := time.Now()

	tests := []struct {
		name        string
		entry       *HabitEntry
		shouldError bool
		errorMsg    string
	}{
		{
			name: "Valid Entry",
			entry: &HabitEntry{
				HabitID: "h-1", UserID: "u-1", CompletionDate: validDate, Value: 1,
			},
			shouldError: false,
		},
		{
			name: "Missing HabitID",
			entry: &HabitEntry{
				HabitID: "", UserID: "u-1", CompletionDate: validDate, Value: 1,
			},
			shouldError: true,
			errorMsg:    "habit_id is required",
		},
		{
			name: "Missing UserID",
			entry: &HabitEntry{
				HabitID: "h-1", UserID: "", CompletionDate: validDate, Value: 1,
			},
			shouldError: true,
			errorMsg:    "user_id is required",
		},
		{
			name: "Only Whitespace ID",
			entry: &HabitEntry{
				HabitID: "   ", UserID: "u-1", CompletionDate: validDate, Value: 1,
			},
			shouldError: true,
			errorMsg:    "habit_id is required",
		},
		{
			name: "Negative Value",
			entry: &HabitEntry{
				HabitID: "h-1", UserID: "u-1", CompletionDate: validDate, Value: -10,
			},
			shouldError: true,
			errorMsg:    "value cannot be negative",
		},
		{
			name: "Zero Date",
			entry: &HabitEntry{
				HabitID: "h-1", UserID: "u-1", CompletionDate: time.Time{}, Value: 1,
			},
			shouldError: true,
			errorMsg:    "completion_date is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entry.Validate()

			if tt.shouldError {
				assert.Error(t, err)
				if err != nil {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
