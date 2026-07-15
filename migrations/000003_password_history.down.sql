-- Rollback password history and session security

DROP INDEX IF EXISTS idx_session_revocation_log_user;
DROP TABLE IF EXISTS session_revocation_log;
DROP INDEX IF EXISTS idx_sessions_metadata_user;
DROP TABLE IF EXISTS sessions_metadata;
DROP INDEX IF EXISTS idx_password_history_user;
DROP TABLE IF EXISTS password_history;
