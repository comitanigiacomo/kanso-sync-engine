package services

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenService struct {
	secretKey     []byte
	issuer        string
	tokenDuration time.Duration
}

func NewTokenService(secretKey string, issuer string, tokenDuration time.Duration) *TokenService {
	return &TokenService{
		secretKey:     []byte(secretKey),
		issuer:        issuer,
		tokenDuration: tokenDuration,
	}
}

func (s *TokenService) GenerateToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(s.tokenDuration).Unix(),
		"iat": time.Now().Unix(),
		"iss": s.issuer,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(s.secretKey)
	if err != nil {
		return "", fmt.Errorf("token service: failed to sign token: %w", err)
	}

	return signedToken, nil
}

func (s *TokenService) ValidateToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secretKey, nil
	})

	if err != nil {
		return "", fmt.Errorf("invalid token: %w", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if iss, ok := claims["iss"].(string); !ok || iss != s.issuer {
			return "", fmt.Errorf("invalid token issuer")
		}

		userID, ok := claims["sub"].(string)
		if !ok {
			return "", fmt.Errorf("invalid token subject")
		}

		return userID, nil
	}

	return "", fmt.Errorf("invalid token claims")
}
