package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type CreateAgentRunFunc func(taskID int, agentID string, status string, activity string) *int
type LogActivityFunc func(taskID int, userID, action, details string)
type BroadcastFunc func(msg string)
type TruncateForLogFunc func(string, int) string

type Service struct {
	repo           *Repository
	createAgentRun CreateAgentRunFunc
	logActivity    LogActivityFunc
	broadcast      BroadcastFunc
	truncateForLog TruncateForLogFunc
}

func NewService(repo *Repository, createAgentRun CreateAgentRunFunc, logActivity LogActivityFunc, broadcast BroadcastFunc, truncateForLog TruncateForLogFunc) *Service {
	return &Service{repo: repo, createAgentRun: createAgentRun, logActivity: logActivity, broadcast: broadcast, truncateForLog: truncateForLog}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (s *Service) StartDispatcher(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			pendingTasks, err := s.repo.ListPendingTasks(ctx)
			if err != nil {
				log.Printf("Dispatcher query failed: %v", err)
				continue
			}
			for _, task := range pendingTasks {
				t := task
				go s.ProcessAgentTask(ctx, t)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) ProcessAgentTask(ctx context.Context, t Task) {
	if t.AssigneeID == nil || *t.AssigneeID == "" {
		return
	}
	agentName := *t.AssigneeID
	if runID := s.createAgentRun(t.ID, agentName, "queued", "Auto-dispatched by MoziBoard"); runID != nil {
		log.Printf("Dispatcher created queued run %d for agent %s task %d", *runID, agentName, t.ID)
	}
	if err := s.repo.MarkTaskDoing(ctx, t.ID); err != nil {
		log.Printf("Dispatcher failed to mark task %d doing: %v", t.ID, err)
		return
	}
	if s.logActivity != nil {
		go s.logActivity(t.ID, agentName, "moved", "Moved to list doing (Auto-dispatched)")
	}
	if s.broadcast != nil {
		go s.broadcast("UPDATE")
	}
	message := fmt.Sprintf("MoziBoard task assignment. Task ID: %d\nTitle: %s\nDescription: %s\nPlease acknowledge, work the task, and report progress back through the connected runtime if available.", t.ID, t.Title, t.Description)
	respBody, err := s.sendAgentMessageViaGateway(agentName, "", message, fmt.Sprintf("moziboard:auto-dispatch:%s", agentName))
	if err != nil {
		log.Printf("OpenClaw gateway error for agent %s on task %d: %v", agentName, t.ID, err)
		return
	}
	tr := respBody
	if s.truncateForLog != nil {
		tr = s.truncateForLog(respBody, 600)
	}
	log.Printf("OpenClaw gateway dispatched for agent %s on task %d. Response: %s", agentName, t.ID, tr)
}

func (s *Service) sendAgentMessageViaGateway(agentID string, sessionKey string, message string, userKey string) (string, error) {
	gatewayURL := strings.TrimSpace(os.Getenv("OPENCLAW_GATEWAY_URL"))
	if gatewayURL == "" {
		gatewayURL = "http://host.docker.internal:18789/v1/chat/completions"
	}
	gatewayToken := strings.TrimSpace(os.Getenv("OPENCLAW_GATEWAY_TOKEN"))
	if gatewayToken == "" {
		return "", fmt.Errorf("gateway_missing_token")
	}
	if strings.TrimSpace(agentID) == "" {
		return "", fmt.Errorf("gateway_missing_agent_id")
	}
	payload := map[string]interface{}{
		"model":    fmt.Sprintf("openclaw:%s", agentID),
		"messages": []map[string]string{{"role": "user", "content": message}},
		"stream":   false,
		"user":     firstNonEmpty(strings.TrimSpace(userKey), fmt.Sprintf("moziboard:%s", agentID)),
	}
	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", gatewayURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("gateway_request_build_failed: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+gatewayToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-openclaw-agent-id", agentID)
	if strings.TrimSpace(sessionKey) != "" {
		req.Header.Set("x-openclaw-session-key", sessionKey)
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("gateway_request_failed: %w", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("gateway_http_%d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}
	return strings.TrimSpace(string(respBytes)), nil
}
