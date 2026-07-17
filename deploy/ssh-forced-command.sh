#!/usr/bin/env bash
# Forced command for the CI deploy SSH key. The box's authorized_keys entry pins this via
#   command="/opt/ducktivity/identity/deploy/ssh-forced-command.sh",restrict <ssh-ed25519 …>
# so the key can do exactly ONE thing and nothing else: run deploy.sh with a single 40-hex git sha. sshd ignores whatever the client asks for and runs THIS instead, passing the client's request in $SSH_ORIGINAL_COMMAND — we parse the sha out of it and reject anything else. With `restrict` also denying shell/pty/port-forwarding, a leak of this key (behind the Access service token that gates the tunnel) can trigger a deploy of a real commit and nothing more.
set -euo pipefail

cmd="${SSH_ORIGINAL_COMMAND:-}"
# Expect exactly: <path>/deploy.sh <40-hex-sha>. Extract the sha or reject.
sha="$(printf '%s' "$cmd" | sed -n -E 's#^.*/deploy\.sh[[:space:]]+([0-9a-f]{40})$#\1#p')"
[ -n "$sha" ] || { echo "rejected: only 'deploy.sh <full-git-sha>' is permitted on this key" >&2; exit 1; }

exec /opt/ducktivity/identity/deploy/deploy.sh "$sha"
