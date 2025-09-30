package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"go-little-userbot-maker/internal/config"
)

type Database struct {
	Pool *pgxpool.Pool
}

func NewDatabase(ctx context.Context, cfg config.DatabaseConfig) (*Database, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("database url is empty")
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	poolConfig.MaxConns = int32(cfg.MaxConns)
	poolConfig.MinConns = int32(cfg.MaxIdle)
	poolConfig.MaxConnLifetime = cfg.MaxLife
	poolConfig.MaxConnIdleTime = time.Minute * 5

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Database{Pool: pool}, nil
}

func (d *Database) Close() {
	if d == nil || d.Pool == nil {
		return
	}
	d.Pool.Close()
}
