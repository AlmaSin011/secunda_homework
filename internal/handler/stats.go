package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/service"
)

type StatsHandler struct {
	svc *service.StatsService
	log *slog.Logger
}

func NewStatsHandler(svc *service.StatsService, log *slog.Logger) *StatsHandler {
	if log == nil {
		log = slog.Default()
	}
	return &StatsHandler{svc: svc, log: log}
}

// GET /api/v1/stats/teams/last-week.
func (h *StatsHandler) LastWeek(c *gin.Context) {
	rows, err := h.svc.LastWeek(c.Request.Context())
	if err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewData(dto.TeamStatsListResponse{Items: rows}))
}

// GET /api/v1/stats/top-creators?since_days=30&limit=3.
func (h *StatsHandler) TopCreators(c *gin.Context) {
	sinceDays := 30
	limit := 3
	if v := c.Query("since_days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sinceDays = n
		}
	}
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	rows, err := h.svc.TopCreators(c.Request.Context(), sinceDays, limit)
	if err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewData(dto.TopCreatorsListResponse{Items: rows}))
}

// GET /api/v1/stats/orphan-tasks.
func (h *StatsHandler) Orphans(c *gin.Context) {
	rows, err := h.svc.Orphans(c.Request.Context())
	if err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewData(dto.OrphanTasksListResponse{Items: rows}))
}
