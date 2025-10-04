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
	"go-little-userbot-maker/internal/orchestrator"
	"go-little-userbot-maker/pkg/logging"
	"go-little-userbot-maker/pkg/storage"
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

	service, err := orchestrator.NewService(cfg.Orchestrator, logger.Named("orchestrator"), db)
	if err != nil {
		logger.Fatal("init orchestrator", zap.Error(err))
	}

	if err := service.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("service exited", zap.Error(err))
		time.Sleep(time.Second)
		os.Exit(1)
	}
}
