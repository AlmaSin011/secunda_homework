package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/middleware"
	"github.com/example/go-project/internal/service"
)

type TeamHandler struct {
	svc *service.TeamService
	log *slog.Logger
}

func NewTeamHandler(svc *service.TeamService, log *slog.Logger) *TeamHandler {
	if log == nil {
		log = slog.Default()
	}
	return &TeamHandler{svc: svc, log: log}
}

func (h *TeamHandler) Create(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewError(dto.CodeUnauthorized, "unauthorized"))
		return
	}
	var req dto.CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	t, err := h.svc.Create(c.Request.Context(), uid, req)
	if err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	resp := dto.TeamResponse{
		ID:        t.ID,
		Name:      t.Name,
		CreatedBy: t.CreatedBy,
		CreatedAt: t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		MyRole:    "owner",
	}
	c.JSON(http.StatusCreated, dto.NewData(resp))
}

func (h *TeamHandler) List(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewError(dto.CodeUnauthorized, "unauthorized"))
		return
	}
	teams, err := h.svc.List(c.Request.Context(), uid)
	if err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewData(dto.TeamsListResponse{Items: teams}))
}

func (h *TeamHandler) Invite(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewError(dto.CodeUnauthorized, "unauthorized"))
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		badRequest(c, errorsInvalid("team id"))
		return
	}
	var req dto.InviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	if err := h.svc.Invite(c.Request.Context(), uid, id, req); err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, dto.NewData(gin.H{"team_id": id, "user_id": req.UserID, "role": req.Role}))
}

func (h *TeamHandler) ListMembers(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewError(dto.CodeUnauthorized, "unauthorized"))
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		badRequest(c, errorsInvalid("team id"))
		return
	}
	members, err := h.svc.ListMembers(c.Request.Context(), uid, id)
	if err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	out := make([]dto.TeamMemberResponse, 0, len(members))
	for _, m := range members {
		out = append(out, dto.TeamMemberResponse{
			UserID:   m.UserID,
			TeamID:   m.TeamID,
			Role:     m.Role,
			JoinedAt: m.JoinedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	c.JSON(http.StatusOK, dto.NewData(gin.H{"items": out}))
}

func errorsInvalid(msg string) error {
	return &handlerErr{msg: msg}
}

type handlerErr struct{ msg string }

func (e *handlerErr) Error() string { return e.msg }
