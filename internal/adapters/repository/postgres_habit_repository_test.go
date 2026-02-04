package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func setupTestDB(t *testing.T) *sqlx.DB {
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "kanso_user"
	}
	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		dbPass = "secret"
	}
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "kanso_db"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, dbHost, dbPort, dbName)

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Skipf("Skipping integration tests: database connection failed: %v", err)
	}
	return db
}

func cleanup(t *testing.T, db *sqlx.DB) {
	_, err := db.Exec("TRUNCATE TABLE habit_entries, habits, users CASCADE")
	require.NoError(t, err, "Failed to clean up database for Habit Repository tests")
}

func TestPostgresHabitRepository_Integration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cleanup(t, db)
	defer cleanup(t, db)

	repo := NewPostgresHabitRepository(db)
	ctx := context.Background()

	var now time.Time
	err := db.QueryRow("SELECT NOW()").Scan(&now)
	require.NoError(t, err)

	userID := "test-user-senior-1"

	_, err = db.Exec(`INSERT INTO users (id, email, password_hash, created_at, updated_at) 
        VALUES ($1, 'habit-test@kanso.app', 'hash', $2, $2)`, userID, now)
	require.NoError(t, err, "Failed to create user fixture")

	reminder := "08:00"
	habitID := uuid.New().String()

	newHabit := &domain.Habit{
		ID:            habitID,
		UserID:        userID,
		Title:         "Test Integration Habit",
		Description:   "Checking if SQL works",
		Color:         "#FFFFFF",
		Icon:          "dumbbell",
		SortOrder:     1,
		Type:          "boolean",
		FrequencyType: "weekly",
		Weekdays:      []int{1, 3, 5},
		ReminderTime:  &reminder,
		Interval:      1,
		TargetValue:   1,
		Unit:          "times",
		StartDate:     now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	t.Run("Create Habit", func(t *testing.T) {
		err := repo.Create(ctx, newHabit)
		assert.NoError(t, err, "Create non dovrebbe fallire")
	})

	t.Run("Get By ID", func(t *testing.T) {
		fetched, err := repo.GetByID(ctx, habitID)
		assert.NoError(t, err)
		assert.NotNil(t, fetched)
		assert.Equal(t, newHabit.ID, fetched.ID)
		assert.Equal(t, 1, fetched.Version, "La versione deve partire da 1")
		assert.Nil(t, fetched.DeletedAt, "Non deve essere cancellato")
	})

	t.Run("Update Habit", func(t *testing.T) {
		oldUpdatedAt := newHabit.UpdatedAt

		newHabit.Title = "Updated Title 60k"
		newHabit.Weekdays = []int{6, 7}

		time.Sleep(100 * time.Millisecond)

		err := repo.Update(ctx, newHabit)
		assert.NoError(t, err)

		updated, err := repo.GetByID(ctx, habitID)
		assert.NoError(t, err)

		assert.Equal(t, "Updated Title 60k", updated.Title)
		assert.True(t, updated.UpdatedAt.After(oldUpdatedAt), "Updated_at non è avanzato: Old=%v, New=%v", oldUpdatedAt, updated.UpdatedAt)
		assert.Equal(t, 2, updated.Version)
	})

	t.Run("List By UserID", func(t *testing.T) {
		list, err := repo.ListByUserID(ctx, userID)
		assert.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, habitID, list[0].ID)
	})

	t.Run("Delete Habit (Soft Delete Check)", func(t *testing.T) {
		err := repo.Delete(ctx, habitID)
		assert.NoError(t, err)

		_, err = repo.GetByID(ctx, habitID)
		assert.Error(t, err)
		assert.Equal(t, domain.ErrHabitNotFound, err)

		var count int
		err = db.QueryRow("SELECT count(*) FROM habits WHERE id=$1 AND deleted_at IS NOT NULL", habitID).Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count, "Il record deve esistere fisicamente nel DB (Soft Delete)")
	})

	t.Run("Handle Null Fields", func(t *testing.T) {
		nullHabitID := uuid.New().String()
		nullHabit := &domain.Habit{
			ID: nullHabitID, UserID: userID, Title: "Null Tester", Type: "boolean", FrequencyType: "daily", StartDate: now, Interval: 1, TargetValue: 1,
		}

		err := repo.Create(ctx, nullHabit)
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, nullHabitID)
		assert.NoError(t, err)
		assert.Nil(t, fetched.ReminderTime)
	})

	t.Run("Update/Delete Non-Existent ID", func(t *testing.T) {
		randomID := uuid.New().String()
		dummyHabit := &domain.Habit{ID: randomID, UserID: userID, Title: "Ghost", Weekdays: []int{1}, Version: 1}

		err := repo.Update(ctx, dummyHabit)
		assert.Error(t, err)
		assert.Equal(t, domain.ErrHabitNotFound, err)

		err = repo.Delete(ctx, randomID)
		assert.Error(t, err)
		assert.Equal(t, domain.ErrHabitNotFound, err)
	})

	t.Run("Constraint Violation", func(t *testing.T) {
		badHabit := &domain.Habit{
			ID: uuid.New().String(), UserID: userID, Title: "Bad Interval", Type: "boolean", FrequencyType: "daily", StartDate: now, Interval: -5,
		}
		err := repo.Create(ctx, badHabit)
		assert.Error(t, err)
	})

	t.Run("Optimistic Locking: Prevent Overwrite", func(t *testing.T) {
		conflictID := uuid.New().String()
		h := &domain.Habit{ID: conflictID, UserID: userID, Title: "Conflict Base", Type: "boolean", FrequencyType: "daily", Interval: 1, TargetValue: 1, StartDate: now, CreatedAt: now, UpdatedAt: now}
		require.NoError(t, repo.Create(ctx, h))

		deviceACopy, err := repo.GetByID(ctx, conflictID)
		require.NoError(t, err)

		deviceBCopy, err := repo.GetByID(ctx, conflictID)
		require.NoError(t, err)

		deviceBCopy.Title = "B wins"
		err = repo.Update(ctx, deviceBCopy)
		require.NoError(t, err)

		deviceACopy.Title = "A loses"
		err = repo.Update(ctx, deviceACopy)

		assert.Error(t, err)
		assert.Equal(t, domain.ErrHabitConflict, err, "Atteso ErrHabitConflict, ricevuto: %v", err)
	})

	t.Run("GetChanges (Delta Sync)", func(t *testing.T) {
		syncUser := "sync-user-final"
		_, err = db.Exec(`INSERT INTO users (id, email, password_hash, created_at, updated_at) 
            VALUES ($1, 'sync-habit@kanso.app', 'hash', $2, $2)`, syncUser, now)
		require.NoError(t, err)

		h1 := &domain.Habit{ID: uuid.New().String(), UserID: syncUser, Title: "H1", Type: "boolean", FrequencyType: "daily", Interval: 1, TargetValue: 1, StartDate: now}
		h2 := &domain.Habit{ID: uuid.New().String(), UserID: syncUser, Title: "H2", Type: "boolean", FrequencyType: "daily", Interval: 1, TargetValue: 1, StartDate: now}

		require.NoError(t, repo.Create(ctx, h1))
		require.NoError(t, repo.Create(ctx, h2))

		time.Sleep(50 * time.Millisecond)

		var lastSync time.Time
		err := db.QueryRow("SELECT NOW()").Scan(&lastSync)
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		h1.Title = "H1 Changed"
		repo.Update(ctx, h1)

		repo.Delete(ctx, h2.ID)

		changes, err := repo.GetChanges(ctx, syncUser, lastSync)
		assert.NoError(t, err)

		assert.Len(t, changes, 2)
	})
}
