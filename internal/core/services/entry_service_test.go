package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/workers"
)

type MockHabitEntryRepo struct {
	mock.Mock
}

func (m *MockHabitEntryRepo) Create(ctx context.Context, entry *domain.HabitEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockHabitEntryRepo) Update(ctx context.Context, entry *domain.HabitEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockHabitEntryRepo) Delete(ctx context.Context, id string, userID string) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

func (m *MockHabitEntryRepo) GetByID(ctx context.Context, id string) (*domain.HabitEntry, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.HabitEntry), args.Error(1)
}

func (m *MockHabitEntryRepo) ListByHabitID(ctx context.Context, habitID string) ([]*domain.HabitEntry, error) {
	args := m.Called(ctx, habitID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.HabitEntry), args.Error(1)
}

func (m *MockHabitEntryRepo) ListByHabitIDWithRange(ctx context.Context, habitID string, from, to time.Time) ([]*domain.HabitEntry, error) {
	args := m.Called(ctx, habitID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.HabitEntry), args.Error(1)
}

func (m *MockHabitEntryRepo) GetChanges(ctx context.Context, userID string, since time.Time) ([]*domain.HabitEntry, error) {
	args := m.Called(ctx, userID, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.HabitEntry), args.Error(1)
}

func (m *MockHabitEntryRepo) ListByUserIDAndDateRange(ctx context.Context, userID string, startDate, endDate time.Time) ([]domain.HabitEntry, error) {
	args := m.Called(ctx, userID, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.HabitEntry), args.Error(1)
}

type MockHabitRepo struct {
	mock.Mock
}

func (m *MockHabitRepo) GetByID(ctx context.Context, id string) (*domain.Habit, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Habit), args.Error(1)
}

func (m *MockHabitRepo) Create(ctx context.Context, h *domain.Habit) error {
	return nil
}

func (m *MockHabitRepo) ListByUserID(ctx context.Context, u string) ([]*domain.Habit, error) {
	args := m.Called(ctx, u)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Habit), args.Error(1)
}

func (m *MockHabitRepo) Update(ctx context.Context, h *domain.Habit) error { return nil }
func (m *MockHabitRepo) Delete(ctx context.Context, id string) error       { return nil }
func (m *MockHabitRepo) GetChanges(ctx context.Context, u string, t time.Time) ([]*domain.Habit, error) {
	return nil, nil
}

func (m *MockHabitRepo) UpdateStreaks(ctx context.Context, id string, current, longest int) error {
	return nil
}

func getTestWorker() *workers.StreakWorker {
	return workers.NewStreakWorker(nil, nil)
}

func TestEntryService_Create(t *testing.T) {
	ctx := context.Background()
	uid := "user-123"
	hid := "habit-abc"
	now := time.Now().UTC()

	t.Run("Success: Should validate ownership, create entry AND enqueue worker", func(t *testing.T) {
		entryRepo := new(MockHabitEntryRepo)
		habitRepo := new(MockHabitRepo)
		worker := getTestWorker()

		svc := services.NewEntryService(entryRepo, habitRepo, worker)

		habitRepo.On("GetByID", ctx, hid).Return(&domain.Habit{ID: hid, UserID: uid}, nil)

		entryRepo.On("Create", ctx, mock.MatchedBy(func(e *domain.HabitEntry) bool {
			return e.HabitID == hid && e.Value == 10
		})).Return(nil)

		input := services.CreateEntryInput{
			HabitID:        hid,
			UserID:         uid,
			CompletionDate: now,
			Value:          10,
		}

		created, err := svc.Create(ctx, input)
		require.NoError(t, err)
		assert.NotNil(t, created)
		assert.Equal(t, 10, created.Value)

		entryRepo.AssertExpectations(t)
	})

	t.Run("Security: Should fail if Habit belongs to another user (IDOR)", func(t *testing.T) {
		entryRepo := new(MockHabitEntryRepo)
		habitRepo := new(MockHabitRepo)
		worker := getTestWorker()
		svc := services.NewEntryService(entryRepo, habitRepo, worker)

		habitRepo.On("GetByID", ctx, hid).Return(&domain.Habit{ID: hid, UserID: "hacker-target"}, nil)

		input := services.CreateEntryInput{HabitID: hid, UserID: "attacker", CompletionDate: now, Value: 10}

		created, err := svc.Create(ctx, input)

		assert.ErrorIs(t, err, domain.ErrUnauthorized)
		assert.Nil(t, created)
		entryRepo.AssertNotCalled(t, "Create")
	})

	t.Run("Fail: Should fail if Habit does not exist", func(t *testing.T) {
		entryRepo := new(MockHabitEntryRepo)
		habitRepo := new(MockHabitRepo)
		worker := getTestWorker()
		svc := services.NewEntryService(entryRepo, habitRepo, worker)

		habitRepo.On("GetByID", ctx, hid).Return(nil, domain.ErrHabitNotFound)

		input := services.CreateEntryInput{HabitID: hid, UserID: uid, CompletionDate: now}
		_, err := svc.Create(ctx, input)

		assert.ErrorIs(t, err, domain.ErrHabitNotFound)
	})
}

