package boards

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

func (r *Repository) ListBoards(ctx context.Context, userID string) ([]Board, error) {
	rows, err := r.db.Query(ctx, `
		SELECT b.id::text, b.user_id, b.title, b.description
		FROM boards b
		WHERE COALESCE(b.user_id, '') = $1
		   OR EXISTS (
			   SELECT 1 FROM board_members bm
			   WHERE bm.board_id = b.id AND bm.member_id = $1
		   )
		ORDER BY b.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []Board
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.ID, &b.UserID, &b.Title, &b.Description); err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	if boards == nil {
		boards = []Board{}
	}
	return boards, nil
}

func (r *Repository) CreateBoard(ctx context.Context, b *Board) error {
	return r.db.QueryRow(ctx,
		"INSERT INTO boards (user_id, title, description) VALUES ($1, $2, $3) RETURNING id::text",
		b.UserID, b.Title, b.Description,
	).Scan(&b.ID)
}

func (r *Repository) ListMembers(ctx context.Context) ([]Member, error) {
	rows, err := r.db.Query(ctx, "SELECT id, name, role, avatar FROM members ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Name, &m.Role, &m.Avatar); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, nil
}

func (r *Repository) ListBoardMembers(ctx context.Context, boardID string) ([]Member, error) {
	rows, err := r.db.Query(ctx, `
		SELECT m.id, m.name, bm.role, m.avatar
		FROM board_members bm
		JOIN members m ON bm.member_id = m.id
		WHERE bm.board_id = $1
		ORDER BY m.name ASC
	`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Name, &m.Role, &m.Avatar); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, nil
}

func (r *Repository) AddBoardMember(ctx context.Context, boardID, memberID, role string) error {
	_, err := r.db.Exec(ctx,
		"INSERT INTO board_members (board_id, member_id, role) VALUES ($1, $2, $3) ON CONFLICT (board_id, member_id) DO UPDATE SET role = EXCLUDED.role",
		boardID, memberID, role,
	)
	return err
}

func (r *Repository) RemoveBoardMember(ctx context.Context, boardID, memberID string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM board_members WHERE board_id=$1 AND member_id=$2", boardID, memberID)
	return err
}
