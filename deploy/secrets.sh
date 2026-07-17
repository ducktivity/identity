#!/usr/bin/env bash
# Operator helper for the git-committed, SOPS-encrypted box secrets (.env.sops). Requires sops + age installed and your OPERATOR age key present (default ~/.config/sops/age/keys.txt, or point SOPS_AGE_KEY_FILE at it). Recipients and the encryption rule live in .sops.yaml.
#
#   deploy/secrets.sh edit      # open the decrypted secrets in $EDITOR, re-encrypt on save
#   deploy/secrets.sh encrypt   # (re)create .env.sops from your local .env.prod
#   deploy/secrets.sh view      # print the decrypted secrets to stdout (do not redirect to a file)
#
# After edit/encrypt, commit .env.sops and push; the box applies it on the next reconcile.
set -euo pipefail
cd "$(dirname "$0")"   # deploy/ — holds .env.prod, .env.sops, and .sops.yaml

case "${1:-}" in
  edit)
    exec sops --input-type dotenv --output-type dotenv .env.sops
    ;;
  encrypt)
    [ -f .env.prod ] || { echo "error: no .env.prod to encrypt (copy .env.example and fill it)" >&2; exit 1; }
    sops --encrypt --input-type dotenv --output-type dotenv .env.prod > .env.sops
    echo "wrote .env.sops — commit it (git add .env.sops)"
    ;;
  view)
    exec sops --decrypt --input-type dotenv --output-type dotenv .env.sops
    ;;
  *)
    echo "usage: $0 {edit|encrypt|view}" >&2
    exit 2
    ;;
esac
