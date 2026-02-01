package domain

import (
	"testing"
	"time"
)

func TestNewUser(t *testing.T) {
	t.Parallel()

	t.Run("Should create user with normalized email", func(t *testing.T) {
		t.Parallel()

		dirtyEmail := "  Test.User@Gmail.COM  "
		id := "123"

		user, err := NewUser(id, dirtyEmail)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		expectedEmail := "test.user@gmail.com"
		if user.Email != expectedEmail {
			t.Errorf("Expected email %s, got %s", expectedEmail, user.Email)
		}

		if user.ID != id {
			t.Errorf("Expected id %s, got %s", id, user.ID)
		}

		if user.CreatedAt.IsZero() {
			t.Error("Expected CreatedAt to be set")
		}
	})

	t.Run("Should fail with invalid email", func(t *testing.T) {
		t.Parallel()
		_, err := NewUser("123", "invalid-email-format")

		if err != ErrInvalidEmail {
			t.Errorf("Expected ErrInvalidEmail, got %v", err)
		}
	})
}

func TestUserPassword(t *testing.T) {
	t.Parallel()

	t.Run("Should hash password correctly and update timestamp", func(t *testing.T) {
		t.Parallel()
		user, _ := NewUser("123", "test@test.com")
		plainPass := "superSecret123"

		oldUpdatedAt := user.UpdatedAt

		time.Sleep(1 * time.Millisecond)

		err := user.SetPassword(plainPass)
		if err != nil {
			t.Fatalf("Expected no error setting password, got %v", err)
		}

		if user.PasswordHash == plainPass {
			t.Error("Password should be hashed, not plain text")
		}

		if len(user.PasswordHash) == 0 {
			t.Error("Password hash should not be empty")
		}

		if !user.UpdatedAt.After(oldUpdatedAt) {
			t.Error("UpdatedAt should be updated after setting password")
		}
	})

	t.Run("Should validate password length", func(t *testing.T) {
		t.Parallel()
		user, _ := NewUser("123", "test@test.com")

		err := user.SetPassword("short")
		if err != ErrPasswordTooShort {
			t.Errorf("Expected ErrPasswordTooShort, got %v", err)
		}
	})

	t.Run("CheckPassword should work", func(t *testing.T) {
		t.Parallel()
		user, _ := NewUser("123", "test@test.com")
		pass := "correctPassword"
		_ = user.SetPassword(pass)

		if err := user.CheckPassword(pass); err != nil {
			t.Errorf("Expected password to match, got error: %v", err)
		}

		if err := user.CheckPassword("wrongPassword"); err == nil {
			t.Error("Expected error for wrong password, got nil")
		}
	})
}
