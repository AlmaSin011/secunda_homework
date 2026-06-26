package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
	"github.com/example/go-project/internal/repository"
)

type TeamService struct {
	teams      TeamRepository
	users      UserLookup
	transactor Transactor
	now        func() time.Time
}

func NewTeamService(teams TeamRepository, users UserLookup, tx Transactor) *TeamService {
	return &TeamService{
		teams:      teams,
		users:      users,
		transactor: tx,
		now:        time.Now,
	}
}

const teamNameMaxLen = 100

// Create создаёт новую команду.
func (s *TeamService) Create(ctx context.Context, actorID uint64, req dto.CreateTeamRequest) (*entity.Team, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrValidation
	}
	if len(name) > teamNameMaxLen {
		return nil, ErrValidation
	}
	// Создатель обязан быть зарегистрированным user'ом.
	if _, err := s.users.FindByID(ctx, actorID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	t := entity.Team{
		Name:      name,
		CreatedBy: actorID,
	}
	id, err := s.teams.Create(ctx, t)
	if err != nil {
		return nil, err
	}
	t.ID = id

	m := entity.TeamMember{
		UserID:   actorID,
		TeamID:   id,
		Role:     entity.RoleOwner,
		JoinedAt: s.now(),
	}
	if err := s.teams.AddMember(ctx, m); err != nil {
		return nil, err
	}
	return &t, nil
}

// Invite добавляет пользователя в команду.
func (s *TeamService) Invite(ctx context.Context, actorID, teamID uint64, req dto.InviteRequest) error {
	// 1. команда должна существовать.
	if _, err := s.teams.FindByID(ctx, teamID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	actor, err := s.teams.GetMember(ctx, teamID, actorID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrForbidden
		}
		return err
	}
	if !actor.Role.CanInvite() {
		return ErrForbidden
	}

	if _, err := s.users.FindByID(ctx, req.UserID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	if !req.Role.IsValid() {
		return ErrValidation
	}

	m := entity.TeamMember{
		UserID:   req.UserID,
		TeamID:   teamID,
		Role:     req.Role,
		JoinedAt: s.now(),
	}
	if err := s.teams.AddMember(ctx, m); err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (s *TeamService) List(ctx context.Context, userID uint64) ([]dto.TeamResponse, error) {
	teams, err := s.teams.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.TeamResponse, 0, len(teams))
	for _, t := range teams {
		out = append(out, dto.TeamResponse{
			ID:        t.Team.ID,
			Name:      t.Team.Name,
			CreatedBy: t.Team.CreatedBy,
			CreatedAt: t.Team.CreatedAt.Format(time.RFC3339),
			MyRole:    t.Role,
		})
	}
	return out, nil
}

func (s *TeamService) ListMembers(ctx context.Context, callerID, teamID uint64) ([]entity.TeamMember, error) {
	if _, err := s.teams.GetMember(ctx, teamID, callerID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrForbidden
		}
		return nil, err
	}
	return s.teams.ListMembers(ctx, teamID)
}
