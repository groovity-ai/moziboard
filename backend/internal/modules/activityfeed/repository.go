package activityfeed

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

func (r *Repository) ListRecentByUser(ctx context.Context, userID string, limit int) ([]Item, error) {
	return r.list(ctx, `
		WITH user_boards AS (
			SELECT b.id, b.title
			FROM boards b
			WHERE COALESCE(b.user_id, '') = $1
			   OR EXISTS (
				   SELECT 1 FROM board_members bm
				   WHERE bm.board_id = b.id AND bm.member_id = $1
			   )
		),
		recent_task_activity AS (
			SELECT DISTINCT ON (t.id)
				'task_activity' AS type,
				'task' AS entity_kind,
				t.id::text AS entity_id,
				COALESCE(NULLIF(ta.action, ''), 'task_updated') AS reason,
				'activities' AS source,
				ub.id::text AS board_id,
				ub.title AS board_title,
				t.id AS task_id,
				t.title AS task_title,
				CASE
					WHEN COALESCE(ta.details, '') <> '' THEN ta.details
					ELSE ('Task activity on ' || t.title)
				END AS title,
				ta.created_at,
				('/board/' || ub.id::text) AS href
			FROM user_boards ub
			JOIN tasks t ON t.board_id = ub.id
			JOIN activities ta ON ta.task_id = t.id
			ORDER BY t.id, ta.created_at DESC
		),
		recent_documents AS (
			SELECT
				'document' AS type,
				'document' AS entity_kind,
				d.id::text AS entity_id,
				'document_updated' AS reason,
				'documents' AS source,
				ub.id::text AS board_id,
				ub.title AS board_title,
				NULL::int AS task_id,
				'' AS task_title,
				('Doc updated: ' || d.title) AS title,
				d.updated_at AS created_at,
				('/board/' || ub.id::text || '/docs') AS href
			FROM user_boards ub
			JOIN documents d ON d.board_id = ub.id
		),
		recent_failed_events AS (
			SELECT
				'agent_event_failure' AS type,
				'agent_event' AS entity_kind,
				ae.id::text AS entity_id,
				COALESCE(NULLIF(ae.event_type, ''), 'delivery_failed') AS reason,
				'agent_events' AS source,
				ub.id::text AS board_id,
				ub.title AS board_title,
				ae.task_id,
				COALESCE(t.title, '') AS task_title,
				('Delivery issue: ' || ae.event_type) AS title,
				COALESCE(ae.last_delivery_at, ae.created_at) AS created_at,
				('/board/' || ub.id::text || '/ops') AS href
			FROM user_boards ub
			JOIN agent_events ae ON ae.board_id = ub.id
			LEFT JOIN tasks t ON t.id = ae.task_id
			WHERE COALESCE(ae.delivery_status, '') IN ('failed', 'dead')
		)
		SELECT type, entity_kind, entity_id, reason, source, board_id, board_title, task_id, task_title, title, created_at, href
		FROM (
			SELECT * FROM recent_task_activity
			UNION ALL
			SELECT * FROM recent_documents
			UNION ALL
			SELECT * FROM recent_failed_events
		) items
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
}

func (r *Repository) ListRecentByBoard(ctx context.Context, boardID string, limit int) ([]Item, error) {
	return r.list(ctx, `
		WITH scoped_board AS (
			SELECT b.id, b.title
			FROM boards b
			WHERE b.id::text = $1
		),
		recent_task_activity AS (
			SELECT DISTINCT ON (t.id)
				'task_activity' AS type,
				'task' AS entity_kind,
				t.id::text AS entity_id,
				COALESCE(NULLIF(ta.action, ''), 'task_updated') AS reason,
				'activities' AS source,
				sb.id::text AS board_id,
				sb.title AS board_title,
				t.id AS task_id,
				t.title AS task_title,
				CASE
					WHEN COALESCE(ta.details, '') <> '' THEN ta.details
					ELSE ('Task activity on ' || t.title)
				END AS title,
				ta.created_at,
				('/board/' || sb.id::text) AS href
			FROM scoped_board sb
			JOIN tasks t ON t.board_id = sb.id
			JOIN activities ta ON ta.task_id = t.id
			ORDER BY t.id, ta.created_at DESC
		),
		recent_documents AS (
			SELECT
				'document' AS type,
				'document' AS entity_kind,
				d.id::text AS entity_id,
				'document_updated' AS reason,
				'documents' AS source,
				sb.id::text AS board_id,
				sb.title AS board_title,
				NULL::int AS task_id,
				'' AS task_title,
				('Doc updated: ' || d.title) AS title,
				d.updated_at AS created_at,
				('/board/' || sb.id::text || '/docs') AS href
			FROM scoped_board sb
			JOIN documents d ON d.board_id = sb.id
		),
		recent_failed_events AS (
			SELECT
				'agent_event_failure' AS type,
				'agent_event' AS entity_kind,
				ae.id::text AS entity_id,
				COALESCE(NULLIF(ae.event_type, ''), 'delivery_failed') AS reason,
				'agent_events' AS source,
				sb.id::text AS board_id,
				sb.title AS board_title,
				ae.task_id,
				COALESCE(t.title, '') AS task_title,
				('Delivery issue: ' || ae.event_type) AS title,
				COALESCE(ae.last_delivery_at, ae.created_at) AS created_at,
				('/board/' || sb.id::text || '/ops') AS href
			FROM scoped_board sb
			JOIN agent_events ae ON ae.board_id = sb.id
			LEFT JOIN tasks t ON t.id = ae.task_id
			WHERE COALESCE(ae.delivery_status, '') IN ('failed', 'dead')
		)
		SELECT type, entity_kind, entity_id, reason, source, board_id, board_title, task_id, task_title, title, created_at, href
		FROM (
			SELECT * FROM recent_task_activity
			UNION ALL
			SELECT * FROM recent_documents
			UNION ALL
			SELECT * FROM recent_failed_events
		) items
		ORDER BY created_at DESC
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
		if err := rows.Scan(&item.Type, &item.EntityKind, &item.EntityID, &item.Reason, &item.Source, &item.BoardID, &item.BoardTitle, &item.TaskID, &item.TaskTitle, &item.Title, &item.CreatedAt, &item.Href); err != nil {
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
