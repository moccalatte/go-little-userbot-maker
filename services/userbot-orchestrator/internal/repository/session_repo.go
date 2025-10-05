package repository

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"go-little-userbot-maker/pkg/logger"
	"go-little-userbot-maker/pkg/storage"
	"go-little-userbot-maker/services/userbot-orchestrator/internal/usecase"
)

// SessionRepository handles all database operations related to sessions.
type SessionRepository struct {
	log *logger.Logger
	db  *storage.Database
}

// NewSessionRepository creates a new session repository.
func NewSessionRepository(log *logger.Logger, db *storage.Database) *SessionRepository {
	return &SessionRepository{log: log, db: db}
}

// Bootstrap loads all active sessions from the database.
func (r *SessionRepository) Bootstrap(ctx context.Context) ([]usecase.SessionRecord, error) {
	if r.db == nil {
		r.log.Warn("database not configured, bootstrap skipped")
		return nil, nil
	}

	rows, err := r.db.Pool.Query(ctx, `SELECT telegram_id, session_encrypted, login_method, metadata, session_hash, request_id, origin, updated_at FROM sessions WHERE status = 'active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []usecase.SessionRecord
	for rows.Next() {
		var rec usecase.SessionRecord
		var metadataBytes []byte
		if err := rows.Scan(&rec.TelegramID, &rec.SessionData, &rec.LoginMethod, &metadataBytes, &rec.SessionHash, &rec.RequestID, &rec.Origin, &rec.UpdatedAt); err != nil {
			r.log.Error("failed to scan session record: %v", err)
			continue
		}
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &rec.Metadata); err != nil {
				r.log.Warn("failed to unmarshal session metadata: %v", err)
			}
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// CreateOrUpdate upserts a session record in the database.
func (r *SessionRepository) CreateOrUpdate(ctx context.Context, record usecase.SessionRecord) error {
	if r.db == nil {
		r.log.Warn("database not configured, session not persisted for telegram_id: %d", record.TelegramID)
		return nil
	}

	tx, err := r.db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	metadataJSON, err := json.Marshal(record.Metadata)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO sessions (telegram_id, session_encrypted, login_method, metadata, session_hash, request_id, origin, status, updated_at, last_seen)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'active', NOW(), NOW())
		ON CONFLICT (telegram_id) DO UPDATE SET
			session_encrypted = EXCLUDED.session_encrypted,
			login_method = EXCLUDED.login_method,
			metadata = EXCLUDED.metadata,
			session_hash = EXCLUDED.session_hash,
			request_id = EXCLUDED.request_id,
			origin = EXCLUDED.origin,
			status = 'active',
			updated_at = NOW(),
			last_seen = NOW()`,
		record.TelegramID, record.SessionData, record.LoginMethod, metadataJSON, record.SessionHash, record.RequestID, record.Origin)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// MarkAsDeleted updates a session's status to 'deleted' in the database.
func (r *SessionRepository) MarkAsDeleted(ctx context.Context, telegramID int64) error {
	if r.db == nil {
		r.log.Warn("database not configured, session not marked as deleted for telegram_id: %d", telegramID)
		return nil
	}
	_, err := r.db.Pool.Exec(ctx, `UPDATE sessions SET status='deleted', updated_at=NOW() WHERE telegram_id=$1`, telegramID)
	return err
}

// Ping checks the database connection.
func (r *SessionRepository) Ping(ctx context.Context) error {
	if r.db == nil {
		return nil // No-op if DB is not configured
	}
	return r.db.Pool.Ping(ctx)
}