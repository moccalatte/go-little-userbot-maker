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
	"go-little-userbot-maker/internal/wizard"
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

	stateStore := wizard.NewMemoryStateStore(cfg.Wizard.StateTTL)
	orchestratorClient := wizard.NewOrchestratorClient(cfg.Wizard.OrchestratorURL, logger.Named("orchestrator_client"))

	service, err := wizard.NewService(cfg.Wizard, logger.Named("wizard"), stateStore, orchestratorClient)
	if err != nil {
		logger.Fatal("init service", zap.Error(err))
	}

	if err := service.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("service exited", zap.Error(err))
		time.Sleep(time.Second)
		os.Exit(1)
	}
}
