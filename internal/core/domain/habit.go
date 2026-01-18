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
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description,omitempty"`
	Color         string     `json:"color"`
	Icon          string     `json:"icon"`
	SortOrder     int        `json:"sort_order"`
	Type          string     `json:"type"`
	ReminderTime  *string    `json:"reminder_time,omitempty"`
	FrequencyType string     `json:"frequency_type"`
	Weekdays      []int      `json:"weekdays,omitempty"`
	Interval      int        `json:"interval,omitempty"`
	TargetValue   int        `json:"target_value"`
	Unit          string     `json:"unit"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ArchivedAt    *time.Time `json:"archived_at,omitempty"`
	StartDate     time.Time  `json:"start_date"`
	EndDate       *time.Time `json:"end_date,omitempty"`
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

func validateAndNormalize(title, desc, color, hType, reminder string, target, interval int, weekdays []int) (string, int, int, error) {
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return "", 0, 0, ErrHabitTitleEmpty
	}
	if len(trimmedTitle) > MaxTitleLen {
		return "", 0, 0, ErrHabitTitleTooLong
	}

	if len(strings.TrimSpace(desc)) > MaxDescLen {
		return "", 0, 0, ErrHabitDescTooLong
	}

	finalTarget := target
	if hType == HabitTypeBoolean {
		finalTarget = 1
	} else if target < 0 {
		return "", 0, 0, ErrInvalidTarget
	}

	switch hType {
	case HabitTypeBoolean, HabitTypeNumeric, HabitTypeTimer:
	default:
		return "", 0, 0, ErrInvalidHabitType
	}

	if reminder != "" && !reminderRegex.MatchString(reminder) {
		return "", 0, 0, ErrInvalidReminder
	}

	if interval < 0 {
		return "", 0, 0, ErrInvalidInterval
	}

	for _, day := range weekdays {
		if day < 0 || day > 6 {
			return "", 0, 0, ErrInvalidWeekdays
		}
	}

	if color != "" && !colorRegex.MatchString(color) {
		return "", 0, 0, ErrInvalidColor
	}

	freqType := HabitFreqDaily
	if len(weekdays) > 0 {
		freqType = HabitFreqSpecificDays
	} else if interval > 1 {
		freqType = HabitFreqInterval
	}

	safeInterval := interval
	if safeInterval < 1 {
		safeInterval = 1
	}

	return freqType, safeInterval, finalTarget, nil
}

func NewHabit(userID, title, description, color, icon, hType, reminder, unit string, target, interval int, weekdays []int) (*Habit, error) {
	if userID == "" {
		return nil, ErrHabitInvalidUserID
	}

	cleanDesc := strings.TrimSpace(description)

	freqType, safeInterval, safeTarget, err := validateAndNormalize(title, cleanDesc, color, hType, reminder, target, interval, weekdays)
	if err != nil {
		return nil, err
	}

	if icon == "" {
		icon = DefaultIcon
	}

	now := time.Now().UTC()

	var remPtr *string
	if reminder != "" {
		remPtr = &reminder
	}

	safeWeekdays := normalizeWeekdays(weekdays)

	return &Habit{
		ID:            uuid.New().String(),
		UserID:        userID,
		Title:         strings.TrimSpace(title),
		Description:   cleanDesc,
		Color:         color,
		Icon:          icon,
		Type:          hType,
		ReminderTime:  remPtr,
		Unit:          unit,
		TargetValue:   safeTarget,
		Weekdays:      safeWeekdays,
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

	cleanDesc := strings.TrimSpace(description)

	freqType, safeInterval, safeTarget, err := validateAndNormalize(title, cleanDesc, color, hType, reminder, target, interval, weekdays)
	if err != nil {
		return err
	}

	if icon == "" {
		icon = DefaultIcon
	}

	var remPtr *string
	if reminder != "" {
		remPtr = &reminder
	} else {
		remPtr = nil
	}

	safeWeekdays := normalizeWeekdays(weekdays)

	h.Title = strings.TrimSpace(title)
	h.Description = cleanDesc
	h.Color = color
	h.Icon = icon
	h.Type = hType
	h.ReminderTime = remPtr
	h.Unit = unit
	h.TargetValue = safeTarget
	h.Weekdays = safeWeekdays
	h.Interval = safeInterval
	h.FrequencyType = freqType

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
