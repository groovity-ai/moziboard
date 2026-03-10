package boardhealth

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

type boardRow struct {
	ID              string
	Title           string
	Description     string
	OpenTasks       int
	ReviewTasks     int
	BlockedTasks    int
	DocsCount       int
	ConnectedAgents int
	AgentIssues     int
	LastActivityAt  *time.Time
	StatsComputedAt *time.Time
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListByUser(ctx context.Context, userID string) ([]boardRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			b.id::text,
			b.title,
			COALESCE(b.description, '') AS description,
			COALESCE(bs.open_tasks, 0) AS open_tasks,
			COALESCE(bs.review_tasks, 0) AS review_tasks,
			COALESCE(bs.blocked_tasks, 0) AS blocked_tasks,
			COALESCE(bs.docs_count, 0) AS docs_count,
			COALESCE(bs.connected_agents, 0) AS connected_agents,
			COALESCE(bs.agent_issues, 0) AS agent_issues,
			bs.last_event_at,
			bs.computed_at
		FROM boards b
		LEFT JOIN board_stats bs ON bs.board_id = b.id
		WHERE COALESCE(b.user_id, '') = $1
		   OR EXISTS (
			   SELECT 1 FROM board_members bm
			   WHERE bm.board_id = b.id AND bm.member_id = $1
		   )
		ORDER BY COALESCE(bs.last_event_at, b.created_at) DESC, b.title ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	return scanBoardRows(rows)
}

func (r *Repository) ListByBoard(ctx context.Context, boardID string) ([]boardRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			b.id::text,
			b.title,
			COALESCE(b.description, '') AS description,
			COALESCE(bs.open_tasks, 0) AS open_tasks,
			COALESCE(bs.review_tasks, 0) AS review_tasks,
			COALESCE(bs.blocked_tasks, 0) AS blocked_tasks,
			COALESCE(bs.docs_count, 0) AS docs_count,
			COALESCE(bs.connected_agents, 0) AS connected_agents,
			COALESCE(bs.agent_issues, 0) AS agent_issues,
			bs.last_event_at,
			bs.computed_at
		FROM boards b
		LEFT JOIN board_stats bs ON bs.board_id = b.id
		WHERE b.id::text = $1
		ORDER BY COALESCE(bs.last_event_at, b.created_at) DESC, b.title ASC
	`, boardID)
	if err != nil {
		return nil, err
	}
	return scanBoardRows(rows)
}

func scanBoardRows(rows pgx.Rows) ([]boardRow, error) {
	defer rows.Close()
	items := []boardRow{}
	for rows.Next() {
		var item boardRow
		if err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.Description,
			&item.OpenTasks,
			&item.ReviewTasks,
			&item.BlockedTasks,
			&item.DocsCount,
			&item.ConnectedAgents,
			&item.AgentIssues,
			&item.LastActivityAt,
			&item.StatsComputedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if items == nil {
		items = []boardRow{}
	}
	return items, nil
}
