package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"go-little-userbot-maker/pkg/storage"
)

// WizardPersistence defines storage operations required to persist wizard flows and command presets.
type WizardPersistence interface {
	StartRun(ctx context.Context, input WizardRunStart) (int64, error)
	SaveStep(ctx context.Context, step WizardStepRecord) error
	UpdateRunStatus(ctx context.Context, runID int64, status string, sessionID *int64, metadata map[string]any) error
	EnsureDefaultCommands(ctx context.Context, sessionID int64, commands []BotCommandRecord) error
}

// WizardRunStart represents the data captured when a wizard run begins.
type WizardRunStart struct {
	TelegramID  int64
	Username    string
	FullName    string
	Flow        string
	SessionType string
	Metadata    map[string]string
}

// WizardStepRecord describes a persisted wizard step snapshot.
type WizardStepRecord struct {
	RunID    int64
	StepName string
	Sequence int
	State    map[string]string
}

// BotCommandRecord represents a command preset created after wizard completion.
type BotCommandRecord struct {
	Command      string
	Description  string
	ResponseType string
	Payload      map[string]any
	Triggers     []BotCommandTriggerRecord
}

// BotCommandTriggerRecord describes a trigger for a command preset.
type BotCommandTriggerRecord struct {
	Type     string
	Value    string
	Metadata map[string]any
}

type sqlWizardPersistence struct {
	log *zap.Logger
	db  *storage.Database
}

type noopWizardPersistence struct {
	log *zap.Logger
}

// NewWizardPersistence builds the appropriate persistence implementation based on DB availability.
func NewWizardPersistence(log *zap.Logger, db *storage.Database) WizardPersistence {
	if db == nil {
		return &noopWizardPersistence{log: log}
	}
	return &sqlWizardPersistence{log: log, db: db}
}

