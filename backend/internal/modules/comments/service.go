package comments

import (
	"context"
	"errors"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListByTask(ctx context.Context, taskID int) ([]Comment, error) {
	return s.repo.ListByTask(ctx, taskID)
}

func (s *Service) Create(ctx context.Context, taskID int, cm *Comment) error {
	cm.UserID = strings.TrimSpace(cm.UserID)
	cm.Content = strings.TrimSpace(cm.Content)
	if cm.UserID == "" || cm.Content == "" {
		return errors.New("user_id and content are required")
	}
	cm.TaskID = taskID
	return s.repo.Create(ctx, taskID, cm)
}
