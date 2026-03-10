package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentEvent struct {
	ID               int
	AgentID          string
	BoardID          *string
	TaskID           *int
	EventType        string
	PayloadJSON      string
	DeliveryStatus   string
	DeliveryAttempts int
	NextAttemptAt    *time.Time
	LastDeliveryAt   *time.Time
	ResponseStatus   string
	ResponseBody     string
	CreatedAt        time.Time
	ProcessedAt      *time.Time
}

type AgentConnector struct {
	ID            int
	AgentID       string
	ConnectorType string
	TransportMode string
	AuthType      string
	EndpointURL   string
	BaseURL       string
	AgentRef      string
	SessionKey    string
	Status        string
	MetadataJSON  string
}

type SignPayloadFunc func(secret, timestamp, body string) string
type TruncateForLogFunc func(string, int) string
type SendInternalFunc func(agentID string, sessionKey string, message string, userKey string) (string, error)

type Service struct {
	db                *pgxpool.Pool
	deliverySemaphore chan struct{}
	signPayload       SignPayloadFunc
	truncateForLog    TruncateForLogFunc
	sendInternal      SendInternalFunc
}

func NewService(db *pgxpool.Pool, semaphore chan struct{}, signPayload SignPayloadFunc, truncateForLog TruncateForLogFunc, sendInternal SendInternalFunc) *Service {
	return &Service{db: db, deliverySemaphore: semaphore, signPayload: signPayload, truncateForLog: truncateForLog, sendInternal: sendInternal}
}

