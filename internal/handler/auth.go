package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/service"
)

type AuthHandler struct {
	svc *service.AuthService
	log *slog.Logger
}

func NewAuthHandler(svc *service.AuthService, log *slog.Logger) *AuthHandler {
	if log == nil {
		log = slog.Default()
	}
	return &AuthHandler{svc: svc, log: log}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	resp, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, dto.NewData(resp))
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	resp, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		writeServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewData(resp))
}
