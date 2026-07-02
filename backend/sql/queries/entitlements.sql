-- name: GetEntitlement :one
-- The user's current suite-wide entitlement. A user with no row is on the free plan (the default for every new account); callers translate ErrNoRows to free.
SELECT plan, until FROM entitlements WHERE user_id = $1;

-- name: UpsertEntitlement :exec
-- Sets a user's suite-wide entitlement. Called from the billing webhook (and the dev grant) so a single payment flips the one entitlement that every app reads. status mirrors the Stripe subscription status for auditing.
INSERT INTO entitlements (user_id, plan, status, until, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (user_id) DO UPDATE
  SET plan = EXCLUDED.plan, status = EXCLUDED.status, until = EXCLUDED.until, updated_at = now();
