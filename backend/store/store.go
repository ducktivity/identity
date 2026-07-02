// Package store is a thin adapter over the sqlc-generated query layer in
// database/dbgen (matching Drinkwater's convention). The hand-written SQL strings
// are gone: queries live in sql/queries/*.sql and are regenerated with
// `./scripts/db-codegen.sh`. The functions here only marshal between the generated
// row/param types (pgtype.Timestamptz, etc.) and the small domain types the HTTP
// handlers expect, keeping pgx specifics out of the handler boundary.
//
// The identity service points at the SAME Neon as the app backends: it owns the
// users / auth_codes / entitlements rows; apps only read users (to satisfy their
// per-row user_id) and never touch identity's data.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ducktivity/identity/backend/database"
	"github.com/ducktivity/identity/backend/database/dbgen"
	"github.com/ducktivity/platform-go/authtoken"
)

// queries returns a Queries bound to the shared pool. Cheap to construct per call,
// mirroring how Drinkwater's handlers do dbgen.New(database.DB).
func queries() *dbgen.Queries { return dbgen.New(database.DB) }

// User is the account shape the handlers work with.
type User struct {
	ID    uuid.UUID
	Email string
}

// AuthCode is the login-code shape the handlers work with.
type AuthCode struct {
	ID       uuid.UUID
	CodeHash string
	Attempts int32
}

// UpsertUserByEmail returns the account for email, creating it on first sight so
// unknown emails transparently get an account.
func UpsertUserByEmail(ctx context.Context, email string) (User, error) {
	row, err := queries().UpsertUserByEmail(ctx, email)
	if err != nil {
		return User{}, err
	}
	return User{ID: row.ID, Email: row.Email}, nil
}

// GetUserByEmail returns the account for email, or pgx.ErrNoRows if none.
func GetUserByEmail(ctx context.Context, email string) (User, error) {
	row, err := queries().GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, err
	}
	return User{ID: row.ID, Email: row.Email}, nil
}

// GetLatestActiveAuthCode returns the newest unconsumed, unexpired code for a user,
// or pgx.ErrNoRows if none. createdAt backs the per-account resend cooldown.
func GetLatestActiveAuthCode(ctx context.Context, userID uuid.UUID) (code AuthCode, createdAt time.Time, err error) {
	row, err := queries().GetLatestActiveAuthCode(ctx, userID)
	if err != nil {
		return AuthCode{}, time.Time{}, err
	}
	return AuthCode{ID: row.ID, CodeHash: row.CodeHash, Attempts: row.Attempts}, row.CreatedAt.Time, nil
}

func CreateAuthCode(ctx context.Context, userID uuid.UUID, codeHash string, expiresAt time.Time) error {
	return queries().CreateAuthCode(ctx, dbgen.CreateAuthCodeParams{
		UserID:    userID,
		CodeHash:  codeHash,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
}

func IncrementAuthCodeAttempts(ctx context.Context, codeID uuid.UUID) error {
	return queries().IncrementAuthCodeAttempts(ctx, codeID)
}

func ConsumeAuthCode(ctx context.Context, codeID uuid.UUID) error {
	return queries().ConsumeAuthCode(ctx, codeID)
}

// GetEntitlement returns the user's current suite-wide entitlement. A user with no
// entitlement row is on the free plan — the default for every new account.
func GetEntitlement(ctx context.Context, userID uuid.UUID) (authtoken.Entitlement, error) {
	row, err := queries().GetEntitlement(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return authtoken.Entitlement{Plan: authtoken.PlanFree}, nil
	}
	if err != nil {
		return authtoken.Entitlement{}, err
	}
	ent := authtoken.Entitlement{Plan: row.Plan}
	if row.Until.Valid {
		ent.Until = row.Until.Time.Unix()
	}
	return ent, nil
}

// UpsertEntitlement sets a user's suite-wide entitlement. Called from the billing
// webhook (and the dev grant) so a single payment flips the one entitlement that
// every app reads. status mirrors the Stripe subscription status for auditing.
func UpsertEntitlement(ctx context.Context, userID uuid.UUID, plan, status string, until *time.Time) error {
	pgUntil := pgtype.Timestamptz{}
	if until != nil {
		pgUntil = pgtype.Timestamptz{Time: *until, Valid: true}
	}
	return queries().UpsertEntitlement(ctx, dbgen.UpsertEntitlementParams{
		UserID: userID,
		Plan:   plan,
		Status: status,
		Until:  pgUntil,
	})
}
