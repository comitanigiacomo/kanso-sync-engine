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
	ErrHabitArchived      = errors.New("cannot update an archived habit")
	ErrInvalidHabitType   = errors.New("invalid habit type (must be boolean, numeric, or timer)")
	ErrInvalidReminder    = errors.New("invalid reminder format (must be HH:MM 24h)")
)

var colorRegex = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)
var reminderRegex = regexp.MustCompile(`^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$`)

const (
	HabitTypeBoolean = "boolean"
	HabitTypeNumeric = "numeric"
	HabitTypeTimer   = "timer"
)

type Habit struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description,omitempty"`
	Color         string     `json:"color"`
	Icon          string     `json:"icon"`
	SortOrder     int        `json:"sort_order"`              // to reorder habit's visual position
	Type          string     `json:"type"`                    // boolean, numeric, type
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

func validateAndNormalize(title, color, hType, reminder string, target, interval int, weekdays []int) (string, int, error) {
	if strings.TrimSpace(title) == "" {
		return "", 0, ErrHabitTitleEmpty
	}

	switch hType {
	case HabitTypeBoolean, HabitTypeNumeric, HabitTypeTimer:
	default:
		return "", 0, ErrInvalidHabitType
	}

	if reminder != "" && !reminderRegex.MatchString(reminder) {
		return "", 0, ErrInvalidReminder
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

func NewHabit(userID, title, description, color, icon, hType, reminder, unit string, target, interval int, weekdays []int) (*Habit, error) {
	if userID == "" {
		return nil, ErrHabitInvalidUserID
	}

	freqType, safeInterval, err := validateAndNormalize(title, color, hType, reminder, target, interval, weekdays)
	if err != nil {
		return nil, err
	}

	if icon == "" {
		icon = "default_icon"
	}

	now := time.Now().UTC()

	var remPtr *string
	if reminder != "" {
		remPtr = &reminder
	}

	return &Habit{
		ID:            uuid.New().String(),
		UserID:        userID,
		Title:         strings.TrimSpace(title),
		Description:   description,
		Color:         color,
		Icon:          icon,
		Type:          hType,
		ReminderTime:  remPtr,
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

func (h *Habit) Update(title, description, color, icon, hType, reminder, unit string, target, interval int, weekdays []int) error {

	if h.ArchivedAt != nil {
		return ErrHabitArchived
	}

	freqType, safeInterval, err := validateAndNormalize(title, color, hType, reminder, target, interval, weekdays)
	if err != nil {
		return err
	}

	if icon == "" {
		icon = "default_icon"
	}

	var remPtr *string
	if reminder != "" {
		remPtr = &reminder
	} else {
		remPtr = nil
	}

	h.Title = strings.TrimSpace(title)
	h.Description = description
	h.Color = color
	h.Icon = icon
	h.Type = hType
	h.ReminderTime = remPtr
	h.Unit = unit
	h.TargetValue = target
	h.Weekdays = weekdays
	h.Interval = safeInterval
	h.FrequencyType = freqType

	h.UpdatedAt = time.Now().UTC()

	return nil
}

func (h *Habit) Archive() {
	if h.ArchivedAt != nil {
		return
	}

	now := time.Now().UTC()
	h.ArchivedAt = &now
	h.UpdatedAt = now
}

func (h *Habit) Restore() {
	if h.ArchivedAt == nil {
		return
	}
	h.ArchivedAt = nil
	h.UpdatedAt = time.Now().UTC()
}
