package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestNewHabit(t *testing.T) {
	t.Run("Success: Creates valid habit with defaults AND Sync fields", func(t *testing.T) {
		h, err := domain.NewHabit("Drink Water", "u1")

		assert.Nil(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, "Drink Water", h.Title)
		assert.Equal(t, "u1", h.UserID)
		assert.NotEmpty(t, h.ID)

		assert.Equal(t, domain.HabitTypeBoolean, h.Type)
		assert.Equal(t, 1, h.TargetValue)
		assert.Equal(t, domain.HabitFreqDaily, h.FrequencyType)

		assert.Equal(t, 0, h.CurrentStreak)
		assert.Equal(t, 0, h.LongestStreak)

		assert.Equal(t, 1, h.Version, "New habits MUST start at Version 1 for Optimistic Locking")
		assert.Nil(t, h.DeletedAt, "New habits MUST NOT be marked as deleted")

		assert.WithinDuration(t, time.Now().UTC(), h.CreatedAt, 2*time.Second)
	})

	t.Run("Error: Empty Title", func(t *testing.T) {
		_, err := domain.NewHabit("", "u1")
		assert.Equal(t, domain.ErrHabitTitleEmpty, err)
	})

	t.Run("Error: Invalid UserID", func(t *testing.T) {
		_, err := domain.NewHabit("Title", "")
		assert.Equal(t, domain.ErrHabitInvalidUserID, err)
	})
}

