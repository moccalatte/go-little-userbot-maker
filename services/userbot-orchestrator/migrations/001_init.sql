-- +migrate Up
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT UNIQUE NOT NULL,
    username TEXT,
    plan TEXT DEFAULT 'free',
    status TEXT DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    telegram_id BIGINT UNIQUE NOT NULL,
    session_encrypted BYTEA NOT NULL,
    secret_version INT DEFAULT 1,
    login_method TEXT,
    metadata JSONB DEFAULT '{}'::JSONB,
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
    command TEXT NOT NULL,
    count BIGINT DEFAULT 0,
    last_used TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_reply_guard_user ON reply_guard_rules(user_id);
CREATE INDEX IF NOT EXISTS idx_broadcast_jobs_next_run ON broadcast_jobs(next_run);
CREATE INDEX IF NOT EXISTS idx_usage_stats_user ON usage_stats(user_id);

-- +migrate Down
DROP TABLE IF EXISTS usage_stats;
DROP TABLE IF EXISTS broadcast_jobs;
DROP TABLE IF EXISTS reply_guard_rules;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
