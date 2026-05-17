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

type AuditLog struct {
	ID           int       `json:"id"`
	ActorType    string    `json:"actor_type"`
	ActorID      string    `json:"actor_id"`
	EntityType   string    `json:"entity_type"`
	EntityID     string    `json:"entity_id"`
	Action       string    `json:"action"`
	OldValueJSON string    `json:"old_value_json,omitempty"`
	NewValueJSON string    `json:"new_value_json,omitempty"`
	MetadataJSON string    `json:"metadata_json,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type MaintenanceReport struct {
	StaleAgentsMarked    int `json:"stale_agents_marked"`
	StaleRunsClosed      int `json:"stale_runs_closed"`
	DriftedTasksRepaired int `json:"drifted_tasks_repaired"`
}

type OpsSummary struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Agents      OpsAgentMetrics     `json:"agents"`
	Runs        OpsRunMetrics       `json:"runs"`
	Events      OpsEventMetrics     `json:"events"`
	Connectors  OpsConnectorMetrics `json:"connectors"`
	AuditLogs   OpsAuditMetrics     `json:"audit_logs"`
}

type OpsAgentMetrics struct {
	Total      int `json:"total"`
	Online     int `json:"online"`
	Offline    int `json:"offline"`
	Stale      int `json:"stale"`
	Blocked    int `json:"blocked"`
	WithIssues int `json:"with_issues"`
}

type OpsRunMetrics struct {
	Total   int `json:"total"`
	Active  int `json:"active"`
	Stale   int `json:"stale"`
	Failed  int `json:"failed"`
	Review  int `json:"review"`
	Blocked int `json:"blocked"`
}

type OpsEventMetrics struct {
	Pending    int `json:"pending"`
	Processing int `json:"processing"`
	Failed     int `json:"failed"`
	Dead       int `json:"dead"`
	Sent       int `json:"sent"`
	Routed     int `json:"routed"`
	Ignored    int `json:"ignored"`
	RetryDue   int `json:"retry_due"`
}

type OpsConnectorMetrics struct {
	Total      int `json:"total"`
	Connected  int `json:"connected"`
	Disabled   int `json:"disabled"`
	Webhook    int `json:"webhook"`
	Internal   int `json:"internal"`
	WithErrors int `json:"with_errors"`
}

type OpsAuditMetrics struct {
	Total24h        int `json:"total_24h"`
	Maintenance24h  int `json:"maintenance_24h"`
	OpsActions24h   int `json:"ops_actions_24h"`
	RuntimeEvents24h int `json:"runtime_events_24h"`
}
