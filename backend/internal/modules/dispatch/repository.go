package dispatch

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) ListPendingTasks(ctx context.Context) ([]Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.board_id::text, t.title, t.description, t.list_id, t.position, t.assignee_id, t.parent_id, t.status, t.blocked_reason
		FROM tasks t
		JOIN board_agents ba ON ba.agent_id = t.assignee_id AND ba.board_id = t.board_id
		LEFT JOIN agent_runs ar ON ar.task_id = t.id AND ar.agent_id = t.assignee_id AND ar.status IN ('queued','running','in_progress','blocked','review')
		WHERE t.assignee_id IS NOT NULL
		  AND t.list_id = 'todo'
		  AND ba.auto_accept_tasks = true
		  AND ar.id IS NULL
		ORDER BY t.position ASC, t.id ASC
		LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.BoardID, &t.Title, &t.Description, &t.ListID, &t.Position, &t.AssigneeID, &t.ParentID, &t.Status, &t.BlockedReason); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, nil
}

func (r *Repository) MarkTaskDoing(ctx context.Context, taskID int) error {
	_, err := r.db.Exec(ctx, `UPDATE tasks SET list_id=$1 WHERE id=$2`, "doing", taskID)
	return err
}

func (r *Repository) GetConnectorForAgent(ctx context.Context, agentID string) (*AgentConnector, error) {
	var conn AgentConnector
	err := r.db.QueryRow(ctx, `
		SELECT id, agent_id, connector_type, transport_mode, auth_type, endpoint_url, base_url, agent_ref, session_key, status, metadata_json::text, last_success_at, last_error, created_at, updated_at
		FROM agent_connectors
		WHERE agent_id=$1 AND status='connected'
		ORDER BY is_primary DESC, updated_at DESC, id DESC
		LIMIT 1`, agentID).
		Scan(&conn.ID, &conn.AgentID, &conn.ConnectorType, &conn.TransportMode, &conn.AuthType, &conn.EndpointURL, &conn.BaseURL, &conn.AgentRef, &conn.SessionKey, &conn.Status, &conn.MetadataJSON, &conn.LastSuccessAt, &conn.LastError, &conn.CreatedAt, &conn.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &conn, nil
}
