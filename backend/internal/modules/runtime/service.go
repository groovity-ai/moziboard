package runtime

import (
	"context"
	"crypto/hmac"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	agentsmodule "moziboard-backend/internal/modules/agents"
	tasksmodule "moziboard-backend/internal/modules/tasks"
)

type HashTokenFunc func(string) string
type SignPayloadFunc func(secret, timestamp, body string) string
type AgentInfra interface {
	CreateAgentRun(taskID int, agentID string, status string, activity string) *int
	CreateNotification(taskID *int, targetAgentID string, sourceAgentID *string, notifType, content string)
	CloseOtherRuns(taskID int, agentID string, keepRunID int)
}
type LogActivityFunc func(taskID int, userID, action, details string)
type BroadcastFunc func(msg string)

type Service struct {
	repo               *Repository
	hashToken          HashTokenFunc
	signWebhookPayload SignPayloadFunc
	agentInfra         AgentInfra
	logActivity        LogActivityFunc
	broadcast          BroadcastFunc
}

func NewService(repo *Repository, hashToken HashTokenFunc, signWebhookPayload SignPayloadFunc, agentInfra AgentInfra, logActivity LogActivityFunc, broadcast BroadcastFunc) *Service {
	return &Service{repo: repo, hashToken: hashToken, signWebhookPayload: signWebhookPayload, agentInfra: agentInfra, logActivity: logActivity, broadcast: broadcast}
}

func extractBearerToken(c *fiber.Ctx) string {
	auth := c.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

func normalizeRunStatus(taskStatus string) string {
	switch strings.ToLower(strings.TrimSpace(taskStatus)) {
	case "todo", "assigned":
		return "queued"
	case "in_progress", "doing":
		return "running"
	case "review", "qa":
		return "review"
	case "blocked":
		return "blocked"
	case "done":
		return "done"
	default:
		return "queued"
	}
}

func (s *Service) ResolveRuntimeAgent(ctx context.Context, c *fiber.Ctx) (*agentsmodule.AgentConnector, error) {
	token := extractBearerToken(c)
	if token == "" {
		return nil, fmt.Errorf("missing bearer token")
	}
	hash := s.hashToken(token)
	if conn, err := s.repo.FindConnectorByMachineTokenHash(ctx, hash); err == nil {
		return conn, nil
	}

	agentRef := strings.TrimSpace(c.Get("X-Mozi-Agent-Ref"))
	sharedSecret := strings.TrimSpace(c.Get("X-Mozi-Shared-Secret"))
	timestamp := strings.TrimSpace(c.Get("X-Mozi-Timestamp"))
	signature := strings.TrimSpace(c.Get("X-Mozi-Signature"))
	if agentRef == "" || sharedSecret == "" {
		return nil, fmt.Errorf("missing bearer token")
	}
	conn, sharedSecretHash, err := s.repo.FindSharedSecretConnector(ctx, agentRef)
	if err != nil {
		return nil, err
	}
	if s.hashToken(sharedSecret) != sharedSecretHash {
		return nil, fmt.Errorf("invalid shared secret")
	}
	if timestamp != "" || signature != "" {
		if timestamp == "" || signature == "" {
			return nil, fmt.Errorf("incomplete signature headers")
		}
		parsedTs, parseErr := time.Parse(time.RFC3339, timestamp)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid signature timestamp")
		}
		if time.Since(parsedTs) > 5*time.Minute || time.Until(parsedTs) > 1*time.Minute {
			return nil, fmt.Errorf("signature timestamp out of range")
		}
		body := string(c.Body())
		expected := s.signWebhookPayload(sharedSecret, timestamp, body)
		if !hmac.Equal([]byte(expected), []byte(signature)) {
			return nil, fmt.Errorf("invalid shared secret signature")
		}
	}
	return conn, nil
}

func (s *Service) ensureAgentBoardAccess(ctx context.Context, agentID string, taskID int) error {
	ok, err := s.repo.EnsureAgentBoardAccess(ctx, agentID, taskID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("agent not connected to task board")
	}
	return nil
}

func (s *Service) ensureActiveRun(ctx context.Context, taskID int, agentID string) (int, error) {
	ids, err := s.repo.ListActiveRunIDs(ctx, taskID, agentID)
	if err != nil {
		return 0, err
	}
	if len(ids) > 0 {
		keep := ids[0]
		if s.agentInfra != nil {
			s.agentInfra.CloseOtherRuns(taskID, agentID, keep)
		} else {
			_ = s.repo.CloseOtherRuns(ctx, taskID, agentID, keep)
		}
		return keep, nil
	}
	if s.agentInfra == nil {
		return 0, fmt.Errorf("agent infra not configured")
	}
	created := s.agentInfra.CreateAgentRun(taskID, agentID, "running", "Started work")
	if created == nil {
		return 0, fmt.Errorf("failed to create run")
	}
	return *created, nil
}

func (s *Service) syncTaskAndRunState(ctx context.Context, taskID int, agentID string, taskStatus string, currentActivity string, blockedReason string, resultSummary string) error {
	listID := tasksmodule.StatusToListID(taskStatus)
	runStatus := normalizeRunStatus(taskStatus)
	return s.repo.UpdateTaskAndRunState(ctx, taskID, agentID, taskStatus, listID, currentActivity, blockedReason, resultSummary, runStatus)
}

