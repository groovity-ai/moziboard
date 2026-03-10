package agentinfra

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	agentsmodule "moziboard-backend/internal/modules/agents"
)

type DeliveryKickFunc func()

type Service struct {
	db           *pgxpool.Pool
	deliveryKick DeliveryKickFunc
}

func NewService(db *pgxpool.Pool, deliveryKick DeliveryKickFunc) *Service {
	return &Service{db: db, deliveryKick: deliveryKick}
}

func (s *Service) CreateNotification(taskID *int, targetAgentID string, sourceAgentID *string, notifType, content string) {
	if targetAgentID == "" {
		return
	}
	_, _ = s.db.Exec(context.Background(),
		"INSERT INTO notifications (task_id, target_agent_id, source_agent_id, type, content) VALUES ($1, $2, $3, $4, $5)",
		taskID, targetAgentID, sourceAgentID, notifType, content)
}

func (s *Service) GetConnectorForAgent(agentID string) (*agentsmodule.AgentConnector, error) {
	var conn agentsmodule.AgentConnector
	err := s.db.QueryRow(context.Background(), `
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

func (s *Service) CreateAgentRun(taskID int, agentID string, status string, activity string) *int {
	conn, _ := s.GetConnectorForAgent(agentID)
	providerType := "internal"
	sessionKey := fmt.Sprintf("mozi:%s:%d", agentID, taskID)
	if conn != nil {
		if conn.ConnectorType != "" {
			providerType = conn.ConnectorType
		}
		if conn.SessionKey != "" {
			sessionKey = conn.SessionKey
		}
	}
	var runID int
	err := s.db.QueryRow(context.Background(),
		"INSERT INTO agent_runs (task_id, agent_id, session_key, provider_type, status, current_activity) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		taskID, agentID, sessionKey, providerType, status, activity).Scan(&runID)
	if err != nil {
		log.Printf("failed to create agent run: %v", err)
		return nil
	}
	return &runID
}

func (s *Service) EnqueueAgentEvent(agentID string, taskID int, eventType string, payload map[string]interface{}) *int {
	payloadBytes, _ := json.Marshal(payload)
	var eventID int
	var boardID string
	_ = s.db.QueryRow(context.Background(), `SELECT board_id::text FROM tasks WHERE id=$1`, taskID).Scan(&boardID)
	err := s.db.QueryRow(context.Background(), `INSERT INTO agent_events (agent_id, board_id, task_id, event_type, payload_json) VALUES ($1,$2,$3,$4,$5::jsonb) RETURNING id`, agentID, boardID, taskID, eventType, string(payloadBytes)).Scan(&eventID)
	if err != nil {
		log.Printf("failed to enqueue agent event: %v", err)
		return nil
	}
	if s.deliveryKick != nil {
		go s.deliveryKick()
	}
	return &eventID
}

func (s *Service) CloseOtherRuns(taskID int, agentID string, keepRunID int) {
	_, _ = s.db.Exec(context.Background(), `UPDATE agent_runs SET status='done', ended_at=CURRENT_TIMESTAMP WHERE task_id=$1 AND agent_id=$2 AND id<>$3 AND status IN ('queued','running','in_progress','blocked','review')`, taskID, agentID, keepRunID)
}
