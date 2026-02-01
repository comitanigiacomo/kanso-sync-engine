package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidEmail       = errors.New("invalid email format")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters long")
)

type User struct {
	ID           string    `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

func NewUser(id, email string) (*User, error) {

	email = strings.TrimSpace(email)

	if !isValidEmail(email) {
		return nil, ErrInvalidEmail
	}

	now := time.Now().UTC()
	return &User{
		ID:        id,
		Email:     strings.ToLower(email),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (u *User) SetPassword(plainPassword string) error {
	if utf8.RuneCountInString(plainPassword) < 8 {
		return ErrPasswordTooShort
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), 12)
	if err != nil {
		return err
	}

	u.PasswordHash = string(hash)
	u.UpdatedAt = time.Now().UTC()
	return nil
}

func (u *User) CheckPassword(plainPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(plainPassword))
}

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}
