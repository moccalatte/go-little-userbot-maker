package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"go-little-userbot-maker/internal/orchestrator/usecase"
	"go-little-userbot-maker/pkg/storage"
)

// SessionRepository handles all database operations related to sessions.
type SessionRepository struct {
	log *zap.Logger
	db  *storage.Database
}

// NewSessionRepository creates a new session repository.
func NewSessionRepository(log *zap.Logger, db *storage.Database) *SessionRepository {
	return &SessionRepository{log: log, db: db}
}

// Bootstrap loads all active sessions from the database.
func (r *SessionRepository) Bootstrap(ctx context.Context) ([]usecase.SessionRecord, error) {
	if r.db == nil {
		r.log.Warn("database not configured, bootstrap skipped")
		return nil, nil
	}

	rows, err := r.db.Pool.Query(ctx, `
        SELECT id,
               user_id,
               owner_user_id,
               telegram_id,
               session_encrypted,
               login_method,
               metadata,
               session_hash,
               request_id,
               origin,
               session_type,
               bot_username,
               bot_display_name,
               features,
               config,
               updated_at
        FROM sessions
        WHERE status = 'active'
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []usecase.SessionRecord
	for rows.Next() {
		var rec usecase.SessionRecord
		var metadataBytes, featuresBytes, configBytes []byte
		var userID, ownerID sql.NullInt64
		if err := rows.Scan(
			&rec.ID,
			&userID,
			&ownerID,
			&rec.TelegramID,
			&rec.SessionData,
			&rec.LoginMethod,
			&metadataBytes,
			&rec.SessionHash,
			&rec.RequestID,
			&rec.Origin,
			&rec.SessionType,
			&rec.BotUsername,
			&rec.BotDisplayName,
			&featuresBytes,
			&configBytes,
			&rec.UpdatedAt,
		); err != nil {
			r.log.Error("failed to scan session record", zap.Error(err))
			continue
		}
		if userID.Valid {
			id := userID.Int64
			rec.UserID = &id
		}
		if ownerID.Valid {
			id := ownerID.Int64
			rec.OwnerUserID = &id
		}
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &rec.Metadata); err != nil {
				r.log.Warn("failed to unmarshal session metadata", zap.Error(err))
			}
		}
		if len(featuresBytes) > 0 {
			if err := json.Unmarshal(featuresBytes, &rec.Features); err != nil {
				r.log.Warn("failed to unmarshal session features", zap.Error(err))
			}
		}
		if len(configBytes) > 0 {
			if err := json.Unmarshal(configBytes, &rec.Config); err != nil {
				r.log.Warn("failed to unmarshal session config", zap.Error(err))
			}
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// CreateOrUpdate upserts a session record in the database.
func (r *SessionRepository) CreateOrUpdate(ctx context.Context, record usecase.SessionRecord) (usecase.CreateSessionResult, error) {
	if r.db == nil {
		r.log.Warn("database not configured, session not persisted", zap.Int64("telegram_id", record.TelegramID))
		return usecase.CreateSessionResult{}, nil
	}

	tx, err := r.db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return usecase.CreateSessionResult{}, err
	}
	defer tx.Rollback(ctx)

	metadataJSON, err := json.Marshal(record.Metadata)
	if err != nil {
		return usecase.CreateSessionResult{}, err
	}
	featuresJSON, err := json.Marshal(record.Features)
	if err != nil {
		return usecase.CreateSessionResult{}, err
	}
	configJSON, err := json.Marshal(record.Config)
	if err != nil {
		return usecase.CreateSessionResult{}, err
	}

	var ownerUserID sql.NullInt64
	if record.OwnerUserID != nil {
		ownerUserID = sql.NullInt64{Int64: *record.OwnerUserID, Valid: true}
	}

	if !ownerUserID.Valid && record.OwnerTelegramID != 0 {
		var ownerID int64
		if err := tx.QueryRow(ctx, `
            INSERT INTO users (telegram_id, username, full_name)
            VALUES ($1, NULLIF($2, ''), NULLIF($3, ''))
            ON CONFLICT (telegram_id) DO UPDATE SET
                username = COALESCE(NULLIF(EXCLUDED.username, ''), users.username),
                full_name = COALESCE(NULLIF(EXCLUDED.full_name, ''), users.full_name),
                updated_at = NOW()
            RETURNING id
        `, record.OwnerTelegramID, record.OwnerUsername, record.OwnerFullName).Scan(&ownerID); err != nil {
			return usecase.CreateSessionResult{}, err
		}
		ownerUserID = sql.NullInt64{Int64: ownerID, Valid: true}
	}

	userIDParam := any(nil)
	if record.UserID != nil {
		userIDParam = *record.UserID
	}
	ownerIDParam := any(nil)
	if ownerUserID.Valid {
		ownerIDParam = ownerUserID.Int64
		if userIDParam == nil {
			userIDParam = ownerUserID.Int64
		}
	}

	botUsernameParam := nullableString(record.BotUsername)
	botDisplayNameParam := nullableString(record.BotDisplayName)

	var sessionID int64
	var persistedOwner sql.NullInt64
	if err := tx.QueryRow(ctx, `
        INSERT INTO sessions (
            telegram_id,
            session_encrypted,
            login_method,
            metadata,
            session_hash,
            request_id,
            origin,
            status,
            updated_at,
            last_seen,
            user_id,
            owner_user_id,
            session_type,
            bot_username,
            bot_display_name,
            features,
            config
        ) VALUES (
            $1, $2, NULLIF($3, ''), $4, $5, $6, NULLIF($7, ''), 'active', NOW(), NOW(), $8, $9, $10, $11, $12, $13, $14
        )
        ON CONFLICT (telegram_id) DO UPDATE SET
            session_encrypted = EXCLUDED.session_encrypted,
            login_method = EXCLUDED.login_method,
            metadata = EXCLUDED.metadata,
            session_hash = EXCLUDED.session_hash,
            request_id = EXCLUDED.request_id,
            origin = EXCLUDED.origin,
            status = 'active',
            updated_at = NOW(),
            last_seen = NOW(),
            owner_user_id = COALESCE(EXCLUDED.owner_user_id, sessions.owner_user_id),
            user_id = COALESCE(EXCLUDED.user_id, sessions.user_id),
            session_type = EXCLUDED.session_type,
            bot_username = EXCLUDED.bot_username,
            bot_display_name = EXCLUDED.bot_display_name,
            features = EXCLUDED.features,
            config = EXCLUDED.config
        RETURNING id, owner_user_id
    `, record.TelegramID, record.SessionData, record.LoginMethod, metadataJSON, record.SessionHash, record.RequestID, record.Origin, userIDParam, ownerIDParam, record.SessionType, botUsernameParam, botDisplayNameParam, featuresJSON, configJSON).Scan(&sessionID, &persistedOwner); err != nil {
		return usecase.CreateSessionResult{}, err
	}

	diffJSON, _ := json.Marshal(map[string]any{
		"session_type":     record.SessionType,
		"bot_username":     record.BotUsername,
		"bot_display_name": record.BotDisplayName,
		"login_method":     record.LoginMethod,
	})
	metadataAudit, _ := json.Marshal(map[string]any{
		"request_id": record.RequestID,
		"origin":     record.Origin,
	})
	actorID := any(nil)
	if persistedOwner.Valid {
		actorID = persistedOwner.Int64
	}
	if _, err := tx.Exec(ctx, `
        INSERT INTO audits (actor_id, target_type, target_id, action, diff, metadata)
        VALUES ($1, 'session', $2, 'upsert', $3, $4)
    `, actorID, sessionID, diffJSON, metadataAudit); err != nil {
		r.log.Warn("failed to append audit log", zap.Error(err))
	}

	if err := tx.Commit(ctx); err != nil {
		return usecase.CreateSessionResult{}, err
	}

	result := usecase.CreateSessionResult{SessionID: sessionID}
	if persistedOwner.Valid {
		result.OwnerUserID = persistedOwner.Int64
	}
	return result, nil
}

// MarkAsDeleted updates a session's status to 'deleted' in the database.
func (r *SessionRepository) MarkAsDeleted(ctx context.Context, telegramID int64) error {
	if r.db == nil {
		r.log.Warn("database not configured, session not marked as deleted", zap.Int64("telegram_id", telegramID))
		return nil
	}

	tx, err := r.db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var sessionID int64
	var ownerID sql.NullInt64
	if err := tx.QueryRow(ctx, `
        UPDATE sessions
        SET status = 'deleted', updated_at = NOW()
        WHERE telegram_id = $1
        RETURNING id, owner_user_id
    `, telegramID).Scan(&sessionID, &ownerID); err != nil {
		return err
	}

	diffJSON, _ := json.Marshal(map[string]any{"status": "deleted"})
	metadataJSON, _ := json.Marshal(map[string]any{"telegram_id": telegramID})
	actorID := any(nil)
	if ownerID.Valid {
		actorID = ownerID.Int64
	}
	if _, err := tx.Exec(ctx, `
        INSERT INTO audits (actor_id, target_type, target_id, action, diff, metadata)
        VALUES ($1, 'session', $2, 'deleted', $3, $4)
    `, actorID, sessionID, diffJSON, metadataJSON); err != nil {
		r.log.Warn("failed to append audit log", zap.Error(err))
	}

	return tx.Commit(ctx)
}

// Ping checks the database connection.
func (r *SessionRepository) Ping(ctx context.Context) error {
	if r.db == nil {
		return nil // No-op if DB is not configured
	}
	return r.db.Pool.Ping(ctx)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
