package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"go-little-userbot-maker/internal/config"
)

// SessionRecord is a core domain entity representing a user's session.
type SessionRecord struct {
	ID              int64
	UserID          *int64
	OwnerUserID     *int64
	OwnerTelegramID int64
	OwnerUsername   string
	OwnerFullName   string
	TelegramID      int64
	SessionType     string
	SessionData     []byte
	LoginMethod     string
	Metadata        map[string]string
	Features        map[string]any
	Config          map[string]any
	SessionHash     string
	RequestID       string
	Origin          string
	BotUsername     string
	BotDisplayName  string
	UpdatedAt       time.Time
}

// SessionRepository defines the interface for session data persistence.
type SessionRepository interface {
	Bootstrap(ctx context.Context) ([]SessionRecord, error)
	CreateOrUpdate(ctx context.Context, record SessionRecord) (CreateSessionResult, error)
	MarkAsDeleted(ctx context.Context, telegramID int64) error
	Ping(ctx context.Context) error
}

// CreateSessionInput is the DTO for creating a new session.
type CreateSessionInput struct {
	TelegramID      int64
	Session         string
	LoginMethod     string
	Metadata        map[string]string
	SessionHash     string
	RequestID       string
	Origin          string
	OwnerTelegramID int64
	OwnerUsername   string
	OwnerFullName   string
	SessionType     string
	BotUsername     string
	BotDisplayName  string
	Features        map[string]any
	Config          map[string]any
}

// CreateSessionResult describes the outcome of a session creation/upsert.
type CreateSessionResult struct {
	SessionID   int64
	OwnerUserID int64
}

// SessionUsecase orchestrates the logic for managing userbot sessions and workers.
type SessionUsecase struct {
	cfg     config.OrchestratorConfig
	log     *zap.Logger
	repo    SessionRepository
	secret  []byte
	workers sync.Map // map[int64]*Worker
}

// NewSessionUsecase creates a new session usecase.
func NewSessionUsecase(cfg config.OrchestratorConfig, log *zap.Logger, repo SessionRepository, secretKey string) *SessionUsecase {
	return &SessionUsecase{
		cfg:    cfg,
		log:    log,
		repo:   repo,
		secret: deriveKey(secretKey),
	}
}

// Bootstrap initializes all active userbot workers from the repository.
func (uc *SessionUsecase) Bootstrap(ctx context.Context) error {
	records, err := uc.repo.Bootstrap(ctx)
	if err != nil {
		return fmt.Errorf("failed to bootstrap sessions: %w", err)
	}

	for _, rec := range records {
		uc.log.Info("bootstrapping worker",
			zap.Int64("telegram_id", rec.TelegramID),
			zap.String("session_type", rec.SessionType),
			zap.String("bot_username", rec.BotUsername))
		worker := NewWorker(rec.TelegramID, rec, uc.log, uc.cfg.HealthInterval)
		worker.Start()
		uc.workers.Store(rec.TelegramID, worker)
	}
	return nil
}

// CreateSession encrypts, persists, and starts a new userbot session.
func (uc *SessionUsecase) CreateSession(ctx context.Context, req CreateSessionInput) (CreateSessionResult, error) {
	if req.TelegramID == 0 {
		return CreateSessionResult{}, errors.New("telegram_id is required")
	}
	encrypted, err := EncryptSession(req.Session, uc.secret)
	if err != nil {
		return CreateSessionResult{}, fmt.Errorf("failed to encrypt session: %w", err)
	}

	if req.Metadata == nil {
		req.Metadata = map[string]string{}
	}
	if req.Features == nil {
		req.Features = map[string]any{}
	}
	if req.Config == nil {
		req.Config = map[string]any{}
	}
	if req.SessionType == "" {
		req.SessionType = "userbot"
	}

	record := SessionRecord{
		TelegramID:      req.TelegramID,
		SessionData:     encrypted,
		LoginMethod:     req.LoginMethod,
		Metadata:        req.Metadata,
		Features:        req.Features,
		Config:          req.Config,
		SessionHash:     req.SessionHash,
		RequestID:       req.RequestID,
		Origin:          req.Origin,
		OwnerTelegramID: req.OwnerTelegramID,
		OwnerUsername:   req.OwnerUsername,
		OwnerFullName:   req.OwnerFullName,
		SessionType:     req.SessionType,
		BotUsername:     req.BotUsername,
		BotDisplayName:  req.BotDisplayName,
		UpdatedAt:       time.Now().UTC(),
	}

	result, err := uc.repo.CreateOrUpdate(ctx, record)
	if err != nil {
		return CreateSessionResult{}, fmt.Errorf("failed to save session: %w", err)
	}

	// Stop existing worker if any
	if existing, ok := uc.workers.Load(record.TelegramID); ok {
		existing.(*Worker).Stop()
		uc.log.Info("stopping existing worker", zap.Int64("telegram_id", record.TelegramID))
	}

	// Start new worker
	worker := NewWorker(record.TelegramID, record, uc.log, uc.cfg.HealthInterval)
	worker.Start()
	uc.workers.Store(record.TelegramID, worker)
	uc.log.Info("started new worker",
		zap.Int64("telegram_id", record.TelegramID),
		zap.String("session_type", record.SessionType),
		zap.String("bot_username", record.BotUsername))

	return result, nil
}

// DeleteSession stops a worker and marks the session as deleted.
func (uc *SessionUsecase) DeleteSession(ctx context.Context, telegramID int64) error {
	if err := uc.repo.MarkAsDeleted(ctx, telegramID); err != nil {
		return fmt.Errorf("failed to mark session as deleted: %w", err)
	}

	if worker, ok := uc.workers.Load(telegramID); ok {
		worker.(*Worker).Stop()
		uc.workers.Delete(telegramID)
		uc.log.Info("stopped and deleted worker", zap.Int64("telegram_id", telegramID))
	}
	return nil
}

// UpdateFeature notifies a running worker about a feature change.
func (uc *SessionUsecase) UpdateFeature(ctx context.Context, feature string, telegramID int64, payload map[string]any) error {
	uc.log.Info("feature update requested", zap.String("feature", feature), zap.Int64("telegram_id", telegramID))
	if worker, ok := uc.workers.Load(telegramID); ok {
		worker.(*Worker).NotifyFeature(feature, payload)
		uc.log.Info("notified worker of feature update", zap.Int64("telegram_id", telegramID))
	}
	return nil
}

// GetStats returns statistics about active workers.
func (uc *SessionUsecase) GetStats() map[string]any {
	count := 0
	uc.workers.Range(func(_, _ any) bool {
		count++
		return true
	})
	return map[string]any{
		"active_workers": count,
	}
}

// HealthCheck pings the underlying repository.
func (uc *SessionUsecase) HealthCheck(ctx context.Context) error {
	return uc.repo.Ping(ctx)
}

// StopAllWorkers gracefully shuts down all active workers.
func (uc *SessionUsecase) StopAllWorkers() {
	uc.workers.Range(func(key, value any) bool {
		value.(*Worker).Stop()
		uc.log.Info("stopping worker on shutdown", zap.Any("telegram_id", key))
		return true
	})
}
