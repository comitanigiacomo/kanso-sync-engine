package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewHabit(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		title        string
		color        string
		icon         string
		target       int
		interval     int
		weekdays     []int
		wantErr      error
		wantFreq     string
		wantInterval int
	}{
		{
			name:         "Success: Daily Habit (Default inputs)",
			userID:       "u1",
			title:        "Bere",
			interval:     0,
			weekdays:     nil,
			wantErr:      nil,
			wantFreq:     "daily",
			wantInterval: 1,
		},
		{
			name:         "Success: Interval (Every 3 Days)",
			userID:       "u1",
			title:        "Leggere",
			interval:     3,
			weekdays:     nil,
			wantErr:      nil,
			wantFreq:     "interval",
			wantInterval: 3,
		},
		{
			name:         "Success: Zero Target is Allowed",
			userID:       "u1",
			title:        "Non fumare",
			target:       0,
			wantErr:      nil,
			wantFreq:     "daily",
			wantInterval: 1,
		},
		{
			name:         "Priority: Weekdays win over Interval",
			userID:       "u1",
			title:        "Gym",
			interval:     5,
			weekdays:     []int{1, 3},
			wantErr:      nil,
			wantFreq:     "specific_days",
			wantInterval: 5,
		},
		{
			name:         "Logic: Interval=1 becomes Daily",
			userID:       "u1",
			title:        "Walk",
			interval:     1,
			weekdays:     nil,
			wantErr:      nil,
			wantFreq:     "daily",
			wantInterval: 1,
		},
		{
			name:         "Success: Short Hex Color",
			userID:       "u1",
			title:        "Short Color",
			color:        "#FFF",
			wantErr:      nil,
			wantFreq:     "daily",
			wantInterval: 1,
		},
		{
			name:    "Error: Color without Hash",
			userID:  "u1",
			title:   "Bad Color",
			color:   "FFFFFF",
			wantErr: ErrInvalidColor,
		},
		{
			name:    "Error: Color Invalid Chars",
			userID:  "u1",
			title:   "Bad Color",
			color:   "#ZZZZZZ",
			wantErr: ErrInvalidColor,
		},
		{
			name:    "Error: Color Wrong Length",
			userID:  "u1",
			title:   "Bad Color",
			color:   "#1234",
			wantErr: ErrInvalidColor,
		},
		{
			name:         "Success: Boundary Days (Sunday 0 & Saturday 6)",
			userID:       "u1",
			title:        "Weekend",
			weekdays:     []int{0, 6},
			wantErr:      nil,
			wantFreq:     "specific_days",
			wantInterval: 1,
		},
		{
			name:     "Error: Weekday 7 (Out of range)",
			userID:   "u1",
			title:    "Bad Day",
			weekdays: []int{7},
			wantErr:  ErrInvalidWeekdays,
		},
		{
			name:     "Error: Negative Weekday",
			userID:   "u1",
			title:    "Bad Day",
			weekdays: []int{-1},
			wantErr:  ErrInvalidWeekdays,
		},
		{
			name:    "Error: Empty Title",
			userID:  "u1",
			title:   "",
			wantErr: ErrHabitTitleEmpty,
		},
		{
			name:    "Error: Whitespace Only Title",
			userID:  "u1",
			title:   "   \t \n ",
			wantErr: ErrHabitTitleEmpty,
		},
		{
			name:    "Error: Negative Target",
			userID:  "u1",
			title:   "Bad Target",
			target:  -1,
			wantErr: ErrInvalidTarget,
		},
		{
			name:     "Error: Negative Interval",
			userID:   "u1",
			title:    "Bad Interval",
			interval: -5,
			wantErr:  ErrInvalidInterval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			habit, err := NewHabit(
				tt.userID, tt.title, "desc", tt.color, tt.icon, "unit",
				tt.target, tt.interval, tt.weekdays,
			)

			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
				assert.Nil(t, habit)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, habit)

				assert.Equal(t, tt.wantFreq, habit.FrequencyType)
				assert.Equal(t, tt.wantInterval, habit.Interval)

				assert.NotEmpty(t, habit.ID)
				assert.WithinDuration(t, time.Now().UTC(), habit.CreatedAt, 2*time.Second)
			}
		})
	}
}
func TestHabit_Update(t *testing.T) {
	createStandardHabit := func() *Habit {
		h, _ := NewHabit("u1", "Original Title", "Original Desc", "#000000", "icon", "ml", 10, 0, nil)
		time.Sleep(1 * time.Millisecond)
		return h
	}

	t.Run("Success: Full Update", func(t *testing.T) {
		habit := createStandardHabit()
		originalTime := habit.UpdatedAt

		err := habit.Update("New Title", "New Desc", "#FFFFFF", "new_icon", "kg", 20, 3, nil)

		assert.Nil(t, err)
		assert.Equal(t, "New Title", habit.Title)
		assert.Equal(t, "New Desc", habit.Description)
		assert.Equal(t, "#FFFFFF", habit.Color)
		assert.Equal(t, "new_icon", habit.Icon)
		assert.Equal(t, "kg", habit.Unit)
		assert.Equal(t, 20, habit.TargetValue)
		assert.Equal(t, 3, habit.Interval)
		assert.Equal(t, "interval", habit.FrequencyType) // Logica ricalcolata
		assert.Nil(t, habit.Weekdays)
		assert.True(t, habit.UpdatedAt.After(originalTime)) // Timestamp aggiornato
	})

	t.Run("Success: Default Icon Logic", func(t *testing.T) {
		habit := createStandardHabit()
		err := habit.Update("Title", "Desc", "#FFF", "", "kg", 10, 1, nil)

		assert.Nil(t, err)
		assert.Equal(t, "default_icon", habit.Icon)
	})

	t.Run("Error: Validation Failure (Empty Title)", func(t *testing.T) {
		habit := createStandardHabit()
		previousTitle := habit.Title

		err := habit.Update("", "Desc", "#FFF", "icon", "kg", 10, 1, nil)

		assert.Equal(t, ErrHabitTitleEmpty, err)
		assert.Equal(t, previousTitle, habit.Title)
	})

	t.Run("Error: Validation Failure (Invalid Color)", func(t *testing.T) {
		habit := createStandardHabit()
		err := habit.Update("Title", "Desc", "INVALID", "icon", "kg", 10, 1, nil)
		assert.Equal(t, ErrInvalidColor, err)
	})

	t.Run("Error: Archived Habit Cannot Be Updated", func(t *testing.T) {
		habit := createStandardHabit()

		now := time.Now()
		habit.ArchivedAt = &now

		err := habit.Update("Try Update", "Desc", "#FFF", "icon", "kg", 10, 1, nil)

		assert.Equal(t, ErrHabitArchived, err)
	})
}
