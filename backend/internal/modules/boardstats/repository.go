package boardstats

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

func (r *Repository) RecomputeAll(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO board_stats (
			board_id,
			open_tasks,
			review_tasks,
			blocked_tasks,
			docs_count,
			connected_agents,
			agent_issues,
			last_task_event_at,
			last_doc_event_at,
			last_agent_event_at,
			last_event_at,
			computed_at
		)
		WITH task_counts AS (
			SELECT
				t.board_id,
				COUNT(*) FILTER (WHERE COALESCE(t.status, '') IN ('todo', 'backlog', 'in_progress', 'assigned')) AS open_tasks,
				COUNT(*) FILTER (WHERE COALESCE(t.status, '') = 'review') AS review_tasks,
				COUNT(*) FILTER (WHERE COALESCE(t.status, '') = 'blocked') AS blocked_tasks,
				MAX(ta.created_at) AS last_task_event_at
			FROM tasks t
			LEFT JOIN activities ta ON ta.task_id = t.id
			GROUP BY t.board_id
		),
		 doc_counts AS (
			SELECT d.board_id, COUNT(*) AS docs_count, MAX(d.updated_at) AS last_doc_event_at
			FROM documents d
			GROUP BY d.board_id
		),
		 agent_counts AS (
			SELECT
				ba.board_id,
				COUNT(*) FILTER (WHERE ba.active = true) AS connected_agents,
				COUNT(*) FILTER (
					WHERE ba.active = true AND (
						LOWER(COALESCE(a.status, '')) IN ('offline', 'failed', 'error', 'blocked')
						OR LOWER(COALESCE(ac.status, '')) IN ('failed', 'error', 'offline', 'disconnected')
					)
				) AS agent_issues,
				MAX(GREATEST(
					COALESCE(a.updated_at, to_timestamp(0)),
					COALESCE(ac.updated_at, to_timestamp(0))
				)) AS last_agent_event_at
			FROM board_agents ba
			LEFT JOIN agents a ON a.id = ba.agent_id
			LEFT JOIN agent_connectors ac ON ac.agent_id = ba.agent_id
			GROUP BY ba.board_id
		),
		 event_activity AS (
			SELECT ae.board_id, MAX(COALESCE(ae.last_delivery_at, ae.created_at)) AS last_event_at
			FROM agent_events ae
			WHERE ae.board_id IS NOT NULL
			GROUP BY ae.board_id
		)
		SELECT
			b.id,
			COALESCE(tc.open_tasks, 0) AS open_tasks,
			COALESCE(tc.review_tasks, 0) AS review_tasks,
			COALESCE(tc.blocked_tasks, 0) AS blocked_tasks,
			COALESCE(dc.docs_count, 0) AS docs_count,
			COALESCE(ac.connected_agents, 0) AS connected_agents,
			COALESCE(ac.agent_issues, 0) AS agent_issues,
			tc.last_task_event_at,
			dc.last_doc_event_at,
			ac.last_agent_event_at,
			NULLIF(GREATEST(
				COALESCE(tc.last_task_event_at, to_timestamp(0)),
				COALESCE(dc.last_doc_event_at, to_timestamp(0)),
				COALESCE(ac.last_agent_event_at, to_timestamp(0)),
				COALESCE(ea.last_event_at, to_timestamp(0)),
				COALESCE(b.created_at, to_timestamp(0))
			), to_timestamp(0)) AS last_event_at,
			CURRENT_TIMESTAMP AS computed_at
		FROM boards b
		LEFT JOIN task_counts tc ON tc.board_id = b.id
		LEFT JOIN doc_counts dc ON dc.board_id = b.id
		LEFT JOIN agent_counts ac ON ac.board_id = b.id
		LEFT JOIN event_activity ea ON ea.board_id = b.id
		ON CONFLICT (board_id) DO UPDATE SET
			open_tasks = EXCLUDED.open_tasks,
			review_tasks = EXCLUDED.review_tasks,
			blocked_tasks = EXCLUDED.blocked_tasks,
			docs_count = EXCLUDED.docs_count,
			connected_agents = EXCLUDED.connected_agents,
			agent_issues = EXCLUDED.agent_issues,
			last_task_event_at = EXCLUDED.last_task_event_at,
			last_doc_event_at = EXCLUDED.last_doc_event_at,
			last_agent_event_at = EXCLUDED.last_agent_event_at,
			last_event_at = EXCLUDED.last_event_at,
			computed_at = EXCLUDED.computed_at
	`)
	return err
}

func (r *Repository) RecomputeBoard(ctx context.Context, boardID string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO board_stats (
			board_id,
			open_tasks,
			review_tasks,
			blocked_tasks,
			docs_count,
			connected_agents,
			agent_issues,
			last_task_event_at,
			last_doc_event_at,
			last_agent_event_at,
			last_event_at,
			computed_at
		)
		WITH scoped_board AS (
			SELECT id, created_at FROM boards WHERE id::text = $1
		),
		 task_counts AS (
			SELECT
				t.board_id,
				COUNT(*) FILTER (WHERE COALESCE(t.status, '') IN ('todo', 'backlog', 'in_progress', 'assigned')) AS open_tasks,
				COUNT(*) FILTER (WHERE COALESCE(t.status, '') = 'review') AS review_tasks,
				COUNT(*) FILTER (WHERE COALESCE(t.status, '') = 'blocked') AS blocked_tasks,
				MAX(ta.created_at) AS last_task_event_at
			FROM tasks t
			LEFT JOIN activities ta ON ta.task_id = t.id
			WHERE t.board_id IN (SELECT id FROM scoped_board)
			GROUP BY t.board_id
		),
		 doc_counts AS (
			SELECT d.board_id, COUNT(*) AS docs_count, MAX(d.updated_at) AS last_doc_event_at
			FROM documents d
			WHERE d.board_id IN (SELECT id FROM scoped_board)
			GROUP BY d.board_id
		),
		 agent_counts AS (
			SELECT
				ba.board_id,
				COUNT(*) FILTER (WHERE ba.active = true) AS connected_agents,
				COUNT(*) FILTER (
					WHERE ba.active = true AND (
						LOWER(COALESCE(a.status, '')) IN ('offline', 'failed', 'error', 'blocked')
						OR LOWER(COALESCE(ac.status, '')) IN ('failed', 'error', 'offline', 'disconnected')
					)
				) AS agent_issues,
				MAX(GREATEST(
					COALESCE(a.updated_at, to_timestamp(0)),
					COALESCE(ac.updated_at, to_timestamp(0))
				)) AS last_agent_event_at
			FROM board_agents ba
			LEFT JOIN agents a ON a.id = ba.agent_id
			LEFT JOIN agent_connectors ac ON ac.agent_id = ba.agent_id
			WHERE ba.board_id IN (SELECT id FROM scoped_board)
			GROUP BY ba.board_id
		),
		 event_activity AS (
			SELECT ae.board_id, MAX(COALESCE(ae.last_delivery_at, ae.created_at)) AS last_event_at
			FROM agent_events ae
			WHERE ae.board_id IN (SELECT id FROM scoped_board)
			GROUP BY ae.board_id
		)
		SELECT
			sb.id,
			COALESCE(tc.open_tasks, 0) AS open_tasks,
			COALESCE(tc.review_tasks, 0) AS review_tasks,
			COALESCE(tc.blocked_tasks, 0) AS blocked_tasks,
			COALESCE(dc.docs_count, 0) AS docs_count,
			COALESCE(ac.connected_agents, 0) AS connected_agents,
			COALESCE(ac.agent_issues, 0) AS agent_issues,
			tc.last_task_event_at,
			dc.last_doc_event_at,
			ac.last_agent_event_at,
			NULLIF(GREATEST(
				COALESCE(tc.last_task_event_at, to_timestamp(0)),
				COALESCE(dc.last_doc_event_at, to_timestamp(0)),
				COALESCE(ac.last_agent_event_at, to_timestamp(0)),
				COALESCE(ea.last_event_at, to_timestamp(0)),
				COALESCE(sb.created_at, to_timestamp(0))
			), to_timestamp(0)) AS last_event_at,
			CURRENT_TIMESTAMP AS computed_at
		FROM scoped_board sb
		LEFT JOIN task_counts tc ON tc.board_id = sb.id
		LEFT JOIN doc_counts dc ON dc.board_id = sb.id
		LEFT JOIN agent_counts ac ON ac.board_id = sb.id
		LEFT JOIN event_activity ea ON ea.board_id = sb.id
		ON CONFLICT (board_id) DO UPDATE SET
			open_tasks = EXCLUDED.open_tasks,
			review_tasks = EXCLUDED.review_tasks,
			blocked_tasks = EXCLUDED.blocked_tasks,
			docs_count = EXCLUDED.docs_count,
			connected_agents = EXCLUDED.connected_agents,
			agent_issues = EXCLUDED.agent_issues,
			last_task_event_at = EXCLUDED.last_task_event_at,
			last_doc_event_at = EXCLUDED.last_doc_event_at,
			last_agent_event_at = EXCLUDED.last_agent_event_at,
			last_event_at = EXCLUDED.last_event_at,
			computed_at = EXCLUDED.computed_at
	`, boardID)
	return err
}

func (r *Repository) GetByBoard(ctx context.Context, boardID string) (*BoardStats, error) {
	row := r.db.QueryRow(ctx, `
		SELECT board_id::text, open_tasks, review_tasks, blocked_tasks, docs_count, connected_agents, agent_issues,
		       last_task_event_at, last_doc_event_at, last_agent_event_at, last_event_at, computed_at
		FROM board_stats
		WHERE board_id::text = $1
	`, boardID)
	var item BoardStats
	if err := row.Scan(
		&item.BoardID,
		&item.OpenTasks,
		&item.ReviewTasks,
		&item.BlockedTasks,
		&item.DocsCount,
		&item.ConnectedAgents,
		&item.AgentIssues,
		&item.LastTaskEventAt,
		&item.LastDocEventAt,
		&item.LastAgentEventAt,
		&item.LastEventAt,
		&item.ComputedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}
