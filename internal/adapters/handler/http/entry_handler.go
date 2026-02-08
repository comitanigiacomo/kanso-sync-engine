package http

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http/middleware"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
)

type EntryHandler struct {
	svc *services.EntryService
}

func NewEntryHandler(svc *services.EntryService) *EntryHandler {
	return &EntryHandler{
		svc: svc,
	}
}

type createEntryRequest struct {
	HabitID        string    `json:"habit_id" binding:"required"`
	CompletionDate time.Time `json:"completion_date" binding:"required"`
	Value          int       `json:"value"`
	Notes          string    `json:"notes"`
}

type updateEntryRequest struct {
	Value   int    `json:"value"`
	Notes   string `json:"notes"`
	Version int    `json:"version" binding:"required"`
}

func (h *EntryHandler) RegisterRoutes(router *gin.RouterGroup) {
	entries := router.Group("/entries")
	{
		entries.POST("", h.Create)
		entries.GET("", h.ListByHabit)
		entries.GET("/sync", h.Sync)
		entries.PUT("/:id", h.Update)
		entries.DELETE("/:id", h.Delete)
	}
}

// Create godoc
// @Summary      Log a habit entry
// @Description  Record a completion or value for a specific habit on a specific date
// @Tags         Entries
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        entry body createEntryRequest true "Entry Data"
// @Success      201  {object}  domain.HabitEntry
// @Failure      400  {object}  map[string]string "Invalid Input"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Router       /entries [post]
func (h *EntryHandler) Create(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user context missing"})
		return
	}

	var req createEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	input := services.CreateEntryInput{
		HabitID:        req.HabitID,
		UserID:         userID,
		CompletionDate: req.CompletionDate,
		Value:          req.Value,
		Notes:          req.Notes,
	}

	entry, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, entry)
}

// Update godoc
// @Summary      Update an entry value
// @Description  Change the value or completion status. Requires current version for optimistic locking.
// @Tags         Entries
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path string true "Entry ID"
// @Param        entry body updateEntryRequest true "Update Data"
// @Success      200  {object}  domain.HabitEntry
// @Failure      400  {object}  map[string]string "Invalid Input"
// @Failure      404  {object}  map[string]string "Entry not found"
// @Failure      409  {object}  map[string]string "Version Conflict"
// @Router       /entries/{id} [put]
func (h *EntryHandler) Update(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user context missing"})
		return
	}

	id := c.Param("id")
	var req updateEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	input := services.UpdateEntryInput{
		ID:      id,
		UserID:  userID,
		Value:   req.Value,
		Notes:   req.Notes,
		Version: req.Version,
	}

	entry, err := h.svc.Update(c.Request.Context(), input)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, entry)
}

// Delete godoc
// @Summary      Delete an entry
// @Description  Permanently remove an entry
// @Tags         Entries
// @Security     BearerAuth
// @Param        id    path string true "Entry ID"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]string "Entry not found"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Router       /entries/{id} [delete]
func (h *EntryHandler) Delete(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user context missing"})
		return
	}

	id := c.Param("id")

	err := h.svc.Delete(c.Request.Context(), id, userID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ListByHabit godoc
// @Summary      List entries for a habit
// @Description  Get history of entries for a specific habit ID within a date range
// @Tags         Entries
// @Produce      json
// @Security     BearerAuth
// @Param        habit_id query string true "Habit ID"
// @Param        from     query string false "Start Date (RFC3339) - Default: 30 days ago"
// @Param        to       query string false "End Date (RFC3339) - Default: Now"
// @Success      200  {array}   domain.HabitEntry
// @Failure      400  {object}  map[string]string "Missing habit_id"
// @Router       /entries [get]
func (h *EntryHandler) ListByHabit(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user context missing"})
		return
	}

	habitID := c.Query("habit_id")
	if habitID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "habit_id is required"})
		return
	}

	to := time.Now().UTC()
	from := to.AddDate(0, 0, -30)

	if t := c.Query("to"); t != "" {
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			to = parsed
		}
	}
	if f := c.Query("from"); f != "" {
		if parsed, err := time.Parse(time.RFC3339, f); err == nil {
			from = parsed
		}
	}

	list, err := h.svc.ListByHabitID(c.Request.Context(), habitID, userID, from, to)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, list)
}

// Sync godoc
// @Summary      Sync entries (Offline-First)
// @Description  Get entries created or modified since the last sync cursor.
// @Tags         Entries
// @Produce      json
// @Security     BearerAuth
// @Param        since query string false "Last Sync Cursor (RFC3339)"
// @Success      200  {object}  map[string]interface{} "Returns {changes: [], timestamp: NextCursor}"
// @Failure      400  {object}  map[string]string "Invalid Date Format"
// @Router       /entries/sync [get]
func (h *EntryHandler) Sync(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user context missing"})
		return
	}

	sinceStr := c.Query("since")
	var since time.Time

	if sinceStr != "" {
		var err error
		since, err = time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format"})
			return
		}
	}

	changes, err := h.svc.GetDelta(c.Request.Context(), userID, since)
	if err != nil {
		handleError(c, err)
		return
	}

	nextCursor := calculateNextCursor(changes, since)

	c.JSON(http.StatusOK, gin.H{
		"changes":   changes,
		"timestamp": nextCursor,
	})
}

func handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})

	case errors.Is(err, domain.ErrEntryNotFound) || errors.Is(err, domain.ErrHabitNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})

	case errors.Is(err, domain.ErrEntryConflict):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "version conflict",
			"message": "data has been modified elsewhere, please sync",
		})

	default:
		log.Printf("[ERROR] Request %s %s failed: %v", c.Request.Method, c.Request.URL.Path, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

func calculateNextCursor(changes []*domain.HabitEntry, fallback time.Time) time.Time {
	if len(changes) == 0 {
		return fallback
	}

	lastChange := changes[len(changes)-1].UpdatedAt

	return lastChange
}
