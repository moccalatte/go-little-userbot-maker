package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"go-little-userbot-maker/internal/config"
	"go-little-userbot-maker/internal/logging"
	"go-little-userbot-maker/internal/orchestrator"
	"go-little-userbot-maker/internal/storage"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger, err := logging.New(cfg.Logging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck

	var db *storage.Database
	if !cfg.Orchestrator.EnableMock {
		db, err = storage.NewDatabase(ctx, cfg.Database)
		if err != nil {
			logger.Fatal("connect database", zap.Error(err))
		}
		defer db.Close()
	} else {
		logger.Warn("running orchestrator in mock mode (no database)")
	}

	redisClient := storage.NewRedis(cfg.Redis)
	if redisClient != nil {
		if err := redisClient.Ping(ctx); err != nil {
			logger.Warn("redis ping failed", zap.Error(err))
		}
		defer redisClient.Close()
	}

	service, err := orchestrator.NewService(cfg.Orchestrator, logger.Named("orchestrator"), db, redisClient)
	if err != nil {
		logger.Fatal("init orchestrator", zap.Error(err))
	}

	if err := service.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("service exited", zap.Error(err))
		time.Sleep(time.Second)
		os.Exit(1)
	}
}
