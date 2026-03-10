package clawn

type ProjectSource struct {
	ProjectID        string   `json:"project_id"`
	DisplayName      string   `json:"display_name"`
	OwnerUserID      string   `json:"owner_user_id,omitempty"`
	Engine           string   `json:"engine,omitempty"`
	Plan             string   `json:"plan,omitempty"`
	Status           string   `json:"status,omitempty"`
	ContainerID      string   `json:"container_id,omitempty"`
	ContainerName    string   `json:"container_name,omitempty"`
	Capabilities     []string `json:"capabilities,omitempty"`
	IsConnectable    bool     `json:"is_connectable"`
	ConnectReason    string   `json:"connect_reason,omitempty"`
	AlreadyConnected bool     `json:"already_connected"`
	ExistingAgentID  *string  `json:"existing_agent_id,omitempty"`
}

type ConnectReq struct {
	BoardID               string `json:"board_id"`
	ClawnProjectID        string `json:"clawn_project_id"`
	BoardRole             string `json:"board_role"`
	AutoAcceptTasks       *bool  `json:"auto_accept_tasks"`
	CanComment            *bool  `json:"can_comment"`
	CanUpdateStatus       *bool  `json:"can_update_status"`
	CanAccessDocs         *bool  `json:"can_access_docs"`
	CanCreateDeliverables *bool  `json:"can_create_deliverables"`
}
