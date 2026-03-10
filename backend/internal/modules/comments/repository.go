package comments

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

func (r *Repository) ListByTask(ctx context.Context, taskID int) ([]Comment, error) {
	rows, err := r.db.Query(ctx,
		"SELECT c.id, c.task_id, c.user_id, c.content, c.created_at FROM comments c WHERE c.task_id=$1 ORDER BY c.created_at ASC",
		taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var comments []Comment
	for rows.Next() {
		var cm Comment
		if err := rows.Scan(&cm.ID, &cm.TaskID, &cm.UserID, &cm.Content, &cm.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, cm)
	}
	if comments == nil {
		comments = []Comment{}
	}
	return comments, nil
}

func (r *Repository) Create(ctx context.Context, taskID int, cm *Comment) error {
	return r.db.QueryRow(ctx,
		"INSERT INTO comments (task_id, user_id, content) VALUES ($1, $2, $3) RETURNING id, created_at",
		taskID, cm.UserID, cm.Content,
	).Scan(&cm.ID, &cm.CreatedAt)
}
