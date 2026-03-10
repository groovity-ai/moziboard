package ops

import "time"

type AgentEvent struct {
	ID               int        `json:"id"`
	AgentID          string     `json:"agent_id"`
	BoardID          *string    `json:"board_id,omitempty"`
	TaskID           *int       `json:"task_id,omitempty"`
	EventType        string     `json:"event_type"`
	PayloadJSON      string     `json:"payload_json"`
	DeliveryStatus   string     `json:"delivery_status"`
	DeliveryAttempts int        `json:"delivery_attempts"`
	NextAttemptAt    *time.Time `json:"next_attempt_at,omitempty"`
	LastDeliveryAt   *time.Time `json:"last_delivery_at,omitempty"`
	ResponseStatus   string     `json:"response_status,omitempty"`
	ResponseBody     string     `json:"response_body,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	ProcessedAt      *time.Time `json:"processed_at,omitempty"`
}
