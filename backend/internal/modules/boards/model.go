package boards

type Board struct {
	ID          string  `json:"id"`
	UserID      *string `json:"user_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
}

type Member struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Avatar string `json:"avatar"`
}

type BoardMemberReq struct {
	MemberID string `json:"member_id"`
	Role     string `json:"role"`
}
