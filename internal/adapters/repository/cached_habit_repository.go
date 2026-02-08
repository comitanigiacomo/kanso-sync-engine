package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/redis/go-redis/v9"
)

var _ domain.HabitRepository = (*CachedHabitRepository)(nil)

type CachedHabitRepository struct {
	next  domain.HabitRepository
	cache *redis.Client
}

func NewCachedHabitRepository(next domain.HabitRepository, cache *redis.Client) *CachedHabitRepository {
	return &CachedHabitRepository{
		next:  next,
		cache: cache,
	}
}

func (r *CachedHabitRepository) cacheKey(userID string) string {
	return fmt.Sprintf("habits:%s", userID)
}

func (r *CachedHabitRepository) invalidate(ctx context.Context, userID string) {
	if err := r.cache.Del(ctx, r.cacheKey(userID)).Err(); err != nil {
		log.Printf("[CACHE] Failed to invalidate for user %s: %v", userID, err)
	}
}

func (r *CachedHabitRepository) ListByUserID(ctx context.Context, userID string) ([]*domain.Habit, error) {
	key := r.cacheKey(userID)

	val, err := r.cache.Get(ctx, key).Result()
	if err == nil {
		var habits []*domain.Habit
		if err := json.Unmarshal([]byte(val), &habits); err == nil {
			return habits, nil
		}

		log.Printf("[CACHE] Corrupted data for user %s, cleaning up key", userID)
		r.cache.Del(ctx, key)
	} else if err != redis.Nil {
		log.Printf("[CACHE] Redis read error: %v", err)
	}

	habits, err := r.next.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if data, err := json.Marshal(habits); err == nil {
		if setErr := r.cache.Set(ctx, key, data, 30*time.Minute).Err(); setErr != nil {
			log.Printf("[CACHE] Redis set error: %v", setErr)
		}
	}

	return habits, nil
}

func (r *CachedHabitRepository) GetByID(ctx context.Context, id string) (*domain.Habit, error) {
	return r.next.GetByID(ctx, id)
}

func (r *CachedHabitRepository) GetChanges(ctx context.Context, userID string, since time.Time) ([]*domain.Habit, error) {
	return r.next.GetChanges(ctx, userID, since)
}

func (r *CachedHabitRepository) Create(ctx context.Context, habit *domain.Habit) error {
	if err := r.next.Create(ctx, habit); err != nil {
		return err
	}
	r.invalidate(ctx, habit.UserID)
	return nil
}

func (r *CachedHabitRepository) Update(ctx context.Context, habit *domain.Habit) error {
	if err := r.next.Update(ctx, habit); err != nil {
		return err
	}
	r.invalidate(ctx, habit.UserID)
	return nil
}

func (r *CachedHabitRepository) Delete(ctx context.Context, id string) error {
	habit, err := r.next.GetByID(ctx, id)
	if err == nil && habit != nil {
		defer r.invalidate(ctx, habit.UserID)
	}

	return r.next.Delete(ctx, id)
}

func (r *CachedHabitRepository) UpdateStreaks(ctx context.Context, id string, current, longest int) error {
	habit, err := r.next.GetByID(ctx, id)
	if err == nil && habit != nil {
		defer r.invalidate(ctx, habit.UserID)
	}

	return r.next.UpdateStreaks(ctx, id, current, longest)
}
