package dispatch

import "time"

type Task struct {
	ID            int     `json:"id"`
	BoardID       string  `json:"board_id"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	ListID        string  `json:"list_id"`
	Position      int     `json:"position"`
	AssigneeID    *string `json:"assignee_id"`
	ParentID      *int    `json:"parent_id,omitempty"`
	UpdatedBy     string  `json:"updated_by,omitempty"`
	Status        string  `json:"status,omitempty"`
	BlockedReason string  `json:"blocked_reason,omitempty"`
}

type AgentConnector struct {
	ID            int        `json:"id"`
	AgentID       string     `json:"agent_id"`
	ConnectorType string     `json:"connector_type"`
	TransportMode string     `json:"transport_mode,omitempty"`
	AuthType      string     `json:"auth_type"`
	EndpointURL   string     `json:"endpoint_url,omitempty"`
	BaseURL       string     `json:"base_url,omitempty"`
	AgentRef      string     `json:"agent_ref,omitempty"`
	SessionKey    string     `json:"session_key,omitempty"`
	Status        string     `json:"status"`
	MetadataJSON  string     `json:"metadata_json,omitempty"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
