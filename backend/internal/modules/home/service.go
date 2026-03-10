package home

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	activityfeedmodule "moziboard-backend/internal/modules/activityfeed"
	attentionmodule "moziboard-backend/internal/modules/attention"
	boardhealthmodule "moziboard-backend/internal/modules/boardhealth"
	boardstatsmodule "moziboard-backend/internal/modules/boardstats"
)

const overviewCacheTTL = 15 * time.Second

type Service struct {
	activityfeed *activityfeedmodule.Service
	attention    *attentionmodule.Service
	boardhealth  *boardhealthmodule.Service
	boardstats   *boardstatsmodule.Service
	redis        *redis.Client
}

func NewService(activityfeed *activityfeedmodule.Service, attention *attentionmodule.Service, boardhealth *boardhealthmodule.Service, boardstats *boardstatsmodule.Service, redisClient *redis.Client) *Service {
	return &Service{activityfeed: activityfeed, attention: attention, boardhealth: boardhealth, boardstats: boardstats, redis: redisClient}
}

func (s *Service) GetOverview(ctx context.Context, userID string) (*Overview, error) {
	version := int64(0)
	if s.boardstats != nil {
		version = s.boardstats.GetOverviewVersion(ctx)
	}
	cacheKey := fmt.Sprintf("home:overview:%s:v%d", userID, version)
	if cached := s.getCachedOverview(ctx, cacheKey); cached != nil {
		cached.Cache.FromCache = true
		return cached, nil
	}

	boards, summary, err := s.boardhealth.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	attention, err := s.attention.ListByUser(ctx, userID, 8)
	if err != nil {
		return nil, err
	}
	recent, err := s.activityfeed.ListRecentByUser(ctx, userID, 12)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	expiresAt := now.Add(overviewCacheTTL)
	overview := &Overview{
		Summary:        summary,
		Boards:         boards,
		Attention:      attention,
		RecentActivity: recent,
		Cache: CacheMeta{
			Version:   version,
			CachedAt:  &now,
			ExpiresAt: &expiresAt,
			FromCache: false,
		},
	}
	if overview.Boards == nil {
		overview.Boards = []boardhealthmodule.BoardStatus{}
	}
	if overview.Attention == nil {
		overview.Attention = []attentionmodule.Item{}
	}
	if overview.RecentActivity == nil {
		overview.RecentActivity = []activityfeedmodule.Item{}
	}
	s.setCachedOverview(ctx, cacheKey, overview)
	return overview, nil
}

func (s *Service) getCachedOverview(ctx context.Context, cacheKey string) *Overview {
	if s.redis == nil {
		return nil
	}
	payload, err := s.redis.Get(ctx, cacheKey).Result()
	if err != nil || payload == "" {
		return nil
	}
	var overview Overview
	if err := json.Unmarshal([]byte(payload), &overview); err != nil {
		return nil
	}
	return &overview
}

func (s *Service) setCachedOverview(ctx context.Context, cacheKey string, overview *Overview) {
	if s.redis == nil || overview == nil {
		return
	}
	payload, err := json.Marshal(overview)
	if err != nil {
		return
	}
	_ = s.redis.Set(ctx, cacheKey, payload, overviewCacheTTL).Err()
}
