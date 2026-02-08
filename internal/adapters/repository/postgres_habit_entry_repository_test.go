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
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T) (*PostgresEntryRepository, *sqlx.DB, func()) {
	t.Helper()

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		getEnv("DB_USER", "kanso_user"),
		getEnv("DB_PASSWORD", "secret"),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "kanso_db"),
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Skipf("Database connection failed (skipping integration tests): %v", err)
	}

	db.MustExec("TRUNCATE TABLE habit_entries, habits, users CASCADE")

	repo := NewPostgresEntryRepository(db)

	return repo, db, func() {
		db.Close()
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestPostgresEntryRepository_Integration(t *testing.T) {
	repo, db, teardown := setupTest(t)
	defer teardown()

	ctx := context.Background()
	uid := uuid.NewString()
	hid := uuid.NewString()

	now := time.Now().UTC().Truncate(time.Second)

	db.MustExec(`
        INSERT INTO users (id, email, password_hash, created_at, updated_at) 
        VALUES ($1, $2, 'dummy_hash_per_test', $3, $3)
    `, uid, "senior@test.com", now)

	db.MustExec(`INSERT INTO habits (id, user_id, title, type, frequency_type, start_date, created_at, updated_at) 
        VALUES ($1, $2, $3, $4, $5, $6, $6, $6)`, hid, uid, "Habit Test", "boolean", "daily", now)

	t.Run("Full CRUD Lifecycle & Soft Delete", func(t *testing.T) {
		entryID := uuid.NewString()
		entry := domain.NewHabitEntry(hid, uid, now, 100)
		entry.ID = entryID
		entry.Notes = "Original Note"

		err := repo.Create(ctx, entry)
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, entryID)
		require.NoError(t, err)
		assert.Equal(t, 100, fetched.Value)
		assert.Equal(t, "Original Note", fetched.Notes)
		assert.Equal(t, 1, fetched.Version)

		fetched.Value = 500
		fetched.Notes = "Updated Note"

		fetched.Version++
		fetched.UpdatedAt = time.Now().UTC()

		err = repo.Update(ctx, fetched)
		assert.NoError(t, err)

		updated, _ := repo.GetByID(ctx, entryID)
		assert.Equal(t, 2, updated.Version)
		assert.Equal(t, 500, updated.Value)

		err = repo.Delete(ctx, entryID, uid)
		assert.NoError(t, err)

		_, err = repo.GetByID(ctx, entryID)
		assert.ErrorIs(t, err, domain.ErrEntryNotFound)

		var exists bool
		err = db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM habit_entries WHERE id=$1 AND deleted_at IS NOT NULL)", entryID)
		assert.NoError(t, err)
		assert.True(t, exists, "Record must remain physically in DB with deleted_at for sync purposes")
	})

	t.Run("Optimistic Locking: Version Conflict", func(t *testing.T) {
		entryID := uuid.NewString()
		e := domain.NewHabitEntry(hid, uid, now, 10)
		e.ID = entryID
		repo.Create(ctx, e)

		clientA, _ := repo.GetByID(ctx, entryID)
		clientB, _ := repo.GetByID(ctx, entryID)

		clientA.Value = 20
		clientA.Version++
		clientA.UpdatedAt = time.Now().UTC()
		require.NoError(t, repo.Update(ctx, clientA))

		clientB.Value = 30
		clientB.Version++
		clientB.UpdatedAt = time.Now().UTC()

		err := repo.Update(ctx, clientB)

		assert.ErrorIs(t, err, domain.ErrEntryConflict, "Update must fail if base version on DB (2) != expected previous version (1)")
	})

	t.Run("List Methods: Worker vs API", func(t *testing.T) {
		localHid := uuid.NewString()
		db.MustExec(`INSERT INTO habits (id, user_id, title, type, frequency_type, start_date, created_at, updated_at) 
            VALUES ($1, $2, $3, $4, $5, $6, $6, $6)`, localHid, uid, "Isolated Habit", "boolean", "daily", now)

		testDates := []time.Time{
			now.AddDate(0, 0, -5),
			now.AddDate(0, 0, -2),
			now.AddDate(0, 0, 0),
		}
		for _, d := range testDates {
			e := domain.NewHabitEntry(localHid, uid, d, 1)
			e.ID = uuid.NewString()
			err := repo.Create(ctx, e)
			require.NoError(t, err)
		}

		from := now.AddDate(0, 0, -3)
		to := now.AddDate(0, 0, 1)

		apiList, err := repo.ListByHabitIDWithRange(ctx, localHid, from, to)
		assert.NoError(t, err)
		assert.Len(t, apiList, 2, "API should return filtered list (2 items)")

		workerList, err := repo.ListByHabitID(ctx, localHid)
		assert.NoError(t, err)
		assert.Len(t, workerList, 3, "Worker should return complete history (3 items)")
	})

	t.Run("Sync Engine: GetChanges Delta", func(t *testing.T) {
		checkpoint := time.Now().UTC()
		time.Sleep(10 * time.Millisecond)

		e := domain.NewHabitEntry(hid, uid, now, 888)
		e.ID = uuid.NewString()
		e.UpdatedAt = time.Now().UTC()
		repo.Create(ctx, e)

		changes, err := repo.GetChanges(ctx, uid, checkpoint)
		assert.NoError(t, err)

		require.GreaterOrEqual(t, len(changes), 1)
		found := false
		for _, c := range changes {
			if c.ID == e.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "GetChanges must return records created after the checkpoint")
	})
}

func TestPostgresEntryRepository_ListByUserIDAndDateRange(t *testing.T) {
	repo, db, teardown := setupTest(t)
	defer teardown()

	ctx := context.Background()

	userID := uuid.NewString()
	habitID := uuid.NewString()
	now := time.Now().UTC()

	db.MustExec(`INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES ($1, $2, 'stats', $3, $3)`, userID, "stats-user@test.com", now)
	db.MustExec(`INSERT INTO habits (id, user_id, title, type, frequency_type, start_date, created_at, updated_at) VALUES ($1, $2, 'Stats', 'numeric', 'daily', $3, $3, $3)`, habitID, userID, now)

	baseDate := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)

	entries := []domain.HabitEntry{
		{ID: uuid.NewString(), HabitID: habitID, UserID: userID, Value: 10, CompletionDate: baseDate},
		{ID: uuid.NewString(), HabitID: habitID, UserID: userID, Value: 20, CompletionDate: baseDate.Add(24 * time.Hour)},
		{ID: uuid.NewString(), HabitID: habitID, UserID: userID, Value: 5, CompletionDate: baseDate.Add(5 * 24 * time.Hour)},
	}

	for _, e := range entries {
		e.Version = 1
		e.CreatedAt = now
		e.UpdatedAt = now
		require.NoError(t, repo.Create(ctx, &e))
	}

	startDate := baseDate.Add(-1 * time.Hour)
	endDate := baseDate.Add(48 * time.Hour)

	results, err := repo.ListByUserIDAndDateRange(ctx, userID, startDate, endDate)

	require.NoError(t, err)
	assert.Len(t, results, 2, "Should return only entries within range")
}
