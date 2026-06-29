package dto

// TeamStatsResponse — результат сложного запроса а):
// "Название команды, число участников, число задач в статусе done за последние 7 дней".
type TeamStatsResponse struct {
	TeamID        uint64 `db:"team_id" json:"team_id"`
	TeamName      string `db:"team_name" json:"team_name"`
	MemberCount   int    `db:"member_count" json:"member_count"`
	DoneLast7Days int    `db:"done_last_7_days" json:"done_last_7_days"`
}

type TeamStatsListResponse struct {
	Items []TeamStatsResponse `json:"items"`
}

// TopCreatorEntry — одна строка результата запроса б):
// "Топ-3 создателей задач в каждой команде за последний месяц".
type TopCreatorEntry struct {
	TeamID     uint64 `db:"team_id" json:"team_id"`
	TeamName   string `db:"team_name" json:"team_name"`
	UserID     uint64 `db:"user_id" json:"user_id"`
	UserName   string `db:"user_name" json:"user_name"`
	UserEmail  string `db:"user_email" json:"user_email"`
	TaskCount  int    `db:"task_count" json:"task_count"`
	Rank       int    `db:"rank" json:"rank"`
	WindowFrom string `db:"window_from" json:"window_from"`
	WindowTo   string `db:"window_to" json:"window_to"`
}

// TopCreatorsListResponse — топ-создатели по командам.
type TopCreatorsListResponse struct {
	Items []TopCreatorEntry `json:"items"`
}

type OrphanTaskResponse struct {
	TaskID        uint64  `db:"task_id" json:"task_id"`
	TeamID        uint64  `db:"team_id" json:"team_id"`
	Title         string  `db:"title" json:"title"`
	AssigneeID    *uint64 `db:"assignee_id" json:"assignee_id"`
	AssigneeEmail *string `db:"assignee_email" json:"assignee_email,omitempty"`
}

type OrphanTasksListResponse struct {
	Items []OrphanTaskResponse `json:"items"`
}
