#!/usr/bin/env bash
# Runs ON THE BOX, invoked by CI over SSH (Cloudflare Access, short-lived cert) right after the image is published — the whole push-based deploy. CI passes the exact git sha; this checks out that commit (so the compose file + .env.sops match the image), decrypts secrets, brings the stack up on the matching sha-<short> image, and gates on /readyz. On failure it rolls back to the last-good image and exits non-zero.
#
# Because CI runs this synchronously and watches the exit code, a failed deploy turns the GitHub Actions run red and notifies you through the normal CI failure path — there is no separate deploy-health alert to build or maintain.
#
# Usage (from CI, over Access SSH):   deploy/deploy.sh <full-git-sha>
# Manual rollback (over Access SSH):  deploy/deploy.sh <older-full-git-sha>
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/ducktivity/identity}"            # the git clone root; the compose project root is $APP_DIR/deploy
DEPLOY_BRANCH="${DEPLOY_BRANCH:-main}"
export SOPS_AGE_KEY_FILE="${SOPS_AGE_KEY_FILE:-/etc/ducktivity/age-key.txt}"
EDGE_NET="${EDGE_NET:-ducktivity_edge}"                    # shared ingress network (edge stack owns it)
READYZ_HOST="${READYZ_HOST:-identity-backend}"            # compose network alias of the app
READYZ_PORT="${READYZ_PORT:-8000}"
IMAGE="${IMAGE:-ghcr.io/ducktivity/identity-backend}"

SHA="${1:?usage: deploy.sh <full-git-sha>}"
# The image to run is pinned from the SHA ARG, not from anything in the checked-out tree, so the git sync above can never make docker deploy the wrong build (e.g. compose's `latest` default). sha-<7hex> matches CI's docker/metadata `type=sha,prefix=sha-` (short = first 7 hex of the sha).
IMAGE_TAG="sha-${SHA:0:7}"

cd "$APP_DIR/deploy"   # compose project root: docker-compose.yml, .env.sops, and the decrypted .env all live here

# Bring the app up (or swap it) at a specific tag. IMAGE_TAG overrides the compose default purely via env, so we never edit files to pin a tag.
deploy_tag() {
  IMAGE_TAG="$1" docker compose pull --quiet
  IMAGE_TAG="$1" docker compose up -d --remove-orphans
}

# The release gate: probe /readyz (which also confirms NeonDB reachability). The app image is distroless (no curl), so we probe from a throwaway curl container on the shared edge network via the app's alias.
ready() {
  for _ in $(seq 1 10); do
    if docker run --rm --network "$EDGE_NET" curlimages/curl:latest \
        -fsS "http://$READYZ_HOST:$READYZ_PORT/readyz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 3
  done
  return 1
}

# 1. Sync the box's clone to the EXACT commit being deployed, so the compose file and .env.sops line up with the image CI built for this sha — and so a rollback to an older sha gets THAT sha's deploy/ tree, not main's tip. On a normal forward deploy $SHA is already main's tip, so this is exactly "pull the latest deploy/ from main"; for rollback it deliberately is not. Fetch first so the object exists (fetching the branch also carries its history, so an older rollback sha is present); --force discards any stray tree state.
#
#    The box only ever needs the deploy runtime, and it all lives under deploy/ (compose file, encrypted env, scripts). The backend source lives in the prebuilt image, so materializing the whole tree on the box is dead weight. Narrow the working tree to deploy/ with cone-mode sparse-checkout. This runs on every deploy and is idempotent — on the hand-made full clone the first run prunes the extra paths (backend/, api-client/, …) and thereafter it is a no-op. sparse-checkout and --force only touch tracked files, so the git-ignored box-local files (.env, .last_good_tag) survive.
git fetch --quiet origin "$DEPLOY_BRANCH"
git sparse-checkout init --cone
git sparse-checkout set deploy
git checkout --quiet --force "$SHA"

# 2. Decrypt the committed secrets into the .env compose auto-loads. Atomic swap so a failed decrypt leaves the last good .env intact; set -e then aborts before any deploy.
umask 077
sops --decrypt --input-type dotenv --output-type dotenv .env.sops > .env.tmp
chmod 600 .env.tmp
mv .env.tmp .env

# 3. Remember what is live now (rollback target), then converge onto the new build.
GOOD="$(cat .last_good_tag 2>/dev/null || true)"

echo "deploying: ${GOOD:-none} -> $IMAGE_TAG ($IMAGE:$IMAGE_TAG, sha $SHA)"
deploy_tag "$IMAGE_TAG"
docker image prune -f >/dev/null 2>&1 || true

if ready; then
  echo "$IMAGE_TAG" > .last_good_tag
  echo "ok: $IMAGE_TAG is live"
else
  if [ -n "$GOOD" ] && [ "$GOOD" != "$IMAGE_TAG" ]; then
    echo "readyz failed for $IMAGE_TAG; rolling back to $GOOD" >&2
    deploy_tag "$GOOD"
    ready || echo "WARNING: rollback to $GOOD also failed readyz — investigate" >&2
  else
    echo "readyz failed for $IMAGE_TAG and no known-good build to roll back to" >&2
  fi
  exit 1   # non-zero -> CI job goes red -> you are notified
fi