func TestHabit_Validation(t *testing.T) {
	tests := []struct {
		name         string
		title        string
		description  string
		color        string
		hType        string
		reminder     string
		target       int
		interval     int
		weekdays     []int
		wantErr      error
		wantTarget   int
		wantFreq     string
		wantInterval int
	}{
		{
			name:         "Success: Interval (Every 3 Days)",
			title:        "Leggere",
			description:  "desc",
			hType:        domain.HabitTypeTimer,
			reminder:     "",
			interval:     3,
			weekdays:     nil,
			target:       30,
			wantTarget:   30,
			wantErr:      nil,
			wantFreq:     domain.HabitFreqInterval,
			wantInterval: 3,
		},
		{
			name:         "Success: Boolean Forces Target to 1",
			title:        "Non fumare",
			description:  "desc",
			hType:        domain.HabitTypeBoolean,
			reminder:     "",
			target:       100,
			wantTarget:   1,
			wantErr:      nil,
			wantFreq:     domain.HabitFreqDaily,
			wantInterval: 1,
		},
		{
			name:         "Success: Valid Reminder",
			title:        "Sveglia",
			description:  "desc",
			hType:        domain.HabitTypeBoolean,
			reminder:     "07:30",
			target:       1,
			wantTarget:   1,
			wantErr:      nil,
			wantFreq:     domain.HabitFreqDaily,
			wantInterval: 1,
		},
		{
			name:         "Priority: Weekdays win over Interval",
			title:        "Gym",
			description:  "desc",
			hType:        domain.HabitTypeBoolean,
			reminder:     "",
			interval:     5,
			weekdays:     []int{1, 3},
			target:       1,
			wantTarget:   1,
			wantErr:      nil,
			wantFreq:     domain.HabitFreqSpecificDays,
			wantInterval: 5,
		},
		{
			name:         "Success: Short Hex Color",
			title:        "Color",
			description:  "desc",
			color:        "#FFF",
			hType:        domain.HabitTypeNumeric,
			target:       1,
			wantTarget:   1,
			wantErr:      nil,
			wantFreq:     domain.HabitFreqDaily,
			wantInterval: 1,
		},
		{
			name:        "Error: Title Too Long",
			title:       strings.Repeat("a", 101),
			description: "desc",
			hType:       domain.HabitTypeNumeric,
			wantErr:     domain.ErrHabitTitleTooLong,
		},
		{
			name:        "Error: Invalid Habit Type",
			title:       "Bad Type",
			description: "desc",
			hType:       "magic_spell",
			wantErr:     domain.ErrInvalidHabitType,
		},
		{
			name:        "Error: Invalid Reminder Format (Letters)",
			title:       "Bad Time",
			description: "desc",
			hType:       domain.HabitTypeBoolean,
			reminder:    "hello",
			wantErr:     domain.ErrInvalidReminder,
		},
		{
			name:        "Error: Invalid Reminder Format (Out of range)",
			title:       "Bad Time",
			description: "desc",
			hType:       domain.HabitTypeBoolean,
			reminder:    "25:00",
			wantErr:     domain.ErrInvalidReminder,
		},
		{
			name:        "Error: Color Invalid Chars",
			title:       "Bad Color",
			description: "desc",
			color:       "#ZZZZZZ",
			hType:       domain.HabitTypeNumeric,
			wantErr:     domain.ErrInvalidColor,
		},
		{
			name:        "Error: Color Wrong Length",
			title:       "Bad Color",
			description: "desc",
			color:       "#1234",
			hType:       domain.HabitTypeNumeric,
			wantErr:     domain.ErrInvalidColor,
		},
		{
			name:         "Success: Boundary Days (Sun 0 & Sat 6)",
			title:        "Weekend",
			description:  "desc",
			hType:        domain.HabitTypeBoolean,
			weekdays:     []int{0, 6},
			target:       1,
			wantTarget:   1,
			wantErr:      nil,
			wantFreq:     domain.HabitFreqSpecificDays,
			wantInterval: 1,
		},
		{
			name:        "Error: Weekday 7 (Out of range)",
			title:       "Bad Day",
			description: "desc",
			hType:       domain.HabitTypeNumeric,
			weekdays:    []int{7},
			wantErr:     domain.ErrInvalidWeekdays,
		},
		{
			name:        "Error: Negative Weekday",
			title:       "Bad Day",
			description: "desc",
			hType:       domain.HabitTypeNumeric,
			weekdays:    []int{-1},
			wantErr:     domain.ErrInvalidWeekdays,
		},
		{
			name:        "Error: Negative Target",
			title:       "Bad Target",
			description: "desc",
			hType:       domain.HabitTypeNumeric,
			target:      -1,
			wantErr:     domain.ErrInvalidTarget,
		},
		{
			name:        "Error: Negative Interval",
			title:       "Bad Interval",
			description: "desc",
			hType:       domain.HabitTypeNumeric,
			interval:    -5,
			wantErr:     domain.ErrInvalidInterval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			habit, _ := domain.NewHabit("Base Title", "u1")

			err := habit.Update(
				tt.title, tt.description, tt.color, "icon",
				tt.hType, tt.reminder, "unit",
				tt.target, tt.interval, tt.weekdays,
			)

			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
			} else {
				assert.Nil(t, err)

				assert.Equal(t, tt.hType, habit.Type)
				assert.Equal(t, tt.wantTarget, habit.TargetValue)
				assert.Equal(t, tt.wantFreq, habit.FrequencyType)
				assert.Equal(t, tt.wantInterval, habit.Interval)

				if tt.reminder != "" {
					assert.NotNil(t, habit.ReminderTime)
					assert.Equal(t, tt.reminder, *habit.ReminderTime)
				} else {
					assert.Nil(t, habit.ReminderTime)
				}
			}
		})
	}
}

