#!/bin/bash
set -e
cd "$(dirname "$0")/.." # Always run from the backend root

# 1. Check if the user provided a command (up, down, status, etc.)
COMMAND=$1
if [ -z "$COMMAND" ]; then
  echo "❌ Error: You must provide a migration command (e.g., up, down, status)"
  echo "Usage: ./scripts/migrate.sh up"
  exit 1
fi

# 2. Extract the DATABASE_URL from the .env file if it exists
DB_URL="postgres://postgres:postgres@localhost:5432/ducktivity?sslmode=disable&options=-c%20search_path%3Didentity" # Default fallback
if [ -f .env ]; then
  # This clever line reads the .env file and extracts the DATABASE_URL specifically
  # Strip only up to the FIRST '=' so values that themselves contain '='
  # (e.g. a Neon URL ending in '?sslmode=require') survive intact.
  ENV_URL=$(grep -v '^#' .env | grep -e "DATABASE_URL" | sed -e 's/^[^=]*=//')
  if [ ! -z "$ENV_URL" ]; then
    DB_URL=$ENV_URL
  fi
fi

echo "🚀 Running Goose migration: '$COMMAND'..."
# Run goose, pointing it to the schema folder and passing the command
go tool goose -dir sql/schema postgres "$DB_URL" "$COMMAND"

echo "✅ Migration completed!"