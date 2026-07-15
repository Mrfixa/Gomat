-- Myrmidons Phase 4: Security Hardening
-- ============================================

-- ============================================
-- Enhanced Session Management
-- ============================================

-- Add login timeout tracking
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS last_activity TIMESTAMPTZ;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS ip_hash TEXT; -- Hashed IP for privacy

-- Create index for session cleanup
CREATE INDEX IF NOT EXISTS idx_sessions_expires_activity ON sessions (expires_at, last_activity);

-- ============================================
-- Enhanced Audit Logging
-- ============================================

CREATE TABLE IF NOT EXISTS audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50),
    resource_id UUID,
    ip_hash TEXT, -- Hashed for privacy
    user_agent TEXT,
    details JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_user ON audit_log (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log (action, created_at DESC);

-- ============================================
-- Login Security
-- ============================================

-- Track failed login attempts by IP (for pattern detection)
CREATE TABLE IF NOT EXISTS failed_login_ip (
    ip_address INET NOT NULL,
    attempt_count INT NOT NULL DEFAULT 1,
    first_attempt TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_attempt TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (ip_address)
);

CREATE INDEX IF NOT EXISTS idx_failed_login_ip_recent ON failed_login_ip (last_attempt DESC);

-- ============================================
-- VPN/Proxy Detection (Informational)
-- ============================================

CREATE TABLE IF NOT EXISTS ip_metadata (
    ip_address INET PRIMARY KEY,
    is_tor BOOLEAN DEFAULT FALSE,
    is_vpn BOOLEAN DEFAULT FALSE,
    is_proxy BOOLEAN DEFAULT FALSE,
    country_code VARCHAR(2),
    last_updated TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================
-- Security Event Notifications
-- ============================================

CREATE TABLE IF NOT EXISTS security_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    event_type VARCHAR(50) NOT NULL, -- login_failed, account_locked, password_changed, etc.
    severity VARCHAR(20) NOT NULL DEFAULT 'info', -- info, warning, critical
    ip_address INET,
    details JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    notified_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_security_events_user ON security_events (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_security_events_type ON security_events (event_type, created_at DESC);

-- ============================================
-- Enhanced Rate Limiting
-- ============================================

CREATE TABLE IF NOT EXISTS rate_limit_buckets (
    key_prefix VARCHAR(100) NOT NULL,
    tokens INT NOT NULL DEFAULT 0,
    last_refill TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (key_prefix)
);

-- ============================================
-- API Key Management (for future API)
-- ============================================

CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    key_hash TEXT NOT NULL, -- SHA-256 hash of the key
    key_prefix VARCHAR(8) NOT NULL, -- First 8 chars for identification
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys (user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys (key_hash);
