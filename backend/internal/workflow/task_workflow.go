package workflow

import (
	"fmt"
	"strings"
)

const (
	StateTodo       = "todo"
	StateAssigned   = "assigned"
	StateInProgress = "in_progress"
	StateReview     = "review"
	StateBlocked    = "blocked"
	StateDone       = "done"
)

const (
	ReviewApprove       = "approve"
	ReviewRequestChange = "request_changes"
	ReviewReopen        = "reopen"
)

const (
	TriggerManualUpdate    = "manual_update"
	TriggerRuntimeAck      = "runtime_ack"
	TriggerRuntimeUpdate   = "runtime_update"
	TriggerReviewAction    = "review_action"
	TriggerAssignment      = "assignment_change"
	TriggerReviewRequested = "review_requested"
)

type TransitionRequest struct {
	FromState    string
	ToState      string
	Trigger      string
	ActorType    string
	ActorID      string
	Note         string
	Reason       string
	ReviewAction string
}

type TransitionResult struct {
	NormalizedFrom string
	NormalizedTo   string
	ActivityAction string
	ActivityDetail string
}

func NormalizeTaskState(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "assigned":
		return StateAssigned
	case "doing", "in_progress":
		return StateInProgress
	case "qa", "review":
		return StateReview
	case "blocked":
		return StateBlocked
	case "done":
		return StateDone
	case "backlog", "todo", "":
		return StateTodo
	default:
		return StateTodo
	}
}

func StatusToListID(status string) string {
	switch NormalizeTaskState(status) {
	case StateInProgress:
		return "doing"
	case StateReview:
		return "qa"
	case StateBlocked:
		return "blocked"
	case StateDone:
		return "done"
	case StateAssigned, StateTodo:
		return "todo"
	default:
		return "todo"
	}
}

func MapTaskStateToRunState(taskState string) string {
	switch NormalizeTaskState(taskState) {
	case StateAssigned:
		return "queued"
	case StateInProgress:
		return "running"
	case StateReview:
		return "review"
	case StateBlocked:
		return "blocked"
	case StateDone:
		return "done"
	default:
		return "queued"
	}
}

func DeriveAgentState(taskState string) (status string, activity string, clearCurrent bool) {
	switch NormalizeTaskState(taskState) {
	case StateAssigned:
		return "online", "Queued for work", false
	case StateInProgress:
		return "busy", "Working on task", false
	case StateReview:
		return "online", "Waiting for review", false
	case StateBlocked:
		return "blocked", "Task blocked", false
	case StateDone:
		return "online", "Idle", true
	default:
		return "online", "Idle", true
	}
}

func ResolveAssignmentState(currentState string, hadAssignee bool, hasAssignee bool) string {
	state := NormalizeTaskState(currentState)
	if !hadAssignee && hasAssignee && state == StateTodo {
		return StateAssigned
	}
	if hadAssignee && !hasAssignee && state == StateAssigned {
		return StateTodo
	}
	return state
}

func ValidateTransition(req TransitionRequest) (TransitionResult, error) {
	from := NormalizeTaskState(req.FromState)
	to := NormalizeTaskState(req.ToState)
	if from == "" {
		from = StateTodo
	}
	if to == "" {
		to = from
	}
	if from == to {
		return TransitionResult{NormalizedFrom: from, NormalizedTo: to, ActivityAction: "task_updated", ActivityDetail: fmt.Sprintf("Task remains %s", to)}, nil
	}
	if to == StateBlocked && strings.TrimSpace(req.Reason) == "" {
		return TransitionResult{}, fmt.Errorf("blocked_reason is required when moving task to blocked")
	}

	allowed := false
	action := "task_status_changed"
	detail := fmt.Sprintf("Status changed from %s to %s", from, to)

	switch from {
	case StateTodo:
		allowed = to == StateAssigned || to == StateInProgress || to == StateBlocked
	case StateAssigned:
		allowed = to == StateInProgress || to == StateBlocked || to == StateTodo
	case StateInProgress:
		allowed = to == StateReview || to == StateBlocked || to == StateDone
	case StateReview:
		switch to {
		case StateDone:
			allowed = req.ReviewAction == ReviewApprove
			action = "review_approved"
			detail = strings.TrimSpace(req.Note)
			if detail == "" {
				detail = "Review approved"
			}
		case StateInProgress:
			allowed = req.ReviewAction == ReviewRequestChange || req.ReviewAction == ReviewReopen
			if req.ReviewAction == ReviewRequestChange {
				action = "review_changes_requested"
				detail = strings.TrimSpace(req.Note)
				if detail == "" {
					detail = "Changes requested during review"
				}
			} else {
				action = "task_reopened"
				detail = strings.TrimSpace(req.Note)
				if detail == "" {
					detail = "Task reopened from review"
				}
			}
		case StateBlocked:
			allowed = true
			action = "task_blocked"
			detail = strings.TrimSpace(req.Reason)
		}
	case StateBlocked:
		allowed = to == StateInProgress || to == StateReview || to == StateTodo
		if to != StateBlocked {
			action = "task_unblocked"
			detail = strings.TrimSpace(req.Note)
			if detail == "" {
				detail = fmt.Sprintf("Task moved from blocked to %s", to)
			}
		}
	case StateDone:
		allowed = to == StateInProgress && req.ReviewAction == ReviewReopen
		action = "task_reopened"
		detail = strings.TrimSpace(req.Note)
		if detail == "" {
			detail = "Completed task reopened"
		}
	}

	if !allowed {
		return TransitionResult{}, fmt.Errorf("invalid task transition: %s -> %s", from, to)
	}
	if to == StateBlocked {
		action = "task_blocked"
		if strings.TrimSpace(detail) == "" {
			detail = strings.TrimSpace(req.Reason)
		}
	}
	if from != StateBlocked && to != StateBlocked && action == "task_status_changed" && strings.TrimSpace(req.Note) != "" {
		detail = strings.TrimSpace(req.Note)
	}
	return TransitionResult{NormalizedFrom: from, NormalizedTo: to, ActivityAction: action, ActivityDetail: detail}, nil
}
