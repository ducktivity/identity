#!/bin/bash
set -e
cd "$(dirname "$0")/.." # Always run from the backend root

echo "🎨 Formatting Go code..."
go fmt ./...

echo "🏗️ Building all packages..."
go build ./...

echo "🔍 Vetting for suspicious constructs..."
go vet ./...

echo "🧹 Running staticcheck..."
go tool staticcheck ./...

echo "🔒 Running gosec security scan..."
go tool gosec -quiet ./...

echo "✅ Backend checks passed!"
