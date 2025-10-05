-- +migrate Up
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT UNIQUE NOT NULL,
    username TEXT,
    full_name TEXT,
    plan TEXT DEFAULT 'free',
    status TEXT DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    owner_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    telegram_id BIGINT UNIQUE NOT NULL,
    session_type TEXT DEFAULT 'userbot',
    bot_username TEXT,
    bot_display_name TEXT,
    session_encrypted BYTEA NOT NULL,
    secret_version INT DEFAULT 1,
    login_method TEXT,
    metadata JSONB DEFAULT '{}'::JSONB,
    features JSONB DEFAULT '{}'::JSONB,
    config JSONB DEFAULT '{}'::JSONB,
    worker_host TEXT,
    status TEXT DEFAULT 'active',
    session_hash TEXT,
    request_id TEXT,
    origin TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    last_seen TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS reply_guard_rules (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    include_keywords JSONB,
    exclude_keywords JSONB,
    regex TEXT,
    targets JSONB,
    reply_text TEXT,
    status TEXT DEFAULT 'inactive',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS broadcast_jobs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    message TEXT NOT NULL,
    interval_minutes INT DEFAULT 60,
    targets_json JSONB,
    next_run TIMESTAMPTZ,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS usage_stats (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    session_id BIGINT REFERENCES sessions(id) ON DELETE CASCADE,
    command TEXT NOT NULL,
    count BIGINT DEFAULT 0,
    last_used TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bot_commands (
    id BIGSERIAL PRIMARY KEY,
    session_id BIGINT REFERENCES sessions(id) ON DELETE CASCADE,
    command TEXT NOT NULL,
    description TEXT,
    response_type TEXT DEFAULT 'text',
    payload JSONB DEFAULT '{}'::JSONB,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (session_id, command)
);

CREATE TABLE IF NOT EXISTS bot_command_triggers (
    id BIGSERIAL PRIMARY KEY,
    command_id BIGINT REFERENCES bot_commands(id) ON DELETE CASCADE,
    trigger_type TEXT NOT NULL,
    trigger_value TEXT,
    metadata JSONB DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS wizard_runs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    session_id BIGINT REFERENCES sessions(id) ON DELETE SET NULL,
    status TEXT DEFAULT 'draft',
    current_step TEXT,
    payload JSONB DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS wizard_steps (
    id BIGSERIAL PRIMARY KEY,
    wizard_run_id BIGINT REFERENCES wizard_runs(id) ON DELETE CASCADE,
    step_name TEXT NOT NULL,
    sequence INT NOT NULL,
    state JSONB DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audits (
    id BIGSERIAL PRIMARY KEY,
    actor_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    target_type TEXT NOT NULL,
    target_id BIGINT,
    action TEXT NOT NULL,
    diff JSONB DEFAULT '{}'::JSONB,
    metadata JSONB DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_owner_status ON sessions(owner_user_id, status);
CREATE INDEX IF NOT EXISTS idx_sessions_type ON sessions(session_type);
CREATE INDEX IF NOT EXISTS idx_reply_guard_user ON reply_guard_rules(user_id);
CREATE INDEX IF NOT EXISTS idx_broadcast_jobs_next_run ON broadcast_jobs(next_run);
CREATE INDEX IF NOT EXISTS idx_usage_stats_user ON usage_stats(user_id);
CREATE INDEX IF NOT EXISTS idx_usage_stats_session ON usage_stats(session_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_bot_username ON sessions(bot_username) WHERE bot_username IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_bot_commands_session ON bot_commands(session_id);
CREATE INDEX IF NOT EXISTS idx_bot_command_triggers_cmd ON bot_command_triggers(command_id);
CREATE INDEX IF NOT EXISTS idx_wizard_runs_user ON wizard_runs(user_id);
CREATE INDEX IF NOT EXISTS idx_wizard_steps_run ON wizard_steps(wizard_run_id);
CREATE INDEX IF NOT EXISTS idx_audits_actor ON audits(actor_id);

-- +migrate Down
DROP TABLE IF EXISTS audits;
DROP TABLE IF EXISTS wizard_steps;
DROP TABLE IF EXISTS wizard_runs;
DROP TABLE IF EXISTS bot_command_triggers;
DROP TABLE IF EXISTS bot_commands;
DROP TABLE IF EXISTS usage_stats;
DROP TABLE IF EXISTS broadcast_jobs;
DROP TABLE IF EXISTS reply_guard_rules;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