func TestEntryService_Update(t *testing.T) {
	ctx := context.Background()
	uid := "user-123"
	entryID := "entry-xyz"

	t.Run("Success: Should update valid entry", func(t *testing.T) {
		entryRepo := new(MockHabitEntryRepo)
		worker := getTestWorker()
		svc := services.NewEntryService(entryRepo, new(MockHabitRepo), worker)

		existing := &domain.HabitEntry{ID: entryID, HabitID: "habit-1", UserID: uid, Value: 5, Version: 1}

		entryRepo.On("GetByID", ctx, entryID).Return(existing, nil)

		entryRepo.On("Update", ctx, mock.MatchedBy(func(e *domain.HabitEntry) bool {
			versionOK := e.Version == 2
			valueOK := e.Value == 10
			dateOK := !e.UpdatedAt.IsZero()
			return versionOK && valueOK && dateOK
		})).Return(nil)

		input := services.UpdateEntryInput{ID: entryID, UserID: uid, Value: 10, Version: 1}
		updated, err := svc.Update(ctx, input)

		require.NoError(t, err)
		assert.Equal(t, 10, updated.Value)
		assert.Equal(t, 2, updated.Version)
		entryRepo.AssertExpectations(t)
	})

	t.Run("Concurrency: Should fail if version conflict", func(t *testing.T) {
		entryRepo := new(MockHabitEntryRepo)
		worker := getTestWorker()
		svc := services.NewEntryService(entryRepo, new(MockHabitRepo), worker)

		existing := &domain.HabitEntry{ID: entryID, UserID: uid, Value: 5, Version: 2}
		entryRepo.On("GetByID", ctx, entryID).Return(existing, nil)

		input := services.UpdateEntryInput{ID: entryID, UserID: uid, Value: 10, Version: 1}

		_, err := svc.Update(ctx, input)

		assert.ErrorIs(t, err, domain.ErrEntryConflict)
		entryRepo.AssertNotCalled(t, "Update")
	})

	t.Run("Security: Should fail if updating entry of another user", func(t *testing.T) {
		entryRepo := new(MockHabitEntryRepo)
		worker := getTestWorker()
		svc := services.NewEntryService(entryRepo, new(MockHabitRepo), worker)

		existing := &domain.HabitEntry{ID: entryID, UserID: "victim", Value: 5}
		entryRepo.On("GetByID", ctx, entryID).Return(existing, nil)

		input := services.UpdateEntryInput{ID: entryID, UserID: "attacker", Value: 10}

		_, err := svc.Update(ctx, input)

		assert.ErrorIs(t, err, domain.ErrUnauthorized)
	})
}

func TestEntryService_Delete(t *testing.T) {
	ctx := context.Background()
	uid := "user-123"
	entryID := "entry-del"

	t.Run("Success: Should delete owned entry", func(t *testing.T) {
		entryRepo := new(MockHabitEntryRepo)
		worker := getTestWorker()
		svc := services.NewEntryService(entryRepo, new(MockHabitRepo), worker)

		entryRepo.On("GetByID", ctx, entryID).Return(&domain.HabitEntry{ID: entryID, HabitID: "habit-1", UserID: uid}, nil)
		entryRepo.On("Delete", ctx, entryID, uid).Return(nil)

		err := svc.Delete(ctx, entryID, uid)
		assert.NoError(t, err)
		entryRepo.AssertExpectations(t)
	})

	t.Run("Security: Should return Unauthorized if user mismatch", func(t *testing.T) {
		entryRepo := new(MockHabitEntryRepo)
		worker := getTestWorker()
		svc := services.NewEntryService(entryRepo, new(MockHabitRepo), worker)

		entryRepo.On("GetByID", ctx, entryID).Return(&domain.HabitEntry{ID: entryID, UserID: "owner"}, nil)

		err := svc.Delete(ctx, entryID, "attacker")
		assert.ErrorIs(t, err, domain.ErrUnauthorized)
		entryRepo.AssertNotCalled(t, "Delete")
	})

	t.Run("Fail: Should return NotFound if entry doesn't exist", func(t *testing.T) {
		entryRepo := new(MockHabitEntryRepo)
		worker := getTestWorker()
		svc := services.NewEntryService(entryRepo, new(MockHabitRepo), worker)

		entryRepo.On("GetByID", ctx, entryID).Return(nil, domain.ErrEntryNotFound)

		err := svc.Delete(ctx, entryID, uid)
		assert.ErrorIs(t, err, domain.ErrEntryNotFound)
	})
}

