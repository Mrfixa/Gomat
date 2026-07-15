-- SECURE: Login attempts tracking for account lockout protection

CREATE TABLE login_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    ip_address TEXT NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for fast lookup of recent attempts by user
CREATE INDEX idx_login_attempts_user_time ON login_attempts (user_id, attempted_at);

-- Index for fast lookup of attempts by IP
CREATE INDEX idx_login_attempts_ip_time ON login_attempts (ip_address, attempted_at);

-- Cleanup old login attempts (older than 24 hours) - can be run as a cron job
CREATE OR REPLACE FUNCTION cleanup_old_login_attempts()
RETURNS void AS $$
BEGIN
    DELETE FROM login_attempts WHERE attempted_at < NOW() - INTERVAL '24 hours';
END;
$$ LANGUAGE plpgsql;

-- Create index for expired session cleanup
CREATE INDEX IF NOT EXISTS idx_sessions_expiry_lookup ON sessions (expiry) WHERE expiry < NOW() + INTERVAL '1 day';
