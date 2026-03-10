package cache

import (
	"github.com/redis/go-redis/v9"
	"moziboard-backend/internal/platform/config"
)

func NewRedis(cfg config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})
}
