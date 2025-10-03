package storage

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"go-little-userbot-maker/internal/config"
)

type Redis struct {
	Client *redis.Client
}

func NewRedis(cfg config.RedisConfig) *Redis {
	if !cfg.Enabled {
		return nil
	}
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Username: cfg.Username,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &Redis{Client: client}
}

func (r *Redis) Close() {
	if r == nil || r.Client == nil {
		return
	}
	_ = r.Client.Close()
}

func (r *Redis) Ping(ctx context.Context) error {
	if r == nil || r.Client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return r.Client.Ping(ctx).Err()
}
