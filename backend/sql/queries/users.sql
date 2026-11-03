-- name: UpsertUser :exec
-- Idempotent provisioning from a validated JWT. An empty token email never
-- overwrites a stored email.
INSERT INTO users (id, email)
VALUES (sqlc.arg(id), sqlc.arg(email))
ON CONFLICT (id) DO UPDATE
SET email      = CASE WHEN EXCLUDED.email = '' THEN users.email ELSE EXCLUDED.email END,
    updated_at = now();

-- name: GetUser :one
SELECT id, email, display_name, created_at, updated_at
FROM users
WHERE id = sqlc.arg(id);
