package deliverables

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type ActivityLogger func(taskID int, userID, action, details string)

type Service struct {
	repo           *Repository
	activityLogger ActivityLogger
}

func NewService(repo *Repository, activityLogger ActivityLogger) *Service {
	return &Service{repo: repo, activityLogger: activityLogger}
}

func (s *Service) ListByTask(ctx context.Context, taskID int) ([]Deliverable, error) {
	return s.repo.ListByTask(ctx, taskID)
}

func (s *Service) Create(ctx context.Context, taskID int, d *Deliverable) error {
	d.Title = strings.TrimSpace(d.Title)
	if d.Title == "" {
		return errors.New("title is required")
	}
	d.TaskID = taskID
	if err := s.repo.Create(ctx, taskID, d); err != nil {
		return err
	}
	if s.activityLogger != nil {
		go s.activityLogger(taskID, "system", "created_deliverable", fmt.Sprintf("Generated deliverable: %s", d.Title))
	}
	return nil
}