func (s *Service) DeliverPendingAgentEvents() {
	_, _ = s.db.Exec(context.Background(), `
		UPDATE agent_events
		SET delivery_status='pending', response_status='processing_timeout_requeued', next_attempt_at=CURRENT_TIMESTAMP + INTERVAL '30 seconds'
		WHERE delivery_status='processing'
		  AND processed_at IS NULL
		  AND last_delivery_at < CURRENT_TIMESTAMP - INTERVAL '60 seconds'`)

	rows, err := s.db.Query(context.Background(), `
		WITH claimed AS (
			SELECT id
			FROM agent_events
			WHERE delivery_status IN ('pending','failed')
			  AND (next_attempt_at IS NULL OR next_attempt_at <= CURRENT_TIMESTAMP)
			  AND delivery_attempts < 8
			ORDER BY created_at ASC
			LIMIT 20
			FOR UPDATE SKIP LOCKED
		)
		UPDATE agent_events ae
		SET delivery_status='processing', last_delivery_at=CURRENT_TIMESTAMP
		FROM claimed
		WHERE ae.id = claimed.id
		RETURNING ae.id, ae.agent_id, ae.board_id::text, ae.task_id, ae.event_type, ae.payload_json::text, ae.delivery_status, ae.delivery_attempts, ae.next_attempt_at, ae.last_delivery_at, ae.response_status, ae.response_body, ae.created_at, ae.processed_at`)
	if err == nil {
		_, _ = s.db.Exec(context.Background(), `
			UPDATE agent_events
			SET delivery_status='dead', response_status='retry_exhausted', processed_at=CURRENT_TIMESTAMP
			WHERE delivery_status IN ('pending','failed')
			  AND delivery_attempts >= 8
			  AND processed_at IS NULL`)
	}
	if err != nil {
		log.Printf("event query failed: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var ev AgentEvent
		if err := rows.Scan(&ev.ID, &ev.AgentID, &ev.BoardID, &ev.TaskID, &ev.EventType, &ev.PayloadJSON, &ev.DeliveryStatus, &ev.DeliveryAttempts, &ev.NextAttemptAt, &ev.LastDeliveryAt, &ev.ResponseStatus, &ev.ResponseBody, &ev.CreatedAt, &ev.ProcessedAt); err != nil {
			log.Printf("event scan failed: %v", err)
			continue
		}
		select {
		case s.deliverySemaphore <- struct{}{}:
			go func(ev AgentEvent) {
				defer func() { <-s.deliverySemaphore }()
				defer func() {
					if r := recover(); r != nil {
						s.markEventFailed(ev, fmt.Sprintf("panic:%v", r), "")
					}
				}()
				s.processAgentEventDelivery(ev)
			}(ev)
		default:
			s.markEventFailed(ev, "dispatcher_backpressure", "")
		}
	}
}

func (s *Service) getConnectorForAgent(agentID string) (*AgentConnector, error) {
	var conn AgentConnector
	err := s.db.QueryRow(context.Background(), `SELECT id, agent_id, connector_type, transport_mode, auth_type, endpoint_url, base_url, agent_ref, session_key, status, metadata_json::text FROM agent_connectors WHERE agent_id=$1 AND status='connected' ORDER BY is_primary DESC, updated_at DESC, id DESC LIMIT 1`, agentID).
		Scan(&conn.ID, &conn.AgentID, &conn.ConnectorType, &conn.TransportMode, &conn.AuthType, &conn.EndpointURL, &conn.BaseURL, &conn.AgentRef, &conn.SessionKey, &conn.Status, &conn.MetadataJSON)
	if err != nil {
		return nil, err
	}
	return &conn, nil
}

func (s *Service) processAgentEventDelivery(ev AgentEvent) {
	conn, err := s.getConnectorForAgent(ev.AgentID)
	if err != nil || conn == nil {
		s.markEventFailed(ev, "connector_missing", "")
		return
	}
	if conn.ConnectorType == "webhook" {
		if conn.EndpointURL == "" {
			s.markEventFailed(ev, "webhook_missing_endpoint", "")
			return
		}
		s.deliverEventHTTPRequest(ev, conn, conn.EndpointURL, ev.PayloadJSON, map[string]string{"Content-Type": "application/json"}, "webhook")
		return
	}
	if conn.ConnectorType == "clawn_native" {
		envelopeBytes, _ := json.Marshal(map[string]interface{}{"event_id": ev.ID, "event_type": ev.EventType, "agent_id": ev.AgentID, "board_id": ev.BoardID, "task_id": ev.TaskID, "transport_mode": conn.TransportMode, "agent_ref": conn.AgentRef, "session_key": conn.SessionKey, "payload": json.RawMessage(ev.PayloadJSON)})
		switch conn.TransportMode {
		case "internal":
			if strings.TrimSpace(conn.SessionKey) == "" {
				s.markEventFailed(ev, "internal_missing_session_key", "")
				return
			}
			if err := s.deliverInternalClawnEvent(ev, conn, string(envelopeBytes)); err != nil {
				s.markEventFailed(ev, err.Error(), fmt.Sprintf(`{"strategy":"internal","agent_ref":%q,"session_key":%q}`, conn.AgentRef, conn.SessionKey))
				return
			}
			return
		case "remote_http":
			if conn.BaseURL == "" {
				s.markEventFailed(ev, "remote_http_missing_base_url", "")
				return
			}
			targetURL := strings.TrimRight(conn.BaseURL, "/") + "/api/runtime/events"
			headers := map[string]string{"Content-Type": "application/json", "X-Mozi-Agent-Ref": conn.AgentRef, "X-Mozi-Session-Key": conn.SessionKey}
			s.deliverEventHTTPRequest(ev, conn, targetURL, string(envelopeBytes), headers, "clawn_remote_http")
			return
		case "pull":
			responseBody := fmt.Sprintf(`{"strategy":"pull","agent_ref":%q}`, conn.AgentRef)
			_, _ = s.db.Exec(context.Background(), `UPDATE agent_events SET delivery_status='routed', delivery_attempts=delivery_attempts+1, response_status='pull_runtime_pending', response_body=$2, processed_at=CURRENT_TIMESTAMP, last_delivery_at=CURRENT_TIMESTAMP WHERE id=$1`, ev.ID, responseBody)
			return
		default:
			s.markEventFailed(ev, "unsupported_transport_mode:"+conn.TransportMode, "")
			return
		}
	}
	_, _ = s.db.Exec(context.Background(), `UPDATE agent_events SET delivery_status='ignored', response_status=$2, processed_at=CURRENT_TIMESTAMP, last_delivery_at=CURRENT_TIMESTAMP WHERE id=$1`, ev.ID, conn.ConnectorType)
}

func nextRetryDelay(attempts int) time.Duration {
	switch {
	case attempts <= 1:
		return 10 * time.Second
	case attempts == 2:
		return 30 * time.Second
	case attempts == 3:
		return 60 * time.Second
	case attempts <= 5:
		return 5 * time.Minute
	default:
		return 15 * time.Minute
	}
}

func (s *Service) markEventFailed(ev AgentEvent, responseStatus string, responseBody string) {
	nextAttempt := time.Now().Add(nextRetryDelay(ev.DeliveryAttempts + 1))
	status := "failed"
	processedAt := "NULL"
	if ev.DeliveryAttempts+1 >= 8 {
		status = "dead"
		processedAt = "CURRENT_TIMESTAMP"
	}
	query := fmt.Sprintf(`UPDATE agent_events SET delivery_status=$2, delivery_attempts=delivery_attempts+1, response_status=$3, response_body=$4, next_attempt_at=$5, last_delivery_at=CURRENT_TIMESTAMP, processed_at=%s WHERE id=$1`, processedAt)
	_, _ = s.db.Exec(context.Background(), query, ev.ID, status, responseStatus, responseBody, nextAttempt)
}

func (s *Service) deliverEventHTTPRequest(ev AgentEvent, conn *AgentConnector, targetURL string, body string, headers map[string]string, strategy string) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	req, err := http.NewRequest("POST", targetURL, strings.NewReader(body))
	if err != nil {
		s.markEventFailed(ev, err.Error(), "")
		return
	}
	for k, v := range headers {
		if strings.TrimSpace(v) != "" {
			req.Header.Set(k, v)
		}
	}
	req.Header.Set("X-Mozi-Timestamp", timestamp)
	if strings.TrimSpace(conn.AgentRef) != "" {
		req.Header.Set("X-Mozi-Agent-Ref", strings.TrimSpace(conn.AgentRef))
	}
	if strings.TrimSpace(conn.AuthType) == "shared_secret" && strings.TrimSpace(conn.MetadataJSON) != "" {
		var meta map[string]interface{}
		if json.Unmarshal([]byte(conn.MetadataJSON), &meta) == nil {
			if plainSecret, ok := meta["shared_secret"].(string); ok && strings.TrimSpace(plainSecret) != "" {
				req.Header.Set("X-Mozi-Signature", s.signPayload(plainSecret, timestamp, body))
			}
		}
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.markEventFailed(ev, err.Error(), fmt.Sprintf(`{"strategy":%q,"target_url":%q}`, strategy, targetURL))
		return
	}
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1000))
	resp.Body.Close()
	status := "failed"
	processedAt := "NULL"
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		status = "sent"
		processedAt = "CURRENT_TIMESTAMP"
	}
	responseBody := fmt.Sprintf(`{"strategy":%q,"target_url":%q,"response_body":%q}`, strategy, targetURL, string(bodyBytes))
	query := fmt.Sprintf(`UPDATE agent_events SET delivery_status=$2, delivery_attempts=delivery_attempts+1, last_delivery_at=CURRENT_TIMESTAMP, response_status=$3, response_body=$4, processed_at=%s WHERE id=$1`, processedAt)
	_, _ = s.db.Exec(context.Background(), query, ev.ID, status, resp.Status, responseBody)
}

func (s *Service) deliverInternalClawnEvent(ev AgentEvent, conn *AgentConnector, envelope string) error {
	message := fmt.Sprintf("MoziBoard internal bridge event. Connector: clawn_native/internal. AgentRef: %s. SessionKey: %s. Event JSON:\n%s", conn.AgentRef, conn.SessionKey, envelope)
	respBody, err := s.sendInternal(conn.AgentRef, conn.SessionKey, message, conn.SessionKey)
	if err != nil {
		return err
	}
	out := respBody
	if s.truncateForLog != nil {
		out = s.truncateForLog(respBody, 1200)
	}
	_, _ = s.db.Exec(context.Background(), `UPDATE agent_events SET delivery_status='sent', delivery_attempts=delivery_attempts+1, response_status='internal_runtime_sent', response_body=$2, processed_at=CURRENT_TIMESTAMP, last_delivery_at=CURRENT_TIMESTAMP WHERE id=$1`, ev.ID, fmt.Sprintf(`{"strategy":"internal_gateway","agent_ref":%q,"session_key":%q,"response":%q}`, conn.AgentRef, conn.SessionKey, out))
	return nil
}
