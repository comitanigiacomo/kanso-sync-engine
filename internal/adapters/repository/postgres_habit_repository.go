package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresHabitRepository struct {
	db *sqlx.DB
}

func NewPostgresHabitRepository(db *sqlx.DB) *PostgresHabitRepository {
	return &PostgresHabitRepository{db: db}
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func (r *PostgresHabitRepository) scanRow(row scannable) (*domain.Habit, error) {
	var h domain.Habit
	var weekdaysJSON []byte

	err := row.Scan(
		&h.ID, &h.UserID, &h.Title, &h.Description, &h.Color, &h.Icon, &h.SortOrder,
		&h.Type, &h.FrequencyType, &weekdaysJSON, &h.ReminderTime,
		&h.Interval, &h.TargetValue, &h.Unit,
		&h.StartDate, &h.EndDate, &h.ArchivedAt,
		&h.Version, &h.DeletedAt, &h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(weekdaysJSON) > 0 {
		if err := json.Unmarshal(weekdaysJSON, &h.Weekdays); err != nil {
			return nil, fmt.Errorf("failed to unmarshal weekdays: %w", err)
		}
	}

	return &h, nil
}

func (r *PostgresHabitRepository) Create(ctx context.Context, h *domain.Habit) error {
	weekdaysJSON, err := json.Marshal(h.Weekdays)
	if err != nil {
		return fmt.Errorf("failed to marshal weekdays: %w", err)
	}

	query := `
        INSERT INTO habits (
            id, user_id, title, description, color, icon, sort_order,
            type, frequency_type, weekdays, reminder_time,
            interval, target_value, unit,
            start_date, end_date, archived_at,
            version, deleted_at, created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7,
            $8, $9, $10, $11,
            $12, $13, $14,
            $15, $16, $17,
            1, NULL, $18, $19
        )`

	_, err = r.db.ExecContext(ctx, query,
		h.ID, h.UserID, h.Title, h.Description, h.Color, h.Icon, h.SortOrder,
		h.Type, h.FrequencyType, weekdaysJSON, h.ReminderTime,
		h.Interval, h.TargetValue, h.Unit,
		h.StartDate, h.EndDate, h.ArchivedAt,
		h.CreatedAt, h.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert habit: %w", err)
	}

	h.Version = 1
	return nil
}

func (r *PostgresHabitRepository) GetByID(ctx context.Context, id string) (*domain.Habit, error) {
	query := `SELECT * FROM habits WHERE id = $1 AND deleted_at IS NULL`

	row := r.db.QueryRowContext(ctx, query, id)

	h, err := r.scanRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrHabitNotFound
		}
		return nil, fmt.Errorf("database scan error: %w", err)
	}

	return h, nil
}

func (r *PostgresHabitRepository) ListByUserID(ctx context.Context, userID string) ([]*domain.Habit, error) {
	query := `
        SELECT * FROM habits 
        WHERE user_id = $1 AND deleted_at IS NULL 
        ORDER BY sort_order ASC, created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	var habits []*domain.Habit

	for rows.Next() {
		h, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("row scan error: %w", err)
		}
		habits = append(habits, h)
	}

	return habits, nil
}

func (r *PostgresHabitRepository) Update(ctx context.Context, h *domain.Habit) error {
	weekdaysJSON, err := json.Marshal(h.Weekdays)
	if err != nil {
		return err
	}

	query := `
        UPDATE habits SET 
            title=$1, description=$2, color=$3, icon=$4, sort_order=$5,
            type=$6, frequency_type=$7, weekdays=$8, reminder_time=$9,
            interval=$10, target_value=$11, unit=$12,
            end_date=$13, archived_at=$14,
            updated_at=NOW(), version = version + 1
        WHERE id=$15 AND version=$16 AND deleted_at IS NULL
        RETURNING version, updated_at`

	row := r.db.QueryRowContext(ctx, query,
		h.Title, h.Description, h.Color, h.Icon, h.SortOrder,
		h.Type, h.FrequencyType, weekdaysJSON, h.ReminderTime,
		h.Interval, h.TargetValue, h.Unit,
		h.EndDate, h.ArchivedAt,
		h.ID, h.Version,
	)

	var newVersion int
	var newUpdatedAt time.Time

	err = row.Scan(&newVersion, &newUpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			existsQuery := `SELECT count(*) FROM habits WHERE id = $1`
			var count int
			if checkErr := r.db.QueryRowContext(ctx, existsQuery, h.ID).Scan(&count); checkErr != nil {
				return fmt.Errorf("existence check failed: %w", checkErr)
			}

			if count == 0 {
				return domain.ErrHabitNotFound
			}
			return domain.ErrHabitConflict
		}
		return fmt.Errorf("update query failed: %w", err)
	}

	h.Version = newVersion
	h.UpdatedAt = newUpdatedAt

	return nil
}

func (r *PostgresHabitRepository) Delete(ctx context.Context, id string) error {
	query := `
        UPDATE habits 
        SET deleted_at = NOW(), updated_at = NOW(), version = version + 1
        WHERE id = $1 AND deleted_at IS NULL`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete query failed: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrHabitNotFound
	}

	return nil
}

func (r *PostgresHabitRepository) GetChanges(ctx context.Context, userID string, since time.Time) ([]*domain.Habit, error) {
	query := `
        SELECT * FROM habits 
        WHERE user_id = $1 AND updated_at > $2
        ORDER BY updated_at ASC`

	rows, err := r.db.QueryContext(ctx, query, userID, since)
	if err != nil {
		return nil, fmt.Errorf("sync query error: %w", err)
	}
	defer rows.Close()

	var habits []*domain.Habit

	for rows.Next() {
		h, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("sync row scan error: %w", err)
		}
		habits = append(habits, h)
	}

	return habits, nil
}
