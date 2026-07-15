-- SECURE: Password history table to prevent password reuse

CREATE TABLE password_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for fast lookup of user's password history
CREATE INDEX idx_password_history_user ON password_history (user_id, created_at DESC);

-- Store last 5 password hashes for reuse checking
-- Old entries can be cleaned up periodically

-- SECURE: Session tracking for revocation capability
CREATE TABLE sessions_metadata (
    token TEXT PRIMARY KEY REFERENCES sessions(token) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    revoked_at TIMESTAMPTZ
);

-- Index for fast lookup of user's active sessions
CREATE INDEX idx_sessions_metadata_user ON sessions_metadata (user_id) WHERE is_revoked = FALSE;

-- SECURE: Track all session revocation events for audit
CREATE TABLE session_revocation_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    revoked_token TEXT NOT NULL,
    revoked_by UUID REFERENCES users(id), -- NULL if self-revoked
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_session_revocation_log_user ON session_revocation_log (user_id, created_at DESC);
