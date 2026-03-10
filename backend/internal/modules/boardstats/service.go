package boardstats

import (
	"context"

	"github.com/redis/go-redis/v9"
)

const overviewVersionKey = "home:overview:version"

type Service struct {
	repo  *Repository
	redis *redis.Client
}

func NewService(repo *Repository, redisClient *redis.Client) *Service {
	return &Service{repo: repo, redis: redisClient}
}

func (s *Service) RecomputeAll(ctx context.Context) error {
	if err := s.repo.RecomputeAll(ctx); err != nil {
		return err
	}
	s.bumpOverviewVersion(ctx)
	return nil
}

func (s *Service) RecomputeBoard(ctx context.Context, boardID string) error {
	if err := s.repo.RecomputeBoard(ctx, boardID); err != nil {
		return err
	}
	s.bumpOverviewVersion(ctx)
	return nil
}

func (s *Service) GetByBoard(ctx context.Context, boardID string) (*BoardStats, error) {
	return s.repo.GetByBoard(ctx, boardID)
}

func (s *Service) GetOverviewVersion(ctx context.Context) int64 {
	if s.redis == nil {
		return 0
	}
	v, err := s.redis.Get(ctx, overviewVersionKey).Int64()
	if err != nil {
		return 0
	}
	return v
}

func (s *Service) bumpOverviewVersion(ctx context.Context) {
	if s.redis == nil {
		return
	}
	_, _ = s.redis.Incr(ctx, overviewVersionKey).Result()
}
