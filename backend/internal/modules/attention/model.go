package attention

import "time"

type Item struct {
	Type        string     `json:"type"`
	Severity    string     `json:"severity"`
	EntityKind  string     `json:"entity_kind,omitempty"`
	EntityID    string     `json:"entity_id,omitempty"`
	Reason      string     `json:"reason,omitempty"`
	Source      string     `json:"source,omitempty"`
	BoardID     string     `json:"board_id"`
	BoardTitle  string     `json:"board_title"`
	TaskID      *int       `json:"task_id,omitempty"`
	TaskTitle   string     `json:"task_title,omitempty"`
	AgentID     string     `json:"agent_id,omitempty"`
	AgentName   string     `json:"agent_name,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	Href        string     `json:"href,omitempty"`
}
