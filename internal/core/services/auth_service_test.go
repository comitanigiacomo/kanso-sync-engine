package services

import (
	"context"
	"errors"
	"testing"
	"time"

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

func (m *MockUserRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestAuthService_Register(t *testing.T) {
	t.Parallel()

	setup := func() (*AuthService, *MockUserRepository) {
		mockRepo := new(MockUserRepository)
		tokenService := NewTokenService("test-secret", "test-issuer", 1*time.Hour, mockRepo)
		return NewAuthService(mockRepo, tokenService), mockRepo
	}

	t.Run("Success: Should register a valid user", func(t *testing.T) {
		t.Parallel()
		service, mockRepo := setup()
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
		t.Parallel()
		service, mockRepo := setup()
		ctx := context.Background()

		input := RegisterInput{Email: "not-an-email", Password: "pass"}

		user, err := service.Register(ctx, input)

		assert.ErrorIs(t, err, domain.ErrInvalidEmail)
		assert.Nil(t, user)

		mockRepo.AssertNotCalled(t, "Create")
	})

	t.Run("Fail: Should return error for short password", func(t *testing.T) {
		t.Parallel()
		service, mockRepo := setup()
		ctx := context.Background()

		input := RegisterInput{Email: "valid@email.com", Password: "short"}

		user, err := service.Register(ctx, input)

		assert.ErrorIs(t, err, domain.ErrPasswordTooShort)
		assert.Nil(t, user)

		mockRepo.AssertNotCalled(t, "Create")
	})

	t.Run("Fail: Should propagate repository error (Duplicate Email)", func(t *testing.T) {
		t.Parallel()
		service, mockRepo := setup()
		ctx := context.Background()

		input := RegisterInput{Email: "duplicate@email.com", Password: "StrongPassword123!"}

		mockRepo.On("Create", ctx, mock.Anything).Return(domain.ErrEmailAlreadyExists)

		user, err := service.Register(ctx, input)

		assert.ErrorIs(t, err, domain.ErrEmailAlreadyExists)
		assert.Nil(t, user)

		mockRepo.AssertExpectations(t)
	})
}

func TestAuthService_Login(t *testing.T) {
	t.Parallel()

	setup := func() (*AuthService, *MockUserRepository, *TokenService) {
		mockRepo := new(MockUserRepository)
		tokenService := NewTokenService("test-secret", "test-issuer", 1*time.Hour, mockRepo)
		return NewAuthService(mockRepo, tokenService), mockRepo, tokenService
	}

	getValidUser := func() *domain.User {
		u, _ := domain.NewUser("user-123", "login@kanso.app")
		_ = u.SetPassword("Password123!")
		return u
	}

	t.Run("Success: Should login with correct credentials", func(t *testing.T) {
		t.Parallel()
		service, mockRepo, tokenService := setup()
		ctx := context.Background()
		validUser := getValidUser()

		input := LoginInput{Email: "login@kanso.app", Password: "Password123!"}

		mockRepo.On("GetByEmail", ctx, input.Email).Return(validUser, nil)
		mockRepo.On("GetByID", mock.Anything, validUser.ID).Return(validUser, nil)

		output, err := service.Login(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.NotEmpty(t, output.Token)
		assert.Equal(t, validUser.Email, output.User.Email)

		userID, err := tokenService.ValidateToken(output.Token)
		assert.NoError(t, err)
		assert.Equal(t, validUser.ID, userID)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Fail: Should return error on wrong password", func(t *testing.T) {
		t.Parallel()
		service, mockRepo, _ := setup()
		ctx := context.Background()
		validUser := getValidUser()

		input := LoginInput{Email: "login@kanso.app", Password: "WrongPassword!"}

		mockRepo.On("GetByEmail", ctx, input.Email).Return(validUser, nil)

		output, err := service.Login(ctx, input)

		assert.ErrorIs(t, err, domain.ErrInvalidCredentials)
		assert.Nil(t, output)
	})

	t.Run("Fail: Should propagate error if user not found", func(t *testing.T) {
		t.Parallel()
		service, mockRepo, _ := setup()
		ctx := context.Background()

		input := LoginInput{Email: "ghost@kanso.app", Password: "Pwd"}

		mockRepo.On("GetByEmail", ctx, input.Email).Return(nil, domain.ErrUserNotFound)

		output, err := service.Login(ctx, input)

		assert.ErrorIs(t, err, domain.ErrUserNotFound)
		assert.Nil(t, output)
	})
}

func TestAuthService_DeleteAccount(t *testing.T) {
	t.Parallel()

	setup := func() (*AuthService, *MockUserRepository) {
		mockRepo := new(MockUserRepository)
		tokenService := NewTokenService("test-secret", "test-issuer", 1*time.Hour, mockRepo)
		return NewAuthService(mockRepo, tokenService), mockRepo
	}

	t.Run("Success: Should delete account successfully", func(t *testing.T) {
		t.Parallel()
		service, mockRepo := setup()
		ctx := context.Background()
		userID := "user-to-delete-123"

		mockRepo.On("Delete", ctx, userID).Return(nil)

		err := service.DeleteAccount(ctx, userID)

		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Fail: Should propagate repository error", func(t *testing.T) {
		t.Parallel()
		service, mockRepo := setup()
		ctx := context.Background()
		userID := "user-error-123"

		expectedErr := errors.New("database connection failed")
		mockRepo.On("Delete", ctx, userID).Return(expectedErr)

		err := service.DeleteAccount(ctx, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), expectedErr.Error())
		mockRepo.AssertExpectations(t)
	})
}
