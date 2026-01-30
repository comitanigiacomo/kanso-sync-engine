package services

import (
	"context"
	"time"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
)

type EntryService struct {
	repo      domain.HabitEntryRepository
	habitRepo domain.HabitRepository
}

func NewEntryService(repo domain.HabitEntryRepository, habitRepo domain.HabitRepository) *EntryService {
	return &EntryService{
		repo:      repo,
		habitRepo: habitRepo,
	}
}

type CreateEntryInput struct {
	HabitID        string
	UserID         string
	CompletionDate time.Time
	Value          int
	Notes          string
}

type UpdateEntryInput struct {
	ID      string
	UserID  string
	Value   int
	Notes   string
	Version int
}

func (s *EntryService) Create(ctx context.Context, input CreateEntryInput) (*domain.HabitEntry, error) {
	entry := domain.NewHabitEntry(input.HabitID, input.UserID, input.CompletionDate, input.Value)
	entry.Notes = input.Notes

	if err := entry.Validate(); err != nil {
		return nil, err
	}

	habit, err := s.habitRepo.GetByID(ctx, entry.HabitID)
	if err != nil {
		return nil, err
	}
	if habit.UserID != entry.UserID {
		return nil, domain.ErrUnauthorized
	}

	if err := s.repo.Create(ctx, entry); err != nil {
		return nil, err
	}

	return entry, nil
}

func (s *EntryService) Update(ctx context.Context, input UpdateEntryInput) (*domain.HabitEntry, error) {
	existing, err := s.GetByID(ctx, input.ID, input.UserID)
	if err != nil {
		return nil, err
	}

	if input.Version > 0 && existing.Version != input.Version {
		return nil, domain.ErrEntryConflict
	}

	existing.Value = input.Value
	existing.Notes = input.Notes

	existing.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

func (s *EntryService) GetByID(ctx context.Context, id string, userID string) (*domain.HabitEntry, error) {
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if entry.UserID != userID {
		return nil, domain.ErrUnauthorized
	}
	return entry, nil
}

func (s *EntryService) ListByHabitID(ctx context.Context, habitID string, userID string, from, to time.Time) ([]*domain.HabitEntry, error) {
	habit, err := s.habitRepo.GetByID(ctx, habitID)
	if err != nil {
		return nil, err
	}
	if habit.UserID != userID {
		return nil, domain.ErrUnauthorized
	}

	return s.repo.ListByHabitID(ctx, habitID, from, to)
}

func (s *EntryService) Delete(ctx context.Context, id string, userID string) error {
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if entry.UserID != userID {
		return domain.ErrUnauthorized
	}

	return s.repo.Delete(ctx, id, userID)
}

func (s *EntryService) GetDelta(ctx context.Context, userID string, since time.Time) ([]*domain.HabitEntry, error) {
	return s.repo.GetChanges(ctx, userID, since)
}
