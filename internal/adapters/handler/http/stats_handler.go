package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/adapters/handler/http/middleware"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
)

type StatsHandler struct {
	svc *services.StatsService
}

func NewStatsHandler(svc *services.StatsService) *StatsHandler {
	return &StatsHandler{svc: svc}
}

func (h *StatsHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/stats/weekly", h.GetWeeklyStats)
}

func (h *StatsHandler) GetWeeklyStats(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserIDKey)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	endDateStr := c.Query("end_date")
	startDateStr := c.Query("start_date")

	var endDate, startDate time.Time
	var err error

	if endDateStr == "" {
		endDate = time.Now().UTC()
	} else {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_date format, expected YYYY-MM-DD"})
			return
		}
	}

	if startDateStr == "" {
		startDate = endDate.AddDate(0, 0, -6)
	} else {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_date format, expected YYYY-MM-DD"})
			return
		}
	}

	if startDate.After(endDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date cannot be after end_date"})
		return
	}

	const maxDaysRange = 366 // Max 1 anno
	daysDiff := endDate.Sub(startDate).Hours() / 24
	if daysDiff > maxDaysRange {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date range too large, max 1 year allowed"})
		return
	}

	input := domain.StatsInput{
		UserID:    userID,
		StartDate: startDate,
		EndDate:   endDate,
	}

	stats, err := h.svc.GetWeeklyStats(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}
