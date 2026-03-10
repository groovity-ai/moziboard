package agents

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) ListAgents(ctx context.Context) ([]Agent, error) {
	rows, err := r.db.Query(ctx, "SELECT id, display_name, role_name, avatar, provider, engine, description, is_native_clawn, soul, memory, rules, cron_schedule, active, status, last_heartbeat_at, last_seen_at, current_task_id, current_run_id, current_activity, health_note, created_at, updated_at FROM agents")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.DisplayName, &a.RoleName, &a.Avatar, &a.Provider, &a.Engine, &a.Description, &a.IsNativeClawn, &a.Soul, &a.Memory, &a.Rules, &a.CronSchedule, &a.Active, &a.Status, &a.LastHeartbeatAt, &a.LastSeenAt, &a.CurrentTaskID, &a.CurrentRunID, &a.CurrentActivity, &a.HealthNote, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	if items == nil {
		items = []Agent{}
	}
	return items, nil
}

func (r *Repository) GetAgentByID(ctx context.Context, agentID string) (Agent, error) {
	var a Agent
	err := r.db.QueryRow(ctx, "SELECT id, display_name, role_name, avatar, provider, engine, description, is_native_clawn, soul, memory, rules, cron_schedule, active, status, last_heartbeat_at, last_seen_at, current_task_id, current_run_id, current_activity, health_note, created_at, updated_at FROM agents WHERE id=$1", agentID).Scan(&a.ID, &a.DisplayName, &a.RoleName, &a.Avatar, &a.Provider, &a.Engine, &a.Description, &a.IsNativeClawn, &a.Soul, &a.Memory, &a.Rules, &a.CronSchedule, &a.Active, &a.Status, &a.LastHeartbeatAt, &a.LastSeenAt, &a.CurrentTaskID, &a.CurrentRunID, &a.CurrentActivity, &a.HealthNote, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

func (r *Repository) UpsertMember(ctx context.Context, id, name, avatar string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO members (id, name, role, avatar) VALUES ($1,$2,'agent',$3) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, role='agent', avatar=EXCLUDED.avatar`, id, name, avatar)
	return err
}

func (r *Repository) UpsertAgent(ctx context.Context, req AgentRegisterReq) error {
	_, err := r.db.Exec(ctx, `INSERT INTO agents (id, display_name, role_name, avatar, provider, engine, description, is_native_clawn, soul, memory, rules, cron_schedule, active, status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'','', '', '*/10 * * * *', true, 'offline') ON CONFLICT (id) DO UPDATE SET display_name=EXCLUDED.display_name, role_name=EXCLUDED.role_name, avatar=EXCLUDED.avatar, provider=EXCLUDED.provider, engine=EXCLUDED.engine, description=EXCLUDED.description, is_native_clawn=EXCLUDED.is_native_clawn, updated_at=CURRENT_TIMESTAMP`, req.ID, req.DisplayName, req.RoleName, req.Avatar, req.Provider, req.Engine, req.Description, req.IsNativeClawn)
	return err
}

func (r *Repository) InsertConnector(ctx context.Context, req AgentRegisterReq, tokenHash, sharedSecretHash, metadataJSON string) (int, error) {
	var connectorID int
	err := r.db.QueryRow(ctx, `INSERT INTO agent_connectors (agent_id, connector_type, transport_mode, auth_type, machine_token_hash, endpoint_url, base_url, agent_ref, session_key, shared_secret_hash, status, metadata_json) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'connected',$11::jsonb) RETURNING id`, req.ID, req.Connector.ConnectorType, req.Connector.TransportMode, req.Connector.AuthType, tokenHash, req.Connector.EndpointURL, req.Connector.BaseURL, req.Connector.AgentRef, req.Connector.SessionKey, sharedSecretHash, metadataJSON).Scan(&connectorID)
	return connectorID, err
}

func (r *Repository) ConnectBoard(ctx context.Context, agentID string, req AgentBoardConnectReq, autoAccept, canComment, canUpdate, canDocs, canDeliv bool, capabilitiesJSON string) (int, error) {
	var id int
	err := r.db.QueryRow(ctx, `INSERT INTO board_agents (board_id, agent_id, board_role, active, auto_accept_tasks, can_comment, can_update_status, can_access_docs, can_create_deliverables, capabilities_json) VALUES ($1,$2,$3,true,$4,$5,$6,$7,$8,$9::jsonb) ON CONFLICT (board_id, agent_id) DO UPDATE SET board_role=EXCLUDED.board_role, active=true, auto_accept_tasks=EXCLUDED.auto_accept_tasks, can_comment=EXCLUDED.can_comment, can_update_status=EXCLUDED.can_update_status, can_access_docs=EXCLUDED.can_access_docs, can_create_deliverables=EXCLUDED.can_create_deliverables, capabilities_json=EXCLUDED.capabilities_json, updated_at=CURRENT_TIMESTAMP RETURNING id`, req.BoardID, agentID, req.BoardRole, autoAccept, canComment, canUpdate, canDocs, canDeliv, capabilitiesJSON).Scan(&id)
	if err != nil {
		return 0, err
	}
	_, _ = r.db.Exec(ctx, `INSERT INTO board_members (board_id, member_id, role) VALUES ($1,$2,'agent') ON CONFLICT DO NOTHING`, req.BoardID, agentID)
	return id, nil
}

func (r *Repository) ListBoardAgents(ctx context.Context, boardID string) ([]BoardAgent, error) {
	rows, err := r.db.Query(ctx, `SELECT ba.id, ba.board_id::text, ba.agent_id, ba.board_role, ba.active, ba.auto_accept_tasks, ba.can_comment, ba.can_update_status, ba.can_access_docs, ba.can_create_deliverables, ba.capabilities_json::text, ba.created_at, ba.updated_at, a.display_name, a.role_name, a.avatar, a.provider, a.engine, a.description, a.is_native_clawn, a.status, a.last_heartbeat_at, a.last_seen_at, a.current_task_id, a.current_run_id, a.current_activity, a.health_note, a.soul, a.memory, a.rules, a.cron_schedule, a.active, a.created_at, a.updated_at FROM board_agents ba JOIN agents a ON a.id = ba.agent_id WHERE ba.board_id=$1 ORDER BY ba.created_at ASC`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []BoardAgent
	for rows.Next() {
		var ba BoardAgent
		var ag Agent
		if err := rows.Scan(&ba.ID, &ba.BoardID, &ba.AgentID, &ba.BoardRole, &ba.Active, &ba.AutoAcceptTasks, &ba.CanComment, &ba.CanUpdateStatus, &ba.CanAccessDocs, &ba.CanCreateDeliverables, &ba.CapabilitiesJSON, &ba.CreatedAt, &ba.UpdatedAt, &ag.DisplayName, &ag.RoleName, &ag.Avatar, &ag.Provider, &ag.Engine, &ag.Description, &ag.IsNativeClawn, &ag.Status, &ag.LastHeartbeatAt, &ag.LastSeenAt, &ag.CurrentTaskID, &ag.CurrentRunID, &ag.CurrentActivity, &ag.HealthNote, &ag.Soul, &ag.Memory, &ag.Rules, &ag.CronSchedule, &ag.Active, &ag.CreatedAt, &ag.UpdatedAt); err != nil {
			return nil, err
		}
		ag.ID = ba.AgentID
		ba.Agent = &ag
		result = append(result, ba)
	}
	if result == nil {
		result = []BoardAgent{}
	}
	return result, nil
}

func (r *Repository) ListRunsByAgent(ctx context.Context, agentID string) ([]AgentRun, error) {
	rows, err := r.db.Query(ctx, "SELECT id, task_id, agent_id, session_key, provider_type, status, started_at, ended_at, current_activity, error_summary, result_summary FROM agent_runs WHERE agent_id=$1 ORDER BY started_at DESC LIMIT 20", agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []AgentRun
	for rows.Next() {
		var run AgentRun
		if err := rows.Scan(&run.ID, &run.TaskID, &run.AgentID, &run.SessionKey, &run.ProviderType, &run.Status, &run.StartedAt, &run.EndedAt, &run.CurrentActivity, &run.ErrorSummary, &run.ResultSummary); err != nil {
			return nil, err
		}
		items = append(items, run)
	}
	if items == nil {
		items = []AgentRun{}
	}
	return items, nil
}

func (r *Repository) ListNotificationsByAgent(ctx context.Context, agentID string) ([]Notification, error) {
	rows, err := r.db.Query(ctx, "SELECT id, task_id, target_agent_id, source_agent_id, type, content, delivered, delivered_at, created_at FROM notifications WHERE target_agent_id=$1 ORDER BY created_at DESC LIMIT 30", agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.TaskID, &n.TargetAgentID, &n.SourceAgentID, &n.Type, &n.Content, &n.Delivered, &n.DeliveredAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, n)
	}
	if items == nil {
		items = []Notification{}
	}
	return items, nil
}

func (r *Repository) TouchHeartbeat(ctx context.Context, agentID string) error {
	_, err := r.db.Exec(ctx, "UPDATE agents SET status='online', last_heartbeat_at=CURRENT_TIMESTAMP, last_seen_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=$1", agentID)
	return err
}

func (r *Repository) UpdateAgentStatus(ctx context.Context, agentID, status, currentActivity, healthNote string) error {
	_, err := r.db.Exec(ctx, "UPDATE agents SET status=$1, current_activity=$2, health_note=$3, last_seen_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=$4", status, currentActivity, healthNote, agentID)
	return err
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
