package service

import (
	"errors"

	"github.com/example/go-project/internal/repository"
)

var (
	ErrNotTeamMember = errors.New("user is not a team member")
	ErrForbidden     = errors.New("forbidden")
	ErrValidation    = errors.New("validation failed")
)

var (
	ErrNotFound      = repository.ErrNotFound
	ErrAlreadyExists = repository.ErrAlreadyExists
)
