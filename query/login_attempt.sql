-- name: RecordLoginAttempt :exec
INSERT INTO login_attempts (user_id, ip_address) VALUES ($1, $2);

-- name: IsUserLocked :one
SELECT EXISTS(
    SELECT 1 FROM login_attempts
    WHERE user_id = $1
    AND attempted_at > NOW() - INTERVAL '30 minutes'
    GROUP BY user_id
    HAVING COUNT(*) >= 5
);

-- name: ClearUserLoginAttempts :exec
DELETE FROM login_attempts WHERE user_id = $1;
