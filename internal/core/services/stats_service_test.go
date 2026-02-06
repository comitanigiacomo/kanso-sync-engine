package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
)

func TestStatsService_GetWeeklyStats(t *testing.T) {
	ctx := context.Background()
	userID := "user-stats-1"

	startDate := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 12, 0, 0, 0, 0, time.UTC)

	t.Run("Success: Calculates rates and fills missing days correctly", func(t *testing.T) {
		habitRepo := new(MockHabitRepo)
		entryRepo := new(MockHabitEntryRepo)

		svc := services.NewStatsService(habitRepo, entryRepo)

		habits := []*domain.Habit{
			{ID: "h1", UserID: userID, Title: "Drink Water", TargetValue: 2000, Unit: "ml"},
			{ID: "h2", UserID: userID, Title: "Read", TargetValue: 1, Unit: "pages"},
		}
		habitRepo.On("ListByUserID", ctx, userID).Return(habits, nil)

		entries := []domain.HabitEntry{
			{ID: "e1", HabitID: "h1", UserID: userID, Value: 2500, CompletionDate: startDate},
			{ID: "e2", HabitID: "h1", UserID: userID, Value: 500, CompletionDate: endDate},

			{ID: "e3", HabitID: "h2", UserID: userID, Value: 5, CompletionDate: endDate},
		}

		entryRepo.On("ListByUserIDAndDateRange", ctx, userID, startDate, endDate).Return(entries, nil)

		input := domain.StatsInput{
			UserID:    userID,
			StartDate: startDate,
			EndDate:   endDate,
		}
		stats, err := svc.GetWeeklyStats(ctx, input)

		require.NoError(t, err)
		require.NotNil(t, stats)

		assert.Equal(t, 2, stats.TotalHabits)
		assert.Equal(t, "2024-01-10", stats.StartDate)
		assert.Equal(t, "2024-01-12", stats.EndDate)

		h1 := findHabitStat(stats.HabitStats, "h1")
		require.NotNil(t, h1)
		assert.Equal(t, 3000, h1.TotalValue)
		assert.Equal(t, 1, h1.DaysCompleted)
		assert.InDelta(t, 33.33, h1.CompletionRate, 0.1)

		assert.Len(t, h1.DailyProgress, 3)
		assert.Equal(t, []int{2500, 0, 500}, h1.DailyProgress)

		h2 := findHabitStat(stats.HabitStats, "h2")
		require.NotNil(t, h2)
		assert.Equal(t, 5, h2.TotalValue)
		assert.Equal(t, []int{0, 0, 5}, h2.DailyProgress)

		assert.InDelta(t, 33.33, stats.OverallRate, 0.1)
	})

	t.Run("Edge Case: No Habits returns zero stats", func(t *testing.T) {
		habitRepo := new(MockHabitRepo)
		entryRepo := new(MockHabitEntryRepo)
		svc := services.NewStatsService(habitRepo, entryRepo)

		habitRepo.On("ListByUserID", ctx, userID).Return([]*domain.Habit{}, nil)
		entryRepo.On("ListByUserIDAndDateRange", ctx, userID, mock.Anything, mock.Anything).Return([]domain.HabitEntry{}, nil)

		input := domain.StatsInput{UserID: userID, StartDate: startDate, EndDate: endDate}
		stats, err := svc.GetWeeklyStats(ctx, input)

		require.NoError(t, err)
		assert.Equal(t, 0, stats.TotalHabits)
		assert.Equal(t, 0.0, stats.OverallRate)
		assert.Empty(t, stats.HabitStats)
	})

	t.Run("Fail: Habit Repo Error propagates", func(t *testing.T) {
		habitRepo := new(MockHabitRepo)
		entryRepo := new(MockHabitEntryRepo)
		svc := services.NewStatsService(habitRepo, entryRepo)

		dbErr := errors.New("db connection lost")
		habitRepo.On("ListByUserID", ctx, userID).Return(nil, dbErr)

		input := domain.StatsInput{UserID: userID, StartDate: startDate, EndDate: endDate}
		stats, err := svc.GetWeeklyStats(ctx, input)

		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, stats)
	})

	t.Run("Fail: Entry Repo Error propagates", func(t *testing.T) {
		habitRepo := new(MockHabitRepo)
		entryRepo := new(MockHabitEntryRepo)
		svc := services.NewStatsService(habitRepo, entryRepo)

		habits := []*domain.Habit{{ID: "h1"}}
		habitRepo.On("ListByUserID", ctx, userID).Return(habits, nil)

		dbErr := errors.New("query timeout")
		entryRepo.On("ListByUserIDAndDateRange", ctx, userID, mock.Anything, mock.Anything).Return(nil, dbErr)

		input := domain.StatsInput{UserID: userID, StartDate: startDate, EndDate: endDate}
		stats, err := svc.GetWeeklyStats(ctx, input)

		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, stats)
	})
}

func findHabitStat(stats []domain.HabitStat, habitID string) *domain.HabitStat {
	for _, s := range stats {
		if s.HabitID == habitID {
			return &s
		}
	}
	return nil
}
