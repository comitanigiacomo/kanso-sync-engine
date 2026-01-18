package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewHabit(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		title        string
		description  string
		color        string
		icon         string
		hType        string
		reminder     string
		target       int
		wantTarget   int
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
			description:  "desc",
			hType:        HabitTypeNumeric,
			reminder:     "",
			interval:     0,
			weekdays:     nil,
			target:       10,
			wantTarget:   10,
			wantErr:      nil,
			wantFreq:     HabitFreqDaily,
			wantInterval: 1,
		},
		{
			name:         "Success: Interval (Every 3 Days)",
			userID:       "u1",
			title:        "Leggere",
			description:  "desc",
			hType:        HabitTypeTimer,
			reminder:     "",
			interval:     3,
			weekdays:     nil,
			target:       30,
			wantTarget:   30,
			wantErr:      nil,
			wantFreq:     HabitFreqInterval,
			wantInterval: 3,
		},
		{
			name:         "Success: Boolean Forces Target to 1",
			userID:       "u1",
			title:        "Non fumare",
			description:  "desc",
			hType:        HabitTypeBoolean,
			reminder:     "",
			target:       100,
			wantTarget:   1,
			wantErr:      nil,
			wantFreq:     HabitFreqDaily,
			wantInterval: 1,
		},
		{
			name:         "Success: Boolean Ignores Negative Target",
			userID:       "u1",
			title:        "Gym",
			description:  "desc",
			hType:        HabitTypeBoolean,
			reminder:     "",
			target:       -5,
			wantTarget:   1,
			wantErr:      nil,
			wantFreq:     HabitFreqDaily,
			wantInterval: 1,
		},
		{
			name:         "Success: Valid Reminder and Type",
			userID:       "u1",
			title:        "Sveglia",
			description:  "desc",
			hType:        HabitTypeBoolean,
			reminder:     "07:30",
			target:       1,
			wantTarget:   1,
			wantErr:      nil,
			wantFreq:     HabitFreqDaily,
			wantInterval: 1,
		},
		{
			name:         "Priority: Weekdays win over Interval",
			userID:       "u1",
			title:        "Gym",
			description:  "desc",
			hType:        HabitTypeBoolean,
			reminder:     "",
			interval:     5,
			weekdays:     []int{1, 3},
			target:       1,
			wantTarget:   1,
			wantErr:      nil,
			wantFreq:     HabitFreqSpecificDays,
			wantInterval: 5,
		},
		{
			name:         "Logic: Interval=1 becomes Daily",
			userID:       "u1",
			title:        "Walk",
			description:  "desc",
			hType:        HabitTypeNumeric,
			reminder:     "",
			interval:     1,
			weekdays:     nil,
			target:       10,
			wantTarget:   10,
			wantErr:      nil,
			wantFreq:     HabitFreqDaily,
			wantInterval: 1,
		},
		{
			name:         "Success: Short Hex Color",
			userID:       "u1",
			title:        "Short Color",
			description:  "desc",
			color:        "#FFF",
			hType:        HabitTypeNumeric,
			reminder:     "",
			target:       1,
			wantTarget:   1,
			wantErr:      nil,
			wantFreq:     HabitFreqDaily,
			wantInterval: 1,
		},
		{
			name:        "Error: Title Too Long",
			userID:      "u1",
			title:       strings.Repeat("a", 101),
			description: "desc",
			hType:       HabitTypeNumeric,
			wantErr:     ErrHabitTitleTooLong,
		},
		{
			name:        "Error: Description Too Long",
			userID:      "u1",
			title:       "Valid Title",
			description: strings.Repeat("a", 501),
			hType:       HabitTypeNumeric,
			wantErr:     ErrHabitDescTooLong,
		},
		{
			name:        "Error: Invalid Habit Type",
			userID:      "u1",
			title:       "Bad Type",
			description: "desc",
			hType:       "magic_spell",
			reminder:    "",
			wantErr:     ErrInvalidHabitType,
		},
		{
			name:        "Error: Invalid Reminder Format (Letters)",
			userID:      "u1",
			title:       "Bad Time",
			description: "desc",
			hType:       HabitTypeBoolean,
			reminder:    "hello",
			wantErr:     ErrInvalidReminder,
		},
		{
			name:        "Error: Invalid Reminder Format (Out of range)",
			userID:      "u1",
			title:       "Bad Time",
			description: "desc",
			hType:       HabitTypeBoolean,
			reminder:    "25:00",
			wantErr:     ErrInvalidReminder,
		},
		{
			name:        "Error: Color without Hash",
			userID:      "u1",
			title:       "Bad Color",
			description: "desc",
			color:       "FFFFFF",
			hType:       HabitTypeNumeric,
			wantErr:     ErrInvalidColor,
		},
		{
			name:        "Error: Color Invalid Chars",
			userID:      "u1",
			title:       "Bad Color",
			description: "desc",
			color:       "#ZZZZZZ",
			hType:       HabitTypeNumeric,
			wantErr:     ErrInvalidColor,
		},
		{
			name:        "Error: Color Wrong Length",
			userID:      "u1",
			title:       "Bad Color",
			description: "desc",
			color:       "#1234",
			hType:       HabitTypeNumeric,
			wantErr:     ErrInvalidColor,
		},
		{
			name:         "Success: Boundary Days (Sunday 0 & Saturday 6)",
			userID:       "u1",
			title:        "Weekend",
			description:  "desc",
			hType:        HabitTypeBoolean,
			weekdays:     []int{0, 6},
			target:       1,
			wantTarget:   1,
			wantErr:      nil,
			wantFreq:     HabitFreqSpecificDays,
			wantInterval: 1,
		},
		{
			name:        "Error: Weekday 7 (Out of range)",
			userID:      "u1",
			title:       "Bad Day",
			description: "desc",
			hType:       HabitTypeNumeric,
			weekdays:    []int{7},
			wantErr:     ErrInvalidWeekdays,
		},
		{
			name:        "Error: Negative Weekday",
			userID:      "u1",
			title:       "Bad Day",
			description: "desc",
			hType:       HabitTypeNumeric,
			weekdays:    []int{-1},
			wantErr:     ErrInvalidWeekdays,
		},
		{
			name:        "Error: Empty Title",
			userID:      "u1",
			title:       "",
			description: "desc",
			hType:       HabitTypeNumeric,
			wantErr:     ErrHabitTitleEmpty,
		},
		{
			name:        "Error: Negative Target (Numeric)",
			userID:      "u1",
			title:       "Bad Target",
			description: "desc",
			hType:       HabitTypeNumeric,
			target:      -1,
			wantErr:     ErrInvalidTarget,
		},
		{
			name:        "Error: Negative Interval",
			userID:      "u1",
			title:       "Bad Interval",
			description: "desc",
			hType:       HabitTypeNumeric,
			interval:    -5,
			wantErr:     ErrInvalidInterval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			habit, err := NewHabit(
				tt.userID, tt.title, tt.description, tt.color, tt.icon,
				tt.hType, tt.reminder, "unit",
				tt.target, tt.interval, tt.weekdays,
			)

			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
				assert.Nil(t, habit)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, habit)

				assert.Equal(t, tt.hType, habit.Type)
				assert.Equal(t, tt.wantTarget, habit.TargetValue)

				if tt.reminder != "" {
					assert.NotNil(t, habit.ReminderTime)
					assert.Equal(t, tt.reminder, *habit.ReminderTime)
				} else {
					assert.Nil(t, habit.ReminderTime)
				}

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
		h, _ := NewHabit("u1", "Original Title", "Original Desc", "#000000", "icon", HabitTypeNumeric, "", "ml", 10, 0, nil)
		time.Sleep(1 * time.Millisecond)
		return h
	}

	t.Run("Success: Full Update including Type and Reminder", func(t *testing.T) {
		habit := createStandardHabit()
		originalTime := habit.UpdatedAt

		err := habit.Update("New Title", "New Desc", "#FFFFFF", "new_icon",
			HabitTypeTimer, "20:00", "kg", 20, 3, nil)

		assert.Nil(t, err)
		assert.Equal(t, "New Title", habit.Title)
		assert.Equal(t, "New Desc", habit.Description)
		assert.Equal(t, "#FFFFFF", habit.Color)
		assert.Equal(t, "new_icon", habit.Icon)
		assert.Equal(t, HabitTypeTimer, habit.Type)
		assert.NotNil(t, habit.ReminderTime)
		assert.Equal(t, "20:00", *habit.ReminderTime)
		assert.Equal(t, "kg", habit.Unit)
		assert.Equal(t, 20, habit.TargetValue)
		assert.Equal(t, 3, habit.Interval)
		assert.Equal(t, HabitFreqInterval, habit.FrequencyType)
		assert.Nil(t, habit.Weekdays)
		assert.True(t, habit.UpdatedAt.After(originalTime))
	})

	t.Run("Success: Boolean Type Forces Target to 1", func(t *testing.T) {
		habit := createStandardHabit()

		err := habit.Update("Title", "Desc", "#FFF", "icon", HabitTypeBoolean, "", "unit", 500, 1, nil)

		assert.Nil(t, err)
		assert.Equal(t, HabitTypeBoolean, habit.Type)
		assert.Equal(t, 1, habit.TargetValue)
	})

	t.Run("Success: Clear Reminder", func(t *testing.T) {
		habit, _ := NewHabit("u1", "With Rem", "", "#000", "", HabitTypeBoolean, "09:00", "", 1, 0, nil)

		err := habit.Update("Title", "Desc", "#FFF", "icon", HabitTypeBoolean, "", "unit", 1, 0, nil)

		assert.Nil(t, err)
		assert.Nil(t, habit.ReminderTime)
	})

	t.Run("Success: Default Icon Logic", func(t *testing.T) {
		habit := createStandardHabit()
		err := habit.Update("Title", "Desc", "#FFF", "", HabitTypeNumeric, "", "kg", 10, 1, nil)

		assert.Nil(t, err)
		assert.Equal(t, DefaultIcon, habit.Icon)
	})

	t.Run("Error: Validation Failure (Empty Title)", func(t *testing.T) {
		habit := createStandardHabit()
		previousTitle := habit.Title

		err := habit.Update("", "Desc", "#FFF", "icon", HabitTypeNumeric, "", "kg", 10, 1, nil)

		assert.Equal(t, ErrHabitTitleEmpty, err)
		assert.Equal(t, previousTitle, habit.Title)
	})

	t.Run("Error: Validation Failure (Invalid Color)", func(t *testing.T) {
		habit := createStandardHabit()
		err := habit.Update("Title", "Desc", "INVALID", "icon", HabitTypeNumeric, "", "kg", 10, 1, nil)
		assert.Equal(t, ErrInvalidColor, err)
	})

	t.Run("Error: Validation Failure (Invalid Type)", func(t *testing.T) {
		habit := createStandardHabit()
		err := habit.Update("Title", "Desc", "#FFF", "icon", "fake_type", "", "kg", 10, 1, nil)
		assert.Equal(t, ErrInvalidHabitType, err)
	})

	t.Run("Error: Validation Failure (Invalid Reminder)", func(t *testing.T) {
		habit := createStandardHabit()
		err := habit.Update("Title", "Desc", "#FFF", "icon", HabitTypeBoolean, "99:99", "kg", 10, 1, nil)
		assert.Equal(t, ErrInvalidReminder, err)
	})

	t.Run("Error: Archived Habit Cannot Be Updated", func(t *testing.T) {
		habit := createStandardHabit()

		now := time.Now()
		habit.ArchivedAt = &now

		err := habit.Update("Try Update", "Desc", "#FFF", "icon", HabitTypeNumeric, "", "kg", 10, 1, nil)

		assert.Equal(t, ErrHabitArchived, err)
	})
}

