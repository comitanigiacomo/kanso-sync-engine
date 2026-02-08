package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http/middleware"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
	"github.com/gin-gonic/gin"
)

type HabitHandler struct {
	svc *services.HabitService
}

func NewHabitHandler(svc *services.HabitService) *HabitHandler {
	return &HabitHandler{
		svc: svc,
	}
}

type createHabitRequest struct {
	Title         string `json:"title" binding:"required"`
	Description   string `json:"description"`
	Color         string `json:"color"`
	Icon          string `json:"icon"`
	Type          string `json:"type"`
	ReminderTime  string `json:"reminder_time"`
	Unit          string `json:"unit"`
	TargetValue   int    `json:"target_value"`
	Interval      int    `json:"interval"`
	Weekdays      []int  `json:"weekdays"`
	FrequencyType string `json:"frequency_type"`
}

type updateHabitRequest struct {
	Title         *string `json:"title"`
	Description   *string `json:"description"`
	Color         *string `json:"color"`
	Icon          *string `json:"icon"`
	Type          *string `json:"type"`
	ReminderTime  *string `json:"reminder_time"`
	Unit          *string `json:"unit"`
	TargetValue   *int    `json:"target_value"`
	Interval      *int    `json:"interval"`
	Weekdays      []int   `json:"weekdays"`
	FrequencyType *string `json:"frequency_type"`
	Version       int     `json:"version" binding:"required"`
}

func (h *HabitHandler) RegisterRoutes(router *gin.RouterGroup) {
	habits := router.Group("/habits")
	{
		habits.POST("", h.Create)
		habits.GET("", h.List)
		habits.GET("/sync", h.Sync)
		habits.PUT("/:id", h.Update)
		habits.DELETE("/:id", h.Delete)
	}
}

// Create godoc
// @Summary      Create a new habit
// @Description  Create a habit with title, type, color, frequency, and tracking details
// @Tags         Habits
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        habit body createHabitRequest true "Habit Data"
// @Success      201  {object}  domain.Habit
// @Failure      400  {object}  map[string]string "Validation Error (Title empty, Invalid Color)"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Router       /habits [post]
func (h *HabitHandler) Create(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user context missing"})
		return
	}

	var req createHabitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := services.CreateHabitInput{
		UserID:        userID,
		Title:         req.Title,
		Description:   req.Description,
		Color:         req.Color,
		Icon:          req.Icon,
		Type:          req.Type,
		ReminderTime:  req.ReminderTime,
		Unit:          req.Unit,
		TargetValue:   req.TargetValue,
		Interval:      req.Interval,
		Weekdays:      req.Weekdays,
		FrequencyType: req.FrequencyType,
	}

	habit, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrHabitTitleEmpty) || errors.Is(err, domain.ErrInvalidColor) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, habit)
}

// List godoc
// @Summary      List all habits
// @Description  Get all active habits for the authenticated user
// @Tags         Habits
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   domain.Habit
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Router       /habits [get]
func (h *HabitHandler) List(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user context missing"})
		return
	}

	list, err := h.svc.ListByUserID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, list)
}

// Sync godoc
// @Summary      Sync habits (Offline-First)
// @Description  Get habits created, updated, or deleted since the provided timestamp cursor.
// @Tags         Habits
// @Produce      json
// @Security     BearerAuth
// @Param        last_sync query string false "Timestamp Cursor (RFC3339 format)"
// @Success      200  {object}  map[string]interface{} "Returns {changes: delta, timestamp: NextCursor}"
// @Failure      400  {object}  map[string]string "Invalid Timestamp Format"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Router       /habits/sync [get]
func (h *HabitHandler) Sync(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user context missing"})
		return
	}

	lastSyncStr := c.Query("last_sync")
	var lastSync time.Time
	var err error

	if lastSyncStr != "" {
		lastSync, err = time.Parse(time.RFC3339, lastSyncStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid last_sync format, use RFC3339"})
			return
		}
	}

	deltas, err := h.svc.GetDelta(c.Request.Context(), userID, lastSync)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sync failed"})
		return
	}

	nextCursor := calculateNextHabitCursor(deltas, lastSync)

	c.JSON(http.StatusOK, gin.H{
		"changes":   deltas,
		"timestamp": nextCursor,
	})
}

// Update godoc
// @Summary      Update a habit
// @Description  Modify an existing habit. Requires 'version' for optimistic locking.
// @Tags         Habits
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path string true "Habit ID"
// @Param        habit body updateHabitRequest true "Update Data"
// @Success      200  "OK"
// @Failure      400  {object}  map[string]string "Invalid Input"
// @Failure      404  {object}  map[string]string "Habit Not Found"
// @Failure      409  {object}  map[string]string "Version Conflict (Data modified elsewhere)"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Router       /habits/{id} [put]
func (h *HabitHandler) Update(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user context missing"})
		return
	}

	id := c.Param("id")

	var req updateHabitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := services.UpdateHabitInput{
		ID:            id,
		UserID:        userID,
		Title:         req.Title,
		Description:   req.Description,
		Color:         req.Color,
		Icon:          req.Icon,
		Type:          req.Type,
		ReminderTime:  req.ReminderTime,
		Unit:          req.Unit,
		TargetValue:   req.TargetValue,
		Interval:      req.Interval,
		Weekdays:      req.Weekdays,
		FrequencyType: req.FrequencyType,
		Version:       req.Version,
	}

	err := h.svc.Update(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrHabitConflict) {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "version conflict",
				"message": "Data has been modified elsewhere. Please sync.",
			})
			return
		}

		if errors.Is(err, domain.ErrHabitNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "habit not found"})
			return
		}
		if errors.Is(err, domain.ErrInvalidColor) || errors.Is(err, domain.ErrHabitTitleEmpty) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.Status(http.StatusOK)
}

// Delete godoc
// @Summary      Soft-delete a habit
// @Description  Mark a habit as deleted (archived)
// @Tags         Habits
// @Security     BearerAuth
// @Param        id    path string true "Habit ID"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]string "Habit Not Found"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Router       /habits/{id} [delete]
func (h *HabitHandler) Delete(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user context missing"})
		return
	}

	id := c.Param("id")

	err := h.svc.Delete(c.Request.Context(), id, userID)
	if err != nil {
		if errors.Is(err, domain.ErrHabitNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "habit not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.Status(http.StatusNoContent)
}

func calculateNextHabitCursor(changes []*domain.Habit, fallback time.Time) time.Time {
	if len(changes) == 0 {
		return fallback
	}
	return changes[len(changes)-1].UpdatedAt
}
