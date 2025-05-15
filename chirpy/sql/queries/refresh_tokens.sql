-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (user_id, token, expires_at, revoked_at, created_at, updated_at)
VALUES (
    $1,
    $2,
    TIMESTAMP 'now' + INTERVAL '60 days',
    NULL,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
);

-- name: GetUserByRefreshToken :one
SELECT user_id FROM refresh_tokens
WHERE token = $1
AND revoked_at IS NULL
AND expires_at > CURRENT_TIMESTAMP
LIMIT 1; 

-- name: RevokeRefreshToken :one
UPDATE refresh_tokens
SET revoked_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE token = $1
RETURNING user_id;