func (s *Service) RuntimeHeartbeat(ctx context.Context, conn *agentsmodule.AgentConnector, req HeartbeatReq) error {
	if req.Status == "" {
		req.Status = "online"
	}
	return s.repo.UpdateRuntimeHeartbeat(ctx, conn.ID, conn.AgentID, req.Status, req.CurrentActivity, req.CurrentTaskID)
}

func (s *Service) RuntimeTaskAck(ctx context.Context, conn *agentsmodule.AgentConnector, req TaskAckReq) (int, error) {
	if err := s.ensureAgentBoardAccess(ctx, conn.AgentID, req.TaskID); err != nil {
		return 0, err
	}
	runID, err := s.ensureActiveRun(ctx, req.TaskID, conn.AgentID)
	if err != nil {
		return 0, err
	}
	msg := req.Message
	if msg == "" {
		msg = "Task accepted, starting work."
	}
	_ = s.repo.InsertComment(ctx, req.TaskID, conn.AgentID, msg)
	_ = s.syncTaskAndRunState(ctx, req.TaskID, conn.AgentID, "in_progress", "Working on task", "", "")
	_ = s.repo.UpdateAgentWorkingState(ctx, conn.AgentID, req.TaskID, runID, "Working on task")
	if s.logActivity != nil {
		go s.logActivity(req.TaskID, conn.AgentID, "acknowledged", msg)
	}
	if s.broadcast != nil {
		go s.broadcast("UPDATE")
	}
	return runID, nil
}

func (s *Service) RuntimeTaskUpdate(ctx context.Context, conn *agentsmodule.AgentConnector, req TaskUpdateReq) (int, error) {
	if err := s.ensureAgentBoardAccess(ctx, conn.AgentID, req.TaskID); err != nil {
		return 0, err
	}
	if req.Status == "" {
		req.Status = "in_progress"
	}
	runID, err := s.ensureActiveRun(ctx, req.TaskID, conn.AgentID)
	if err != nil {
		return 0, err
	}
	if req.ProgressMessage != "" {
		_ = s.repo.InsertComment(ctx, req.TaskID, conn.AgentID, req.ProgressMessage)
	}
	_ = s.syncTaskAndRunState(ctx, req.TaskID, conn.AgentID, req.Status, req.CurrentActivity, req.BlockedReason, "")
	agentStatus := "busy"
	if req.Status == "blocked" {
		agentStatus = "blocked"
	}
	_ = s.repo.UpdateAgentTaskState(ctx, conn.AgentID, agentStatus, req.TaskID, runID, req.CurrentActivity, req.BlockedReason)
	if req.Status == "blocked" && s.agentInfra != nil {
		s.agentInfra.CreateNotification(&req.TaskID, conn.AgentID, nil, "blocked", req.BlockedReason)
	}
	if s.logActivity != nil {
		go s.logActivity(req.TaskID, conn.AgentID, "runtime_update", fmt.Sprintf("Status updated to %s", req.Status))
	}
	if s.broadcast != nil {
		go s.broadcast("UPDATE")
	}
	return runID, nil
}

func (s *Service) RuntimeTaskComment(ctx context.Context, conn *agentsmodule.AgentConnector, req TaskCommentReq) error {
	if err := s.ensureAgentBoardAccess(ctx, conn.AgentID, req.TaskID); err != nil {
		return err
	}
	if err := s.repo.InsertComment(ctx, req.TaskID, conn.AgentID, req.Content); err != nil {
		return err
	}
	if s.logActivity != nil {
		go s.logActivity(req.TaskID, conn.AgentID, "commented", req.Content)
	}
	if s.broadcast != nil {
		go s.broadcast("UPDATE")
	}
	return nil
}

func (s *Service) RuntimeTaskDeliverable(ctx context.Context, conn *agentsmodule.AgentConnector, req TaskDeliverableReq) (int, error) {
	if err := s.ensureAgentBoardAccess(ctx, conn.AgentID, req.TaskID); err != nil {
		return 0, err
	}
	if err := s.repo.InsertDeliverable(ctx, req.TaskID, req.Title, req.Summary, req.ArtifactType, req.Content); err != nil {
		return 0, err
	}
	runID, _ := s.ensureActiveRun(ctx, req.TaskID, conn.AgentID)
	_ = s.repo.UpdateRunSummary(ctx, runID, req.Summary)
	if s.logActivity != nil {
		go s.logActivity(req.TaskID, conn.AgentID, "deliverable", fmt.Sprintf("Submitted deliverable: %s", req.Title))
	}
	if s.broadcast != nil {
		go s.broadcast("UPDATE")
	}
	return runID, nil
}

func (s *Service) RuntimeTaskReviewRequest(ctx context.Context, conn *agentsmodule.AgentConnector, req TaskReviewReq) (int, error) {
	if err := s.ensureAgentBoardAccess(ctx, conn.AgentID, req.TaskID); err != nil {
		return 0, err
	}
	runID, _ := s.ensureActiveRun(ctx, req.TaskID, conn.AgentID)
	_ = s.syncTaskAndRunState(ctx, req.TaskID, conn.AgentID, "review", "Waiting for review", "", req.Summary)
	_ = s.repo.UpdateAgentWaitingReview(ctx, conn.AgentID)
	if req.Summary != "" {
		_ = s.repo.InsertComment(ctx, req.TaskID, conn.AgentID, req.Summary)
	}
	if s.logActivity != nil {
		go s.logActivity(req.TaskID, conn.AgentID, "review_requested", req.Summary)
	}
	if s.broadcast != nil {
		go s.broadcast("UPDATE")
	}
	return runID, nil
}
