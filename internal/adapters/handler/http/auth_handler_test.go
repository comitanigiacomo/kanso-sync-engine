package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
	"github.com/gin-gonic/gin"
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

func setupHandler() (*gin.Engine, *MockUserRepository) {
	gin.SetMode(gin.TestMode)

	mockRepo := new(MockUserRepository)
	authService := services.NewAuthService(mockRepo)
	authHandler := NewAuthHandler(authService)

	router := gin.New()
	authHandler.RegisterRoutes(router.Group(""))

	return router, mockRepo
}

func TestAuthHandler_Register(t *testing.T) {
	t.Run("Success: Should return 201 and created user (No Password)", func(t *testing.T) {
		router, mockRepo := setupHandler()

		payload := map[string]string{
			"email":    "api_test@kanso.app",
			"password": "PasswordSuperSegreta1!",
		}
		body, _ := json.Marshal(payload)

		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.User")).Return(nil)

		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response userResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, payload["email"], response.Email)
		assert.NotEmpty(t, response.ID)

		assert.NotContains(t, w.Body.String(), "password")

		mockRepo.AssertExpectations(t)
	})

	t.Run("Fail: Should return 400 for Bad JSON (Invalid Email)", func(t *testing.T) {
		router, mockRepo := setupHandler()

		payload := map[string]string{
			"email":    "not-an-email",
			"password": "Password123!",
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockRepo.AssertNotCalled(t, "Create")
	})

	t.Run("Fail: Should return 400 for Bad JSON (Password too short)", func(t *testing.T) {
		router, mockRepo := setupHandler()

		payload := map[string]string{
			"email":    "valid@email.com",
			"password": "short",
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockRepo.AssertNotCalled(t, "Create")
	})

	t.Run("Fail: Should return 409 Conflict if email exists", func(t *testing.T) {
		router, mockRepo := setupHandler()

		payload := map[string]string{
			"email":    "duplicate@kanso.app",
			"password": "PasswordValidissima!",
		}
		body, _ := json.Marshal(payload)

		mockRepo.On("Create", mock.Anything, mock.Anything).Return(domain.ErrEmailAlreadyExists)

		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "email already exists")
	})

	t.Run("Fail: Should return 500 Internal Server Error on DB failure", func(t *testing.T) {
		router, mockRepo := setupHandler()

		payload := map[string]string{
			"email":    "crash@kanso.app",
			"password": "PasswordValidissima!",
		}
		body, _ := json.Marshal(payload)

		mockRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db connection lost"))

		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "internal server error")
	})
}
