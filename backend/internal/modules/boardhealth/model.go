package boardhealth

import "time"

type Metrics struct {
	OpenTasks       int `json:"open_tasks"`
	ReviewTasks     int `json:"review_tasks"`
	BlockedTasks    int `json:"blocked_tasks"`
	DocsCount       int `json:"docs_count"`
	ConnectedAgents int `json:"connected_agents"`
	AgentIssues     int `json:"agent_issues"`
}

type HealthSignal struct {
	Severity string `json:"severity"`
	Reason   string `json:"reason"`
	Label    string `json:"label"`
}

type BoardStatus struct {
	ID              string         `json:"id"`
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	Metrics         Metrics        `json:"metrics"`
	Health          string         `json:"health"`
	HealthScore     int            `json:"health_score"`
	Signals         []HealthSignal `json:"signals"`
	LastActivityAt  *time.Time     `json:"last_activity_at,omitempty"`
	StatsComputedAt *time.Time     `json:"stats_computed_at,omitempty"`
	StatsAgeSeconds int64          `json:"stats_age_seconds"`
	IsStale         bool           `json:"is_stale"`
}

type Summary struct {
	Boards         int `json:"boards"`
	NeedsAttention int `json:"needs_attention"`
	BlockedTasks   int `json:"blocked_tasks"`
	PendingReviews int `json:"pending_reviews"`
	AgentIssues    int `json:"agent_issues"`
	HealthyBoards  int `json:"healthy_boards"`
	QuietBoards    int `json:"quiet_boards"`
}
