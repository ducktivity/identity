package database

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB holds our connection pool global instance. The identity service points at the SAME Neon as the app backends: it owns the users / auth_codes / entitlements rows (search_path=identity), while apps only read users and never touch identity's data.
var DB *pgxpool.Pool

// Ping verifies the database is reachable. It backs the /readyz readiness probe, so callers should pass a short-deadline context to keep the probe responsive.
func Ping(ctx context.Context) error {
	if DB == nil {
		return errors.New("database pool not initialized")
	}
	return DB.Ping(ctx)
}

func Connect() {
	// The deployment injects the connection string via this env var: in prod it is Neon's pooled URL from the box's .env; locally it falls back to the line below.
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		// Default fallback for running a local Postgres container. These are throwaway localhost dev credentials, not a real secret.
		connStr = "postgres://postgres:postgres@localhost:5432/ducktivity?sslmode=disable&options=-c%20search_path%3Didentity" // #nosec G101 -- local-dev fallback, no real credential
	}

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		slog.Error("Unable to parse DATABASE_URL", "error", err)
		os.Exit(1)
	}

	// Configure pool settings for optimal performance
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnIdleTime = 30 * time.Minute

	// Create the connection pool with a strict 5-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	DB, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		slog.Error("Unable to create database connection pool", "error", err)
		os.Exit(1)
	}

	// Ping the database to ensure connection is actually alive
	if err := DB.Ping(ctx); err != nil {
		slog.Error("Database ping failed, server unreachable", "error", err)
		os.Exit(1)
	}

	slog.Info("Successfully connected to PostgreSQL database connection pool")
}