func TestHabit_ArchiveAndRestore(t *testing.T) {
	createActiveHabit := func() *Habit {
		h, _ := NewHabit("u1", "Title", "", "#000", "", HabitTypeNumeric, "", "unit", 1, 0, nil)
		time.Sleep(1 * time.Millisecond)
		return h
	}

	createArchivedHabit := func() *Habit {
		h := createActiveHabit()
		h.Archive()
		time.Sleep(1 * time.Millisecond)
		return h
	}

	t.Run("Archive: Soft Delete", func(t *testing.T) {
		habit := createActiveHabit()
		originalTime := habit.UpdatedAt
		time.Sleep(1 * time.Millisecond)

		habit.Archive()

		assert.NotNil(t, habit.ArchivedAt)
		assert.True(t, habit.UpdatedAt.After(originalTime))
	})

	t.Run("Archive: Idempotency", func(t *testing.T) {
		habit := createArchivedHabit()
		archivedAt := habit.ArchivedAt
		updatedAt := habit.UpdatedAt
		time.Sleep(1 * time.Millisecond)

		habit.Archive()

		assert.Equal(t, archivedAt, habit.ArchivedAt)
		assert.Equal(t, updatedAt, habit.UpdatedAt)
	})

	t.Run("Security: Update Blocked when Archived", func(t *testing.T) {
		habit := createArchivedHabit()

		err := habit.Update("Change", "", "#000", "", HabitTypeNumeric, "", "kg", 1, 0, nil)

		assert.Equal(t, ErrHabitArchived, err)
	})

	t.Run("Restore: Bring back to life", func(t *testing.T) {
		habit := createArchivedHabit()
		beforeRestore := habit.UpdatedAt
		time.Sleep(1 * time.Millisecond)

		habit.Restore()

		assert.Nil(t, habit.ArchivedAt)
		assert.True(t, habit.UpdatedAt.After(beforeRestore))
	})

	t.Run("Restore: Idempotency", func(t *testing.T) {
		habit := createActiveHabit()
		originalTime := habit.UpdatedAt
		time.Sleep(1 * time.Millisecond)

		habit.Restore()

		assert.Nil(t, habit.ArchivedAt)
		assert.Equal(t, originalTime, habit.UpdatedAt)
	})

	t.Run("Security: Update Allowed after Restore", func(t *testing.T) {
		habit := createArchivedHabit()
		habit.Restore()

		err := habit.Update("Change", "", "#000", "", HabitTypeNumeric, "", "kg", 1, 0, nil)

		assert.Nil(t, err)
	})
}

