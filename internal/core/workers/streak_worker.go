package workers

import (
	"context"
	"log"
	"sort"
	"time"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
)

type HabitRepository interface {
	GetByID(ctx context.Context, id string) (*domain.Habit, error)
	Update(ctx context.Context, habit *domain.Habit) error
}

type EntryRepository interface {
	ListByHabitID(ctx context.Context, habitID string) ([]*domain.HabitEntry, error)
}

type StreakJob struct {
	HabitID string
}

type StreakWorker struct {
	habitRepo HabitRepository
	entryRepo EntryRepository
	jobs      chan StreakJob
}

func NewStreakWorker(hRepo HabitRepository, eRepo EntryRepository) *StreakWorker {
	return &StreakWorker{
		habitRepo: hRepo,
		entryRepo: eRepo,
		jobs:      make(chan StreakJob, 100),
	}
}

func (w *StreakWorker) Start(ctx context.Context) {
	go func() {
		log.Println("Streak Worker started in background...")
		for {
			select {
			case job := <-w.jobs:
				w.processJob(ctx, job)
			case <-ctx.Done():
				log.Println("Streak Worker shutting down...")
				return
			}
		}
	}()
}

func (w *StreakWorker) Enqueue(habitID string) {
	select {
	case w.jobs <- StreakJob{HabitID: habitID}:
	default:
		log.Printf("Streak Worker queue full! Dropping job for habit %s", habitID)
	}
}

func (w *StreakWorker) processJob(ctx context.Context, job StreakJob) {
	habit, err := w.habitRepo.GetByID(ctx, job.HabitID)
	if err != nil {
		log.Printf("Worker Error fetching habit %s: %v", job.HabitID, err)
		return
	}

	entries, err := w.entryRepo.ListByHabitID(ctx, job.HabitID)
	if err != nil {
		log.Printf("Worker Error fetching entries for %s: %v", job.HabitID, err)
		return
	}

	current, longest := calculateStreaks(entries)

	if habit.CurrentStreak != current || habit.LongestStreak != longest {
		habit.UpdateStreak(current, longest)
		if err := w.habitRepo.Update(ctx, habit); err != nil {
			log.Printf("Worker Failed to update streak for %s: %v", job.HabitID, err)
		} else {
			log.Printf("Streak updated for %s: Current=%d, Longest=%d", habit.Title, current, longest)
		}
	}
}

func calculateStreaks(entries []*domain.HabitEntry) (int, int) {
	if len(entries) == 0 {
		return 0, 0
	}

	uniqueDays := make(map[string]bool)
	var sortedDates []time.Time

	for _, e := range entries {
		dateKey := e.CompletionDate.UTC().Format("2006-01-02")
		if !uniqueDays[dateKey] {
			uniqueDays[dateKey] = true
			t, _ := time.Parse("2006-01-02", dateKey)
			sortedDates = append(sortedDates, t)
		}
	}

	sort.Slice(sortedDates, func(i, j int) bool {
		return sortedDates[i].After(sortedDates[j])
	})

	if len(sortedDates) == 0 {
		return 0, 0
	}

	currentStreak := 0
	now := time.Now().UTC().Truncate(24 * time.Hour)
	lastEntryDate := sortedDates[0]

	diff := now.Sub(lastEntryDate).Hours() / 24

	if diff <= 1 {
		currentStreak = 1
		for i := 0; i < len(sortedDates)-1; i++ {
			thisDate := sortedDates[i]
			prevDate := sortedDates[i+1]

			if thisDate.Sub(prevDate).Hours() == 24 {
				currentStreak++
			} else {
				break
			}
		}
	}

	longestStreak := 0
	tempStreak := 1

	for i := 0; i < len(sortedDates)-1; i++ {
		thisDate := sortedDates[i]
		prevDate := sortedDates[i+1]

		if thisDate.Sub(prevDate).Hours() == 24 {
			tempStreak++
		} else {
			if tempStreak > longestStreak {
				longestStreak = tempStreak
			}
			tempStreak = 1
		}
	}
	if tempStreak > longestStreak {
		longestStreak = tempStreak
	}

	return currentStreak, longestStreak
}
