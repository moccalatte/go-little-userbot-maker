package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"go-little-userbot-maker/internal/config"
	"go-little-userbot-maker/internal/storage"
)

type SessionManager struct {
	cfg     config.OrchestratorConfig
	log     *zap.Logger
	db      *storage.Database
	secret  []byte
	workers sync.Map // map[int64]*Worker
	memory  sync.Map // fallback store when db unavailable
}

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

func NewSessionManager(cfg config.OrchestratorConfig, log *zap.Logger, db *storage.Database) *SessionManager {
	return &SessionManager{
		cfg:    cfg,
		log:    log,
		db:     db,
		secret: deriveKey(cfg.SecretKey),
	}
}

func (m *SessionManager) Bootstrap(ctx context.Context) error {
	if m.db == nil {
		m.log.Warn("database not configured, bootstrap skipped")
		return nil
	}
	rows, err := m.db.Pool.Query(ctx, `SELECT telegram_id, session_encrypted, login_method, metadata, session_hash, request_id, origin, updated_at FROM sessions WHERE status = 'active'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var rec SessionRecord
		var metadataBytes []byte
		if err := rows.Scan(&rec.TelegramID, &rec.SessionData, &rec.LoginMethod, &metadataBytes, &rec.SessionHash, &rec.RequestID, &rec.Origin, &rec.UpdatedAt); err != nil {
			m.log.Error("scan session", zap.Error(err))
			continue
		}
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &rec.Metadata); err != nil {
				m.log.Warn("unmarshal metadata", zap.Error(err))
			}
		}
		worker := NewWorker(rec.TelegramID, rec, m.log, m.cfg.HealthInterval)
		worker.Start()
		m.workers.Store(rec.TelegramID, worker)
	}
	return rows.Err()
}

func (m *SessionManager) CreateSession(ctx context.Context, req SessionRequest) error {
	if req.TelegramID == 0 {
		return errors.New("telegram_id wajib")
	}
	encrypted, err := EncryptSession(req.Session, m.secret)
	if err != nil {
		return fmt.Errorf("encrypt session: %w", err)
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

	if m.db != nil {
		tx, err := m.db.Pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		metadataJSON, err := json.Marshal(record.Metadata)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO sessions (telegram_id, session_encrypted, login_method, metadata, session_hash, request_id, origin, status, updated_at, last_seen)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'active', NOW(), NOW())
ON CONFLICT (telegram_id) DO UPDATE SET session_encrypted = EXCLUDED.session_encrypted, login_method = EXCLUDED.login_method, metadata = EXCLUDED.metadata, session_hash = EXCLUDED.session_hash, request_id = EXCLUDED.request_id, origin = EXCLUDED.origin, status = 'active', updated_at = NOW(), last_seen = NOW()`,
			record.TelegramID, record.SessionData, record.LoginMethod, metadataJSON, record.SessionHash, record.RequestID, record.Origin)
		if err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	} else {
		m.memory.Store(record.TelegramID, record)
	}

	if existing, ok := m.workers.Load(record.TelegramID); ok {
		existing.(*Worker).Stop()
	}
	worker := NewWorker(record.TelegramID, record, m.log, m.cfg.HealthInterval)
	worker.Start()
	m.workers.Store(record.TelegramID, worker)
	return nil
}

func (m *SessionManager) DeleteSession(ctx context.Context, telegramID int64) error {
	if m.db != nil {
		_, err := m.db.Pool.Exec(ctx, `UPDATE sessions SET status='deleted', updated_at=NOW() WHERE telegram_id=$1`, telegramID)
		if err != nil {
			return err
		}
	}
	m.memory.Delete(telegramID)
	if worker, ok := m.workers.Load(telegramID); ok {
		worker.(*Worker).Stop()
		m.workers.Delete(telegramID)
	}
	return nil
}

func (m *SessionManager) UpdateFeature(ctx context.Context, feature string, telegramID int64, payload map[string]any) error {
	m.log.Info("feature update", zap.String("feature", feature), zap.Int64("telegram_id", telegramID), zap.Any("payload", payload))
	if worker, ok := m.workers.Load(telegramID); ok {
		worker.(*Worker).NotifyFeature(feature, payload)
	}
	return nil
}

func (m *SessionManager) Stats() map[string]any {
	count := 0
	m.workers.Range(func(_, _ any) bool {
		count++
		return true
	})
	return map[string]any{
		"active_workers": count,
		"mock_mode":      m.db == nil,
	}
}

func (m *SessionManager) Health(ctx context.Context) error {
	if m.db != nil {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := m.db.Pool.Ping(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (m *SessionManager) Stop() {
	m.workers.Range(func(key, value any) bool {
		value.(*Worker).Stop()
		return true
	})
}