func TestHabit_ChangePosition(t *testing.T) {
	createStandardHabit := func() *Habit {
		h, _ := NewHabit("u1", "Original Title", "Original Desc", "#000000", "icon", HabitTypeNumeric, "", "ml", 10, 0, nil)
		time.Sleep(1 * time.Millisecond)
		return h
	}

	t.Run("Success: Change Sort Order", func(t *testing.T) {
		habit := createStandardHabit()
		originalTime := habit.UpdatedAt
		time.Sleep(1 * time.Millisecond)

		err := habit.ChangePosition(5)

		assert.Nil(t, err)
		assert.Equal(t, 5, habit.SortOrder)
		assert.True(t, habit.UpdatedAt.After(originalTime))
	})

	t.Run("Error: Cannot Change Position of Archived Habit", func(t *testing.T) {
		habit := createStandardHabit()
		habit.Archive()

		err := habit.ChangePosition(10)

		assert.Equal(t, ErrHabitArchived, err)
	})
}

func TestHabit_DefensiveCopyAndHygiene(t *testing.T) {
	t.Run("Safety: NewHabit isolates Weekdays slice", func(t *testing.T) {
		inputWeekdays := []int{1, 2}

		habit, _ := NewHabit("u1", "Defensive", "", "#000", "", HabitTypeBoolean, "", "unit", 1, 0, inputWeekdays)

		inputWeekdays[0] = 6

		assert.Equal(t, 1, habit.Weekdays[0], "The habit must not be affected by external modifications")
		assert.Equal(t, 2, habit.Weekdays[1])
	})

	t.Run("Safety: Update isolates Weekdays slice", func(t *testing.T) {
		habit, _ := NewHabit("u1", "Defensive", "", "#000", "", HabitTypeBoolean, "", "unit", 1, 0, []int{1})

		inputWeekdays := []int{3, 4}

		_ = habit.Update("Defensive", "", "#000", "", HabitTypeBoolean, "", "unit", 1, 0, inputWeekdays)

		inputWeekdays[0] = 5

		assert.Equal(t, 3, habit.Weekdays[0])
	})

	t.Run("Hygiene: NewHabit normalizes (sorts & dedups) Weekdays", func(t *testing.T) {
		inputWeekdays := []int{5, 1, 1, 3}

		habit, _ := NewHabit("u1", "Sort", "", "#000", "", HabitTypeBoolean, "", "unit", 1, 0, inputWeekdays)

		assert.Equal(t, []int{1, 3, 5}, habit.Weekdays, "Days must be sorted and in order")
	})
}
