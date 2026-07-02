#!/bin/bash
set -e
cd "$(dirname "$0")/.." # Always run from the backend root

echo "🛠️ Generating type-safe Go code from SQL schema..."
go tool sqlc generate

echo "✅ Database types generated successfully in database/dbgen!"