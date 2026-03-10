package boardhealth

import (
	"context"
	"time"
)

const staleAfter = 60 * time.Second

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListByUser(ctx context.Context, userID string) ([]BoardStatus, Summary, error) {
	rows, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, Summary{}, err
	}
	return s.buildStatuses(rows)
}

func (s *Service) ListByBoard(ctx context.Context, boardID string) ([]BoardStatus, Summary, error) {
	rows, err := s.repo.ListByBoard(ctx, boardID)
	if err != nil {
		return nil, Summary{}, err
	}
	return s.buildStatuses(rows)
}

func (s *Service) buildStatuses(rows []boardRow) ([]BoardStatus, Summary, error) {
	items := make([]BoardStatus, 0, len(rows))
	summary := Summary{}
	now := time.Now()
	for _, row := range rows {
		health, score, signals := computeBoardHealth(row)
		statsAgeSeconds := int64(0)
		isStale := true
		if row.StatsComputedAt != nil {
			statsAgeSeconds = int64(now.Sub(*row.StatsComputedAt).Seconds())
			if statsAgeSeconds < 0 {
				statsAgeSeconds = 0
			}
			isStale = now.Sub(*row.StatsComputedAt) > staleAfter
		}
		item := BoardStatus{
			ID:          row.ID,
			Title:       row.Title,
			Description: row.Description,
			Metrics: Metrics{
				OpenTasks:       row.OpenTasks,
				ReviewTasks:     row.ReviewTasks,
				BlockedTasks:    row.BlockedTasks,
				DocsCount:       row.DocsCount,
				ConnectedAgents: row.ConnectedAgents,
				AgentIssues:     row.AgentIssues,
			},
			Health:          health,
			HealthScore:     score,
			Signals:         signals,
			LastActivityAt:  row.LastActivityAt,
			StatsComputedAt: row.StatsComputedAt,
			StatsAgeSeconds: statsAgeSeconds,
			IsStale:         isStale,
		}
		items = append(items, item)
		summary.Boards++
		summary.BlockedTasks += row.BlockedTasks
		summary.PendingReviews += row.ReviewTasks
		summary.AgentIssues += row.AgentIssues
		switch item.Health {
		case "blocked", "needs_review", "agent_issue", "at_risk":
			summary.NeedsAttention++
		case "healthy":
			summary.HealthyBoards++
		case "quiet":
			summary.QuietBoards++
		}
	}
	if items == nil {
		items = []BoardStatus{}
	}
	return items, summary, nil
}

func computeBoardHealth(row boardRow) (string, int, []HealthSignal) {
	signals := make([]HealthSignal, 0, 4)
	score := 100

	if row.BlockedTasks > 0 {
		signals = append(signals, HealthSignal{Severity: "high", Reason: "blocked_tasks", Label: "Blocked tasks need action"})
		score -= min(60, row.BlockedTasks*20)
	}
	if row.ReviewTasks > 0 {
		signals = append(signals, HealthSignal{Severity: "medium", Reason: "pending_review", Label: "Work is waiting for review"})
		score -= min(30, row.ReviewTasks*10)
	}
	if row.AgentIssues > 0 {
		signals = append(signals, HealthSignal{Severity: "high", Reason: "agent_issues", Label: "Agent or connector health issue detected"})
		score -= min(40, row.AgentIssues*15)
	}
	if row.OpenTasks > 0 && row.ConnectedAgents == 0 {
		signals = append(signals, HealthSignal{Severity: "medium", Reason: "unstaffed_work", Label: "Open work exists without connected agents"})
		score -= 15
	}
	if row.OpenTasks == 0 && row.ReviewTasks == 0 && row.BlockedTasks == 0 && row.AgentIssues == 0 && row.DocsCount == 0 && row.ConnectedAgents == 0 {
		return "quiet", 100, []HealthSignal{}
	}
	if score < 0 {
		score = 0
	}

	switch {
	case row.BlockedTasks > 0:
		return "blocked", score, signals
	case row.AgentIssues > 0:
		return "agent_issue", score, signals
	case row.ReviewTasks > 0:
		return "needs_review", score, signals
	case score < 85:
		if len(signals) == 0 {
			signals = append(signals, HealthSignal{Severity: "medium", Reason: "attention_needed", Label: "Board needs attention"})
		}
		return "at_risk", score, signals
	default:
		if len(signals) == 0 {
			signals = append(signals, HealthSignal{Severity: "low", Reason: "operating_normally", Label: "Board is operating normally"})
		}
		return "healthy", score, signals
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
