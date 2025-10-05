package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-little-userbot-maker/pkg/config"
	"go-little-userbot-maker/pkg/logger"
	"go-little-userbot-maker/pkg/storage"
	orchestrator "go-little-userbot-maker/services/userbot-orchestrator/internal"
)

func main() {
	// Initialize logger first to capture all subsequent output.
	mainLogger, err := logger.New("userbot-orchestrator", "main")
	if err != nil {
		// If logger fails, print to original stderr and exit.
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}

	// Redirect stdout and stderr to the logger.
	if err := mainLogger.SetOutput(); err != nil {
		mainLogger.Error("failed to redirect output: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		mainLogger.Error("load config: %v", err)
		os.Exit(1)
	}

	var db *storage.Database
	if !cfg.Orchestrator.EnableMock {
		db, err = storage.NewDatabase(ctx, cfg.Database)
		if err != nil {
			mainLogger.Error("connect database: %v", err)
			os.Exit(1)
		}
		defer db.Close()
	} else {
		mainLogger.Warn("running orchestrator in mock mode (no database)")
	}

	service, err := orchestrator.NewService(cfg.Orchestrator, mainLogger, db)
	if err != nil {
		mainLogger.Error("init orchestrator: %v", err)
		os.Exit(1)
	}

	mainLogger.Info("starting userbot-orchestrator service")

	if err := service.Run(ctx); err != nil && err != context.Canceled {
		mainLogger.Error("service exited: %v", err)
		time.Sleep(time.Second)
		os.Exit(1)
	}

	mainLogger.Info("userbot-orchestrator service stopped gracefully")
}