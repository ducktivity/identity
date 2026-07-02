-- name: UpsertUserByEmail :one
-- Requesting a code for an unknown email transparently creates the account, so there is no separate sign-up step. The no-op ON CONFLICT update lets us return the existing row's id when the account already exists.
INSERT INTO users (email)
VALUES ($1)
ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
RETURNING id, email;

-- name: GetUserByEmail :one
SELECT id, email FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email FROM users WHERE id = $1;

-- name: GetLatestActiveAuthCode :one
-- Newest unspent, unexpired code for a user. Verification matches against this one row so old codes cannot be reused once a newer one is requested.
SELECT id, code_hash, attempts, created_at
FROM auth_codes
WHERE user_id = $1
  AND consumed_at IS NULL
  AND expires_at > now()
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateAuthCode :exec
INSERT INTO auth_codes (user_id, code_hash, expires_at)
VALUES ($1, $2, $3);

-- name: IncrementAuthCodeAttempts :exec
UPDATE auth_codes SET attempts = attempts + 1 WHERE id = $1;

-- name: ConsumeAuthCode :exec
UPDATE auth_codes SET consumed_at = now() WHERE id = $1;
