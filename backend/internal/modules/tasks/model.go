package tasks

import "time"

type Task struct {
	ID            int     `json:"id"`
	BoardID       string  `json:"board_id"`
	SearchScore   float64 `json:"search_score,omitempty"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	ListID        string  `json:"list_id"`
	Position      int     `json:"position"`
	AssigneeID    *string `json:"assignee_id"`
	ParentID      *int    `json:"parent_id,omitempty"`
	UpdatedBy     string  `json:"updated_by,omitempty"`
	Status        string  `json:"status,omitempty"`
	BlockedReason string  `json:"blocked_reason,omitempty"`
}

type Activity struct {
	ID        int       `json:"id"`
	TaskID    int       `json:"task_id"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}
