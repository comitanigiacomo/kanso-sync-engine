package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrHabitTitleEmpty    = errors.New("habit title cannot be empty")
	ErrHabitInvalidUserID = errors.New("invalid user id")
	ErrInvalidColor       = errors.New("invalid color format (must be #RRGGBB)")
	ErrInvalidWeekdays    = errors.New("invalid weekdays (must be 0-6)")
	ErrInvalidTarget      = errors.New("target cannot be negative")
	ErrInvalidInterval    = errors.New("interval cannot be negative")
)

var colorRegex = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)

type Habit struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description,omitempty"`
	Color         string     `json:"color"`
	Icon          string     `json:"icon"`
	SortOrder     int        `json:"sort_order"`              // to reorder habit's visual position
	ReminderTime  *string    `json:"reminder_time,omitempty"` // at what time the user will be sent a notification
	FrequencyType string     `json:"frequency_type"`          // "daily", "specific_days", "interval"
	Weekdays      []int      `json:"weekdays,omitempty"`      // sunday, monday
	Interval      int        `json:"interval,omitempty"`      // once a week, twice a week
	TargetValue   int        `json:"target_value"`            // 200ml of water, 1kg of beef
	Unit          string     `json:"unit"`                    // ml, kg
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ArchivedAt    *time.Time `json:"archived_at,omitempty"`
	StartDate     time.Time  `json:"start_date"`
	EndDate       *time.Time `json:"end_date,omitempty"`
}

func validateAndNormalize(title, color string, target, interval int, weekdays []int) (string, int, error) {
	if strings.TrimSpace(title) == "" {
		return "", 0, ErrHabitTitleEmpty
	}

	if target < 0 {
		return "", 0, ErrInvalidTarget
	}

	if interval < 0 {
		return "", 0, ErrInvalidInterval
	}

	for _, day := range weekdays {
		if day < 0 || day > 6 {
			return "", 0, ErrInvalidWeekdays
		}
	}

	if color != "" && !colorRegex.MatchString(color) {
		return "", 0, ErrInvalidColor
	}

	freqType := "daily"
	if len(weekdays) > 0 {
		freqType = "specific_days"
	} else if interval > 1 {
		freqType = "interval"
	}

	safeInterval := interval
	if safeInterval < 1 {
		safeInterval = 1
	}

	return freqType, safeInterval, nil
}

func NewHabit(userID, title, description, color, icon, unit string, target, interval int, weekdays []int) (*Habit, error) {
	if userID == "" {
		return nil, ErrHabitInvalidUserID
	}

	freqType, safeInterval, err := validateAndNormalize(title, color, target, interval, weekdays)
	if err != nil {
		return nil, err
	}

	if icon == "" {
		icon = "default_icon"
	}

	now := time.Now().UTC()

	return &Habit{
		ID:            uuid.New().String(),
		UserID:        userID,
		Title:         strings.TrimSpace(title),
		Description:   description,
		Color:         color,
		Icon:          icon,
		Unit:          unit,
		TargetValue:   target,
		Weekdays:      weekdays,
		Interval:      safeInterval,
		FrequencyType: freqType,
		SortOrder:     0,
		CreatedAt:     now,
		UpdatedAt:     now,
		StartDate:     now,
	}, nil
}
