#!/bin/bash
set -e
cd "$(dirname "$0")/.." # Always run from the backend root

# 1. Check if the user provided a migration name
MIGRATION_NAME=$1
if [ -z "$MIGRATION_NAME" ]; then
  echo "❌ Error: You must provide a migration name (e.g., add_some_column)"
  echo "Usage: ./scripts/db-new-schema.sh add_some_column"
  exit 1
fi

echo "🚀 Creating new Goose migration: '$MIGRATION_NAME'..."
go tool goose -dir sql/schema create "$MIGRATION_NAME" sql

echo "✅ Migration files created in sql/"
