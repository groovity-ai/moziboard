package runtime

type TaskRef struct {
	TaskID int `json:"task_id"`
}

type HeartbeatReq struct {
	Status          string `json:"status"`
	CurrentActivity string `json:"current_activity"`
	CurrentTaskID   *int   `json:"current_task_id"`
}

type TaskAckReq struct {
	TaskID  int    `json:"task_id"`
	Message string `json:"message"`
}

type TaskUpdateReq struct {
	TaskID          int    `json:"task_id"`
	Status          string `json:"status"`
	ProgressMessage string `json:"progress_message"`
	CurrentActivity string `json:"current_activity"`
	BlockedReason   string `json:"blocked_reason"`
}

type TaskCommentReq struct {
	TaskID      int    `json:"task_id"`
	Content     string `json:"content"`
	MessageType string `json:"message_type"`
}

type TaskDeliverableReq struct {
	TaskID       int    `json:"task_id"`
	Title        string `json:"title"`
	ArtifactType string `json:"artifact_type"`
	Content      string `json:"content"`
	Summary      string `json:"summary"`
}

type TaskReviewReq struct {
	TaskID  int    `json:"task_id"`
	Summary string `json:"summary"`
}
