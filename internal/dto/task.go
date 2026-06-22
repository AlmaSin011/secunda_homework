package dto

import "github.com/example/go-project/internal/entity"

// CreateTaskRequest — входные данные для POST /api/v1/tasks.
type CreateTaskRequest struct {
	TeamID      uint64            `json:"team_id"`
	Title       string            `json:"title"`
	Description *string           `json:"description"`
	Status      entity.TaskStatus `json:"status"`
	AssigneeID  *uint64           `json:"assignee_id"`
}

// UpdateTaskRequest — входные данные для PUT /api/v1/tasks/{id}.
type UpdateTaskRequest struct {
	Title       *string            `json:"title"`
	Description *string            `json:"description"`
	Status      *entity.TaskStatus `json:"status"`
	AssigneeID  *uint64            `json:"assignee_id"` // явный nil значит "снять исполнителя"
}

type TaskResponse struct {
	ID          uint64            `json:"id"`
	TeamID      uint64            `json:"team_id"`
	Title       string            `json:"title"`
	Description *string           `json:"description"`
	Status      entity.TaskStatus `json:"status"`
	AssigneeID  *uint64           `json:"assignee_id"`
	CreatedBy   uint64            `json:"created_by"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// TasksListResponse — список задач с пагинацией.
type TasksListResponse struct {
	Items []TaskResponse `json:"items"`
	Meta  Pagination     `json:"meta"`
}

// TaskHistoryResponse — одна запись истории задачи.
type TaskHistoryResponse struct {
	ID        uint64  `json:"id"`
	TaskID    uint64  `json:"task_id"`
	ChangedBy uint64  `json:"changed_by"`
	Field     string  `json:"field"`
	OldValue  *string `json:"old_value"`
	NewValue  *string `json:"new_value"`
	ChangedAt string  `json:"changed_at"`
}

type TaskHistoryListResponse struct {
	Items []TaskHistoryResponse `json:"items"`
}

// CreateCommentRequest — POST /api/v1/tasks/{id}/comments.
type CreateCommentRequest struct {
	Body string `json:"body"`
}

type CommentResponse struct {
	ID        uint64 `json:"id"`
	TaskID    uint64 `json:"task_id"`
	UserID    uint64 `json:"user_id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type CommentsListResponse struct {
	Items []CommentResponse `json:"items"`
}
