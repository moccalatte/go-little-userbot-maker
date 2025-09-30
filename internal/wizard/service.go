package wizard

import (
	"context"
	"errors"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"go-little-userbot-maker/internal/config"
)

type Bot interface {
	GetUpdatesChan(tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
}

type Service struct {
	cfg          config.WizardConfig
	log          *zap.Logger
	bot          Bot
	state        StateStore
	orchestrator *OrchestratorClient
	transcripts  *TranscriptWriter
}

func NewService(cfg config.WizardConfig, log *zap.Logger, state StateStore, orchestrator *OrchestratorClient) (*Service, error) {
	if log == nil {
		return nil, errors.New("logger is nil")
	}
	if state == nil {
		return nil, errors.New("state store is nil")
	}
	if orchestrator == nil {
		return nil, errors.New("orchestrator client is nil")
	}

	var bot Bot
	if cfg.UseMock {
		bot = NewMockBot(log)
	} else {
		if cfg.BotToken == "" {
			return nil, errors.New("bot token missing")
		}
		api, err := tgbotapi.NewBotAPI(cfg.BotToken)
		if err != nil {
			return nil, err
		}
		api.Debug = true
		bot = api
	}

	transcripts := NewTranscriptWriter(cfg.StoragePath, log)

	return &Service{
		cfg:          cfg,
		log:          log,
		bot:          bot,
		state:        state,
		orchestrator: orchestrator,
		transcripts:  transcripts,
	}, nil
}

func (s *Service) Run(ctx context.Context) error {
	s.log.Info("wizard service starting", zap.String("listen", s.cfg.ListenAddr))
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
				time.Sleep(time.Second)
				continue
			}
			if err := s.handleUpdate(ctx, update); err != nil {
				s.log.Error("handle update", zap.Error(err))
			}
		}
	}
}
