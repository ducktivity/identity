# Deploy

Push to `main` → **Backend CD** builds, migrates Neon, SSHes to the box, and runs `deploy.sh`. A red run = failed deploy (box already rolled back). Tag: `sha-<short-gitsha>`.

### Prerequisites (your machine)

- `sops`, `age` installed
- Operator age key:

```sh
age-keygen -o "$APPDATA/sops/age/keys.txt"   # C:\Users\<you>\AppData\Roaming\sops\age\keys.txt
```

For `remote-deploy.sh` also:

- `cloudflared` installed
- Access service token sourced: `TUNNEL_SERVICE_TOKEN_ID`, `TUNNEL_SERVICE_TOKEN_SECRET`

### Prerequisites (the box)

- Docker daemon DNS. The container resolves NeonDB/Resend hostnames via whatever upstream it inherits; the host's systemd-resolved stub (`127.0.0.53`) is unreachable from the container netns whenever resolved blinks, which kills all outbound lookups (`connection refused`). Pin reliable public resolvers box-wide so no container depends on the host stub:

```sh
sudo tee /etc/docker/daemon.json >/dev/null <<'EOF'
{
  "dns": ["1.1.1.1", "8.8.8.8"]
}
EOF
sudo systemctl restart docker
```

This replaces the compose-level `dns:` override for every container (Docker still resolves internal aliases via its embedded `127.0.0.11`).

- `sops`, `age` installed
- Box age key:

```sh
sudo mkdir -p /etc/ducktivity
sudo age-keygen -o /etc/ducktivity/age-key.txt
sudo chown deploy:deploy /etc/ducktivity/age-key.txt && sudo chmod 600 /etc/ducktivity/age-key.txt
```

- Deploy SSH key. On your machine, create the CI keypair (no passphrase):

```sh
ssh-keygen -t ed25519 -f ./deploy_ci_key -N '' -C 'identity-ci-deploy'
```

Add the **public** half to the box's `~/.ssh/authorized_keys`, forced to the deploy wrapper (paste `deploy_ci_key.pub` where shown):

```sh
command="/opt/ducktivity/identity/deploy/ssh-forced-command.sh",restrict <public key here>
```

Put the **private** `deploy_ci_key` in the GitHub `DEPLOY_SSH_KEY` secret (for CI) or keep it locally for `remote-deploy.sh`. Then delete `deploy_ci_key`/`deploy_ci_key.pub` once both halves are in place.

### First-time secrets

```bash
cp deploy/.env.example deploy/.env.prod   # fill it in
./deploy/secrets.sh encrypt                # writes deploy/.env.sops
git add deploy/.env.sops && git commit && git push
```

### Deploy

```bash
git push        # merge to main; Backend CD deploys it live
```

### Roll back to a commit (image revert only)

```bash
./deploy/remote-deploy.sh <full-git-sha>
```

- Usage: full 40-hex sha of the commit you want live.

### Edit box secrets

```bash
./deploy/secrets.sh edit      # open decrypted .env.sops in $EDITOR, re-encrypt on save
```

Then commit `.env.sops` and push — applied on the next deploy.

- `./deploy/secrets.sh encrypt` — (re)create `.env.sops` from local `.env.prod`
- `./deploy/secrets.sh view` — print decrypted secrets to stdout
