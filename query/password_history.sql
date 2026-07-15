-- name: AddPasswordHistory :exec
INSERT INTO password_history (user_id, password_hash) VALUES ($1, $2);

-- name: GetPasswordHistory :many
SELECT id, user_id, password_hash, created_at
FROM password_history
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: CleanupOldPasswordHistory :exec
DELETE FROM password_history
WHERE user_id = $1
AND id NOT IN (
    SELECT id FROM password_history
    WHERE user_id = $1
    ORDER BY created_at DESC
    LIMIT $2
);
