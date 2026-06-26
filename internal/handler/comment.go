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

type CommentHandler struct {
	svc *service.CommentService
	log *slog.Logger
}

func NewCommentHandler(svc *service.CommentService, log *slog.Logger) *CommentHandler {
	if log == nil {
		log = slog.Default()
	}
	return &CommentHandler{svc: svc, log: log}
}

func (h *CommentHandler) Create(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewError(dto.CodeUnauthorized, "unauthorized"))
		return
	}
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || taskID == 0 {
		badRequest(c, &handlerErr{msg: "task id"})
		return
	}
	var req dto.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	id, err := h.svc.Create(c.Request.Context(), uid, taskID, req)
	if err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, dto.NewData(gin.H{"id": id, "task_id": taskID}))
}

func (h *CommentHandler) List(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewError(dto.CodeUnauthorized, "unauthorized"))
		return
	}
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || taskID == 0 {
		badRequest(c, &handlerErr{msg: "task id"})
		return
	}
	items, err := h.svc.List(c.Request.Context(), uid, taskID)
	if err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewData(dto.CommentsListResponse{Items: items}))
}
