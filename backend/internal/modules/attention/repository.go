package attention

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListByUser(ctx context.Context, userID string, limit int) ([]Item, error) {
	return r.list(ctx, `
		WITH scoped_boards AS (
			SELECT b.id, b.title
			FROM boards b
			WHERE COALESCE(b.user_id, '') = $1
			   OR EXISTS (
				   SELECT 1 FROM board_members bm
				   WHERE bm.board_id = b.id AND bm.member_id = $1
			   )
		),
		candidate_boards AS (
			SELECT sb.id, sb.title
			FROM scoped_boards sb
			LEFT JOIN board_stats bs ON bs.board_id = sb.id
			WHERE COALESCE(bs.blocked_tasks, 0) > 0
			   OR COALESCE(bs.review_tasks, 0) > 0
			   OR COALESCE(bs.agent_issues, 0) > 0
		)
		SELECT type, severity, entity_kind, entity_id, reason, source, board_id, board_title, task_id, task_title, agent_id, agent_name, title, description, created_at, href
		FROM (
			SELECT
				'blocked_task' AS type,
				'high' AS severity,
				'task' AS entity_kind,
				t.id::text AS entity_id,
				'task_blocked' AS reason,
				'tasks' AS source,
				cb.id::text AS board_id,
				cb.title AS board_title,
				t.id AS task_id,
				t.title AS task_title,
				'' AS agent_id,
				'' AS agent_name,
				('Blocked: ' || t.title) AS title,
				COALESCE(NULLIF(t.blocked_reason, ''), 'Task needs attention') AS description,
				NULL::timestamp AS created_at,
				('/board/' || cb.id::text) AS href
			FROM candidate_boards cb
			JOIN tasks t ON t.board_id = cb.id
			WHERE COALESCE(t.status, '') = 'blocked'

			UNION ALL

			SELECT
				'review_task' AS type,
				'medium' AS severity,
				'task' AS entity_kind,
				t.id::text AS entity_id,
				'task_waiting_review' AS reason,
				'tasks' AS source,
				cb.id::text AS board_id,
				cb.title AS board_title,
				t.id AS task_id,
				t.title AS task_title,
				'' AS agent_id,
				'' AS agent_name,
				('Needs review: ' || t.title) AS title,
				'Task is waiting in review' AS description,
				NULL::timestamp AS created_at,
				('/board/' || cb.id::text) AS href
			FROM candidate_boards cb
			JOIN tasks t ON t.board_id = cb.id
			WHERE COALESCE(t.status, '') = 'review'

			UNION ALL

			SELECT
				'agent_issue' AS type,
				'high' AS severity,
				'agent' AS entity_kind,
				a.id AS entity_id,
				'agent_or_connector_unhealthy' AS reason,
				'agents' AS source,
				cb.id::text AS board_id,
				cb.title AS board_title,
				NULL::int AS task_id,
				'' AS task_title,
				a.id AS agent_id,
				COALESCE(NULLIF(a.display_name, ''), a.id) AS agent_name,
				('Agent issue: ' || COALESCE(NULLIF(a.display_name, ''), a.id)) AS title,
				COALESCE(NULLIF(a.health_note, ''), 'Agent or connector needs attention') AS description,
				COALESCE(a.updated_at, a.created_at) AS created_at,
				('/board/' || cb.id::text || '/agents') AS href
			FROM candidate_boards cb
			JOIN board_agents ba ON ba.board_id = cb.id AND ba.active = true
			JOIN agents a ON a.id = ba.agent_id
			LEFT JOIN agent_connectors ac ON ac.agent_id = a.id
			WHERE LOWER(COALESCE(a.status, '')) IN ('offline', 'failed', 'error', 'blocked')
			   OR LOWER(COALESCE(ac.status, '')) IN ('failed', 'error', 'offline', 'disconnected')
		) items
		ORDER BY COALESCE(created_at, now()) DESC, board_title ASC
		LIMIT $2
	`, userID, limit)
}

