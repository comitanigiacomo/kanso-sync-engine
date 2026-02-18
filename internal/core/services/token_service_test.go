package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockUserRepoForToken struct {
	mock.Mock
}

func (m *MockUserRepoForToken) Create(ctx context.Context, user *domain.User) error {
	return m.Called(ctx, user).Error(0)
}
func (m *MockUserRepoForToken) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepoForToken) GetByID(ctx context.Context, id string) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepoForToken) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

func TestTokenService_GenerateAndValidate(t *testing.T) {
	secret := "super-secret-key-for-testing"
	issuer := "kanso-test"
	userID := "user-123-uuid"

	setup := func() (*TokenService, *MockUserRepoForToken) {
		mockRepo := new(MockUserRepoForToken)
		return NewTokenService(secret, issuer, 1*time.Hour, mockRepo), mockRepo
	}

	t.Run("Success: Should generate and validate a token", func(t *testing.T) {
		service, mockRepo := setup()

		mockRepo.On("GetByID", mock.Anything, userID).Return(&domain.User{ID: userID}, nil)

		tokenString, err := service.GenerateToken(userID)
		assert.NoError(t, err)
		assert.NotEmpty(t, tokenString)

		extractedID, err := service.ValidateToken(tokenString)
		assert.NoError(t, err)
		assert.Equal(t, userID, extractedID)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Fail: Should reject valid token if user is deleted (DB check)", func(t *testing.T) {
		service, mockRepo := setup()

		mockRepo.On("GetByID", mock.Anything, userID).Return(nil, errors.New("user not found"))

		tokenString, err := service.GenerateToken(userID)
		assert.NoError(t, err)

		extractedID, err := service.ValidateToken(tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user no longer exists")
		assert.Empty(t, extractedID)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Fail: Should reject expired token", func(t *testing.T) {
		mockRepo := new(MockUserRepoForToken)
		service := NewTokenService(secret, issuer, -1*time.Second, mockRepo)

		tokenString, err := service.GenerateToken(userID)
		assert.NoError(t, err)

		extractedID, err := service.ValidateToken(tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token is expired")
		assert.Empty(t, extractedID)
	})

	t.Run("Fail: Should reject token with wrong secret (Tampered)", func(t *testing.T) {
		service, _ := setup()
		tokenString, _ := service.GenerateToken(userID)

		mockRepoAttacker := new(MockUserRepoForToken)
		attackerService := NewTokenService("wrong-key", issuer, 1*time.Hour, mockRepoAttacker)

		extractedID, err := attackerService.ValidateToken(tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid token")
		assert.Empty(t, extractedID)
	})

	t.Run("Fail: Should reject token with wrong issuer", func(t *testing.T) {
		mockRepo := new(MockUserRepoForToken)
		serviceA := NewTokenService(secret, "correct-issuer", 1*time.Hour, mockRepo)
		tokenString, _ := serviceA.GenerateToken(userID)

		serviceB := NewTokenService(secret, "wrong-issuer", 1*time.Hour, mockRepo)

		extractedID, err := serviceB.ValidateToken(tokenString)
		assert.Error(t, err)
		assert.Equal(t, "invalid token issuer", err.Error())
		assert.Empty(t, extractedID)
	})

	t.Run("Fail: Should reject 'None' algorithm attack", func(t *testing.T) {
		token := jwt.New(jwt.SigningMethodNone)
		claims := token.Claims.(jwt.MapClaims)
		claims["sub"] = userID
		claims["iss"] = issuer

		fakeTokenString, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

		service, _ := setup()
		_, err := service.ValidateToken(fakeTokenString)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected signing method")
	})

	t.Run("Fail: Should reject malformed token string", func(t *testing.T) {
		service, _ := setup()

		extractedID, err := service.ValidateToken("this-is-not-a-jwt")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid token")
		assert.Empty(t, extractedID)
	})
}
