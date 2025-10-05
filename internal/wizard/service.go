package wizard

import (
	"context"
	"errors"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"go-little-userbot-maker/internal/config"
	"go-little-userbot-maker/internal/wizard/delivery/telegram"
	"go-little-userbot-maker/internal/wizard/repository"
	"go-little-userbot-maker/internal/wizard/usecase"
	"go-little-userbot-maker/pkg/storage"
)

// Bot defines the interface for a Telegram bot, allowing for mock implementations.
type Bot interface {
	GetUpdatesChan(tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
}

// Service encapsulates the bot's runtime, holding the bot instance and the Telegram handler.
type Service struct {
	log     *zap.Logger
	bot     Bot
	handler *telegram.Handler
}

// NewService creates and wires all components for the wizard service.
func NewService(cfg config.WizardConfig, log *zap.Logger, db *storage.Database) (*Service, error) {
	if log == nil {
		return nil, errors.New("logger is nil")
	}

	// 1. Initialize Bot API (either mock or real)
	var bot Bot
	if cfg.UseMock {
		bot = repository.NewMockBot(log)
	} else {
		if cfg.BotToken == "" {
			return nil, errors.New("bot token is missing")
		}
		api, err := tgbotapi.NewBotAPI(cfg.BotToken)
		if err != nil {
			return nil, err
		}
		api.Debug = cfg.Debug
		bot = api
	}

	// 2. Initialize Repositories
	stateRepo := repository.NewMemoryStateStore(cfg.StateTTL)
	orchRepo := repository.NewOrchestratorClient(cfg.OrchestratorURL, log.Named("orchestrator_client"))
	persistRepo := repository.NewWizardPersistence(log.Named("wizard_persistence"), db)

	// 3. Initialize Usecase
	wizardUsecase := usecase.NewWizardUsecase(
		log.Named("wizard_usecase"),
		stateRepo,
		orchRepo,
		bot,
		cfg.AdminIDs,
		cfg.OrchestratorURL,
		persistRepo,
	)

	// 4. Initialize Delivery Handler
	tgHandler := telegram.NewHandler(wizardUsecase, log)

	return &Service{
		log:     log,
		bot:     bot,
		handler: tgHandler,
	}, nil
}

// Run starts the service's main loop to listen for and process updates.
func (s *Service) Run(ctx context.Context) error {
	s.log.Info("wizard service starting")
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := s.bot.GetUpdatesChan(updateConfig)

	for {
		select {
		case <-ctx.Done():
			s.log.Info("wizard service shutting down")
			return ctx.Err()
		case update, ok := <-updates:
			if !ok {
				s.log.Warn("updates channel closed, attempting to reconnect")
				time.Sleep(time.Second)
				updates = s.bot.GetUpdatesChan(updateConfig)
				continue
			}
			s.handler.HandleUpdate(ctx, update)
		}
	}
}
