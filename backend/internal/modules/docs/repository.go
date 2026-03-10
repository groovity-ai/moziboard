package docs

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

func (r *Repository) ListByBoard(ctx context.Context, boardID string) ([]Document, error) {
	rows, err := r.db.Query(ctx,
		"SELECT id, board_id::text, title, content, created_at, updated_at FROM documents WHERE board_id=$1 ORDER BY updated_at DESC",
		boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.BoardID, &d.Title, &d.Content, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	if docs == nil {
		docs = []Document{}
	}
	return docs, nil
}

func (r *Repository) Create(ctx context.Context, boardID string, d *Document) error {
	return r.db.QueryRow(ctx,
		"INSERT INTO documents (board_id, title, content) VALUES ($1, $2, $3) RETURNING id, created_at, updated_at",
		boardID, d.Title, d.Content,
	).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
}

func (r *Repository) GetByID(ctx context.Context, id int) (Document, error) {
	var d Document
	err := r.db.QueryRow(ctx,
		"SELECT id, board_id::text, title, content, created_at, updated_at FROM documents WHERE id=$1", id,
	).Scan(&d.ID, &d.BoardID, &d.Title, &d.Content, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func (r *Repository) Update(ctx context.Context, id int, title string, content string) error {
	_, err := r.db.Exec(ctx,
		"UPDATE documents SET title=$1, content=$2, updated_at=CURRENT_TIMESTAMP WHERE id=$3",
		title, content, id,
	)
	return err
}

func (r *Repository) Delete(ctx context.Context, id int) (bool, error) {
	result, err := r.db.Exec(ctx, "DELETE FROM documents WHERE id=$1", id)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() > 0, nil
}

func (r *Repository) Search(ctx context.Context, boardID string, embedding string) ([]Document, error) {
	var (
		rows pgxRows
		err  error
	)
	if boardID != "" {
		rows, err = r.db.Query(ctx, "SELECT id, board_id::text, title, content, created_at, updated_at FROM documents WHERE board_id=$1 AND embedding IS NOT NULL ORDER BY embedding <=> $2 LIMIT 5", boardID, embedding)
	} else {
		rows, err = r.db.Query(ctx, "SELECT id, board_id::text, title, content, created_at, updated_at FROM documents WHERE embedding IS NOT NULL ORDER BY embedding <=> $1 LIMIT 5", embedding)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.BoardID, &d.Title, &d.Content, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	if docs == nil {
		docs = []Document{}
	}
	return docs, nil
}

type pgxRows interface {
	Close()
	Next() bool
	Scan(dest ...any) error
}
