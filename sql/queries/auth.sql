-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, user_id, created_at, updated_at, expires_at, revoked_at)
VALUES (
    $1,
    $2,
    NOW(),
    NOW(),
    $3,
    NULL
)
RETURNING *;


-- name: GetDBRefreshTokenByRefreshToken :one
SELECT * FROM refresh_tokens WHERE token=$1;

-- name: RevokeRefreshToken :exec 
UPDATE refresh_tokens
SET revoked_at = NOW(), 
updated_at = NOW()
WHERE token=$1;


