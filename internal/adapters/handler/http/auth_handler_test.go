package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func (m *MockUserRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func setupHandler(middleware gin.HandlerFunc) (*gin.Engine, *MockUserRepository) {
	gin.SetMode(gin.TestMode)

	mockRepo := new(MockUserRepository)

	tokenService := services.NewTokenService("test-secret-key", "test-issuer", 1*time.Hour, mockRepo)

	authService := services.NewAuthService(mockRepo, tokenService)
	authHandler := NewAuthHandler(authService)

	router := gin.New()

	if middleware == nil {
		middleware = func(c *gin.Context) {
			c.Next()
		}
	}

	authHandler.RegisterRoutes(router.Group(""), middleware)

	return router, mockRepo
}

func TestAuthHandler_Register(t *testing.T) {
	t.Run("Success: Should return 201 and created user (No Password)", func(t *testing.T) {
		router, mockRepo := setupHandler(nil)

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
		router, mockRepo := setupHandler(nil)

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

	t.Run("Fail: Should return 409 Conflict if email exists", func(t *testing.T) {
		router, mockRepo := setupHandler(nil)

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
	})

	t.Run("Fail: Should return 500 Internal Server Error on DB failure", func(t *testing.T) {
		router, mockRepo := setupHandler(nil)

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
	})
}

func TestAuthHandler_Login(t *testing.T) {
	validUser, _ := domain.NewUser("user-123", "login@kanso.app")
	_ = validUser.SetPassword("Password123!")

	t.Run("Success: Should return 200 and Token", func(t *testing.T) {
		router, mockRepo := setupHandler(nil)

		payload := map[string]string{
			"email":    "login@kanso.app",
			"password": "Password123!",
		}
		body, _ := json.Marshal(payload)

		mockRepo.On("GetByEmail", mock.Anything, payload["email"]).Return(validUser, nil)

		req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response loginResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.NotEmpty(t, response.Token)
		assert.Equal(t, validUser.ID, response.User.ID)
		assert.Equal(t, validUser.Email, response.User.Email)
	})

	t.Run("Fail: Should return 401 on Wrong Password", func(t *testing.T) {
		router, mockRepo := setupHandler(nil)

		payload := map[string]string{
			"email":    "login@kanso.app",
			"password": "WrongPassword!",
		}
		body, _ := json.Marshal(payload)

		mockRepo.On("GetByEmail", mock.Anything, payload["email"]).Return(validUser, nil)

		req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "invalid credentials")
	})

	t.Run("Fail: Should return 401 (not 404) if User Not Found (Security)", func(t *testing.T) {
		router, mockRepo := setupHandler(nil)

		payload := map[string]string{
			"email":    "ghost@kanso.app",
			"password": "Password123!",
		}
		body, _ := json.Marshal(payload)

		mockRepo.On("GetByEmail", mock.Anything, payload["email"]).Return(nil, domain.ErrUserNotFound)

		req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "invalid credentials")
	})

	t.Run("Fail: Should return 400 for Bad JSON", func(t *testing.T) {
		router, _ := setupHandler(nil)

		payload := map[string]string{"email": "not-an-email"}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAuthHandler_DeleteAccount(t *testing.T) {

	t.Run("Success: Should delete account if authenticated", func(t *testing.T) {
		authMiddleware := func(c *gin.Context) {
			c.Set("userID", "user-to-delete")
			c.Next()
		}

		router, mockRepo := setupHandler(authMiddleware)

		mockRepo.On("Delete", mock.Anything, "user-to-delete").Return(nil)

		req, _ := http.NewRequest(http.MethodDelete, "/auth/user", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "account deleted")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Fail: Should return 401 if middleware does not set userID", func(t *testing.T) {
		emptyMiddleware := func(c *gin.Context) { c.Next() }

		router, _ := setupHandler(emptyMiddleware)

		req, _ := http.NewRequest(http.MethodDelete, "/auth/user", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Fail: Should return 500 if Service fails", func(t *testing.T) {
		authMiddleware := func(c *gin.Context) {
			c.Set("userID", "user-error")
			c.Next()
		}

		router, mockRepo := setupHandler(authMiddleware)

		mockRepo.On("Delete", mock.Anything, "user-error").Return(errors.New("delete failed"))

		req, _ := http.NewRequest(http.MethodDelete, "/auth/user", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
