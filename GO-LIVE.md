# identity — go-live checklist (throwaway)

First-time deploy to the shared box. CI migrates + builds on push to `main`;
go-live is one command from your machine (`deploy/remote-deploy.sh`). Delete this
once identity is live.

## 4. Prep (local, once)

- [ ] Copy `.env.example` → `.env`, fill it (your secrets; git-ignored, never committed).
- [ ] `cloudflared` installed + Access service token sourced
      (`TUNNEL_SERVICE_TOKEN_ID` / `TUNNEL_SERVICE_TOKEN_SECRET`).
- [ ] GHCR pull auth: make the `identity-backend` package public, **or** deploy with
      `GHCR_TOKEN=<read:packages token> ./deploy/remote-deploy.sh …`.

## 5. Deploy

- [ ] Push `main`. Backend CD runs `changes → check → migrate → image`. Note the `sha-<short>` tag.
- [ ] From your machine:
  ```bash
  ./deploy/remote-deploy.sh sha-<short>
  ```
  Pushes compose + reconcile script + `.env` to the box and runs the reconcile
  (pull → up → `/readyz` → auto-rollback). Prints `deploy ok` on success.

## 6. Verify

```bash
curl -fsS https://id.ducktvt.com/readyz
```
