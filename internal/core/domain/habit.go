package domain

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrHabitTitleEmpty    = errors.New("habit title cannot be empty")
	ErrHabitTitleTooLong  = errors.New("habit title is too long (max 100 chars)")
	ErrHabitDescTooLong   = errors.New("habit description is too long (max 500 chars)")
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
var reminderRegex = regexp.MustCompile(`^([0-1][0-9]|2[0-3]):[0-5][0-9]$`)

const (
	HabitTypeBoolean      = "boolean"
	HabitTypeNumeric      = "numeric"
	HabitTypeTimer        = "timer"
	HabitFreqDaily        = "daily"
	HabitFreqSpecificDays = "specific_days"
	HabitFreqInterval     = "interval"
	DefaultIcon           = "default_icon"
	MaxTitleLen           = 100
	MaxDescLen            = 500
)

type Habit struct {
	ID     string `json:"id" db:"id"`
	UserID string `json:"user_id" db:"user_id"`

	Title       string `json:"title" db:"title"`
	Description string `json:"description" db:"description"`
	Color       string `json:"color" db:"color"`
	Icon        string `json:"icon" db:"icon"`
	SortOrder   int    `json:"sort_order" db:"sort_order"`

	Type          string `json:"type" db:"type"`
	FrequencyType string `json:"frequency_type" db:"frequency_type"`

	Weekdays []int `json:"weekdays,omitempty" db:"weekdays"`

	ReminderTime *string `json:"reminder_time,omitempty" db:"reminder_time"`
	Interval     int     `json:"interval,omitempty" db:"interval"`
	TargetValue  int     `json:"target_value" db:"target_value"`
	Unit         string  `json:"unit" db:"unit"`

	StartDate  time.Time  `json:"start_date" db:"start_date"`
	EndDate    *time.Time `json:"end_date,omitempty" db:"end_date"`
	ArchivedAt *time.Time `json:"archived_at,omitempty" db:"archived_at"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type habitData struct {
	Title         string
	Description   string
	Color         string
	Type          string
	ReminderTime  *string
	Unit          string
	TargetValue   int
	Interval      int
	Weekdays      []int
	FrequencyType string
}

func normalizeWeekdays(days []int) []int {
	if len(days) == 0 {
		return nil
	}
	uniqueMap := make(map[int]bool)
	var uniqueDays []int
	for _, d := range days {
		if !uniqueMap[d] {
			uniqueMap[d] = true
			uniqueDays = append(uniqueDays, d)
		}
	}
	sort.Ints(uniqueDays)
	return uniqueDays
}

func prepareHabitData(title, desc, color, hType, reminder, unit string, target, interval int, weekdays []int) (*habitData, error) {
	trimmedTitle := strings.TrimSpace(title)
	cleanDesc := strings.TrimSpace(desc)

	if trimmedTitle == "" {
		return nil, ErrHabitTitleEmpty
	}
	if len(trimmedTitle) > MaxTitleLen {
		return nil, ErrHabitTitleTooLong
	}
	if len(cleanDesc) > MaxDescLen {
		return nil, ErrHabitDescTooLong
	}
	if color != "" && !colorRegex.MatchString(color) {
		return nil, ErrInvalidColor
	}
	if reminder != "" && !reminderRegex.MatchString(reminder) {
		return nil, ErrInvalidReminder
	}
	if interval < 0 {
		return nil, ErrInvalidInterval
	}
	for _, day := range weekdays {
		if day < 0 || day > 6 {
			return nil, ErrInvalidWeekdays
		}
	}

	finalTarget := target
	switch hType {
	case HabitTypeBoolean:
		finalTarget = 1
	case HabitTypeNumeric, HabitTypeTimer:
		if target < 0 {
			return nil, ErrInvalidTarget
		}
	default:
		return nil, ErrInvalidHabitType
	}

	safeWeekdays := normalizeWeekdays(weekdays)

	freqType := HabitFreqDaily
	if len(safeWeekdays) > 0 {
		freqType = HabitFreqSpecificDays
	} else if interval > 1 {
		freqType = HabitFreqInterval
	}

	safeInterval := interval
	if safeInterval < 1 {
		safeInterval = 1
	}

	var remPtr *string
	if reminder != "" {
		remPtr = &reminder
	}

	return &habitData{
		Title:         trimmedTitle,
		Description:   cleanDesc,
		Color:         color,
		Type:          hType,
		ReminderTime:  remPtr,
		Unit:          unit,
		TargetValue:   finalTarget,
		Interval:      safeInterval,
		Weekdays:      safeWeekdays,
		FrequencyType: freqType,
	}, nil
}

func (h *Habit) applyChanges(data *habitData, iconInput string) {
	h.Title = data.Title
	h.Description = data.Description
	h.Color = data.Color
	h.Type = data.Type
	h.ReminderTime = data.ReminderTime
	h.Unit = data.Unit
	h.TargetValue = data.TargetValue
	h.Interval = data.Interval
	h.Weekdays = data.Weekdays
	h.FrequencyType = data.FrequencyType

	if iconInput == "" {
		h.Icon = DefaultIcon
	} else {
		h.Icon = iconInput
	}
}

func NewHabit(title, userID string) (*Habit, error) {
	if userID == "" {
		return nil, ErrHabitInvalidUserID
	}

	data, err := prepareHabitData(title, "", "", HabitTypeBoolean, "", "", 1, 1, nil)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	h := &Habit{
		ID:        uuid.New().String(),
		UserID:    userID,
		SortOrder: 0,
		CreatedAt: now,
		UpdatedAt: now,
		StartDate: now,
	}

	h.applyChanges(data, "")

	return h, nil
}

func (h *Habit) Update(title, description, color, icon, hType, reminder, unit string, target, interval int, weekdays []int) error {
	if h.ArchivedAt != nil {
		return ErrHabitArchived
	}

	data, err := prepareHabitData(title, description, color, hType, reminder, unit, target, interval, weekdays)
	if err != nil {
		return err
	}

	h.applyChanges(data, icon)
	h.UpdatedAt = time.Now().UTC()

	return nil
}

func (h *Habit) ChangePosition(newOrder int) error {
	if h.ArchivedAt != nil {
		return ErrHabitArchived
	}

	h.SortOrder = newOrder
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
