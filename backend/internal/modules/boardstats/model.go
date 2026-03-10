package boardstats

import "time"

type BoardStats struct {
	BoardID          string     `json:"board_id"`
	OpenTasks        int        `json:"open_tasks"`
	ReviewTasks      int        `json:"review_tasks"`
	BlockedTasks     int        `json:"blocked_tasks"`
	DocsCount        int        `json:"docs_count"`
	ConnectedAgents  int        `json:"connected_agents"`
	AgentIssues      int        `json:"agent_issues"`
	LastTaskEventAt  *time.Time `json:"last_task_event_at,omitempty"`
	LastDocEventAt   *time.Time `json:"last_doc_event_at,omitempty"`
	LastAgentEventAt *time.Time `json:"last_agent_event_at,omitempty"`
	LastEventAt      *time.Time `json:"last_event_at,omitempty"`
	ComputedAt       time.Time  `json:"computed_at"`
}
