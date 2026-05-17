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
	"moziboard-backend/internal/workflow"
)

type HashTokenFunc func(string) string
type SignPayloadFunc func(secret, timestamp, body string) string
type AgentInfra interface {
	CreateAgentRun(taskID int, agentID string, status string, activity string) *int
	CreateNotification(taskID *int, targetAgentID string, sourceAgentID *string, notifType, content string)
	CloseOtherRuns(taskID int, agentID string, keepRunID int)
}
type LogActivityFunc func(taskID int, userID, action, details string)
type AuditLogFunc func(actorType, actorID, entityType, entityID, action string, oldValue, newValue, metadata interface{})
type BroadcastFunc func(msg string)

type BoardStatsUpdater interface {
	RecomputeBoard(ctx context.Context, boardID string) error
}

type Service struct {
	repo               *Repository
	hashToken          HashTokenFunc
	signWebhookPayload SignPayloadFunc
	agentInfra         AgentInfra
	logActivity        LogActivityFunc
	auditLog           AuditLogFunc
	broadcast          BroadcastFunc
	boardStats         BoardStatsUpdater
}

func NewService(repo *Repository, hashToken HashTokenFunc, signWebhookPayload SignPayloadFunc, agentInfra AgentInfra, logActivity LogActivityFunc, auditLog AuditLogFunc, broadcast BroadcastFunc, boardStats BoardStatsUpdater) *Service {
	return &Service{repo: repo, hashToken: hashToken, signWebhookPayload: signWebhookPayload, agentInfra: agentInfra, logActivity: logActivity, auditLog: auditLog, broadcast: broadcast, boardStats: boardStats}
}

func extractBearerToken(c *fiber.Ctx) string {
	auth := c.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
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
	created := s.agentInfra.CreateAgentRun(taskID, agentID, workflow.MapTaskStateToRunState(workflow.StateInProgress), "Started work")
	if created == nil {
		return 0, fmt.Errorf("failed to create run")
	}
	return *created, nil
}

func (s *Service) RuntimeHeartbeat(ctx context.Context, conn *agentsmodule.AgentConnector, req HeartbeatReq) error {
	if req.Status == "" {
		req.Status = "online"
	}
	if err := s.repo.UpdateRuntimeHeartbeat(ctx, conn.ID, conn.AgentID, req.Status, req.CurrentActivity, req.CurrentTaskID); err != nil {
		return err
	}
	if s.boardStats != nil {
		boardIDs, err := s.repo.ListBoardIDsByAgent(ctx, conn.AgentID)
		if err == nil {
			for _, boardID := range boardIDs {
				go s.boardStats.RecomputeBoard(context.Background(), boardID)
			}
		}
	}
	return nil
}