func (r *Repository) ListByBoard(ctx context.Context, boardID string, limit int) ([]Item, error) {
	return r.list(ctx, `
		WITH candidate_boards AS (
			SELECT b.id, b.title
			FROM boards b
			WHERE b.id::text = $1
		)
		SELECT type, severity, entity_kind, entity_id, reason, source, board_id, board_title, task_id, task_title, agent_id, agent_name, title, description, created_at, href
		FROM (
			SELECT
				'blocked_task' AS type,
				'high' AS severity,
				'task' AS entity_kind,
				t.id::text AS entity_id,
				'task_blocked' AS reason,
				'tasks' AS source,
				cb.id::text AS board_id,
				cb.title AS board_title,
				t.id AS task_id,
				t.title AS task_title,
				'' AS agent_id,
				'' AS agent_name,
				('Blocked: ' || t.title) AS title,
				COALESCE(NULLIF(t.blocked_reason, ''), 'Task needs attention') AS description,
				NULL::timestamp AS created_at,
				('/board/' || cb.id::text) AS href
			FROM candidate_boards cb
			JOIN tasks t ON t.board_id = cb.id
			WHERE COALESCE(t.status, '') = 'blocked'

			UNION ALL

			SELECT
				'review_task' AS type,
				'medium' AS severity,
				'task' AS entity_kind,
				t.id::text AS entity_id,
				'task_waiting_review' AS reason,
				'tasks' AS source,
				cb.id::text AS board_id,
				cb.title AS board_title,
				t.id AS task_id,
				t.title AS task_title,
				'' AS agent_id,
				'' AS agent_name,
				('Needs review: ' || t.title) AS title,
				'Task is waiting in review' AS description,
				NULL::timestamp AS created_at,
				('/board/' || cb.id::text) AS href
			FROM candidate_boards cb
			JOIN tasks t ON t.board_id = cb.id
			WHERE COALESCE(t.status, '') = 'review'

			UNION ALL

			SELECT
				'agent_issue' AS type,
				'high' AS severity,
				'agent' AS entity_kind,
				a.id AS entity_id,
				'agent_or_connector_unhealthy' AS reason,
				'agents' AS source,
				cb.id::text AS board_id,
				cb.title AS board_title,
				NULL::int AS task_id,
				'' AS task_title,
				a.id AS agent_id,
				COALESCE(NULLIF(a.display_name, ''), a.id) AS agent_name,
				('Agent issue: ' || COALESCE(NULLIF(a.display_name, ''), a.id)) AS title,
				COALESCE(NULLIF(a.health_note, ''), 'Agent or connector needs attention') AS description,
				COALESCE(a.updated_at, a.created_at) AS created_at,
				('/board/' || cb.id::text || '/agents') AS href
			FROM candidate_boards cb
			JOIN board_agents ba ON ba.board_id = cb.id AND ba.active = true
			JOIN agents a ON a.id = ba.agent_id
			LEFT JOIN agent_connectors ac ON ac.agent_id = a.id
			WHERE LOWER(COALESCE(a.status, '')) IN ('offline', 'failed', 'error', 'blocked')
			   OR LOWER(COALESCE(ac.status, '')) IN ('failed', 'error', 'offline', 'disconnected')
		) items
		ORDER BY COALESCE(created_at, now()) DESC, board_title ASC
		LIMIT $2
	`, boardID, limit)
}

func (r *Repository) list(ctx context.Context, query string, arg1 string, limit int) ([]Item, error) {
	rows, err := r.db.Query(ctx, query, arg1, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.Type, &item.Severity, &item.EntityKind, &item.EntityID, &item.Reason, &item.Source, &item.BoardID, &item.BoardTitle, &item.TaskID, &item.TaskTitle, &item.AgentID, &item.AgentName, &item.Title, &item.Description, &item.CreatedAt, &item.Href); err != nil {
			return nil, err
		}
		item.TaskTitle = strings.TrimSpace(item.TaskTitle)
		items = append(items, item)
	}
	if items == nil {
		items = []Item{}
	}
	return items, nil
}
