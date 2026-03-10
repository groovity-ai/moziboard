package clawn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type TokenGenerator func() (plain string, hash string, err error)

type Service struct {
	repo          *Repository
	generateToken TokenGenerator
}

func NewService(repo *Repository, generateToken TokenGenerator) *Service {
	return &Service{repo: repo, generateToken: generateToken}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func interfaceToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case fmt.Stringer:
		return strings.TrimSpace(t.String())
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(t, 'f', -1, 64))
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return t.String()
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func interfaceToStringSlice(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s := interfaceToString(item)
			if s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	default:
		return []string{}
	}
}

func buildAuthHeaders(c *fiber.Ctx, req *http.Request) bool {
	authHeader := strings.TrimSpace(c.Get("Authorization"))
	authCookie := strings.TrimSpace(c.Get("Cookie"))
	if authHeader == "" && authCookie == "" {
		return false
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if authCookie != "" {
		req.Header.Set("Cookie", authCookie)
	}
	return true
}

func (s *Service) FetchProjects(c *fiber.Ctx) ([]map[string]interface{}, int, error) {
	proxyURL := "http://172.17.0.1:4001/api/projects"
	req, err := http.NewRequest("GET", proxyURL, nil)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to create proxy request")
	}
	if !buildAuthHeaders(c, req) {
		return nil, 401, fmt.Errorf("unauthorized: missing authentication header or cookie")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 502, fmt.Errorf("failed to connect to AiAgenz backend: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2000))
		return nil, resp.StatusCode, fmt.Errorf("AiAgenz API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, 500, fmt.Errorf("failed to parse AiAgenz response")
	}
	if payload.Data == nil {
		payload.Data = []map[string]interface{}{}
	}
	return payload.Data, 200, nil
}

func normalizeProject(item map[string]interface{}) ProjectSource {
	projectID := firstNonEmpty(interfaceToString(item["project_id"]), interfaceToString(item["id"]))
	displayName := firstNonEmpty(interfaceToString(item["display_name"]), interfaceToString(item["name"]), projectID)
	status := firstNonEmpty(interfaceToString(item["status"]), "unknown")
	engine := firstNonEmpty(interfaceToString(item["engine"]), "openclaw")
	caps := interfaceToStringSlice(item["capabilities"])
	isConnectable := projectID != ""
	connectReason := "project_runtime_available"
	if projectID == "" {
		isConnectable = false
		connectReason = "missing_project_id"
	} else if status == "failed" || status == "deleted" {
		isConnectable = false
		connectReason = "project_not_available"
	}
	return ProjectSource{ProjectID: projectID, DisplayName: displayName, OwnerUserID: interfaceToString(item["userId"]), Engine: engine, Plan: interfaceToString(item["plan"]), Status: status, ContainerID: interfaceToString(item["containerId"]), ContainerName: interfaceToString(item["containerName"]), Capabilities: caps, IsConnectable: isConnectable, ConnectReason: connectReason}
}

func (s *Service) ListProjectSources(c *fiber.Ctx, boardID string) ([]ProjectSource, int, error) {
	rawProjects, statusCode, err := s.FetchProjects(c)
	if err != nil {
		return nil, statusCode, err
	}
	existingByProjectID := s.repo.ExistingProjectConnections(context.Background(), boardID)
	items := make([]ProjectSource, 0, len(rawProjects))
	for _, raw := range rawProjects {
		item := normalizeProject(raw)
		if existingAgentID, ok := existingByProjectID[item.ProjectID]; ok {
			item.AlreadyConnected = true
			item.ExistingAgentID = &existingAgentID
		}
		items = append(items, item)
	}
	return items, 200, nil
}

func (s *Service) ConnectProject(c *fiber.Ctx, req ConnectReq) (map[string]interface{}, int, error) {
	if strings.TrimSpace(req.BoardID) == "" || strings.TrimSpace(req.ClawnProjectID) == "" {
		return nil, 400, fmt.Errorf("board_id and clawn_project_id are required")
	}
	rawProjects, statusCode, err := s.FetchProjects(c)
	if err != nil {
		return nil, statusCode, err
	}
	var selected *ProjectSource
	for _, raw := range rawProjects {
		item := normalizeProject(raw)
		if item.ProjectID == req.ClawnProjectID {
			selected = &item
			break
		}
	}
	if selected == nil {
		return nil, 404, fmt.Errorf("clawn_project_not_found")
	}
	if !selected.IsConnectable {
		return nil, 409, fmt.Errorf("clawn_project_not_connectable:%s", selected.ConnectReason)
	}
	plainToken, tokenHash, err := s.generateToken()
	if err != nil {
		return nil, 500, err
	}
	agentID, connectorID, boardAgentID, err := s.repo.UpsertConnectedProject(context.Background(), req, *selected, tokenHash)
	if err != nil {
		return nil, 500, err
	}
	return map[string]interface{}{
		"ok":            true,
		"agent":         map[string]interface{}{"id": agentID, "display_name": firstNonEmpty(selected.DisplayName, selected.ProjectID)},
		"connector":     map[string]interface{}{"id": connectorID, "connector_type": "clawn_native", "transport_mode": "internal", "status": "connected"},
		"board_agent":   map[string]interface{}{"id": boardAgentID, "board_id": req.BoardID, "agent_id": agentID},
		"machine_token": plainToken,
	}, 200, nil
}