func TestEntryService_GetDelta(t *testing.T) {
	ctx := context.Background()
	uid := "user-sync"
	since := time.Now().Add(-24 * time.Hour)

	t.Run("Success: Should propagate sync parameters to repo", func(t *testing.T) {
		entryRepo := new(MockHabitEntryRepo)
		worker := getTestWorker()
		svc := services.NewEntryService(entryRepo, new(MockHabitRepo), worker)

		expectedList := []*domain.HabitEntry{{ID: "1"}, {ID: "2"}}
		entryRepo.On("GetChanges", ctx, uid, since).Return(expectedList, nil)

		result, err := svc.GetDelta(ctx, uid, since)

		require.NoError(t, err)
		assert.Len(t, result, 2)
		entryRepo.AssertExpectations(t)
	})
}

func TestEntryService_GetByID(t *testing.T) {
	ctx := context.Background()
	uid := "user-123"
	entryID := "entry-read"

	t.Run("Success: Should return entry if owned by user", func(t *testing.T) {
		entryRepo := new(MockHabitEntryRepo)
		worker := getTestWorker()
		svc := services.NewEntryService(entryRepo, new(MockHabitRepo), worker)

		expected := &domain.HabitEntry{ID: entryID, UserID: uid, Value: 10}
		entryRepo.On("GetByID", ctx, entryID).Return(expected, nil)

		result, err := svc.GetByID(ctx, entryID, uid)

		require.NoError(t, err)
		assert.Equal(t, expected, result)
	})

	t.Run("Security: Should prevent reading other users' entries", func(t *testing.T) {
		entryRepo := new(MockHabitEntryRepo)
		worker := getTestWorker()
		svc := services.NewEntryService(entryRepo, new(MockHabitRepo), worker)

		found := &domain.HabitEntry{ID: entryID, UserID: "other-user"}
		entryRepo.On("GetByID", ctx, entryID).Return(found, nil)

		result, err := svc.GetByID(ctx, entryID, uid)

		assert.ErrorIs(t, err, domain.ErrUnauthorized)
		assert.Nil(t, result)
	})
}

func TestEntryService_ListByHabitID(t *testing.T) {
	ctx := context.Background()
	uid := "user-123"
	hid := "habit-123"
	now := time.Now()

	t.Run("Success: Should list entries if habit owned by user", func(t *testing.T) {
		entryRepo := new(MockHabitEntryRepo)
		habitRepo := new(MockHabitRepo)
		worker := getTestWorker()
		svc := services.NewEntryService(entryRepo, habitRepo, worker)

		habitRepo.On("GetByID", ctx, hid).Return(&domain.Habit{ID: hid, UserID: uid}, nil)

		expectedList := []*domain.HabitEntry{{ID: "1"}, {ID: "2"}}

		entryRepo.On("ListByHabitIDWithRange", ctx, hid, mock.Anything, mock.Anything).Return(expectedList, nil)

		list, err := svc.ListByHabitID(ctx, hid, uid, now, now)

		require.NoError(t, err)
		assert.Len(t, list, 2)
	})

	t.Run("Security: Should prevent listing if habit belongs to another", func(t *testing.T) {
		entryRepo := new(MockHabitEntryRepo)
		habitRepo := new(MockHabitRepo)
		worker := getTestWorker()
		svc := services.NewEntryService(entryRepo, habitRepo, worker)

		habitRepo.On("GetByID", ctx, hid).Return(&domain.Habit{ID: hid, UserID: "stranger"}, nil)

		list, err := svc.ListByHabitID(ctx, hid, uid, now, now)

		assert.ErrorIs(t, err, domain.ErrUnauthorized)
		assert.Nil(t, list)
		entryRepo.AssertNotCalled(t, "ListByHabitIDWithRange")
	})
}
