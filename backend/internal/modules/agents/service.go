package agents

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

type TokenGenerator func() (plain string, hash string, err error)
type TokenHasher func(token string) string

type Service struct {
	repo          *Repository
	generateToken TokenGenerator
	hashToken     TokenHasher
}

func NewService(repo *Repository, generateToken TokenGenerator, hashToken TokenHasher) *Service {
	return &Service{repo: repo, generateToken: generateToken, hashToken: hashToken}
}

func (s *Service) ListAgents(ctx context.Context) ([]Agent, error) { return s.repo.ListAgents(ctx) }
func (s *Service) GetAgent(ctx context.Context, agentID string) (Agent, error) {
	return s.repo.GetAgentByID(ctx, agentID)
}
func (s *Service) ListBoardAgents(ctx context.Context, boardID string) ([]BoardAgent, error) {
	return s.repo.ListBoardAgents(ctx, boardID)
}
func (s *Service) ListRuns(ctx context.Context, agentID string) ([]AgentRun, error) {
	return s.repo.ListRunsByAgent(ctx, agentID)
}
func (s *Service) ListNotifications(ctx context.Context, agentID string) ([]Notification, error) {
	return s.repo.ListNotificationsByAgent(ctx, agentID)
}
func (s *Service) Heartbeat(ctx context.Context, agentID string) error {
	return s.repo.TouchHeartbeat(ctx, agentID)
}

func (s *Service) UpdateStatus(ctx context.Context, agentID string, req AgentStatusUpdateReq) error {
	if strings.TrimSpace(req.Status) == "" {
		req.Status = "online"
	}
	return s.repo.UpdateAgentStatus(ctx, agentID, req.Status, req.CurrentActivity, req.HealthNote)
}

func (s *Service) Register(ctx context.Context, req AgentRegisterReq) (map[string]interface{}, error) {
	if strings.TrimSpace(req.ID) == "" || strings.TrimSpace(req.DisplayName) == "" {
		return nil, errors.New("id and display_name are required")
	}
	if req.RoleName == "" {
		req.RoleName = "Agent"
	}
	if req.Avatar == "" {
		req.Avatar = "🤖"
	}
	if req.Provider == "" {
		req.Provider = "external"
	}
	if req.Engine == "" {
		req.Engine = "custom"
	}
	if req.Connector.ConnectorType == "" {
		req.Connector.ConnectorType = "custom"
	}
	if req.Connector.AuthType == "" {
		req.Connector.AuthType = "machine_token"
	}
	if req.Connector.TransportMode == "" {
		if req.Connector.ConnectorType == "clawn_native" {
			req.Connector.TransportMode = "internal"
		} else {
			req.Connector.TransportMode = "push"
		}
	}
	if req.Capabilities == nil {
		req.Capabilities = map[string]interface{}{}
	}
	metaBytes, _ := json.Marshal(req.Connector.Metadata)
	plainToken, tokenHash, err := s.generateToken()
	if err != nil {
		return nil, err
	}
	sharedSecretHash := ""
	if strings.TrimSpace(req.Connector.SharedSecret) != "" {
		sharedSecretHash = s.hashToken(strings.TrimSpace(req.Connector.SharedSecret))
	}
	if err := s.repo.UpsertMember(ctx, req.ID, req.DisplayName, req.Avatar); err != nil {
		return nil, err
	}
	if err := s.repo.UpsertAgent(ctx, req); err != nil {
		return nil, err
	}
	connectorID, err := s.repo.InsertConnector(ctx, req, tokenHash, sharedSecretHash, string(metaBytes))
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"agent":         map[string]interface{}{"id": req.ID, "display_name": req.DisplayName, "provider": req.Provider, "engine": req.Engine},
		"connector":     map[string]interface{}{"id": connectorID, "connector_type": req.Connector.ConnectorType, "transport_mode": req.Connector.TransportMode, "base_url": req.Connector.BaseURL, "agent_ref": req.Connector.AgentRef, "status": "connected"},
		"machine_token": plainToken,
	}, nil
}

func (s *Service) ConnectToBoard(ctx context.Context, agentID string, req AgentBoardConnectReq) (int, error) {
	if strings.TrimSpace(req.BoardID) == "" {
		return 0, errors.New("board_id is required")
	}
	if req.BoardRole == "" {
		req.BoardRole = "worker"
	}
	autoAccept := true
	if req.AutoAcceptTasks != nil {
		autoAccept = *req.AutoAcceptTasks
	}
	canComment, canUpdate, canDocs, canDeliv := true, true, true, true
	if req.Permissions != nil {
		if v, ok := req.Permissions["can_comment"].(bool); ok {
			canComment = v
		}
		if v, ok := req.Permissions["can_update_status"].(bool); ok {
			canUpdate = v
		}
		if v, ok := req.Permissions["can_access_docs"].(bool); ok {
			canDocs = v
		}
		if v, ok := req.Permissions["can_create_deliverables"].(bool); ok {
			canDeliv = v
		}
	}
	capBytes, _ := json.Marshal(req.Permissions)
	return s.repo.ConnectBoard(ctx, agentID, req, autoAccept, canComment, canUpdate, canDocs, canDeliv, string(capBytes))
}
