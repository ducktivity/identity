#!/usr/bin/env bash
# Runs ON THE BOX. deploy/remote-deploy.sh copies it (plus docker-compose.yml and
# .env) into the app dir from your machine and runs it over SSH with the new image
# tag as $1. It pulls the new image, swaps the stack, waits for /readyz, and rolls
# back to the last good image if the new one never reports ready — so the deploy
# self-heals.
set -euo pipefail
cd "$(dirname "$0")/.."

NEW_TAG="${1:?usage: reconcile-backend.sh <image-tag>}"
PREV_TAG="$(cat .last_good_tag 2>/dev/null || echo latest)"

# IMAGE_TAG in the shell env overrides docker-compose's default, so we never edit
# files to pin a tag.
deploy() {
  IMAGE_TAG="$1" docker compose pull
  IMAGE_TAG="$1" docker compose up -d --remove-orphans
}

# The app publishes no host port; reach it over the shared edge network by its alias.
# The app image is distroless (no curl), so probe from a throwaway curl container
# attached to ducktivity_edge. Every suite backend listens on 8000.
ready() {
  for _ in $(seq 1 10); do
    if docker run --rm --network ducktivity_edge curlimages/curl:latest \
        -fsS http://identity-service:8000/readyz >/dev/null 2>&1; then return 0; fi
    sleep 3
  done
  return 1
}

echo "deploying $NEW_TAG (previous good: $PREV_TAG)"
deploy "$NEW_TAG"
docker image prune -f

if ready; then
  echo "$NEW_TAG" > .last_good_tag
  echo "deploy ok: $NEW_TAG is live"
else
  echo "readyz failed; rolling back to $PREV_TAG" >&2
  deploy "$PREV_TAG"
  exit 1
fi
