package services

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestTokenService_GenerateAndValidate(t *testing.T) {
	secret := "super-secret-key-for-testing"
	issuer := "kanso-test"
	userID := "user-123-uuid"

	t.Run("Success: Should generate and validate a token", func(t *testing.T) {
		service := NewTokenService(secret, issuer, 1*time.Hour)

		tokenString, err := service.GenerateToken(userID)
		assert.NoError(t, err)
		assert.NotEmpty(t, tokenString)

		extractedID, err := service.ValidateToken(tokenString)
		assert.NoError(t, err)
		assert.Equal(t, userID, extractedID)
	})

	t.Run("Fail: Should reject expired token", func(t *testing.T) {
		service := NewTokenService(secret, issuer, -1*time.Second)

		tokenString, err := service.GenerateToken(userID)
		assert.NoError(t, err)

		extractedID, err := service.ValidateToken(tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token is expired")
		assert.Empty(t, extractedID)
	})

	t.Run("Fail: Should reject token with wrong secret (Tampered)", func(t *testing.T) {
		service := NewTokenService(secret, issuer, 1*time.Hour)
		tokenString, _ := service.GenerateToken(userID)

		attackerService := NewTokenService("wrong-key", issuer, 1*time.Hour)

		extractedID, err := attackerService.ValidateToken(tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "signature is invalid")
		assert.Empty(t, extractedID)
	})

	t.Run("Fail: Should reject token with wrong issuer", func(t *testing.T) {
		serviceA := NewTokenService(secret, "correct-issuer", 1*time.Hour)
		tokenString, _ := serviceA.GenerateToken(userID)

		serviceB := NewTokenService(secret, "wrong-issuer", 1*time.Hour)

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

		service := NewTokenService(secret, issuer, 1*time.Hour)
		_, err := service.ValidateToken(fakeTokenString)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected signing method")
	})

	t.Run("Fail: Should reject malformed token string", func(t *testing.T) {
		service := NewTokenService(secret, issuer, 1*time.Hour)

		extractedID, err := service.ValidateToken("this-is-not-a-jwt")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid token")
		assert.Empty(t, extractedID)
	})
}
