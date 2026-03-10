package gatewayutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func TruncateForLog(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func SendAgentMessageViaGateway(agentID string, sessionKey string, message string, userKey string) (string, error) {
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
		"user":     FirstNonEmpty(strings.TrimSpace(userKey), fmt.Sprintf("moziboard:%s", agentID)),
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
