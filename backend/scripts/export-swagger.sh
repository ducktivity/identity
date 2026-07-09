#!/bin/bash
set -e
cd "$(dirname "$0")/.." # Always run from the backend root

echo "🔄 Generating OpenAPI schema using Swag..."
go tool swag init --parseDependency --parseInternal

echo "🚚 Moving swagger.json to shared-schemas..."
mv docs/swagger.json ../shared-schemas/swagger.json

echo "🧹 Cleaning up docs folder..."
rm -rf docs/

echo "✅ Schema successfully synced as JSON!"