func (s *Service) RuntimeTaskAck(ctx context.Context, conn *agentsmodule.AgentConnector, req TaskAckReq) (int, error) {
	if err := s.ensureAgentBoardAccess(ctx, conn.AgentID, req.TaskID); err != nil {
		return 0, err
	}
	task, err := s.repo.GetTaskByID(ctx, req.TaskID)
	if err != nil {
		return 0, err
	}
	if _, err := workflow.ValidateTransition(workflow.TransitionRequest{FromState: task.Status, ToState: workflow.StateInProgress, Trigger: workflow.TriggerRuntimeAck, ActorType: "agent", ActorID: conn.AgentID}); err != nil {
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
	if err := s.repo.ApplyTaskStateSync(ctx, req.TaskID, conn.AgentID, runID, workflow.StateInProgress, "Working on task", "", ""); err != nil {
		return 0, err
	}
	if s.logActivity != nil {
		go s.logActivity(req.TaskID, conn.AgentID, "acknowledged", msg)
	}
	if s.auditLog != nil {
		s.auditLog("agent", conn.AgentID, "task", fmt.Sprintf("%d", req.TaskID), "runtime_ack", map[string]interface{}{"status": task.Status}, map[string]interface{}{"status": workflow.StateInProgress}, map[string]interface{}{"message": msg, "run_id": runID})
	}
	s.afterTaskMutation(ctx, req.TaskID)
	return runID, nil
}

func (s *Service) RuntimeTaskUpdate(ctx context.Context, conn *agentsmodule.AgentConnector, req TaskUpdateReq) (int, error) {
	if err := s.ensureAgentBoardAccess(ctx, conn.AgentID, req.TaskID); err != nil {
		return 0, err
	}
	task, err := s.repo.GetTaskByID(ctx, req.TaskID)
	if err != nil {
		return 0, err
	}
	toState := workflow.NormalizeTaskState(req.Status)
	if strings.TrimSpace(req.Status) == "" {
		toState = workflow.NormalizeTaskState(task.Status)
	}
	result, err := workflow.ValidateTransition(workflow.TransitionRequest{FromState: task.Status, ToState: toState, Trigger: workflow.TriggerRuntimeUpdate, ActorType: "agent", ActorID: conn.AgentID, Reason: req.BlockedReason})
	if err != nil {
		return 0, err
	}
	runID, err := s.ensureActiveRun(ctx, req.TaskID, conn.AgentID)
	if err != nil {
		return 0, err
	}
	if req.ProgressMessage != "" {
		_ = s.repo.InsertComment(ctx, req.TaskID, conn.AgentID, req.ProgressMessage)
	}
	if err := s.repo.ApplyTaskStateSync(ctx, req.TaskID, conn.AgentID, runID, result.NormalizedTo, req.CurrentActivity, req.BlockedReason, ""); err != nil {
		return 0, err
	}
	if result.NormalizedTo == workflow.StateBlocked && s.agentInfra != nil {
		s.agentInfra.CreateNotification(&req.TaskID, conn.AgentID, nil, "blocked", req.BlockedReason)
	}
	if s.logActivity != nil {
		details := fmt.Sprintf("Status updated to %s", result.NormalizedTo)
		if req.ProgressMessage != "" {
			details = req.ProgressMessage
		}
		go s.logActivity(req.TaskID, conn.AgentID, result.ActivityAction, details)
	}
	if s.auditLog != nil {
		s.auditLog("agent", conn.AgentID, "task", fmt.Sprintf("%d", req.TaskID), result.ActivityAction, map[string]interface{}{"status": task.Status}, map[string]interface{}{"status": result.NormalizedTo}, map[string]interface{}{"progress_message": req.ProgressMessage, "blocked_reason": req.BlockedReason, "run_id": runID})
	}
	s.afterTaskMutation(ctx, req.TaskID)
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
	s.afterTaskMutation(ctx, req.TaskID)
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
	s.afterTaskMutation(ctx, req.TaskID)
	return runID, nil
}

func (s *Service) RuntimeTaskReviewRequest(ctx context.Context, conn *agentsmodule.AgentConnector, req TaskReviewReq) (int, error) {
	if err := s.ensureAgentBoardAccess(ctx, conn.AgentID, req.TaskID); err != nil {
		return 0, err
	}
	task, err := s.repo.GetTaskByID(ctx, req.TaskID)
	if err != nil {
		return 0, err
	}
	result, err := workflow.ValidateTransition(workflow.TransitionRequest{FromState: task.Status, ToState: workflow.StateReview, Trigger: workflow.TriggerReviewRequested, ActorType: "agent", ActorID: conn.AgentID})
	if err != nil {
		return 0, err
	}
	runID, _ := s.ensureActiveRun(ctx, req.TaskID, conn.AgentID)
	if err := s.repo.ApplyTaskStateSync(ctx, req.TaskID, conn.AgentID, runID, result.NormalizedTo, "Waiting for review", "", req.Summary); err != nil {
		return 0, err
	}
	if req.Summary != "" {
		_ = s.repo.InsertComment(ctx, req.TaskID, conn.AgentID, req.Summary)
	}
	if s.logActivity != nil {
		go s.logActivity(req.TaskID, conn.AgentID, "review_requested", req.Summary)
	}
	if s.auditLog != nil {
		s.auditLog("agent", conn.AgentID, "task", fmt.Sprintf("%d", req.TaskID), "review_requested", map[string]interface{}{"status": task.Status}, map[string]interface{}{"status": result.NormalizedTo}, map[string]interface{}{"summary": req.Summary, "run_id": runID})
	}
	s.afterTaskMutation(ctx, req.TaskID)
	return runID, nil
}

func (s *Service) afterTaskMutation(ctx context.Context, taskID int) {
	if s.broadcast != nil {
		go s.broadcast("UPDATE")
	}
	if s.boardStats != nil {
		if boardID, err := s.repo.GetBoardIDByTaskID(ctx, taskID); err == nil {
			go s.boardStats.RecomputeBoard(context.Background(), boardID)
		}
	}
}
