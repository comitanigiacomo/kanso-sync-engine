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
		dbUser = "kanso_user" // Default pubblico del docker-compose
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
	require.NoError(t, err, "Impossibile connettersi al DB di test. Controlla che Docker sia su e le variabili d'ambiente siano corrette.")

	return db
}

func cleanup(t *testing.T, db *sqlx.DB) {
	_, err := db.Exec("TRUNCATE TABLE habits")
	require.NoError(t, err, "Impossibile pulire il DB")
}

func TestPostgresHabitRepository_Integration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cleanup(t, db)
	defer cleanup(t, db)

	repo := NewPostgresHabitRepository(db)
	ctx := context.Background()

	habitID := uuid.New().String()
	userID := "test-user-senior-1"
	now := time.Now().Truncate(time.Microsecond)
	reminder := "08:00"

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
		assert.Equal(t, newHabit.Title, fetched.Title)

		assert.Equal(t, newHabit.Weekdays, fetched.Weekdays, "Il JSONB dei giorni deve corrispondere")

		assert.NotNil(t, fetched.ReminderTime)
		assert.Equal(t, *newHabit.ReminderTime, *fetched.ReminderTime)
	})

	t.Run("Update Habit", func(t *testing.T) {
		newHabit.Title = "Updated Title 60k"
		newHabit.Weekdays = []int{6, 7}

		time.Sleep(100 * time.Millisecond)

		err := repo.Update(ctx, newHabit)
		assert.NoError(t, err)

		updated, err := repo.GetByID(ctx, habitID)
		assert.NoError(t, err)

		assert.Equal(t, "Updated Title 60k", updated.Title)
		assert.Equal(t, []int{6, 7}, updated.Weekdays)

		assert.True(t, updated.UpdatedAt.After(newHabit.UpdatedAt), "Il trigger SQL dovrebbe aver aggiornato updated_at")
	})

	t.Run("List By UserID", func(t *testing.T) {
		list, err := repo.ListByUserID(ctx, userID)
		assert.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, habitID, list[0].ID)
	})

	t.Run("Delete Habit", func(t *testing.T) {
		err := repo.Delete(ctx, habitID)
		assert.NoError(t, err)

		_, err = repo.GetByID(ctx, habitID)
		assert.Error(t, err)
		assert.Equal(t, domain.ErrHabitNotFound, err)
	})

	t.Run("Handle Null Fields", func(t *testing.T) {
		nullHabitID := uuid.New().String()
		nullHabit := &domain.Habit{
			ID:            nullHabitID,
			UserID:        userID,
			Title:         "Null Tester",
			Type:          "boolean",
			FrequencyType: "daily",
			StartDate:     now,

			Interval:    1,
			TargetValue: 1,
		}

		err := repo.Create(ctx, nullHabit)
		assert.NoError(t, err, "Create non deve fallire se Interval/Target sono validi")

		fetched, err := repo.GetByID(ctx, nullHabitID)
		assert.NoError(t, err)

		if assert.NotNil(t, fetched) {
			assert.Nil(t, fetched.ReminderTime)
			assert.Nil(t, fetched.EndDate)
			assert.Nil(t, fetched.ArchivedAt)

			assert.Empty(t, fetched.Weekdays)
		}
	})

	t.Run("Update/Delete Non-Existent ID", func(t *testing.T) {
		randomID := uuid.New().String()

		dummyHabit := &domain.Habit{ID: randomID, Title: "Ghost", Weekdays: []int{1}}
		err := repo.Update(ctx, dummyHabit)
		assert.Error(t, err)
		assert.Equal(t, domain.ErrHabitNotFound, err, "Update su ID inesistente deve tornare ErrHabitNotFound")

		err = repo.Delete(ctx, randomID)
		assert.Error(t, err)
		assert.Equal(t, domain.ErrHabitNotFound, err, "Delete su ID inesistente deve tornare ErrHabitNotFound")
	})

	t.Run("Constraint Violation", func(t *testing.T) {
		badHabit := &domain.Habit{
			ID:            uuid.New().String(),
			UserID:        userID,
			Title:         "Bad Interval",
			Type:          "boolean",
			FrequencyType: "daily",
			StartDate:     now,
			Interval:      -5,
		}

		err := repo.Create(ctx, badHabit)
		assert.Error(t, err)
	})
}
