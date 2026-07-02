#!/usr/bin/env bash
# Runs on YOUR machine (Git Bash / WSL), NOT the box. One command to go live:
# copies the compose file, the reconcile script, and your local .env to the box
# over Cloudflare Access SSH, then runs reconcile-backend.sh (pull -> up -> /readyz
# -> auto-rollback). Replaces the old Ansible playbook — no ansible, no vault, no
# secrets in git or GitHub. Your secrets stay in identity/.env on your disk only.
#
# Migrations are NOT run here; CI applies them (expand-only) when it builds the
# image, so the tag you deploy already has its schema live.
#
# Prereqs on your machine: cloudflared installed + the Cloudflare Access service
# token sourced (TUNNEL_SERVICE_TOKEN_ID / TUNNEL_SERVICE_TOKEN_SECRET), and a
# filled identity/.env (copy from .env.example).
#
# Usage:  ./deploy/remote-deploy.sh <image-tag>      e.g. sha-1a2b3c4  (or latest)
set -euo pipefail
cd "$(dirname "$0")/.."   # repo root (identity/)

TAG="${1:?usage: remote-deploy.sh <image-tag>   e.g. sha-1a2b3c4}"

# Overridable config (sane suite defaults).
SSH_HOST="${SSH_HOST:-ducktivity-ssh.ducktvt.com}"
SSH_USER="${SSH_USER:-deploy}"
APP_DIR="${APP_DIR:-/opt/ducktivity/identity}"
SECRETS="${SECRETS:-.env}"   # local, git-ignored

[ -f "$SECRETS" ] || { echo "error: $SECRETS not found — copy .env.example to .env and fill it." >&2; exit 1; }

# SSH rides Cloudflare Access (no open port on the box). ProxyCommand needs
# cloudflared + a sourced service token; see the prereqs above.
SSH_OPTS=(-o "ProxyCommand=cloudflared access ssh --hostname %h" -o StrictHostKeyChecking=accept-new)
DEST="$SSH_USER@$SSH_HOST"

echo "==> staging runtime files on $DEST:$APP_DIR"
ssh "${SSH_OPTS[@]}" "$DEST" "mkdir -p '$APP_DIR/deploy'"
scp "${SSH_OPTS[@]}" docker-compose.yml          "$DEST:$APP_DIR/docker-compose.yml"
scp "${SSH_OPTS[@]}" deploy/reconcile-backend.sh "$DEST:$APP_DIR/deploy/reconcile-backend.sh"
scp "${SSH_OPTS[@]}" "$SECRETS"                  "$DEST:$APP_DIR/.env"

# Optional: authenticate the box to GHCR if the image package is private. Skip this
# by leaving GHCR_TOKEN unset and making the GHCR package public instead.
if [ -n "${GHCR_TOKEN:-}" ]; then
  echo "==> logging box in to GHCR"
  ssh "${SSH_OPTS[@]}" "$DEST" "echo '$GHCR_TOKEN' | docker login ghcr.io -u '${GHCR_USER:-$SSH_USER}' --password-stdin"
fi

echo "==> reconciling to $TAG"
ssh "${SSH_OPTS[@]}" "$DEST" \
  "chmod 600 '$APP_DIR/.env' && chmod +x '$APP_DIR/deploy/reconcile-backend.sh' && '$APP_DIR/deploy/reconcile-backend.sh' '$TAG'"
