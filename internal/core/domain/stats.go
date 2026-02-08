package domain

import "time"

type WeeklyStats struct {
	StartDate   string      `json:"start_date"`
	EndDate     string      `json:"end_date"`
	TotalHabits int         `json:"total_habits"`
	OverallRate float64     `json:"overall_completion_rate"`
	HabitStats  []HabitStat `json:"habits"`
}

type HabitStat struct {
	HabitID        string  `json:"habit_id"`
	HabitTitle     string  `json:"habit_title"`
	Color          string  `json:"color"`
	Icon           string  `json:"icon"`
	TargetValue    int     `json:"target_value"`
	Unit           string  `json:"unit"`
	TotalValue     int     `json:"total_value"`
	CompletionRate float64 `json:"completion_rate"`
	DaysCompleted  int     `json:"days_completed"`
	DailyProgress  []int   `json:"daily_progress"`
}

type StatsInput struct {
	UserID    string
	StartDate time.Time
	EndDate   time.Time
	Location  *time.Location
}