func TestHabit_Lifecycle(t *testing.T) {
	createStandardHabit := func() *domain.Habit {
		h, _ := domain.NewHabit("Original Title", "u1")
		_ = h.Update("Original Title", "Desc", "#000", "icon", domain.HabitTypeNumeric, "", "ml", 10, 1, nil)
		time.Sleep(1 * time.Millisecond)
		return h
	}

	t.Run("Success: Update changes UpdatedAt BUT NOT Version", func(t *testing.T) {
		habit := createStandardHabit()
		originalTime := habit.UpdatedAt
		originalVersion := habit.Version

		err := habit.Update("New Title", "New Desc", "#FFF", "new_icon",
			domain.HabitTypeTimer, "20:00", "kg", 20, 3, nil)

		assert.Nil(t, err)
		assert.Equal(t, "New Title", habit.Title)
		assert.True(t, habit.UpdatedAt.After(originalTime))

		assert.Equal(t, originalVersion, habit.Version, "Domain Update must NOT increment version manually")
	})

	t.Run("Success: Clear Reminder", func(t *testing.T) {
		habit := createStandardHabit()
		_ = habit.Update("T", "D", "#000", "i", domain.HabitTypeBoolean, "09:00", "u", 1, 1, nil)
		assert.NotNil(t, habit.ReminderTime)

		err := habit.Update("T", "D", "#000", "i", domain.HabitTypeBoolean, "", "u", 1, 1, nil)

		assert.Nil(t, err)
		assert.Nil(t, habit.ReminderTime)
	})

	t.Run("Archive: Soft Delete Flow", func(t *testing.T) {
		habit := createStandardHabit()

		habit.Archive()

		assert.NotNil(t, habit.ArchivedAt)

		err := habit.Update("Fail", "", "", "", domain.HabitTypeBoolean, "", "", 1, 1, nil)
		assert.Equal(t, domain.ErrHabitArchived, err)

		habit.Restore()
		assert.Nil(t, habit.ArchivedAt)

		err = habit.Update("Success", "", "", "", domain.HabitTypeBoolean, "", "", 1, 1, nil)
		assert.Nil(t, err)
	})
}

func TestHabit_UpdateStreak(t *testing.T) {
	t.Run("Success: Update Streak values and timestamp", func(t *testing.T) {
		habit, _ := domain.NewHabit("Streak Test", "u1")
		originalTime := habit.UpdatedAt
		time.Sleep(1 * time.Millisecond)

		habit.UpdateStreak(5, 10)

		assert.Equal(t, 5, habit.CurrentStreak)
		assert.Equal(t, 10, habit.LongestStreak)
		assert.True(t, habit.UpdatedAt.After(originalTime), "UpdateStreak must update UpdatedAt")
	})
}

func TestHabit_ChangePosition(t *testing.T) {
	h, _ := domain.NewHabit("Sort Me", "u1")
	originalUpdate := h.UpdatedAt
	time.Sleep(1 * time.Millisecond)

	t.Run("Success: Change Sort Order", func(t *testing.T) {
		err := h.ChangePosition(5)

		assert.Nil(t, err)
		assert.Equal(t, 5, h.SortOrder)
		assert.True(t, h.UpdatedAt.After(originalUpdate))
	})

	t.Run("Error: Cannot Change Position of Archived", func(t *testing.T) {
		h.Archive()
		err := h.ChangePosition(10)
		assert.Equal(t, domain.ErrHabitArchived, err)
	})
}

func TestHabit_DefensiveCopyAndHygiene(t *testing.T) {
	t.Run("Safety: Update isolates Weekdays slice", func(t *testing.T) {
		habit, _ := domain.NewHabit("Defensive", "u1")

		inputWeekdays := []int{1, 2}

		_ = habit.Update("Defensive", "", "", "", domain.HabitTypeBoolean, "", "", 1, 1, inputWeekdays)

		inputWeekdays[0] = 6

		assert.Equal(t, 1, habit.Weekdays[0], "Habit internal state leaked!")
	})

	t.Run("Hygiene: Normalizes (sorts & dedups) Weekdays", func(t *testing.T) {
		habit, _ := domain.NewHabit("Sort", "u1")
		inputWeekdays := []int{5, 1, 1, 3}

		_ = habit.Update("Sort", "", "", "", domain.HabitTypeBoolean, "", "", 1, 1, inputWeekdays)

		assert.Equal(t, []int{1, 3, 5}, habit.Weekdays, "Days must be sorted and unique")
	})
}
