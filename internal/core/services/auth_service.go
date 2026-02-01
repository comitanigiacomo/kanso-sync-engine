package services

import (
	"context"
	"fmt"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/google/uuid"
)

type AuthService struct {
	repo         domain.UserRepository
	tokenService *TokenService
}

func NewAuthService(repo domain.UserRepository, tokenService *TokenService) *AuthService {
	return &AuthService{
		repo:         repo,
		tokenService: tokenService,
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

type LoginInput struct {
	Email    string
	Password string
}

type LoginOutput struct {
	Token string
	User  *domain.User
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*LoginOutput, error) {
	user, err := s.repo.GetByEmail(ctx, input.Email)
	if err != nil {
		return nil, fmt.Errorf("auth service: user lookup failed: %w", err)
	}

	if err := user.CheckPassword(input.Password); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	token, err := s.tokenService.GenerateToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("auth service: failed to generate token: %w", err)
	}

	return &LoginOutput{
		Token: token,
		User:  user,
	}, nil
}
