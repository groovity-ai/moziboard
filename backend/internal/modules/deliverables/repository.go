package deliverables

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

func (r *Repository) ListByTask(ctx context.Context, taskID int) ([]Deliverable, error) {
	rows, err := r.db.Query(ctx,
		"SELECT id, task_id, title, description, artifact_type, content, created_at, updated_at FROM deliverables WHERE task_id=$1 ORDER BY created_at DESC",
		taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deliverables []Deliverable
	for rows.Next() {
		var d Deliverable
		if err := rows.Scan(&d.ID, &d.TaskID, &d.Title, &d.Description, &d.ArtifactType, &d.Content, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		deliverables = append(deliverables, d)
	}
	if deliverables == nil {
		deliverables = []Deliverable{}
	}
	return deliverables, nil
}

func (r *Repository) Create(ctx context.Context, taskID int, d *Deliverable) error {
	return r.db.QueryRow(ctx,
		"INSERT INTO deliverables (task_id, title, description, artifact_type, content) VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at, updated_at",
		taskID, d.Title, d.Description, d.ArtifactType, d.Content,
	).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
}
