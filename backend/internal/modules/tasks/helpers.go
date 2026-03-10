package tasks

import "strings"

func NormalizeTaskStatus(listID string) string {
	switch strings.ToLower(listID) {
	case "doing", "in_progress":
		return "in_progress"
	case "qa", "review":
		return "review"
	case "blocked":
		return "blocked"
	case "backlog":
		return "backlog"
	case "done":
		return "done"
	default:
		return "todo"
	}
}

func StatusToListID(status string) string {
	switch strings.ToLower(status) {
	case "in_progress":
		return "doing"
	case "review":
		return "qa"
	default:
		return strings.ToLower(status)
	}
}
