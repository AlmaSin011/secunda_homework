package dto

import "time"

// TeamStatsResponse — результат сложного запроса а):
// "Название команды, число участников, число задач в статусе done за последние 7 дней".
type TeamStatsResponse struct {
	TeamID        uint64 `json:"team_id"`
	TeamName      string `json:"team_name"`
	MemberCount   int    `json:"member_count"`
	DoneLast7Days int    `json:"done_last_7_days"`
}

type TeamStatsListResponse struct {
	Items []TeamStatsResponse `json:"items"`
}

// TopCreatorEntry — одна строка результата запроса б):
// "Топ-3 создателей задач в каждой команде за последний месяц".
type TopCreatorEntry struct {
	TeamID     uint64    `json:"team_id"`
	TeamName   string    `json:"team_name"`
	UserID     uint64    `json:"user_id"`
	UserName   string    `json:"user_name"`
	UserEmail  string    `json:"user_email"`
	TaskCount  int       `json:"task_count"`
	Rank       int       `json:"rank"`
	WindowFrom time.Time `json:"window_from"`
	WindowTo   time.Time `json:"window_to"`
}

// TopCreatorsListResponse — топ-создатели по командам.
type TopCreatorsListResponse struct {
	Items []TopCreatorEntry `json:"items"`
}

type OrphanTaskResponse struct {
	TaskID        uint64  `json:"task_id"`
	TeamID        uint64  `json:"team_id"`
	Title         string  `json:"title"`
	AssigneeID    *uint64 `json:"assignee_id"`
	AssigneeEmail *string `json:"assignee_email,omitempty"`
}

type OrphanTasksListResponse struct {
	Items []OrphanTaskResponse `json:"items"`
}
