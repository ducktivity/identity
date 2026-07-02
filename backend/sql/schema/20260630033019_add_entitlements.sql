-- +goose Up

-- The single suite-wide entitlement. One row per user gates access to EVERY app
-- in the suite (Drinkwater, Wallet, …) — there is deliberately no per-app column,
-- because one payment unlocks everything. The billing webhook upserts this row
-- from Stripe subscription events; apps never read it directly — they receive the
-- resolved entitlement inside their session token.
--
-- This migration runs under the identity service's OWN goose history, kept in its
-- own schema (identity.goose_db_version, selected by search_path=identity), so the
-- two services migrate the shared Neon independently. users and auth_codes are
-- created by identity's base migration (20260616052531) which runs before this one.
CREATE TABLE IF NOT EXISTS entitlements (
    user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    plan       TEXT NOT NULL DEFAULT 'free',         -- 'free' | 'pro'
    status     TEXT NOT NULL DEFAULT 'none',         -- mirrors the Stripe subscription status for auditing
    until      TIMESTAMP WITH TIME ZONE,             -- paid-plan expiry (Stripe current_period_end); NULL = no expiry
    stripe_customer_id TEXT,                         -- links the account to its Stripe customer
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS entitlements;
