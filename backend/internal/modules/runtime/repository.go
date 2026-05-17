package runtime

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	agentsmodule "moziboard-backend/internal/modules/agents"
	"moziboard-backend/internal/workflow"
)

type Repository struct{ db *pgxpool.Pool }

type TaskRecord struct {
	ID         int
	BoardID    string
	Status     string
	AssigneeID *string
}

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) FindConnectorByMachineTokenHash(ctx context.Context, hash string) (*agentsmodule.AgentConnector, error) {
	var conn agentsmodule.AgentConnector
	err := r.db.QueryRow(ctx, `SELECT id, agent_id, connector_type, transport_mode, auth_type, endpoint_url, base_url, agent_ref, session_key, status, metadata_json::text, last_success_at, last_error, created_at, updated_at FROM agent_connectors WHERE machine_token_hash=$1`, hash).
		Scan(&conn.ID, &conn.AgentID, &conn.ConnectorType, &conn.TransportMode, &conn.AuthType, &conn.EndpointURL, &conn.BaseURL, &conn.AgentRef, &conn.SessionKey, &conn.Status, &conn.MetadataJSON, &conn.LastSuccessAt, &conn.LastError, &conn.CreatedAt, &conn.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &conn, nil
}

func (r *Repository) FindSharedSecretConnector(ctx context.Context, agentRef string) (*agentsmodule.AgentConnector, string, error) {
	var conn agentsmodule.AgentConnector
	var sharedSecretHash string
	err := r.db.QueryRow(ctx, `SELECT id, agent_id, connector_type, transport_mode, auth_type, endpoint_url, base_url, agent_ref, session_key, status, metadata_json::text, last_success_at, last_error, created_at, updated_at, shared_secret_hash FROM agent_connectors WHERE agent_ref=$1 AND auth_type='shared_secret' AND status='connected' ORDER BY updated_at DESC, id DESC LIMIT 1`, agentRef).
		Scan(&conn.ID, &conn.AgentID, &conn.ConnectorType, &conn.TransportMode, &conn.AuthType, &conn.EndpointURL, &conn.BaseURL, &conn.AgentRef, &conn.SessionKey, &conn.Status, &conn.MetadataJSON, &conn.LastSuccessAt, &conn.LastError, &conn.CreatedAt, &conn.UpdatedAt, &sharedSecretHash)
	if err != nil {
		return nil, "", err
	}
	return &conn, sharedSecretHash, nil
}

func (r *Repository) EnsureAgentBoardAccess(ctx context.Context, agentID string, taskID int) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tasks t JOIN board_agents ba ON ba.board_id = t.board_id AND ba.agent_id=$1 AND ba.active=true WHERE t.id=$2)`, agentID, taskID).Scan(&ok)
	return ok, err
}

func (r *Repository) GetTaskByID(ctx context.Context, taskID int) (TaskRecord, error) {
	var rec TaskRecord
	err := r.db.QueryRow(ctx, `SELECT id, board_id::text, status, assignee_id FROM tasks WHERE id=$1`, taskID).Scan(&rec.ID, &rec.BoardID, &rec.Status, &rec.AssigneeID)
	return rec, err
}

func (r *Repository) ListActiveRunIDs(ctx context.Context, taskID int, agentID string) ([]int, error) {
	rows, err := r.db.Query(ctx, `SELECT id FROM agent_runs WHERE task_id=$1 AND agent_id=$2 AND status IN ('queued','running','in_progress','blocked','review') ORDER BY started_at DESC`, taskID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (r *Repository) CloseOtherRuns(ctx context.Context, taskID int, agentID string, keepRunID int) error {
	_, err := r.db.Exec(ctx, `UPDATE agent_runs SET status='done', ended_at=CURRENT_TIMESTAMP WHERE task_id=$1 AND agent_id=$2 AND id<>$3 AND status IN ('queued','running','in_progress','blocked','review')`, taskID, agentID, keepRunID)
	return err
}

func (r *Repository) ApplyTaskStateSync(ctx context.Context, taskID int, agentID string, runID int, taskState string, currentActivity string, blockedReason string, resultSummary string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	listID := workflow.StatusToListID(taskState)
	runStatus := workflow.MapTaskStateToRunState(taskState)
	agentStatus, agentActivity, clearCurrent := workflow.DeriveAgentState(taskState)
	if currentActivity == "" {
		currentActivity = agentActivity
	}
	if _, err := tx.Exec(ctx, `UPDATE tasks SET status=$1, list_id=$2, blocked_reason=$3 WHERE id=$4`, taskState, listID, blockedReason, taskID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE agent_runs SET status=$1, current_activity=COALESCE(NULLIF($2,''), current_activity), error_summary=CASE WHEN $1='blocked' THEN $3 ELSE error_summary END, result_summary=CASE WHEN $4<>'' THEN $4 ELSE result_summary END, ended_at=CASE WHEN $1='done' THEN CURRENT_TIMESTAMP ELSE ended_at END WHERE id=$5`, runStatus, currentActivity, blockedReason, resultSummary, runID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE agent_runs SET status='done', ended_at=CURRENT_TIMESTAMP WHERE task_id=$1 AND agent_id=$2 AND id<>$3 AND status IN ('queued','running','in_progress','blocked','review')`, taskID, agentID, runID); err != nil {
		return err
	}
	if clearCurrent {
		if _, err := tx.Exec(ctx, `UPDATE agents SET status=$1, current_task_id=NULL, current_run_id=NULL, current_activity=$2, health_note='', last_seen_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=$3`, agentStatus, currentActivity, agentID); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(ctx, `UPDATE agents SET status=$1, current_task_id=$2, current_run_id=$3, current_activity=$4, health_note=$5, last_seen_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=$6`, agentStatus, taskID, runID, currentActivity, blockedReason, agentID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Repository) UpdateRuntimeHeartbeat(ctx context.Context, connectorID int, agentID, status, currentActivity string, currentTaskID *int) error {
	_, err := r.db.Exec(ctx, `UPDATE agents SET status=$1, current_activity=$2, current_task_id=COALESCE($3, current_task_id), last_heartbeat_at=CURRENT_TIMESTAMP, last_seen_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=$4`, status, currentActivity, currentTaskID, agentID)
	if err != nil {
		return err
	}
	_, _ = r.db.Exec(ctx, `UPDATE agent_connectors SET last_success_at=CURRENT_TIMESTAMP, status='connected', updated_at=CURRENT_TIMESTAMP WHERE id=$1`, connectorID)
	return nil
}

func (r *Repository) InsertComment(ctx context.Context, taskID int, userID, content string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO comments (task_id, user_id, content) VALUES ($1,$2,$3)`, taskID, userID, content)
	return err
}

