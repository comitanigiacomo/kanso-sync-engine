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

// GetWeeklyStats godoc
// @Summary      Get habit statistics
// @Description  Returns completion data respecting user timezone.
// @Tags         Stats
// @Produce      json
// @Security     BearerAuth
// @Param        start_date query string false "Start Date (YYYY-MM-DD)"
// @Param        end_date   query string false "End Date (YYYY-MM-DD)"
// @Param        X-Timezone header string false "User Timezone (e.g. Europe/Rome). Defaults to UTC."
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string "Invalid Date/Timezone"
// @Failure      401  {object}  map[string]string "Unauthorized"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Router       /stats/weekly [get]
func (h *StatsHandler) GetWeeklyStats(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	tzHeader := c.GetHeader("X-Timezone")
	location := time.UTC
	if tzHeader != "" {
		loc, err := time.LoadLocation(tzHeader)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid timezone format (use IANA name like 'Europe/Rome')"})
			return
		}
		location = loc
	}

	endDateStr := c.Query("end_date")
	startDateStr := c.Query("start_date")

	var endDate, startDate time.Time
	var err error

	if endDateStr == "" {
		endDate = time.Now().In(location)
	} else {
		endDate, err = time.ParseInLocation("2006-01-02", endDateStr, location)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_date format, expected YYYY-MM-DD"})
			return
		}
	}

	if startDateStr == "" {
		startDate = endDate.AddDate(0, 0, -6)
	} else {
		startDate, err = time.ParseInLocation("2006-01-02", startDateStr, location)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_date format, expected YYYY-MM-DD"})
			return
		}
	}

	if startDate.After(endDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date cannot be after end_date"})
		return
	}

	const maxDaysRange = 366
	daysDiff := endDate.Sub(startDate).Hours() / 24
	if daysDiff > maxDaysRange {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date range too large, max 1 year allowed"})
		return
	}

	input := domain.StatsInput{
		UserID:    userID,
		StartDate: startDate,
		EndDate:   endDate,
		Location:  location,
	}

	stats, err := h.svc.GetWeeklyStats(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}
