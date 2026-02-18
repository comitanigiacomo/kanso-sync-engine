package middleware

import (
	"context"
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

type MockUserRepo struct {
	mock.Mock
}

func (m *MockUserRepo) Create(ctx context.Context, user *domain.User) error {
	return m.Called(ctx, user).Error(0)
}
func (m *MockUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Parallel()

	setupRouter := func(tokenService *services.TokenService) *gin.Engine {
		router := gin.New()
		router.Use(AuthMiddleware(tokenService))
		router.GET("/protected", func(c *gin.Context) {
			userID, ok := GetUserID(c)
			if !ok {
				c.String(http.StatusInternalServerError, "UserID not found in context")
				return
			}
			c.String(http.StatusOK, "Hello "+userID)
		})
		return router
	}

	secret := "test-secret-middleware"
	issuer := "test-issuer"

	t.Run("Success: Valid Token", func(t *testing.T) {
		t.Parallel()
		mockRepo := new(MockUserRepo)
		tokenService := services.NewTokenService(secret, issuer, 1*time.Hour, mockRepo)
		router := setupRouter(tokenService)

		userID := "user-123"
		mockRepo.On("GetByID", mock.Anything, userID).Return(&domain.User{ID: userID}, nil)

		validToken, _ := tokenService.GenerateToken(userID)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "Hello user-123", w.Body.String())
	})

	t.Run("Fail: Missing Authorization Header", func(t *testing.T) {
		t.Parallel()
		mockRepo := new(MockUserRepo)
		tokenService := services.NewTokenService(secret, issuer, 1*time.Hour, mockRepo)
		router := setupRouter(tokenService)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "authorization header required")
	})

	t.Run("Fail: Invalid Header Format", func(t *testing.T) {
		t.Parallel()
		mockRepo := new(MockUserRepo)
		tokenService := services.NewTokenService(secret, issuer, 1*time.Hour, mockRepo)
		router := setupRouter(tokenService)

		formats := []string{
			"Bearer",
			"Token 12345",
			"Bearer12345",
			"Bearer ",
		}

		for _, h := range formats {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", h)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code, "Should fail for header: "+h)
		}
	})

	t.Run("Fail: Token with Wrong Signature (Tampered)", func(t *testing.T) {
		t.Parallel()
		mockRepo := new(MockUserRepo)
		serviceMiddleware := services.NewTokenService(secret, issuer, 1*time.Hour, mockRepo)
		serviceAttacker := services.NewTokenService("wrong-secret", issuer, 1*time.Hour, mockRepo)

		router := setupRouter(serviceMiddleware)
		badToken, _ := serviceAttacker.GenerateToken("attacker")

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+badToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "invalid or expired token")
	})

	t.Run("Fail: Expired Token", func(t *testing.T) {
		t.Parallel()
		mockRepo := new(MockUserRepo)
		expiredService := services.NewTokenService(secret, issuer, -1*time.Second, mockRepo)

		router := setupRouter(expiredService)

		expiredToken, _ := expiredService.GenerateToken("user-expired")

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+expiredToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "invalid or expired token")
	})
}
