package workers

import (
	"testing"
	"time"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestCalculateStreaks(t *testing.T) {
	today := time.Now().UTC()
	daysAgo := func(n int) time.Time {
		return today.AddDate(0, 0, -n)
	}

	tests := []struct {
		name        string
		entries     []*domain.HabitEntry
		wantCurrent int
		wantLongest int
	}{
		{
			name:        "Empty entries",
			entries:     []*domain.HabitEntry{},
			wantCurrent: 0,
			wantLongest: 0,
		},
		{
			name: "Single entry today",
			entries: []*domain.HabitEntry{
				{CompletionDate: today},
			},
			wantCurrent: 1,
			wantLongest: 1,
		},
		{
			name: "Single entry yesterday (Streak still alive)",
			entries: []*domain.HabitEntry{
				{CompletionDate: daysAgo(1)},
			},
			wantCurrent: 1,
			wantLongest: 1,
		},
		{
			name: "Single entry 2 days ago (Streak broken)",
			entries: []*domain.HabitEntry{
				{CompletionDate: daysAgo(2)},
			},
			wantCurrent: 0,
			wantLongest: 1,
		},
		{
			name: "Perfect streak (Today, Yesterday, 2 days ago)",
			entries: []*domain.HabitEntry{
				{CompletionDate: today},
				{CompletionDate: daysAgo(1)},
				{CompletionDate: daysAgo(2)},
			},
			wantCurrent: 3,
			wantLongest: 3,
		},
		{
			name: "Broken streak with gap (Today, Yesterday, [GAP], 4 days ago)",
			entries: []*domain.HabitEntry{
				{CompletionDate: today},
				{CompletionDate: daysAgo(1)},
				{CompletionDate: daysAgo(4)},
			},
			wantCurrent: 2,
			wantLongest: 2,
		},
		{
			name: "Longest streak in the past",
			entries: []*domain.HabitEntry{
				{CompletionDate: today},
				{CompletionDate: daysAgo(10)},
				{CompletionDate: daysAgo(11)},
				{CompletionDate: daysAgo(12)},
			},
			wantCurrent: 1,
			wantLongest: 3,
		},
		{
			name: "Unsorted entries (should be sorted internally)",
			entries: []*domain.HabitEntry{
				{CompletionDate: daysAgo(2)},
				{CompletionDate: today},
				{CompletionDate: daysAgo(1)},
			},
			wantCurrent: 3,
			wantLongest: 3,
		},
		{
			name: "Duplicate entries same day (should count as 1)",
			entries: []*domain.HabitEntry{
				{CompletionDate: today},
				{CompletionDate: today.Add(1 * time.Hour)},
				{CompletionDate: daysAgo(1)},
			},
			wantCurrent: 2,
			wantLongest: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCurrent, gotLongest := calculateStreaks(tt.entries)
			assert.Equal(t, tt.wantCurrent, gotCurrent, "Current Streak mismatch")
			assert.Equal(t, tt.wantLongest, gotLongest, "Longest Streak mismatch")
		})
	}
}
