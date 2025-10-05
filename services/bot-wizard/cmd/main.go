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
	wizard "go-little-userbot-maker/services/bot-wizard/internal"
)

func main() {
	// Initialize logger first to capture all subsequent output.
	mainLogger, err := logger.New("bot-wizard", "main")
	if err != nil {
		// If logger fails, print to original stderr and exit.
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}

	// Redirect stdout and stderr to the logger.
	if err := mainLogger.SetOutput(); err != nil {
		// Log the error using the logger itself.
		mainLogger.Error("failed to redirect output: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		mainLogger.Error("load config: %v", err)
		os.Exit(1)
	}

	service, err := wizard.NewService(cfg.Wizard, mainLogger)
	if err != nil {
		mainLogger.Error("init service: %v", err)
		os.Exit(1)
	}

	mainLogger.Info("starting bot-wizard service")

	if err := service.Run(ctx); err != nil && err != context.Canceled {
		mainLogger.Error("service exited: %v", err)
		time.Sleep(time.Second)
		os.Exit(1)
	}

	mainLogger.Info("bot-wizard service stopped gracefully")
}