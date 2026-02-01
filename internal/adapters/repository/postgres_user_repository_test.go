package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "kanso_user"
	}

	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		dbPass = "secret"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "kanso_db"
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, dbHost, dbPort, dbName)

	var err error
	testDB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Cannot connect to DB: %v", err)
	}

	for i := 0; i < 5; i++ {
		if err := testDB.Ping(); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
		if i == 4 {
			log.Fatal("Cannot ping DB after retries")
		}
	}

	code := m.Run()

	testDB.Close()
	os.Exit(code)
}

func TestPostgresUserRepository_Create(t *testing.T) {
	t.Parallel()

	repo := NewPostgresUserRepository(testDB)
	ctx := context.Background()

	t.Run("Should create a user successfully", func(t *testing.T) {
		t.Parallel()

		email := fmt.Sprintf("test_%s@example.com", uuid.NewString())
		id := uuid.NewString()

		user, err := domain.NewUser(id, email)
		if err != nil {
			t.Fatalf("Failed to create domain user: %v", err)
		}
		_ = user.SetPassword("passwordStrong123")

		err = repo.Create(ctx, user)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		savedUser, err := repo.GetByEmail(ctx, user.Email)
		if err != nil {
			t.Fatalf("Could not retrieve saved user: %v", err)
		}

		if savedUser.ID != user.ID {
			t.Errorf("Expected ID %s, got %s", user.ID, savedUser.ID)
		}
		if savedUser.CreatedAt.IsZero() || savedUser.UpdatedAt.IsZero() {
			t.Error("Timestamps should not be zero")
		}
	})

	t.Run("Should fail on duplicate email", func(t *testing.T) {
		t.Parallel()

		email := fmt.Sprintf("duplicate_%s@example.com", uuid.NewString())
		user1, _ := domain.NewUser(uuid.NewString(), email)

		_ = user1.SetPassword("passwordSuperSicura1")
		_ = repo.Create(ctx, user1)

		user2, _ := domain.NewUser(uuid.NewString(), email)
		_ = user2.SetPassword("passwordSuperSicura2")

		err := repo.Create(ctx, user2)

		if err != domain.ErrEmailAlreadyExists {
			t.Errorf("Expected ErrEmailAlreadyExists, got %v", err)
		}
	})
}

func TestPostgresUserRepository_GetByID(t *testing.T) {
	t.Parallel()
	repo := NewPostgresUserRepository(testDB)
	ctx := context.Background()

	t.Run("Should retrieve existing user by ID", func(t *testing.T) {
		t.Parallel()

		email := fmt.Sprintf("id_test_%s@example.com", uuid.NewString())
		id := uuid.NewString()
		user, _ := domain.NewUser(id, email)

		_ = user.SetPassword("passwordLunga123")

		err := repo.Create(ctx, user)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		foundUser, err := repo.GetByID(ctx, id)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if foundUser.Email != user.Email {
			t.Errorf("Expected email %s, got %s", user.Email, foundUser.Email)
		}
	})

	t.Run("Should return ErrUserNotFound for non-existent ID", func(t *testing.T) {
		t.Parallel()
		_, err := repo.GetByID(ctx, uuid.NewString())

		if err != domain.ErrUserNotFound {
			t.Errorf("Expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestPostgresUserRepository_GetByEmail(t *testing.T) {
	t.Parallel()
	repo := NewPostgresUserRepository(testDB)
	ctx := context.Background()

	t.Run("Should retrieve existing user by Email", func(t *testing.T) {
		t.Parallel()

		email := fmt.Sprintf("email_test_%s@example.com", uuid.NewString())
		id := uuid.NewString()
		user, _ := domain.NewUser(id, email)

		_ = user.SetPassword("Longpassword123")

		err := repo.Create(ctx, user)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		foundUser, err := repo.GetByEmail(ctx, email)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if foundUser.ID != user.ID {
			t.Errorf("Expected ID %s, got %s", user.ID, foundUser.ID)
		}
	})
	t.Run("Should return ErrUserNotFound for non-existent email", func(t *testing.T) {
		t.Parallel()
		_, err := repo.GetByEmail(ctx, "nonexistent@ghost.com")

		if err != domain.ErrUserNotFound {
			t.Errorf("Expected ErrUserNotFound, got %v", err)
		}
	})
}
