package http

import (
	"net/http"

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
	Title        string `json:"title" binding:"required"`
	Description  string `json:"description"`
	Color        string `json:"color"`
	Icon         string `json:"icon"`
	Type         string `json:"type"`
	ReminderTime string `json:"reminder_time"`
	Unit         string `json:"unit"`
	TargetValue  int    `json:"target_value"`
	Interval     int    `json:"interval"`
	Weekdays     []int  `json:"weekdays"`
}

type updateHabitRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	Color        string `json:"color"`
	Icon         string `json:"icon"`
	Type         string `json:"type"`
	ReminderTime string `json:"reminder_time"`
	Unit         string `json:"unit"`
	TargetValue  int    `json:"target_value"`
	Interval     int    `json:"interval"`
	Weekdays     []int  `json:"weekdays"`
}

func (h *HabitHandler) RegisterRoutes(router *gin.RouterGroup) {
	habits := router.Group("/habits")
	{
		habits.POST("", h.Create)
		habits.GET("", h.List)
		habits.PUT("/:id", h.Update)
		habits.DELETE("/:id", h.Delete)
	}
}

func (h *HabitHandler) Create(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id header"})
		return
	}

	var req createHabitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := services.CreateHabitInput{
		UserID:       userID,
		Title:        req.Title,
		Description:  req.Description,
		Color:        req.Color,
		Icon:         req.Icon,
		Type:         req.Type,
		ReminderTime: req.ReminderTime,
		Unit:         req.Unit,
		TargetValue:  req.TargetValue,
		Interval:     req.Interval,
		Weekdays:     req.Weekdays,
	}

	habit, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		if err == domain.ErrHabitTitleEmpty || err == domain.ErrInvalidColor {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, habit)
}

func (h *HabitHandler) List(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id header"})
		return
	}

	list, err := h.svc.ListByUserID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, list)
}

func (h *HabitHandler) Update(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id header"})
		return
	}

	id := c.Param("id")

	var req updateHabitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := services.UpdateHabitInput{
		ID:           id,
		UserID:       userID,
		Title:        req.Title,
		Description:  req.Description,
		Color:        req.Color,
		Icon:         req.Icon,
		Type:         req.Type,
		ReminderTime: req.ReminderTime,
		Unit:         req.Unit,
		TargetValue:  req.TargetValue,
		Interval:     req.Interval,
		Weekdays:     req.Weekdays,
	}

	err := h.svc.Update(c.Request.Context(), input)
	if err != nil {
		if err == domain.ErrHabitNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "habit not found"})
			return
		}
		if err == domain.ErrInvalidColor || err == domain.ErrHabitTitleEmpty {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.Status(http.StatusOK)
}

func (h *HabitHandler) Delete(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id header"})
		return
	}

	id := c.Param("id")

	err := h.svc.Delete(c.Request.Context(), id, userID)
	if err != nil {
		if err == domain.ErrHabitNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "habit not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.Status(http.StatusNoContent)
}
