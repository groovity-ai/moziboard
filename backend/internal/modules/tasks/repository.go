package tasks

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListByBoard(ctx context.Context, boardID string) ([]Task, error) {
	rows, err := r.db.Query(ctx, "SELECT id, board_id::text, title, description, list_id, position, assignee_id, parent_id, status, blocked_reason FROM tasks WHERE board_id=$1 ORDER BY position ASC", boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.BoardID, &t.Title, &t.Description, &t.ListID, &t.Position, &t.AssigneeID, &t.ParentID, &t.Status, &t.BlockedReason); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	if tasks == nil {
		tasks = []Task{}
	}
	return tasks, nil
}

func (r *Repository) GetByID(ctx context.Context, id int) (Task, error) {
	var t Task
	err := r.db.QueryRow(ctx, "SELECT id, board_id::text, title, description, list_id, position, assignee_id, parent_id, status, blocked_reason FROM tasks WHERE id=$1", id).Scan(&t.ID, &t.BoardID, &t.Title, &t.Description, &t.ListID, &t.Position, &t.AssigneeID, &t.ParentID, &t.Status, &t.BlockedReason)
	return t, err
}

func (r *Repository) GetDefaultBoardID(ctx context.Context) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, "SELECT id::text FROM boards LIMIT 1").Scan(&id)
	return id, err
}

func (r *Repository) Create(ctx context.Context, t *Task) error {
	return r.db.QueryRow(ctx, "INSERT INTO tasks (board_id, title, description, list_id, position, assignee_id, parent_id, status, blocked_reason) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id", t.BoardID, t.Title, t.Description, t.ListID, t.Position, t.AssigneeID, t.ParentID, t.Status, t.BlockedReason).Scan(&t.ID)
}

func (r *Repository) Update(ctx context.Context, id int, t *Task) error {
	_, err := r.db.Exec(ctx, "UPDATE tasks SET title=$1, description=$2, list_id=$3, position=$4, assignee_id=$5, board_id=$6, parent_id=$7, status=$8, blocked_reason=$9 WHERE id=$10", t.Title, t.Description, t.ListID, t.Position, t.AssigneeID, t.BoardID, t.ParentID, t.Status, t.BlockedReason, id)
	return err
}

func (r *Repository) UpdateEmbedding(ctx context.Context, id int, embedding string) error {
	_, err := r.db.Exec(ctx, "UPDATE tasks SET embedding = $1 WHERE id = $2", embedding, id)
	return err
}

func (r *Repository) ListActivities(ctx context.Context, taskID int) ([]Activity, error) {
	rows, err := r.db.Query(ctx, "SELECT id, task_id, user_id, action, details, created_at FROM activities WHERE task_id=$1 ORDER BY created_at DESC", taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var activities []Activity
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.TaskID, &a.UserID, &a.Action, &a.Details, &a.CreatedAt); err != nil {
			return nil, err
		}
		activities = append(activities, a)
	}
	if activities == nil {
		activities = []Activity{}
	}
	return activities, nil
}

func (r *Repository) SearchByEmbedding(ctx context.Context, embedding string) ([]Task, error) {
	rows, err := r.db.Query(ctx, "SELECT id, board_id::text, title, description, list_id, position, assignee_id, parent_id, status, blocked_reason FROM tasks ORDER BY embedding <=> $1 LIMIT 5", embedding)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.BoardID, &t.Title, &t.Description, &t.ListID, &t.Position, &t.AssigneeID, &t.ParentID, &t.Status, &t.BlockedReason); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	if tasks == nil {
		tasks = []Task{}
	}
	return tasks, nil
}
