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

func (r *Repository) ListAuditLogs(ctx context.Context, entityType, entityID, action, boardID string) ([]AuditLog, error) {
	query := `SELECT id, actor_type, actor_id, entity_type, entity_id, action, old_value_json::text, new_value_json::text, metadata_json::text, created_at FROM audit_logs WHERE 1=1`
	args := []interface{}{}
	idx := 1
	if strings.TrimSpace(entityType) != "" {
		query += fmt.Sprintf(" AND entity_type=$%d", idx)
		args = append(args, entityType)
		idx++
	}
	if strings.TrimSpace(entityID) != "" {
		query += fmt.Sprintf(" AND entity_id=$%d", idx)
		args = append(args, entityID)
		idx++
	}
	if strings.TrimSpace(action) != "" {
		query += fmt.Sprintf(" AND action=$%d", idx)
		args = append(args, action)
		idx++
	}
	if strings.TrimSpace(boardID) != "" {
		query += fmt.Sprintf(" AND (entity_id=$%d OR metadata_json::text ILIKE $%d OR new_value_json::text ILIKE $%d OR old_value_json::text ILIKE $%d)", idx, idx+1, idx+1, idx+1)
		args = append(args, boardID, "%"+boardID+"%")
		idx += 2
	}
	query += " ORDER BY created_at DESC, id DESC LIMIT 200"
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AuditLog{}
	for rows.Next() {
		var a AuditLog
		if err := rows.Scan(&a.ID, &a.ActorType, &a.ActorID, &a.EntityType, &a.EntityID, &a.Action, &a.OldValueJSON, &a.NewValueJSON, &a.MetadataJSON, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
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

func (r *Repository) GetEventBoardContext(ctx context.Context, eventID string) (map[string]interface{}, error) {
	metadata := map[string]interface{}{}
	var boardID *string
	var agentID string
	var taskID *int
	err := r.db.QueryRow(ctx, `SELECT board_id::text, agent_id, task_id FROM agent_events WHERE id=$1`, eventID).Scan(&boardID, &agentID, &taskID)
	if err != nil {
		return nil, err
	}
	if boardID != nil && strings.TrimSpace(*boardID) != "" {
		metadata["board_id"] = *boardID
	}
	metadata["agent_id"] = agentID
	if taskID != nil {
		metadata["task_id"] = *taskID
	}
	return metadata, nil
}

func (r *Repository) GetConnectorBoardContext(ctx context.Context, connectorID string) (map[string]interface{}, error) {
	metadata := map[string]interface{}{}
	var agentID string
	err := r.db.QueryRow(ctx, `SELECT agent_id FROM agent_connectors WHERE id=$1`, connectorID).Scan(&agentID)
	if err != nil {
		return nil, err
	}
	metadata["agent_id"] = agentID
	rows, err := r.db.Query(ctx, `SELECT board_id::text FROM board_agents WHERE agent_id=$1 ORDER BY board_id::text`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	boardIDs := []string{}
	for rows.Next() {
		var boardID string
		if err := rows.Scan(&boardID); err != nil {
			return nil, err
		}
		boardIDs = append(boardIDs, boardID)
	}
	if len(boardIDs) == 1 {
		metadata["board_id"] = boardIDs[0]
	}
	if len(boardIDs) > 0 {
		metadata["board_ids"] = boardIDs
	}
	return metadata, nil
}

func (r *Repository) MarkStaleAgentsOffline(ctx context.Context) (int, error) {
	cmd, err := r.db.Exec(ctx, `UPDATE agents SET status='offline', current_activity='Heartbeat stale', updated_at=CURRENT_TIMESTAMP WHERE status NOT IN ('offline') AND last_heartbeat_at IS NOT NULL AND last_heartbeat_at < CURRENT_TIMESTAMP - INTERVAL '15 minutes'`)
	if err != nil {
		return 0, err
	}
	return int(cmd.RowsAffected()), nil
}

func (r *Repository) CloseStaleRuns(ctx context.Context) (int, error) {
	cmd, err := r.db.Exec(ctx, `UPDATE agent_runs SET status='failed', error_summary=CASE WHEN error_summary='' THEN 'stale_run_timeout' ELSE error_summary END, ended_at=CURRENT_TIMESTAMP WHERE status IN ('queued','running','in_progress','blocked','review') AND started_at < CURRENT_TIMESTAMP - INTERVAL '6 hours'`)
	if err != nil {
		return 0, err
	}
	return int(cmd.RowsAffected()), nil
}

func (r *Repository) RepairTaskRunDrift(ctx context.Context) (int, error) {
	cmd, err := r.db.Exec(ctx, `UPDATE agent_runs ar SET status='done', current_activity='Reconciled to task.done', ended_at=COALESCE(ar.ended_at, CURRENT_TIMESTAMP)
FROM tasks t
WHERE ar.task_id = t.id
  AND t.status = 'done'
  AND ar.status IN ('queued','running','in_progress','blocked','review')`)
	if err != nil {
		return 0, err
	}
	return int(cmd.RowsAffected()), nil
}

func (r *Repository) GetOpsSummary(ctx context.Context, boardID string) (OpsSummary, error) {
	summary := OpsSummary{}
	boardID = strings.TrimSpace(boardID)
	if boardID == "" {
		query := `
SELECT
	CURRENT_TIMESTAMP,
	(SELECT COUNT(*) FROM agents),
	(SELECT COUNT(*) FROM agents WHERE status = 'online'),
	(SELECT COUNT(*) FROM agents WHERE status = 'offline'),
	(SELECT COUNT(*) FROM agents WHERE last_heartbeat_at IS NOT NULL AND last_heartbeat_at < CURRENT_TIMESTAMP - INTERVAL '15 minutes' AND status <> 'offline'),
	(SELECT COUNT(*) FROM agents WHERE status = 'blocked'),
	(SELECT COUNT(*) FROM agents WHERE COALESCE(health_note, '') <> ''),
	(SELECT COUNT(*) FROM agent_runs),
	(SELECT COUNT(*) FROM agent_runs WHERE status IN ('queued','running','in_progress','blocked','review') AND ended_at IS NULL),
	(SELECT COUNT(*) FROM agent_runs WHERE status IN ('queued','running','in_progress','blocked','review') AND started_at < CURRENT_TIMESTAMP - INTERVAL '6 hours'),
	(SELECT COUNT(*) FROM agent_runs WHERE status = 'failed'),
	(SELECT COUNT(*) FROM agent_runs WHERE status = 'review'),
	(SELECT COUNT(*) FROM agent_runs WHERE status = 'blocked'),
	(SELECT COUNT(*) FROM agent_events WHERE delivery_status = 'pending'),
	(SELECT COUNT(*) FROM agent_events WHERE delivery_status = 'processing'),
	(SELECT COUNT(*) FROM agent_events WHERE delivery_status = 'failed'),
	(SELECT COUNT(*) FROM agent_events WHERE delivery_status = 'dead'),
	(SELECT COUNT(*) FROM agent_events WHERE delivery_status = 'sent'),
	(SELECT COUNT(*) FROM agent_events WHERE delivery_status = 'routed'),
	(SELECT COUNT(*) FROM agent_events WHERE delivery_status = 'ignored'),
	(SELECT COUNT(*) FROM agent_events WHERE delivery_status IN ('pending','failed') AND (next_attempt_at IS NULL OR next_attempt_at <= CURRENT_TIMESTAMP)),
	(SELECT COUNT(*) FROM agent_connectors),
	(SELECT COUNT(*) FROM agent_connectors WHERE status = 'connected'),
	(SELECT COUNT(*) FROM agent_connectors WHERE status = 'disabled'),
	(SELECT COUNT(*) FROM agent_connectors WHERE connector_type = 'webhook'),
	(SELECT COUNT(*) FROM agent_connectors WHERE transport_mode = 'internal'),
	(SELECT COUNT(*) FROM agent_connectors WHERE COALESCE(last_error, '') <> ''),
	(SELECT COUNT(*) FROM audit_logs WHERE created_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours'),
	(SELECT COUNT(*) FROM audit_logs WHERE entity_type = 'maintenance' AND created_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours'),
	(SELECT COUNT(*) FROM audit_logs WHERE actor_id = 'ops' AND created_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours'),
	(SELECT COUNT(*) FROM audit_logs WHERE actor_type = 'agent' AND created_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours')
`
		err := r.db.QueryRow(ctx, query).Scan(
			&summary.GeneratedAt,
			&summary.Agents.Total,
			&summary.Agents.Online,
			&summary.Agents.Offline,
			&summary.Agents.Stale,
			&summary.Agents.Blocked,
			&summary.Agents.WithIssues,
			&summary.Runs.Total,
			&summary.Runs.Active,
			&summary.Runs.Stale,
			&summary.Runs.Failed,
			&summary.Runs.Review,
			&summary.Runs.Blocked,
			&summary.Events.Pending,
			&summary.Events.Processing,
			&summary.Events.Failed,
			&summary.Events.Dead,
			&summary.Events.Sent,
			&summary.Events.Routed,
			&summary.Events.Ignored,
			&summary.Events.RetryDue,
			&summary.Connectors.Total,
			&summary.Connectors.Connected,
			&summary.Connectors.Disabled,
			&summary.Connectors.Webhook,
			&summary.Connectors.Internal,
			&summary.Connectors.WithErrors,
			&summary.AuditLogs.Total24h,
			&summary.AuditLogs.Maintenance24h,
			&summary.AuditLogs.OpsActions24h,
			&summary.AuditLogs.RuntimeEvents24h,
		)
		return summary, err
	}

	query := `
WITH scoped_board_agents AS (
	SELECT DISTINCT ba.agent_id
	FROM board_agents ba
	WHERE ba.board_id::text = $1
),
board_runs AS (
	SELECT ar.*
	FROM agent_runs ar
	JOIN tasks t ON t.id = ar.task_id
	WHERE t.board_id::text = $1
),
board_events AS (
	SELECT ae.*
	FROM agent_events ae
	WHERE ae.board_id::text = $1
),
board_connectors AS (
	SELECT ac.*
	FROM agent_connectors ac
	JOIN scoped_board_agents ba ON ba.agent_id = ac.agent_id
),
board_agent_rows AS (
	SELECT a.*
	FROM agents a
	JOIN scoped_board_agents ba ON ba.agent_id = a.id
)
SELECT
	CURRENT_TIMESTAMP,
	(SELECT COUNT(*) FROM scoped_board_agents),
	(SELECT COUNT(*) FROM board_agent_rows WHERE status = 'online'),
	(SELECT COUNT(*) FROM board_agent_rows WHERE status = 'offline'),
	(SELECT COUNT(*) FROM board_agent_rows WHERE last_heartbeat_at IS NOT NULL AND last_heartbeat_at < CURRENT_TIMESTAMP - INTERVAL '15 minutes' AND status <> 'offline'),
	(SELECT COUNT(*) FROM board_agent_rows WHERE status = 'blocked'),
	(SELECT COUNT(*) FROM board_agent_rows WHERE COALESCE(health_note, '') <> ''),
	(SELECT COUNT(*) FROM board_runs),
	(SELECT COUNT(*) FROM board_runs WHERE status IN ('queued','running','in_progress','blocked','review') AND ended_at IS NULL),
	(SELECT COUNT(*) FROM board_runs WHERE status IN ('queued','running','in_progress','blocked','review') AND started_at < CURRENT_TIMESTAMP - INTERVAL '6 hours'),
	(SELECT COUNT(*) FROM board_runs WHERE status = 'failed'),
	(SELECT COUNT(*) FROM board_runs WHERE status = 'review'),
	(SELECT COUNT(*) FROM board_runs WHERE status = 'blocked'),
	(SELECT COUNT(*) FROM board_events WHERE delivery_status = 'pending'),
	(SELECT COUNT(*) FROM board_events WHERE delivery_status = 'processing'),
	(SELECT COUNT(*) FROM board_events WHERE delivery_status = 'failed'),
	(SELECT COUNT(*) FROM board_events WHERE delivery_status = 'dead'),
	(SELECT COUNT(*) FROM board_events WHERE delivery_status = 'sent'),
	(SELECT COUNT(*) FROM board_events WHERE delivery_status = 'routed'),
	(SELECT COUNT(*) FROM board_events WHERE delivery_status = 'ignored'),
	(SELECT COUNT(*) FROM board_events WHERE delivery_status IN ('pending','failed') AND (next_attempt_at IS NULL OR next_attempt_at <= CURRENT_TIMESTAMP)),
	(SELECT COUNT(*) FROM board_connectors),
	(SELECT COUNT(*) FROM board_connectors WHERE status = 'connected'),
	(SELECT COUNT(*) FROM board_connectors WHERE status = 'disabled'),
	(SELECT COUNT(*) FROM board_connectors WHERE connector_type = 'webhook'),
	(SELECT COUNT(*) FROM board_connectors WHERE transport_mode = 'internal'),
	(SELECT COUNT(*) FROM board_connectors WHERE COALESCE(last_error, '') <> ''),
	(SELECT COUNT(*) FROM audit_logs WHERE created_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours' AND (entity_id = $1 OR metadata_json::text ILIKE '%' || $1 || '%')),
	(SELECT COUNT(*) FROM audit_logs WHERE entity_type = 'maintenance' AND created_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours'),
	(SELECT COUNT(*) FROM audit_logs WHERE actor_id = 'ops' AND created_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours'),
	(SELECT COUNT(*) FROM audit_logs WHERE actor_type = 'agent' AND created_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours')
`
	err := r.db.QueryRow(ctx, query, boardID).Scan(
		&summary.GeneratedAt,
		&summary.Agents.Total,
		&summary.Agents.Online,
		&summary.Agents.Offline,
		&summary.Agents.Stale,
		&summary.Agents.Blocked,
		&summary.Agents.WithIssues,
		&summary.Runs.Total,
		&summary.Runs.Active,
		&summary.Runs.Stale,
		&summary.Runs.Failed,
		&summary.Runs.Review,
		&summary.Runs.Blocked,
		&summary.Events.Pending,
		&summary.Events.Processing,
		&summary.Events.Failed,
		&summary.Events.Dead,
		&summary.Events.Sent,
		&summary.Events.Routed,
		&summary.Events.Ignored,
		&summary.Events.RetryDue,
		&summary.Connectors.Total,
		&summary.Connectors.Connected,
		&summary.Connectors.Disabled,
		&summary.Connectors.Webhook,
		&summary.Connectors.Internal,
		&summary.Connectors.WithErrors,
		&summary.AuditLogs.Total24h,
		&summary.AuditLogs.Maintenance24h,
		&summary.AuditLogs.OpsActions24h,
		&summary.AuditLogs.RuntimeEvents24h,
	)
	return summary, err
}
