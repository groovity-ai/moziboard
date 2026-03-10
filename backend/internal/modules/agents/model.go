package agents

import "time"

type Agent struct {
	ID              string     `json:"id"`
	DisplayName     string     `json:"display_name,omitempty"`
	RoleName        string     `json:"role_name,omitempty"`
	Avatar          string     `json:"avatar,omitempty"`
	Provider        string     `json:"provider,omitempty"`
	Engine          string     `json:"engine,omitempty"`
	Description     string     `json:"description,omitempty"`
	IsNativeClawn   bool       `json:"is_native_clawn"`
	Soul            string     `json:"soul"`
	Memory          string     `json:"memory"`
	Rules           string     `json:"rules"`
	CronSchedule    string     `json:"cron_schedule"`
	Active          bool       `json:"active"`
	Status          string     `json:"status"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	LastSeenAt      *time.Time `json:"last_seen_at,omitempty"`
	CurrentTaskID   *int       `json:"current_task_id,omitempty"`
	CurrentRunID    *int       `json:"current_run_id,omitempty"`
	CurrentActivity string     `json:"current_activity,omitempty"`
	HealthNote      string     `json:"health_note,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
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

type BoardAgent struct {
	ID                    int       `json:"id"`
	BoardID               string    `json:"board_id"`
	AgentID               string    `json:"agent_id"`
	BoardRole             string    `json:"board_role"`
	Active                bool      `json:"active"`
	AutoAcceptTasks       bool      `json:"auto_accept_tasks"`
	CanComment            bool      `json:"can_comment"`
	CanUpdateStatus       bool      `json:"can_update_status"`
	CanAccessDocs         bool      `json:"can_access_docs"`
	CanCreateDeliverables bool      `json:"can_create_deliverables"`
	CapabilitiesJSON      string    `json:"capabilities_json,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
	Agent                 *Agent    `json:"agent,omitempty"`
}

type AgentRun struct {
	ID              int        `json:"id"`
	TaskID          int        `json:"task_id"`
	AgentID         string     `json:"agent_id"`
	SessionKey      string     `json:"session_key"`
	ProviderType    string     `json:"provider_type"`
	Status          string     `json:"status"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	CurrentActivity string     `json:"current_activity,omitempty"`
	ErrorSummary    string     `json:"error_summary,omitempty"`
	ResultSummary   string     `json:"result_summary,omitempty"`
}

type Notification struct {
	ID            int        `json:"id"`
	TaskID        *int       `json:"task_id,omitempty"`
	TargetAgentID string     `json:"target_agent_id"`
	SourceAgentID *string    `json:"source_agent_id,omitempty"`
	Type          string     `json:"type"`
	Content       string     `json:"content"`
	Delivered     bool       `json:"delivered"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type AgentRegisterReq struct {
	ID            string                 `json:"id"`
	DisplayName   string                 `json:"display_name"`
	RoleName      string                 `json:"role_name"`
	Avatar        string                 `json:"avatar"`
	Provider      string                 `json:"provider"`
	Engine        string                 `json:"engine"`
	Description   string                 `json:"description"`
	IsNativeClawn bool                   `json:"is_native_clawn"`
	Connector     AgentConnectorInput    `json:"connector"`
	Capabilities  map[string]interface{} `json:"capabilities"`
}

type AgentConnectorInput struct {
	ConnectorType string                 `json:"connector_type"`
	TransportMode string                 `json:"transport_mode"`
	AuthType      string                 `json:"auth_type"`
	EndpointURL   string                 `json:"endpoint_url"`
	BaseURL       string                 `json:"base_url"`
	AgentRef      string                 `json:"agent_ref"`
	SessionKey    string                 `json:"session_key"`
	SharedSecret  string                 `json:"shared_secret"`
	Metadata      map[string]interface{} `json:"metadata"`
}

type AgentBoardConnectReq struct {
	BoardID         string                 `json:"board_id"`
	BoardRole       string                 `json:"board_role"`
	AutoAcceptTasks *bool                  `json:"auto_accept_tasks"`
	Permissions     map[string]interface{} `json:"permissions"`
}

type AgentStatusUpdateReq struct {
	Status          string `json:"status"`
	CurrentActivity string `json:"current_activity"`
	HealthNote      string `json:"health_note"`
}
