package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

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
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11,
			$12, $13, $14,
			$15, $16, $17,
			$18, $19
		)`

	_, err = r.db.ExecContext(ctx, query,
		h.ID, h.UserID, h.Title, h.Description, h.Color, h.Icon, h.SortOrder,
		h.Type, h.FrequencyType, weekdaysJSON, h.ReminderTime, // Pointers are handled automatically by driver
		h.Interval, h.TargetValue, h.Unit,
		h.StartDate, h.EndDate, h.ArchivedAt,
		h.CreatedAt, h.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert habit: %w", err)
	}

	return nil
}

func (r *PostgresHabitRepository) GetByID(ctx context.Context, id string) (*domain.Habit, error) {
	var h domain.Habit
	var weekdaysJSON []byte
	query := `
		SELECT 
			id, user_id, title, description, color, icon, sort_order,
			type, frequency_type, weekdays, reminder_time,
			interval, target_value, unit,
			start_date, end_date, archived_at,
			created_at, updated_at
		FROM habits 
		WHERE id = $1`

	row := r.db.QueryRowContext(ctx, query, id)

	err := row.Scan(
		&h.ID, &h.UserID, &h.Title, &h.Description, &h.Color, &h.Icon, &h.SortOrder,
		&h.Type, &h.FrequencyType, &weekdaysJSON, &h.ReminderTime,
		&h.Interval, &h.TargetValue, &h.Unit,
		&h.StartDate, &h.EndDate, &h.ArchivedAt,
		&h.CreatedAt, &h.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrHabitNotFound
		}
		return nil, fmt.Errorf("database scan error: %w", err)
	}

	if len(weekdaysJSON) > 0 {
		if err := json.Unmarshal(weekdaysJSON, &h.Weekdays); err != nil {
			return nil, fmt.Errorf("data corruption: failed to unmarshal weekdays: %w", err)
		}
	}

	return &h, nil
}

func (r *PostgresHabitRepository) ListByUserID(ctx context.Context, userID string) ([]*domain.Habit, error) {
	query := `
		SELECT 
			id, user_id, title, description, color, icon, sort_order,
			type, frequency_type, weekdays, reminder_time,
			interval, target_value, unit,
			start_date, end_date, archived_at,
			created_at, updated_at
		FROM habits 
		WHERE user_id = $1 
		ORDER BY sort_order ASC, created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	var habits []*domain.Habit

	for rows.Next() {
		var h domain.Habit
		var weekdaysJSON []byte

		err := rows.Scan(
			&h.ID, &h.UserID, &h.Title, &h.Description, &h.Color, &h.Icon, &h.SortOrder,
			&h.Type, &h.FrequencyType, &weekdaysJSON, &h.ReminderTime,
			&h.Interval, &h.TargetValue, &h.Unit,
			&h.StartDate, &h.EndDate, &h.ArchivedAt,
			&h.CreatedAt, &h.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("row scan error: %w", err)
		}

		if len(weekdaysJSON) > 0 {
			_ = json.Unmarshal(weekdaysJSON, &h.Weekdays)
		}

		habits = append(habits, &h)
	}

	if err := rows.Err(); err != nil {
		return nil, err
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
			end_date=$13, archived_at=$14
		WHERE id=$15`

	res, err := r.db.ExecContext(ctx, query,
		h.Title, h.Description, h.Color, h.Icon, h.SortOrder,
		h.Type, h.FrequencyType, weekdaysJSON, h.ReminderTime,
		h.Interval, h.TargetValue, h.Unit,
		h.EndDate, h.ArchivedAt,
		h.ID,
	)

	if err != nil {
		return fmt.Errorf("update query failed: %w", err)
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

func (r *PostgresHabitRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM habits WHERE id = $1`

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
