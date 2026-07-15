-- Rollback security improvements

DROP INDEX IF EXISTS idx_sessions_expiry_lookup;
DROP FUNCTION IF EXISTS cleanup_old_login_attempts();
DROP INDEX IF EXISTS idx_login_attempts_ip_time;
DROP INDEX IF EXISTS idx_login_attempts_user_time;
DROP TABLE IF EXISTS login_attempts;
