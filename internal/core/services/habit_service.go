package services

import (
	"context"

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
	UserID       string
	Title        string
	Description  string
	Color        string
	Icon         string
	Type         string
	ReminderTime string
	Unit         string
	TargetValue  int
	Interval     int
	Weekdays     []int
}

type UpdateHabitInput struct {
	ID           string
	UserID       string
	Title        string
	Description  string
	Color        string
	Icon         string
	Type         string
	ReminderTime string
	Unit         string
	TargetValue  int
	Interval     int
	Weekdays     []int
}

func (s *HabitService) Create(ctx context.Context, input CreateHabitInput) (*domain.Habit, error) {
	habit, err := domain.NewHabit(input.Title, input.UserID)
	if err != nil {
		return nil, err
	}

	err = habit.Update(
		input.Title,
		input.Description,
		input.Color,
		input.Icon,
		input.Type,
		input.ReminderTime,
		input.Unit,
		input.TargetValue,
		input.Interval,
		input.Weekdays,
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, habit); err != nil {
		return nil, err
	}

	return habit, nil
}

func (s *HabitService) ListByUserID(ctx context.Context, userID string) ([]*domain.Habit, error) {
	return s.repo.ListByUserID(ctx, userID)
}

func (s *HabitService) Update(ctx context.Context, input UpdateHabitInput) error {
	habit, err := s.repo.GetByID(ctx, input.ID)
	if err != nil {
		return err
	}

	if habit.UserID != input.UserID {
		return domain.ErrHabitNotFound
	}

	err = habit.Update(
		input.Title,
		input.Description,
		input.Color,
		input.Icon,
		input.Type,
		input.ReminderTime,
		input.Unit,
		input.TargetValue,
		input.Interval,
		input.Weekdays,
	)
	if err != nil {
		return err
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
