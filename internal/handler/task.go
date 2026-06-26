package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
	"github.com/example/go-project/internal/middleware"
	"github.com/example/go-project/internal/service"
)

type TaskHandler struct {
	svc *service.TaskService
	log *slog.Logger
}

func NewTaskHandler(svc *service.TaskService, log *slog.Logger) *TaskHandler {
	if log == nil {
		log = slog.Default()
	}
	return &TaskHandler{svc: svc, log: log}
}

func (h *TaskHandler) Create(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewError(dto.CodeUnauthorized, "unauthorized"))
		return
	}
	var req dto.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	t, err := h.svc.Create(c.Request.Context(), uid, req)
	if err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, dto.NewData(toTaskResponseDTO(t)))
}

func (h *TaskHandler) Get(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewError(dto.CodeUnauthorized, "unauthorized"))
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		badRequest(c, &handlerErr{msg: "task id"})
		return
	}
	t, err := h.svc.Get(c.Request.Context(), uid, id)
	if err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewData(toTaskResponseDTO(t)))
}

func (h *TaskHandler) List(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewError(dto.CodeUnauthorized, "unauthorized"))
		return
	}
	filter := dto.BindTaskFilter(c.Query)
	list, err := h.svc.List(c.Request.Context(), uid, filter)
	if err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewData(list))
}

func (h *TaskHandler) Update(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewError(dto.CodeUnauthorized, "unauthorized"))
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		badRequest(c, &handlerErr{msg: "task id"})
		return
	}
	var req dto.UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	if !req.HasAnyUpdate() {
		badRequest(c, &handlerErr{msg: "no fields to update"})
		return
	}
	t, err := h.svc.Update(c.Request.Context(), uid, id, req)
	if err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewData(toTaskResponseDTO(t)))
}

func (h *TaskHandler) Delete(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewError(dto.CodeUnauthorized, "unauthorized"))
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		badRequest(c, &handlerErr{msg: "task id"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), uid, id); err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TaskHandler) History(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewError(dto.CodeUnauthorized, "unauthorized"))
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		badRequest(c, &handlerErr{msg: "task id"})
		return
	}
	rows, err := h.svc.History(c.Request.Context(), uid, id)
	if err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewData(dto.TaskHistoryListResponse{Items: rows}))
}

func toTaskResponseDTO(t *entity.Task) dto.TaskResponse {
	createdAt := ""
	updatedAt := ""
	if !t.CreatedAt.IsZero() {
		createdAt = t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	if !t.UpdatedAt.IsZero() {
		updatedAt = t.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return dto.TaskResponse{
		ID:          t.ID,
		TeamID:      t.TeamID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status,
		AssigneeID:  t.AssigneeID,
		CreatedBy:   t.CreatedBy,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}
