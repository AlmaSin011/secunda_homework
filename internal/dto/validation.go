package dto

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/example/go-project/internal/entity"
)

const (
	MaxNameLen     = 100
	MaxTitleLen    = 255
	MaxBodyLen     = 10_000
	MaxPasswordLen = 72 // bcrypt обрезает всё, что длиннее 72 байт
	MinPasswordLen = 8
)

// Ошибки валидации. Возвращаются Validate(), оборачиваются хендлером в Envelope.
var (
	ErrEmptyEmail       = errors.New("email is required")
	ErrInvalidEmail     = errors.New("email is invalid")
	ErrEmptyPassword    = errors.New("password is required")
	ErrShortPassword    = fmt.Errorf("password must be at least %d chars", MinPasswordLen)
	ErrLongPassword     = fmt.Errorf("password must be at most %d chars", MaxPasswordLen)
	ErrEmptyName        = errors.New("name is required")
	ErrLongName         = fmt.Errorf("name must be at most %d chars", MaxNameLen)
	ErrEmptyTeamName    = errors.New("team name is required")
	ErrLongTeamName     = fmt.Errorf("team name must be at most %d chars", MaxNameLen)
	ErrEmptyTaskTitle   = errors.New("task title is required")
	ErrLongTaskTitle    = fmt.Errorf("task title must be at most %d chars", MaxTitleLen)
	ErrZeroTeamID       = errors.New("team_id is required")
	ErrZeroUserID       = errors.New("user_id is required")
	ErrEmptyCommentBody = errors.New("comment body is required")
	ErrLongCommentBody  = fmt.Errorf("comment body must be at most %d chars", MaxBodyLen)
	ErrInvalidRole      = errors.New("role is invalid")
	ErrInvalidStatus    = errors.New("status is invalid")
)

func (r RegisterRequest) Validate() error {
	r.Email = strings.TrimSpace(r.Email)
	if r.Email == "" {
		return ErrEmptyEmail
	}
	if _, err := mail.ParseAddress(r.Email); err != nil {
		return ErrInvalidEmail
	}
	if r.Password == "" {
		return ErrEmptyPassword
	}
	if len(r.Password) < MinPasswordLen {
		return ErrShortPassword
	}
	if len(r.Password) > MaxPasswordLen {
		return ErrLongPassword
	}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return ErrEmptyName
	}
	if len(r.Name) > MaxNameLen {
		return ErrLongName
	}
	return nil
}

func (r LoginRequest) Validate() error {
	r.Email = strings.TrimSpace(r.Email)
	if r.Email == "" {
		return ErrEmptyEmail
	}
	if _, err := mail.ParseAddress(r.Email); err != nil {
		return ErrInvalidEmail
	}
	if r.Password == "" {
		return ErrEmptyPassword
	}
	return nil
}

func (r CreateTeamRequest) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return ErrEmptyTeamName
	}
	if len(r.Name) > MaxNameLen {
		return ErrLongTeamName
	}
	return nil
}

func (r *InviteRequest) Validate() error {
	if r.UserID == 0 {
		return ErrZeroUserID
	}
	parsed, err := entity.ParseRole(string(r.Role))
	if err != nil {
		return ErrInvalidRole
	}
	r.Role = parsed
	return nil
}

func (r *CreateTaskRequest) Validate() error {
	if r.TeamID == 0 {
		return ErrZeroTeamID
	}
	r.Title = strings.TrimSpace(r.Title)
	if r.Title == "" {
		return ErrEmptyTaskTitle
	}
	if len(r.Title) > MaxTitleLen {
		return ErrLongTaskTitle
	}
	// Статус опционален. Если пришёл — парсим, иначе дефолт todo.
	if r.Status != "" {
		parsed, err := entity.ParseTaskStatus(string(r.Status))
		if err != nil {
			return ErrInvalidStatus
		}
		r.Status = parsed
	} else {
		r.Status = entity.TaskTodo
	}
	return nil
}

func (r UpdateTaskRequest) Validate() error {
	if r.Title != nil {
		t := strings.TrimSpace(*r.Title)
		if t == "" {
			return ErrEmptyTaskTitle
		}
		if len(t) > MaxTitleLen {
			return ErrLongTaskTitle
		}
		r.Title = &t
	}
	if r.Status != nil {
		parsed, err := entity.ParseTaskStatus(string(*r.Status))
		if err != nil {
			return ErrInvalidStatus
		}
		r.Status = &parsed
	}

	return nil
}

func (r UpdateTaskRequest) HasAnyUpdate() bool {
	return r.Title != nil || r.Description != nil || r.Status != nil || r.AssigneeID != nil
}

func (r CreateCommentRequest) Validate() error {
	r.Body = strings.TrimSpace(r.Body)
	if r.Body == "" {
		return ErrEmptyCommentBody
	}
	if len(r.Body) > MaxBodyLen {
		return ErrLongCommentBody
	}
	return nil
}
