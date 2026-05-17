package tasks

import "moziboard-backend/internal/workflow"

func NormalizeTaskStatus(listID string) string {
	return workflow.NormalizeTaskState(listID)
}

func StatusToListID(status string) string {
	return workflow.StatusToListID(status)
}
