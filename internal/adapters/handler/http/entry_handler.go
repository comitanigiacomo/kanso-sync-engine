package http

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

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

func (h *EntryHandler) Create(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id header"})
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

func (h *EntryHandler) Update(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id header"})
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

func (h *EntryHandler) Delete(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id header"})
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

func (h *EntryHandler) ListByHabit(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id header"})
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

func (h *EntryHandler) Sync(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id header"})
		return
	}

	sinceStr := c.Query("since")
	var since time.Time

	if sinceStr != "" {
		var err error
		since, err = time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format (use RFC3339)"})
			return
		}
	}

	changes, err := h.svc.GetDelta(c.Request.Context(), userID, since)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"changes":   changes,
		"timestamp": time.Now().UTC(),
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
