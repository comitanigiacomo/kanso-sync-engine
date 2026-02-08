package services

import (
	"context"
	"time"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
)

type StatsService struct {
	habitRepo domain.HabitRepository
	entryRepo domain.HabitEntryRepository
}

func NewStatsService(habitRepo domain.HabitRepository, entryRepo domain.HabitEntryRepository) *StatsService {
	return &StatsService{
		habitRepo: habitRepo,
		entryRepo: entryRepo,
	}
}

func (s *StatsService) GetWeeklyStats(ctx context.Context, input domain.StatsInput) (*domain.WeeklyStats, error) {
	localStart := time.Date(input.StartDate.Year(), input.StartDate.Month(), input.StartDate.Day(), 0, 0, 0, 0, input.Location)
	localEnd := time.Date(input.EndDate.Year(), input.EndDate.Month(), input.EndDate.Day(), 23, 59, 59, 999999999, input.Location)

	dbStart := localStart.UTC()
	dbEnd := localEnd.UTC()

	habits, err := s.habitRepo.ListByUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	entries, err := s.entryRepo.ListByUserIDAndDateRange(ctx, input.UserID, dbStart, dbEnd)
	if err != nil {
		return nil, err
	}

	entriesMap := make(map[string]map[string]int)
	for _, e := range entries {
		if _, exists := entriesMap[e.HabitID]; !exists {
			entriesMap[e.HabitID] = make(map[string]int)
		}

		localTime := e.CompletionDate.In(input.Location)
		dateKey := localTime.Format("2006-01-02")

		entriesMap[e.HabitID][dateKey] += e.Value
	}

	stats := &domain.WeeklyStats{
		StartDate:   localStart.Format("2006-01-02"),
		EndDate:     localEnd.Format("2006-01-02"),
		TotalHabits: len(habits),
		HabitStats:  make([]domain.HabitStat, 0, len(habits)),
	}

	totalDaysPossible := 0
	totalDaysCompleted := 0

	for _, h := range habits {
		hStat := domain.HabitStat{
			HabitID:       h.ID,
			HabitTitle:    h.Title,
			Color:         h.Color,
			Icon:          h.Icon,
			TargetValue:   h.TargetValue,
			Unit:          h.Unit,
			DailyProgress: make([]int, 0),
		}

		daysInPeriod := 0
		daysAchieved := 0

		currentDate := localStart
		for !currentDate.After(localEnd) {
			dateKey := currentDate.Format("2006-01-02")

			val := entriesMap[h.ID][dateKey]

			hStat.TotalValue += val
			hStat.DailyProgress = append(hStat.DailyProgress, val)

			if val >= h.TargetValue {
				daysAchieved++
				totalDaysCompleted++
			}

			daysInPeriod++
			totalDaysPossible++

			currentDate = currentDate.AddDate(0, 0, 1)
		}

		hStat.DaysCompleted = daysAchieved
		if daysInPeriod > 0 {
			hStat.CompletionRate = float64(daysAchieved) / float64(daysInPeriod) * 100
		}

		stats.HabitStats = append(stats.HabitStats, hStat)
	}

	if totalDaysPossible > 0 {
		stats.OverallRate = float64(totalDaysCompleted) / float64(totalDaysPossible) * 100
	}

	return stats, nil
}
