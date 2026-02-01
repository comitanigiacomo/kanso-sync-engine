package services

import (
	"context"
	"fmt"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/google/uuid"
)

type AuthService struct {
	repo domain.UserRepository
}

func NewAuthService(repo domain.UserRepository) *AuthService {
	return &AuthService{
		repo: repo,
	}
}

type RegisterInput struct {
	Email    string
	Password string
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*domain.User, error) {
	id := uuid.NewString()
	user, err := domain.NewUser(id, input.Email)
	if err != nil {
		return nil, err
	}

	if err := user.SetPassword(input.Password); err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("auth service: failed to create user: %w", err)
	}

	return user, nil
}
