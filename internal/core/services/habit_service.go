package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/redis/go-redis/v9"
)

type HabitService struct {
	repo  domain.HabitRepository
	redis *redis.Client
}

func NewHabitService(repo domain.HabitRepository, rdb *redis.Client) *HabitService {
	return &HabitService{
		repo:  repo,
		redis: rdb,
	}
}

func (s *HabitService) cacheKey(userID string) string {
	return fmt.Sprintf("habits:%s", userID)
}

func (s *HabitService) invalidateCache(ctx context.Context, userID string) {
	if err := s.redis.Del(ctx, s.cacheKey(userID)).Err(); err != nil {
		log.Printf("Failed to invalidate cache for user %s: %v", userID, err)
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
	Version       int
}

func (s *HabitService) Create(ctx context.Context, input CreateHabitInput) (*domain.Habit, error) {
	habit, err := domain.NewHabit(input.Title, input.UserID)
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

	if err := s.repo.Create(ctx, habit); err != nil {
		return nil, err
	}

	s.invalidateCache(ctx, input.UserID)

	return habit, nil
}

func (s *HabitService) ListByUserID(ctx context.Context, userID string) ([]*domain.Habit, error) {
	key := s.cacheKey(userID)

	val, err := s.redis.Get(ctx, key).Result()
	if err == nil {
		var habits []*domain.Habit
		if err := json.Unmarshal([]byte(val), &habits); err == nil {
			return habits, nil
		}
		log.Printf("Cache corrupted for user %s", userID)
	}

	habits, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if data, err := json.Marshal(habits); err == nil {
		s.redis.Set(ctx, key, data, 30*time.Minute)
	}

	return habits, nil
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
		return err
	}

	if input.FrequencyType != nil {
		habit.FrequencyType = *input.FrequencyType
	}

	if err := s.repo.Update(ctx, habit); err != nil {
		return err
	}

	s.invalidateCache(ctx, input.UserID)

	return nil
}

func (s *HabitService) Delete(ctx context.Context, id string, userID string) error {
	habit, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if habit.UserID != userID {
		return domain.ErrHabitNotFound
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	s.invalidateCache(ctx, userID)

	return nil
}
