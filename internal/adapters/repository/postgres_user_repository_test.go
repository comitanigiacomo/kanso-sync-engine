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
	// SENIOR FIX 1: Configurazione Dinamica
	// Leggiamo dall'ambiente (per la CI/CD), usiamo default per locale.
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

	// Retry mechanism per CI/CD lenti (Opzionale ma consigliato)
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
	t.Parallel() // SENIOR FIX 2: Esecuzione Parallela

	repo := NewPostgresUserRepository(testDB)
	ctx := context.Background()

	t.Run("Should create a user successfully", func(t *testing.T) {
		t.Parallel()

		// Usiamo UUID per evitare collisioni nei test paralleli
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

		// Verifica lettura
		savedUser, err := repo.GetByEmail(ctx, user.Email)
		if err != nil {
			t.Fatalf("Could not retrieve saved user: %v", err)
		}

		if savedUser.ID != user.ID {
			t.Errorf("Expected ID %s, got %s", user.ID, savedUser.ID)
		}
		// Verifica che i timestamp siano stati salvati (non zero)
		if savedUser.CreatedAt.IsZero() || savedUser.UpdatedAt.IsZero() {
			t.Error("Timestamps should not be zero")
		}
	})

	t.Run("Should fail on duplicate email", func(t *testing.T) {
		t.Parallel()

		email := fmt.Sprintf("duplicate_%s@example.com", uuid.NewString())
		user1, _ := domain.NewUser(uuid.NewString(), email)
		_ = user1.SetPassword("pass1")
		_ = repo.Create(ctx, user1)

		user2, _ := domain.NewUser(uuid.NewString(), email) // ID diverso, stessa email
		_ = user2.SetPassword("pass2")

		err := repo.Create(ctx, user2)

		if err != domain.ErrEmailAlreadyExists {
			t.Errorf("Expected ErrEmailAlreadyExists, got %v", err)
		}
	})
}

// SENIOR FIX 3: Aggiunto test mancante per GetByID
func TestPostgresUserRepository_GetByID(t *testing.T) {
	t.Parallel()
	repo := NewPostgresUserRepository(testDB)
	ctx := context.Background()

	t.Run("Should retrieve existing user by ID", func(t *testing.T) {
		t.Parallel()

		// Arrange
		email := fmt.Sprintf("id_test_%s@example.com", uuid.NewString())
		id := uuid.NewString()
		user, _ := domain.NewUser(id, email)
		_ = user.SetPassword("pass123")
		_ = repo.Create(ctx, user)

		// Act
		foundUser, err := repo.GetByID(ctx, id)

		// Assert
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if foundUser.Email != user.Email {
			t.Errorf("Expected email %s, got %s", user.Email, foundUser.Email)
		}
	})

	t.Run("Should return ErrUserNotFound for non-existent ID", func(t *testing.T) {
		t.Parallel()
		_, err := repo.GetByID(ctx, uuid.NewString()) // ID random mai salvato

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

		// Arrange
		email := fmt.Sprintf("email_test_%s@example.com", uuid.NewString())
		id := uuid.NewString()
		user, _ := domain.NewUser(id, email)
		_ = user.SetPassword("pass123")
		_ = repo.Create(ctx, user)

		// Act
		foundUser, err := repo.GetByEmail(ctx, email)

		// Assert
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
