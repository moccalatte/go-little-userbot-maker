package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go-little-userbot-maker/pkg/config"
	"go-little-userbot-maker/pkg/logger"
)

// SessionRecord is a core domain entity representing a user's session.
type SessionRecord struct {
	TelegramID  int64
	SessionData []byte
	LoginMethod string
	Metadata    map[string]string
	SessionHash string
	RequestID   string
	Origin      string
	UpdatedAt   time.Time
}

// SessionRepository defines the interface for session data persistence.
type SessionRepository interface {
	Bootstrap(ctx context.Context) ([]SessionRecord, error)
	CreateOrUpdate(ctx context.Context, record SessionRecord) error
	MarkAsDeleted(ctx context.Context, telegramID int64) error
	Ping(ctx context.Context) error
}

// CreateSessionInput is the DTO for creating a new session.
type CreateSessionInput struct {
	TelegramID  int64
	Session     string
	LoginMethod string
	Metadata    map[string]string
	SessionHash string
	RequestID   string
	Origin      string
}

// SessionUsecase orchestrates the logic for managing userbot sessions and workers.
type SessionUsecase struct {
	cfg      config.OrchestratorConfig
	log      *logger.Logger
	repo     SessionRepository
	secret   []byte
	workers  sync.Map // map[int64]*Worker
}

// NewSessionUsecase creates a new session usecase.
func NewSessionUsecase(cfg config.OrchestratorConfig, log *logger.Logger, repo SessionRepository, secretKey string) *SessionUsecase {
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
		uc.log.Info("bootstrapping worker for telegram_id: %d", rec.TelegramID)
		workerLogger, err := logger.New("userbot-worker", fmt.Sprintf("%d", rec.TelegramID))
		if err != nil {
			uc.log.Error("failed to create logger for worker %d: %v", rec.TelegramID, err)
			continue
		}
		worker := NewWorker(rec.TelegramID, rec, workerLogger, uc.cfg.HealthInterval)
		worker.Start()
		uc.workers.Store(rec.TelegramID, worker)
	}
	return nil
}

// CreateSession encrypts, persists, and starts a new userbot session.
func (uc *SessionUsecase) CreateSession(ctx context.Context, req CreateSessionInput) error {
	if req.TelegramID == 0 {
		return errors.New("telegram_id is required")
	}
	encrypted, err := EncryptSession(req.Session, uc.secret)
	if err != nil {
		return fmt.Errorf("failed to encrypt session: %w", err)
	}

	record := SessionRecord{
		TelegramID:  req.TelegramID,
		SessionData: encrypted,
		LoginMethod: req.LoginMethod,
		Metadata:    req.Metadata,
		SessionHash: req.SessionHash,
		RequestID:   req.RequestID,
		Origin:      req.Origin,
		UpdatedAt:   time.Now().UTC(),
	}

	if err := uc.repo.CreateOrUpdate(ctx, record); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	// Stop existing worker if any
	if existing, ok := uc.workers.Load(record.TelegramID); ok {
		existing.(*Worker).Stop()
		uc.log.Info("stopping existing worker for telegram_id: %d", record.TelegramID)
	}

	// Start new worker
	workerLogger, err := logger.New("userbot-worker", fmt.Sprintf("%d", record.TelegramID))
	if err != nil {
		uc.log.Error("failed to create logger for new worker %d: %v", record.TelegramID, err)
		// Continue without a dedicated logger for the worker, it will use the main one.
	}
	worker := NewWorker(record.TelegramID, record, workerLogger, uc.cfg.HealthInterval)
	worker.Start()
	uc.workers.Store(record.TelegramID, worker)
	uc.log.Info("started new worker for telegram_id: %d", record.TelegramID)

	return nil
}

// DeleteSession stops a worker and marks the session as deleted.
func (uc *SessionUsecase) DeleteSession(ctx context.Context, telegramID int64) error {
	if err := uc.repo.MarkAsDeleted(ctx, telegramID); err != nil {
		return fmt.Errorf("failed to mark session as deleted: %w", err)
	}

	if worker, ok := uc.workers.Load(telegramID); ok {
		worker.(*Worker).Stop()
		uc.workers.Delete(telegramID)
		uc.log.Info("stopped and deleted worker for telegram_id: %d", telegramID)
	}
	return nil
}

// UpdateFeature notifies a running worker about a feature change.
func (uc *SessionUsecase) UpdateFeature(ctx context.Context, feature string, telegramID int64, payload map[string]any) error {
	uc.log.Info("feature update requested for feature '%s' on telegram_id: %d", feature, telegramID)
	if worker, ok := uc.workers.Load(telegramID); ok {
		worker.(*Worker).NotifyFeature(feature, payload)
		uc.log.Info("notified worker of feature update for telegram_id: %d", telegramID)
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
		uc.log.Info("stopping worker on shutdown for telegram_id: %v", key)
		return true
	})
}