package docs

import "time"

type Document struct {
	ID        int       `json:"id"`
	BoardID   string    `json:"board_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
