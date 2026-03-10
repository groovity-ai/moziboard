package activityfeed

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListRecentByUser(ctx context.Context, userID string, limit int) ([]Item, error) {
	return s.repo.ListRecentByUser(ctx, userID, limit)
}

func (s *Service) ListRecentByBoard(ctx context.Context, boardID string, limit int) ([]Item, error) {
	return s.repo.ListRecentByBoard(ctx, boardID, limit)
}
