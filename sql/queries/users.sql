-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1,
    $2
)
RETURNING *;

-- name: FindUserByEmail :one
SELECT * FROM users WHERE email=$1;

-- name: GetUserById :one
SELECT * FROM users WHERE id=$1;

-- name: DeleteAllUsers :exec
DELETE FROM users;

-- name: UpdateUserById :one
UPDATE users
SET hashed_password = $1,
email = $2,
updated_at = NOW()
WHERE id = $3
RETURNING *;

-- name: UpgradeUserToRed :exec
UPDATE users
SET is_chirpy_red=true
WHERE id=$1;