// StartRun stores a new wizard run record and ensures the initiating user exists.
func (p *sqlWizardPersistence) StartRun(ctx context.Context, input WizardRunStart) (int64, error) {
	tx, err := p.db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var userID sql.NullInt64
	if input.TelegramID != 0 {
		var id int64
		if err := tx.QueryRow(ctx, `
            INSERT INTO users (telegram_id, username, full_name)
            VALUES ($1, NULLIF($2, ''), NULLIF($3, ''))
            ON CONFLICT (telegram_id) DO UPDATE SET
                username = COALESCE(NULLIF(EXCLUDED.username, ''), users.username),
                full_name = COALESCE(NULLIF(EXCLUDED.full_name, ''), users.full_name),
                updated_at = NOW()
            RETURNING id
        `, input.TelegramID, input.Username, input.FullName).Scan(&id); err != nil {
			return 0, err
		}
		userID = sql.NullInt64{Int64: id, Valid: true}
	}

	payload := map[string]any{
		"flow":         input.Flow,
		"session_type": input.SessionType,
		"metadata":     input.Metadata,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	var runID int64
	if err := tx.QueryRow(ctx, `
        INSERT INTO wizard_runs (user_id, status, current_step, payload)
        VALUES ($1, 'in_progress', $2, $3)
        RETURNING id
    `, nullableInt(userID), input.Flow, payloadJSON).Scan(&runID); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return runID, nil
}

// SaveStep appends a step snapshot and updates the run's current step marker.
func (p *sqlWizardPersistence) SaveStep(ctx context.Context, step WizardStepRecord) error {
	tx, err := p.db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	stateJSON, err := json.Marshal(step.State)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
        INSERT INTO wizard_steps (wizard_run_id, step_name, sequence, state)
        VALUES ($1, $2, $3, $4)
    `, step.RunID, step.StepName, step.Sequence, stateJSON); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
        UPDATE wizard_runs
        SET current_step = $1, updated_at = NOW()
        WHERE id = $2
    `, step.StepName, step.RunID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// UpdateRunStatus updates the run status, links the created session, and records additional metadata.
func (p *sqlWizardPersistence) UpdateRunStatus(ctx context.Context, runID int64, status string, sessionID *int64, metadata map[string]any) error {
	if runID == 0 {
		return nil
	}
	tx, err := p.db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	payloadJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	var sessionParam any
	if sessionID != nil {
		sessionParam = *sessionID
	}

	if _, err := tx.Exec(ctx, `
        UPDATE wizard_runs
        SET status = $2,
            session_id = COALESCE($3::bigint, session_id),
            updated_at = NOW(),
            completed_at = CASE WHEN $2 = 'completed' THEN NOW() ELSE completed_at END,
            payload = COALESCE(payload, '{}'::jsonb) || $4::jsonb
        WHERE id = $1
    `, runID, status, sessionParam, payloadJSON); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// EnsureDefaultCommands inserts default commands for a new session if they are not already present.
func (p *sqlWizardPersistence) EnsureDefaultCommands(ctx context.Context, sessionID int64, commands []BotCommandRecord) error {
	if sessionID == 0 || len(commands) == 0 {
		return nil
	}

	tx, err := p.db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, cmd := range commands {
		var commandID int64
		err := tx.QueryRow(ctx, `
            SELECT id FROM bot_commands WHERE session_id = $1 AND command = $2
        `, sessionID, cmd.Command).Scan(&commandID)
		if err == nil {
			// Command already exists; skip to avoid overriding user customisations.
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		payloadJSON, err := json.Marshal(cmd.Payload)
		if err != nil {
			return err
		}

		if err := tx.QueryRow(ctx, `
            INSERT INTO bot_commands (session_id, command, description, response_type, payload)
            VALUES ($1, $2, $3, $4, $5)
            RETURNING id
        `, sessionID, cmd.Command, cmd.Description, cmd.ResponseType, payloadJSON).Scan(&commandID); err != nil {
			return err
		}

		for _, trigger := range cmd.Triggers {
			metadataJSON, err := json.Marshal(trigger.Metadata)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
                INSERT INTO bot_command_triggers (command_id, trigger_type, trigger_value, metadata)
                VALUES ($1, $2, $3, $4)
            `, commandID, trigger.Type, trigger.Value, metadataJSON); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

// Noop implementations ensure the wizard still functions without a database connection.
func (p *noopWizardPersistence) StartRun(context.Context, WizardRunStart) (int64, error) {
	if p.log != nil {
		p.log.Debug("wizard persistence disabled: StartRun skipped")
	}
	return 0, nil
}

func (p *noopWizardPersistence) SaveStep(context.Context, WizardStepRecord) error {
	return nil
}

func (p *noopWizardPersistence) UpdateRunStatus(context.Context, int64, string, *int64, map[string]any) error {
	return nil
}

func (p *noopWizardPersistence) EnsureDefaultCommands(context.Context, int64, []BotCommandRecord) error {
	return nil
}

func nullableInt(n sql.NullInt64) any {
	if n.Valid {
		return n.Int64
	}
	return nil
}

// DefaultBotCommands returns a baseline command set for new userbots managed via the wizard.
func DefaultBotCommands() []BotCommandRecord {
	return []BotCommandRecord{
		{
			Command:      "ping",
			Description:  "Cek apakah userbot sedang aktif",
			ResponseType: "text",
			Payload: map[string]any{
				"template": "Pong! Userbot aktif.",
			},
			Triggers: []BotCommandTriggerRecord{
				{Type: "command", Value: "ping", Metadata: map[string]any{"scope": "chat"}},
			},
		},
		{
			Command:      "help",
			Description:  "Tampilkan bantuan singkat",
			ResponseType: "text",
			Payload: map[string]any{
				"template": "Gunakan /ping untuk cek status atau /stop untuk menonaktifkan sementara.",
			},
			Triggers: []BotCommandTriggerRecord{
				{Type: "command", Value: "help", Metadata: map[string]any{"scope": "chat"}},
			},
		},
	}
}
