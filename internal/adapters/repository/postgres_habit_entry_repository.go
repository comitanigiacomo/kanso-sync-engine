package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
)

type PostgresEntryRepository struct {
	db *sqlx.DB
}

func NewPostgresEntryRepository(db *sqlx.DB) *PostgresEntryRepository {
	return &PostgresEntryRepository{db: db}
}

func (r *PostgresEntryRepository) Create(ctx context.Context, entry *domain.HabitEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}

	query := `
		INSERT INTO habit_entries (
			id, habit_id, user_id, 
			completion_date, value, notes, 
			version, created_at, updated_at, deleted_at
		) VALUES (
			:id, :habit_id, :user_id, 
			:completion_date, :value, :notes, 
			:version, :created_at, :updated_at, :deleted_at
		)`

	_, err := r.db.NamedExecContext(ctx, query, entry)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23503" {
				return errors.New("referenced habit or user does not exist")
			}
			if pqErr.Code == "23505" {
				return domain.ErrEntryConflict
			}
		}
		return err
	}
	return nil
}

func (r *PostgresEntryRepository) GetByID(ctx context.Context, id string) (*domain.HabitEntry, error) {
	var entry domain.HabitEntry
	query := `SELECT * FROM habit_entries WHERE id = $1 AND deleted_at IS NULL`

	err := r.db.GetContext(ctx, &entry, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrEntryNotFound
		}
		return nil, err
	}
	return &entry, nil
}

func (r *PostgresEntryRepository) ListByHabitID(ctx context.Context, habitID string, from, to time.Time) ([]*domain.HabitEntry, error) {
	entries := []*domain.HabitEntry{}

	query := `
		SELECT * FROM habit_entries 
		WHERE habit_id = $1 
		  AND completion_date >= $2 
		  AND completion_date <= $3
		  AND deleted_at IS NULL
		ORDER BY completion_date DESC`

	err := r.db.SelectContext(ctx, &entries, query, habitID, from, to)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *PostgresEntryRepository) Update(ctx context.Context, entry *domain.HabitEntry) error {
	entry.Version++
	entry.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE habit_entries 
		SET value = :value,
		    notes = :notes,
		    completion_date = :completion_date,
		    version = :version,
		    updated_at = :updated_at
		WHERE id = :id 
		  AND version = :version - 1  -- Optimistic Lock check
		  AND deleted_at IS NULL`

	result, err := r.db.NamedExecContext(ctx, query, entry)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		exists, _ := r.exists(ctx, entry.ID)
		if !exists {
			return domain.ErrEntryNotFound
		}
		return domain.ErrEntryConflict
	}

	return nil
}

func (r *PostgresEntryRepository) Delete(ctx context.Context, id string, userID string) error {
	now := time.Now().UTC()

	query := `
		UPDATE habit_entries 
		SET deleted_at = $1,
		    updated_at = $1,
		    version = version + 1
		WHERE id = $2 
		  AND user_id = $3 -- Security Check
		  AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, now, id, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrEntryNotFound
	}

	return nil
}

func (r *PostgresEntryRepository) GetChanges(ctx context.Context, userID string, since time.Time) ([]*domain.HabitEntry, error) {
	entries := []*domain.HabitEntry{}

	query := `
		SELECT * FROM habit_entries 
		WHERE user_id = $1 
		  AND updated_at > $2
		ORDER BY updated_at ASC`

	err := r.db.SelectContext(ctx, &entries, query, userID, since)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *PostgresEntryRepository) exists(ctx context.Context, id string) (bool, error) {
	var count int
	err := r.db.GetContext(ctx, &count, "SELECT count(*) FROM habit_entries WHERE id = $1", id)
	return count > 0, err
}
