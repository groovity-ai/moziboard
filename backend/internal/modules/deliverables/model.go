package deliverables

import "time"

type Deliverable struct {
	ID           int       `json:"id"`
	TaskID       int       `json:"task_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	ArtifactType string    `json:"artifact_type"`
	Content      string    `json:"content"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
