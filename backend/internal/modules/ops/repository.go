package ops

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	agentsmodule "moziboard-backend/internal/modules/agents"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) ListAgentEvents(ctx context.Context, status, agentID, eventType, boardID string) ([]AgentEvent, error) {
	query := `SELECT id, agent_id, board_id::text, task_id, event_type, payload_json::text, delivery_status, delivery_attempts, next_attempt_at, last_delivery_at, response_status, response_body, created_at, processed_at FROM agent_events WHERE 1=1`
	args := []interface{}{}
	idx := 1
	if strings.TrimSpace(status) != "" {
		query += fmt.Sprintf(" AND delivery_status=$%d", idx)
		args = append(args, status)
		idx++
	}
	if strings.TrimSpace(agentID) != "" {
		query += fmt.Sprintf(" AND agent_id=$%d", idx)
		args = append(args, agentID)
		idx++
	}
	if strings.TrimSpace(eventType) != "" {
		query += fmt.Sprintf(" AND event_type=$%d", idx)
		args = append(args, eventType)
		idx++
	}
	if strings.TrimSpace(boardID) != "" {
		query += fmt.Sprintf(" AND board_id::text=$%d", idx)
		args = append(args, boardID)
		idx++
	}
	query += " ORDER BY created_at DESC LIMIT 100"
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []AgentEvent
	for rows.Next() {
		var ev AgentEvent
		if err := rows.Scan(&ev.ID, &ev.AgentID, &ev.BoardID, &ev.TaskID, &ev.EventType, &ev.PayloadJSON, &ev.DeliveryStatus, &ev.DeliveryAttempts, &ev.NextAttemptAt, &ev.LastDeliveryAt, &ev.ResponseStatus, &ev.ResponseBody, &ev.CreatedAt, &ev.ProcessedAt); err != nil {
			return nil, err
		}
		items = append(items, ev)
	}
	if items == nil {
		items = []AgentEvent{}
	}
	return items, nil
}

func (r *Repository) ListConnectors(ctx context.Context, status, agentID, connectorType, transportMode string) ([]agentsmodule.AgentConnector, error) {
	query := `SELECT id, agent_id, connector_type, transport_mode, auth_type, endpoint_url, base_url, agent_ref, session_key, status, metadata_json::text, last_success_at, last_error, created_at, updated_at FROM agent_connectors WHERE 1=1`
	args := []interface{}{}
	idx := 1
	if strings.TrimSpace(status) != "" {
		query += fmt.Sprintf(" AND status=$%d", idx)
		args = append(args, status)
		idx++
	}
	if strings.TrimSpace(agentID) != "" {
		query += fmt.Sprintf(" AND agent_id=$%d", idx)
		args = append(args, agentID)
		idx++
	}
	if strings.TrimSpace(connectorType) != "" {
		query += fmt.Sprintf(" AND connector_type=$%d", idx)
		args = append(args, connectorType)
		idx++
	}
	if strings.TrimSpace(transportMode) != "" {
		query += fmt.Sprintf(" AND transport_mode=$%d", idx)
		args = append(args, transportMode)
		idx++
	}
	query += " ORDER BY updated_at DESC, id DESC LIMIT 100"
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []agentsmodule.AgentConnector
	for rows.Next() {
		var conn agentsmodule.AgentConnector
		if err := rows.Scan(&conn.ID, &conn.AgentID, &conn.ConnectorType, &conn.TransportMode, &conn.AuthType, &conn.EndpointURL, &conn.BaseURL, &conn.AgentRef, &conn.SessionKey, &conn.Status, &conn.MetadataJSON, &conn.LastSuccessAt, &conn.LastError, &conn.CreatedAt, &conn.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, conn)
	}
	if items == nil {
		items = []agentsmodule.AgentConnector{}
	}
	return items, nil
}

func (r *Repository) RetryEvent(ctx context.Context, eventID string) (bool, error) {
	cmd, err := r.db.Exec(ctx, `UPDATE agent_events SET delivery_status='pending', next_attempt_at=CURRENT_TIMESTAMP, processed_at=NULL, response_status='manual_retry_requested', response_body='' WHERE id=$1 AND delivery_status IN ('failed','processing')`, eventID)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}

func (r *Repository) RequeueEvent(ctx context.Context, eventID string) (bool, error) {
	cmd, err := r.db.Exec(ctx, `UPDATE agent_events SET delivery_status='pending', delivery_attempts=0, next_attempt_at=CURRENT_TIMESTAMP, processed_at=NULL, response_status='manual_requeue_requested', response_body='' WHERE id=$1 AND delivery_status IN ('dead','failed','ignored','routed')`, eventID)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}

func (r *Repository) SetConnectorStatus(ctx context.Context, connectorID, status string) (bool, error) {
	cmd, err := r.db.Exec(ctx, `UPDATE agent_connectors SET status=$2, updated_at=CURRENT_TIMESTAMP WHERE id=$1`, connectorID, status)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}