func (r *Repository) InsertDeliverable(ctx context.Context, taskID int, title, summary, artifactType, content string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO deliverables (task_id, title, description, artifact_type, content) VALUES ($1,$2,$3,$4,$5)`, taskID, title, summary, artifactType, content)
	return err
}

func (r *Repository) UpdateRunSummary(ctx context.Context, runID int, summary string) error {
	_, err := r.db.Exec(ctx, `UPDATE agent_runs SET result_summary=$1 WHERE id=$2`, summary, runID)
	return err
}

func (r *Repository) TouchConnectorSuccess(ctx context.Context, connectorID int) error {
	_, err := r.db.Exec(ctx, `UPDATE agent_connectors SET last_success_at=$2, status='connected', updated_at=CURRENT_TIMESTAMP WHERE id=$1`, connectorID, time.Now())
	return err
}

func (r *Repository) GetBoardIDByTaskID(ctx context.Context, taskID int) (string, error) {
	var boardID string
	err := r.db.QueryRow(ctx, `SELECT board_id::text FROM tasks WHERE id=$1`, taskID).Scan(&boardID)
	return boardID, err
}

func (r *Repository) ListBoardIDsByAgent(ctx context.Context, agentID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT board_id::text FROM board_agents WHERE agent_id=$1 AND active=true`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var boardID string
		if err := rows.Scan(&boardID); err != nil {
			return nil, err
		}
		items = append(items, boardID)
	}
	if items == nil {
		items = []string{}
	}
	return items, nil
}
