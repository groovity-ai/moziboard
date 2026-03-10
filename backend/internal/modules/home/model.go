package home

import (
	"time"

	activityfeedmodule "moziboard-backend/internal/modules/activityfeed"
	attentionmodule "moziboard-backend/internal/modules/attention"
	boardhealthmodule "moziboard-backend/internal/modules/boardhealth"
)

type CacheMeta struct {
	Version   int64      `json:"version"`
	CachedAt  *time.Time `json:"cached_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	FromCache bool       `json:"from_cache"`
}

type Overview struct {
	Summary        boardhealthmodule.Summary       `json:"summary"`
	Boards         []boardhealthmodule.BoardStatus `json:"boards"`
	Attention      []attentionmodule.Item          `json:"attention"`
	RecentActivity []activityfeedmodule.Item       `json:"recent_activity"`
	Cache          CacheMeta                       `json:"cache"`
}

type Summary = boardhealthmodule.Summary

type BoardOverview = boardhealthmodule.BoardStatus

type Metrics = boardhealthmodule.Metrics

type AttentionItem = attentionmodule.Item

type ActivityItem = activityfeedmodule.Item
