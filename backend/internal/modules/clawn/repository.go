package clawn

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) ExistingProjectConnections(ctx context.Context, boardID string) map[string]string {
	existingByProjectID := map[string]string{}
	if strings.TrimSpace(boardID) == "" {
		return existingByProjectID
	}
	rows, err := r.db.Query(ctx, `
		SELECT ba.agent_id, ac.metadata_json::text
		FROM board_agents ba
		JOIN agent_connectors ac ON ac.agent_id = ba.agent_id
		WHERE ba.board_id = $1
		  AND ac.connector_type = 'clawn_native'`, boardID)
	if err != nil {
		return existingByProjectID
	}
	defer rows.Close()
	for rows.Next() {
		var agentID, metaText string
		if err := rows.Scan(&agentID, &metaText); err != nil {
			continue
		}
		if strings.TrimSpace(metaText) == "" {
			continue
		}
		var meta map[string]interface{}
		if json.Unmarshal([]byte(metaText), &meta) != nil {
			continue
		}
		projectID := interfaceToString(meta["clawn_project_id"])
		if projectID != "" {
			existingByProjectID[projectID] = agentID
		}
	}
	return existingByProjectID
}

func (r *Repository) UpsertConnectedProject(ctx context.Context, req ConnectReq, selected ProjectSource, plainTokenHash string) (string, int, int, error) {
	agentID := "clawn-project:" + selected.ProjectID
	displayName := firstNonEmpty(selected.DisplayName, selected.ProjectID)
	avatar := "🦊"
	roleName := "Clawn Runtime"
	description := "Connected from Clawn project " + selected.DisplayName + " (" + selected.Engine + ")"
	boardRole := firstNonEmpty(strings.TrimSpace(req.BoardRole), "worker")
	autoAccept := true
	if req.AutoAcceptTasks != nil {
		autoAccept = *req.AutoAcceptTasks
	}
	canComment := req.CanComment == nil || *req.CanComment
	canUpdate := req.CanUpdateStatus == nil || *req.CanUpdateStatus
	canDocs := req.CanAccessDocs == nil || *req.CanAccessDocs
	canDeliver := req.CanCreateDeliverables == nil || *req.CanCreateDeliverables

	_, err := r.db.Exec(ctx, `INSERT INTO members (id, name, role, avatar) VALUES ($1,$2,'agent',$3) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, role='agent', avatar=EXCLUDED.avatar`, agentID, displayName, avatar)
	if err != nil {
		return "", 0, 0, err
	}
	_, err = r.db.Exec(ctx, `INSERT INTO agents (id, display_name, role_name, avatar, provider, engine, description, is_native_clawn, soul, memory, rules, cron_schedule, active, status) VALUES ($1,$2,$3,$4,$5,$6,$7,true,'','', '', '*/10 * * * *', true, $8) ON CONFLICT (id) DO UPDATE SET display_name=EXCLUDED.display_name, role_name=EXCLUDED.role_name, avatar=EXCLUDED.avatar, provider=EXCLUDED.provider, engine=EXCLUDED.engine, description=EXCLUDED.description, is_native_clawn=true, status=EXCLUDED.status, updated_at=CURRENT_TIMESTAMP`, agentID, displayName, roleName, avatar, "clawn", selected.Engine, description, selected.Status)
	if err != nil {
		return "", 0, 0, err
	}

	meta := map[string]interface{}{
		"source":           "clawn",
		"clawn_project_id": selected.ProjectID,
		"owner_user_id":    selected.OwnerUserID,
		"engine":           selected.Engine,
		"plan":             selected.Plan,
		"status":           selected.Status,
		"container_id":     selected.ContainerID,
		"container_name":   selected.ContainerName,
		"capabilities":     selected.Capabilities,
		"linkage_mode":     "project-centric",
	}
	metaBytes, _ := json.Marshal(meta)
	var connectorID int
	err = r.db.QueryRow(ctx, `
		INSERT INTO agent_connectors (agent_id, connector_type, transport_mode, auth_type, machine_token_hash, endpoint_url, base_url, agent_ref, session_key, shared_secret_hash, status, metadata_json)
		VALUES ($1,'clawn_native','internal','machine_token',$2,'','','','', '', 'connected', $3::jsonb)
		ON CONFLICT DO NOTHING
		RETURNING id`, agentID, plainTokenHash, string(metaBytes)).Scan(&connectorID)
	if err != nil {
		_ = r.db.QueryRow(ctx, `UPDATE agent_connectors SET metadata_json=$2::jsonb, status='connected', updated_at=CURRENT_TIMESTAMP WHERE agent_id=$1 AND connector_type='clawn_native' RETURNING id`, agentID, string(metaBytes)).Scan(&connectorID)
	}
	capMap := map[string]interface{}{
		"can_comment":             canComment,
		"can_update_status":       canUpdate,
		"can_access_docs":         canDocs,
		"can_create_deliverables": canDeliver,
	}
	capBytes, _ := json.Marshal(capMap)
	var boardAgentID int
	err = r.db.QueryRow(ctx, `INSERT INTO board_agents (board_id, agent_id, board_role, active, auto_accept_tasks, can_comment, can_update_status, can_access_docs, can_create_deliverables, capabilities_json) VALUES ($1,$2,$3,true,$4,$5,$6,$7,$8,$9::jsonb) ON CONFLICT (board_id, agent_id) DO UPDATE SET board_role=EXCLUDED.board_role, active=true, auto_accept_tasks=EXCLUDED.auto_accept_tasks, can_comment=EXCLUDED.can_comment, can_update_status=EXCLUDED.can_update_status, can_access_docs=EXCLUDED.can_access_docs, can_create_deliverables=EXCLUDED.can_create_deliverables, capabilities_json=EXCLUDED.capabilities_json, updated_at=CURRENT_TIMESTAMP RETURNING id`, req.BoardID, agentID, boardRole, autoAccept, canComment, canUpdate, canDocs, canDeliver, string(capBytes)).Scan(&boardAgentID)
	if err != nil {
		return "", 0, 0, err
	}
	_, _ = r.db.Exec(ctx, `INSERT INTO board_members (board_id, member_id, role) VALUES ($1,$2,'agent') ON CONFLICT DO NOTHING`, req.BoardID, agentID)
	return agentID, connectorID, boardAgentID, nil
}
