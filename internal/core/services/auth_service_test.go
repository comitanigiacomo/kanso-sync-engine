package services

import (
	"context"
	"testing"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func TestAuthService_Register(t *testing.T) {
	t.Parallel()

	t.Run("Success: Should register a valid user", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		service := NewAuthService(mockRepo)
		ctx := context.Background()

		input := RegisterInput{
			Email:    "test_success@kanso.app",
			Password: "StrongPassword123!",
		}

		mockRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).Return(nil)

		user, err := service.Register(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, input.Email, user.Email)
		assert.NotEmpty(t, user.ID)
		assert.NotEmpty(t, user.PasswordHash)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Fail: Should return error for invalid email", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		service := NewAuthService(mockRepo)
		ctx := context.Background()

		input := RegisterInput{Email: "not-an-email", Password: "pass"}

		user, err := service.Register(ctx, input)

		assert.ErrorIs(t, err, domain.ErrInvalidEmail)
		assert.Nil(t, user)

		mockRepo.AssertNotCalled(t, "Create")
	})

	t.Run("Fail: Should return error for short password", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		service := NewAuthService(mockRepo)
		ctx := context.Background()

		input := RegisterInput{Email: "valid@email.com", Password: "short"}

		user, err := service.Register(ctx, input)

		assert.ErrorIs(t, err, domain.ErrPasswordTooShort)
		assert.Nil(t, user)

		mockRepo.AssertNotCalled(t, "Create")
	})

	t.Run("Fail: Should propagate repository error (Duplicate Email)", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		service := NewAuthService(mockRepo)
		ctx := context.Background()

		input := RegisterInput{Email: "duplicate@email.com", Password: "StrongPassword123!"}

		mockRepo.On("Create", ctx, mock.Anything).Return(domain.ErrEmailAlreadyExists)

		user, err := service.Register(ctx, input)

		assert.ErrorIs(t, err, domain.ErrEmailAlreadyExists)
		assert.Nil(t, user)

		mockRepo.AssertExpectations(t)
	})
}
