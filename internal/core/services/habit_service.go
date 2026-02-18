package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
)

type HabitService struct {
	repo domain.HabitRepository
}

func NewHabitService(repo domain.HabitRepository) *HabitService {
	return &HabitService{
		repo: repo,
	}
}

type CreateHabitInput struct {
	ID            string
	UserID        string
	Title         string
	Description   string
	Color         string
	Icon          string
	Type          string
	ReminderTime  string
	Unit          string
	TargetValue   int
	Interval      int
	Weekdays      []int
	FrequencyType string
}

type UpdateHabitInput struct {
	ID            string
	UserID        string
	Title         *string
	Description   *string
	Color         *string
	Icon          *string
	Type          *string
	ReminderTime  *string
	Unit          *string
	TargetValue   *int
	Interval      *int
	Weekdays      []int
	FrequencyType *string
	ArchivedAt    *string
	Version       int
}

func getStringOrDefault(ptr *string, def string) string {
	if ptr != nil {
		return *ptr
	}
	return def
}

func getIntOrDefault(ptr *int, def int) int {
	if ptr != nil {
		return *ptr
	}
	return def
}

func (s *HabitService) Create(ctx context.Context, input CreateHabitInput) (*domain.Habit, error) {
	habit, err := domain.NewHabit(input.ID, input.Title, input.UserID)
	if err != nil {
		return nil, err
	}

	if input.Interval < 1 {
		input.Interval = 1
	}
	if input.TargetValue < 1 {
		input.TargetValue = 1
	}

	habitType := input.Type
	if habitType == "" {
		habitType = domain.HabitTypeBoolean
	}

	err = habit.Update(
		input.Title,
		input.Description,
		input.Color,
		input.Icon,
		habitType,
		input.ReminderTime,
		input.Unit,
		input.TargetValue,
		input.Interval,
		input.Weekdays,
	)
	if err != nil {
		return nil, err
	}

	if input.FrequencyType != "" {
		habit.FrequencyType = input.FrequencyType
	} else {
		habit.FrequencyType = domain.HabitFreqDaily
	}

	habit.Version = 1

	existing, err := s.repo.GetByID(ctx, habit.ID)
	if err == nil && existing != nil {
		if existing.UserID == input.UserID {
			return existing, nil
		}
		return nil, domain.ErrHabitConflict
	}

	if err := s.repo.Create(ctx, habit); err != nil {
		fmt.Printf("Sync: Create failed for %s. Attempting hard resurrection via Update.\n", habit.ID)

		habit.DeletedAt = nil
		habit.Version++

		if updateErr := s.repo.Update(ctx, habit); updateErr != nil {
			fmt.Printf("Resurrection failed: %v\n", updateErr)
			return nil, err
		}

		fmt.Printf("Resurrection success for %s\n", habit.ID)
		return habit, nil
	}

	return habit, nil
}

func (s *HabitService) GetByID(ctx context.Context, id string, userID string) (*domain.Habit, error) {
	habit, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if habit.UserID != userID {
		return nil, domain.ErrHabitNotFound
	}
	return habit, nil
}

func (s *HabitService) ListByUserID(ctx context.Context, userID string) ([]*domain.Habit, error) {
	return s.repo.ListByUserID(ctx, userID)
}

func (s *HabitService) GetDelta(ctx context.Context, userID string, lastSync time.Time) ([]*domain.Habit, error) {
	return s.repo.GetChanges(ctx, userID, lastSync)
}

func (s *HabitService) Update(ctx context.Context, input UpdateHabitInput) (*domain.Habit, error) {
	habit, err := s.repo.GetByID(ctx, input.ID)

	if errors.Is(err, domain.ErrHabitNotFound) && input.Title != nil {
		fmt.Printf("Resurrecting Ghost Habit (Upsert): %s\n", input.ID)

		createInput := CreateHabitInput{
			ID:            input.ID,
			UserID:        input.UserID,
			Title:         *input.Title,
			Description:   getStringOrDefault(input.Description, ""),
			Color:         getStringOrDefault(input.Color, "#000000"),
			Icon:          getStringOrDefault(input.Icon, "default"),
			Type:          getStringOrDefault(input.Type, domain.HabitTypeBoolean),
			ReminderTime:  getStringOrDefault(input.ReminderTime, ""),
			Unit:          getStringOrDefault(input.Unit, ""),
			TargetValue:   getIntOrDefault(input.TargetValue, 1),
			Interval:      getIntOrDefault(input.Interval, 1),
			Weekdays:      input.Weekdays,
			FrequencyType: getStringOrDefault(input.FrequencyType, domain.HabitFreqDaily),
		}
		return s.Create(ctx, createInput)
	}

	if err != nil {
		return nil, err
	}

	if habit.UserID != input.UserID {
		return nil, domain.ErrHabitNotFound
	}

	if input.Version > 0 && habit.Version != input.Version {
		return nil, fmt.Errorf("%w: client v%d vs server v%d", domain.ErrHabitConflict, input.Version, habit.Version)
	}

	if input.Title != nil {
		habit.Title = *input.Title
	}
	if input.Description != nil {
		habit.Description = *input.Description
	}
	if input.Color != nil {
		habit.Color = *input.Color
	}
	if input.Icon != nil {
		habit.Icon = *input.Icon
	}
	if input.Type != nil {
		habit.Type = *input.Type
	}

	if input.TargetValue != nil {
		if *input.TargetValue > 0 {
			habit.TargetValue = *input.TargetValue
		}
	}

	if input.Interval != nil {
		if *input.Interval > 0 {
			habit.Interval = *input.Interval
		}
	}

	if input.Weekdays != nil {
		habit.Weekdays = input.Weekdays
	}

	var reminderStringToPass string
	if input.ReminderTime != nil {
		reminderStringToPass = *input.ReminderTime
	} else {
		if habit.ReminderTime != nil {
			reminderStringToPass = *habit.ReminderTime
		}
	}

	if input.Unit != nil {
		habit.Unit = *input.Unit
	}

	err = habit.Update(
		habit.Title,
		habit.Description,
		habit.Color,
		habit.Icon,
		habit.Type,
		reminderStringToPass,
		habit.Unit,
		habit.TargetValue,
		habit.Interval,
		habit.Weekdays,
	)
	if err != nil {
		return nil, err
	}

	if input.FrequencyType != nil {
		habit.FrequencyType = *input.FrequencyType
	}

	if input.ArchivedAt != nil {
		dateStr := *input.ArchivedAt
		if dateStr == "" {
			habit.ArchivedAt = nil
		} else {
			t, err := time.Parse(time.RFC3339, dateStr)
			if err == nil {
				habit.ArchivedAt = &t
			}
		}
	}

	habit.Version++
	habit.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, habit); err != nil {
		return nil, err
	}

	return habit, nil
}

func (s *HabitService) Delete(ctx context.Context, id string, userID string) error {
	habit, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if habit.UserID != userID {
		return domain.ErrHabitNotFound
	}

	now := time.Now().UTC()
	habit.DeletedAt = &now
	habit.Version++
	habit.UpdatedAt = now

	if err := s.repo.Update(ctx, habit); err != nil {
		return err
	}

	return nil
}
