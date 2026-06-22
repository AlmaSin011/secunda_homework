package dto

import "github.com/example/go-project/internal/entity"

// CreateTeamRequest — входные данные для POST /api/v1/teams.
type CreateTeamRequest struct {
	Name string `json:"name"`
}

// InviteRequest — входные данные для POST /api/v1/teams/{id}/invite.
type InviteRequest struct {
	UserID uint64      `json:"user_id"`
	Role   entity.Role `json:"role"`
}

type TeamResponse struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	CreatedBy uint64 `json:"created_by"`
	CreatedAt string `json:"created_at"`
	// MyRole — роль запрашивающего пользователя в команде (owner/admin/member).
	// чтобы не делать отдельный запрос.
	MyRole entity.Role `json:"my_role,omitempty"`
}

// TeamsListResponse — список команд пользователя.
type TeamsListResponse struct {
	Items []TeamResponse `json:"items"`
}

// TeamMemberResponse — участник команды.
type TeamMemberResponse struct {
	UserID   uint64      `json:"user_id"`
	TeamID   uint64      `json:"team_id"`
	Role     entity.Role `json:"role"`
	JoinedAt string      `json:"joined_at"`
	Email    string      `json:"email,omitempty"`
	Name     string      `json:"name,omitempty"`
}
