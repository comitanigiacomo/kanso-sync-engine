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
