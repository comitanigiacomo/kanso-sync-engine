package services

import (
	"context"
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
	Version       int
}

func mergeString(newVal, oldVal string) string {
	if newVal == "" {
		return oldVal
	}
	return newVal
}

func (s *HabitService) Create(ctx context.Context, input CreateHabitInput) (*domain.Habit, error) {
	habit, err := domain.NewHabit(input.Title, input.UserID)
	if err != nil {
		return nil, err
	}

	finalType := mergeString(input.Type, habit.Type)

	if input.Interval < 1 {
		input.Interval = 1
	}
	if input.TargetValue < 1 {
		input.TargetValue = 1
	}

	err = habit.Update(
		input.Title,
		input.Description,
		input.Color,
		input.Icon,
		finalType,
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
	} else if habit.FrequencyType == "" {
		habit.FrequencyType = "daily"
	}

	if err := s.repo.Create(ctx, habit); err != nil {
		return nil, err
	}

	return habit, nil
}

func (s *HabitService) ListByUserID(ctx context.Context, userID string) ([]*domain.Habit, error) {
	return s.repo.ListByUserID(ctx, userID)
}

func (s *HabitService) GetDelta(ctx context.Context, userID string, lastSync time.Time) ([]*domain.Habit, error) {
	return s.repo.GetChanges(ctx, userID, lastSync)
}

func (s *HabitService) Update(ctx context.Context, input UpdateHabitInput) error {
	habit, err := s.repo.GetByID(ctx, input.ID)
	if err != nil {
		return err
	}

	if habit.UserID != input.UserID {
		return domain.ErrHabitNotFound
	}

	if input.Version > 0 && habit.Version != input.Version {
		return fmt.Errorf("%w: client v%d vs server v%d", domain.ErrHabitConflict, input.Version, habit.Version)
	}

	title := mergeString(input.Title, habit.Title)
	desc := mergeString(input.Description, habit.Description)
	color := mergeString(input.Color, habit.Color)
	icon := mergeString(input.Icon, habit.Icon)
	hType := mergeString(input.Type, habit.Type)

	target := habit.TargetValue
	if input.TargetValue > 0 {
		target = input.TargetValue
	}

	interval := habit.Interval
	if input.Interval > 0 {
		interval = input.Interval
	}

	weekdays := habit.Weekdays
	if input.Weekdays != nil {
		weekdays = input.Weekdays
	}

	err = habit.Update(
		title,
		desc,
		color,
		icon,
		hType,
		input.ReminderTime,
		input.Unit,
		target,
		interval,
		weekdays,
	)
	if err != nil {
		return err
	}

	if input.FrequencyType != "" {
		habit.FrequencyType = input.FrequencyType
	}

	return s.repo.Update(ctx, habit)
}

func (s *HabitService) Delete(ctx context.Context, id string, userID string) error {
	habit, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if habit.UserID != userID {
		return domain.ErrHabitNotFound
	}

	return s.repo.Delete(ctx, id)
}
