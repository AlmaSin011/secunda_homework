package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/go-project/internal/auth"
	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/service"
)

func writeServiceError(c *gin.Context, log *slog.Logger, err error) {
	if err == nil {
		return
	}

	switch {
	case errors.Is(err, service.ErrValidation):
		c.JSON(http.StatusBadRequest, dto.NewError(dto.CodeValidation, err.Error()))
	case errors.Is(err, service.ErrForbidden):
		c.JSON(http.StatusForbidden, dto.NewError(dto.CodeForbidden, "forbidden"))
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, dto.NewError(dto.CodeNotFound, "not found"))
	case errors.Is(err, service.ErrAlreadyExists):
		c.JSON(http.StatusConflict, dto.NewError(dto.CodeConflict, "already exists"))
	case errors.Is(err, service.ErrEmailTaken):
		c.JSON(http.StatusConflict, dto.NewError(dto.CodeConflict, "email already taken"))
	case errors.Is(err, service.ErrInvalidCredentials),
		errors.Is(err, auth.ErrInvalidToken),
		errors.Is(err, auth.ErrExpiredToken),
		errors.Is(err, auth.ErrTokenSignature):
		c.JSON(http.StatusUnauthorized, dto.NewError(dto.CodeUnauthorized, "unauthorized"))
	case errors.Is(err, service.ErrNotTeamMember):
		c.JSON(http.StatusForbidden, dto.NewError(dto.CodeForbidden, "not a team member"))
	default:
		log.Error("internal handler error",
			slog.String("err", err.Error()),
			slog.String("path", c.Request.URL.Path),
		)
		c.JSON(http.StatusInternalServerError, dto.NewError(dto.CodeInternal, "internal error"))
	}
}

func badRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, dto.NewError(dto.CodeBadRequest, err.Error()))
}
