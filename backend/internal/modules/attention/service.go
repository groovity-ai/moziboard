package attention

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListByUser(ctx context.Context, userID string, limit int) ([]Item, error) {
	return s.repo.ListByUser(ctx, userID, limit)
}

func (s *Service) ListByBoard(ctx context.Context, boardID string, limit int) ([]Item, error) {
	return s.repo.ListByBoard(ctx, boardID, limit)
}
