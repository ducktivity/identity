#!/usr/bin/env bash
# Runs on YOUR machine with an UNRESTRICTED ssh key — the manual, owner-driven deploy/rollback. Bootstrapping sibling of deploy.sh (CI's restricted-key path): where deploy.sh assumes a clone exists, this converges the box from nothing. Box-side steps stream inline over Cloudflare Access SSH (bash -s), so there's no second script.
#
# Carries nothing to the box: secrets live only as the committed deploy/.env.sops, decrypted on the box with SOPS (same as deploy.sh). Image is pinned from the SHA arg (sha-<short>), gated on /readyz, auto-rolled-back to the last-good tag.
#
# Migrations run in CI, not here — this is an IMAGE revert only; never down-migrate the schema.
#
# Prereqs — machine: cloudflared + sourced Access service token (TUNNEL_SERVICE_TOKEN_ID/SECRET). Box: SOPS age key at /etc/ducktivity/age-key.txt, docker, and git/GHCR creds for the first-run clone + (if private) image pull.
#
# Usage:  ./deploy/remote-deploy.sh <full-git-sha>   (40-hex; deploy or roll back to that commit)
set -euo pipefail

SHA="${1:?usage: remote-deploy.sh <full-git-sha>   (40-hex; deploy or roll back to that commit)}"
# git checkout needs the real object; IMAGE_TAG is sha-<first 7 hex>.
[[ "$SHA" =~ ^[0-9a-f]{40}$ ]] || { echo "error: expected a full 40-hex git sha, got: '$SHA'" >&2; exit 2; }

# Overridable config (sane suite defaults).
SSH_HOST="${SSH_HOST:-ducktivity-ssh.ducktvt.com}"
SSH_USER="${SSH_USER:-deploy}"
APP_DIR="${APP_DIR:-/opt/ducktivity/identity}"            # clone root; compose project root is $APP_DIR/deploy
REPO_URL="${REPO_URL:-https://github.com/ducktivity/identity.git}"   # first-run clone only
DEPLOY_BRANCH="${DEPLOY_BRANCH:-main}"                    # fetched so the sha's object (fwd or rollback) is present

# SSH rides Cloudflare Access (no open port). ProxyCommand needs cloudflared + a sourced service token.
SSH_OPTS=(-o "ProxyCommand=cloudflared access ssh --hostname %h")
DEST="$SSH_USER@$SSH_HOST"

# Optional: log the box in to GHCR for a private image. Skip by leaving GHCR_TOKEN unset and making the package public.
if [ -n "${GHCR_TOKEN:-}" ]; then
  echo "==> logging box in to GHCR"
  ssh "${SSH_OPTS[@]}" "$DEST" "echo '$GHCR_TOKEN' | docker login ghcr.io -u '${GHCR_USER:-$SSH_USER}' --password-stdin"
fi

echo "==> deploying git sha $SHA on $DEST"
# Reconcile runs on the box; args passed positionally, quoted heredoc stays literal.
ssh "${SSH_OPTS[@]}" "$DEST" "bash -s -- '$APP_DIR' '$SHA' '$REPO_URL' '$DEPLOY_BRANCH'" <<'REMOTE'
set -euo pipefail
APP_DIR="$1"; SHA="$2"; REPO_URL="$3"; DEPLOY_BRANCH="$4"

export SOPS_AGE_KEY_FILE="${SOPS_AGE_KEY_FILE:-/etc/ducktivity/age-key.txt}"
EDGE_NET="${EDGE_NET:-ducktivity_edge}"                    # shared ingress network (edge stack owns it)
READYZ_HOST="${READYZ_HOST:-identity-backend}"           # app's compose network alias
READYZ_PORT="${READYZ_PORT:-8000}"
IMAGE="${IMAGE:-ghcr.io/ducktivity/identity-backend}"
# Pinned from the SHA arg, not the tree, so the git sync can't deploy the wrong build. Matches CI's `type=sha,prefix=sha-`.
IMAGE_TAG="sha-${SHA:0:7}"

# IMAGE_TAG overrides the compose default via env — no file edits to pin a tag.
deploy_tag() {
  IMAGE_TAG="$1" docker compose pull --quiet
  IMAGE_TAG="$1" docker compose up -d --remove-orphans
}

# Release gate: probe /readyz (also confirms NeonDB). Image is distroless (no curl), so probe from a throwaway curl container on the edge network.
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

# 1. Converge the box's clone onto the exact commit — idempotent. First run: mkdir + clone (--no-checkout, nothing materialized before sparse-checkout narrows to deploy/). Later runs: skip clone, just fetch + re-point. --force only touches tracked files, so box-local .env/.last_good_tag survive.
mkdir -p "$APP_DIR"
cd "$APP_DIR"
if [ ! -d .git ]; then
  git clone --quiet --no-checkout "$REPO_URL" .
fi
git fetch --quiet origin "$DEPLOY_BRANCH"
git sparse-checkout init --cone
git sparse-checkout set deploy
git checkout --quiet --force "$SHA"

cd "$APP_DIR/deploy"   # docker-compose.yml, .env.sops, and the decrypted .env live here

# 2. Decrypt secrets into the .env compose auto-loads. Atomic swap so a failed decrypt leaves the last good .env intact.
umask 077
sops --decrypt --input-type dotenv --output-type dotenv .env.sops > .env.tmp
chmod 600 .env.tmp
mv .env.tmp .env

# 3. Record the live tag (rollback target), then converge.
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
  exit 1
fi
REMOTE
