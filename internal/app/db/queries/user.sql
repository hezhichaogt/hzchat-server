-- name: CreateUser :one
-- Registers a new user with core credentials.
INSERT INTO users (
    username, 
    password_hash, 
    nickname
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- name: GetUserByUsername :one
-- Retrieves an active user by their username for authentication purposes.
-- Only returns users who have not been soft-deleted.
SELECT 
    id, 
    username, 
    password_hash, 
    nickname, 
    avatar_url, 
    plan_type
FROM users
WHERE username = $1 
  AND deleted_at IS NULL 
LIMIT 1;

-- name: GetUserByID :one
-- Retrieves a user's display profile and service plan by their UUID.
SELECT 
    id, 
    nickname, 
    avatar_url, 
    plan_type,
    last_login_at,
    password_hash
FROM users
WHERE id = $1 
  AND deleted_at IS NULL 
LIMIT 1;

-- name: UpdateLastLogin :exec
-- Updates the last login timestamp for a specific user.
UPDATE users 
SET last_login_at = NOW()
WHERE id = $1 
  AND deleted_at IS NULL;

-- name: UpdateUserProfile :one
-- Updates the user's nickname and avatar, and refreshes updated_at.
UPDATE users 
SET 
  nickname = $2,
  avatar_url = $3,
  updated_at = NOW()
WHERE id = $1 
  AND deleted_at IS NULL
RETURNING id, nickname, avatar_url, updated_at;

-- name: UpdateUserPassword :exec
UPDATE users 
SET 
  password_hash = $2,
  updated_at = NOW()
WHERE id = $1 
  AND deleted_at IS NULL;