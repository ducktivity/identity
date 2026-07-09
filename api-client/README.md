# @ducktivity/identity-client

Generated TypeScript client + types for the Ducktivity **identity service** (`id.ducktvt.com`) —
the suite-wide sole issuer of login codes and session tokens.

It lives inside the identity repo (not a separate repo), so an identity contract change and its
client update are **one commit**. It is the frontend counterpart to [`platform-go`](https://github.com/ducktivity/platform-go)
(the shared Go verifier module): one is how app **backends** verify identity's tokens, this is how
app **frontends** call identity's login endpoints with types that can't drift.

## Source of truth

```
backend/api/dto.go                 (Go DTOs — the SSOT)
  └─ swaggo (backend/scripts/export-swagger.sh) ─▶ shared-schemas/swagger.json  (OpenAPI 2.0)
       └─ swagger2openapi                        ─▶ openapi.json                 (OpenAPI 3.0, gitignored)
            └─ openapi-typescript                ─▶ src/schema.ts                (TS types, committed)
                 └─ tsc                          ─▶ dist/                        (published build)
```

`src/schema.ts` is **generated — never hand-edit it.** To pick up an identity contract change:

```bash
# From api-client/. Regenerates types.
pnpm run generate-types

# Then bump the version and commit; the push to main auto-publishes (see below).
# edit package.json version -> e.g. 0.1.1
git commit -am "feat: <what changed> + sync identity client to 0.1.1"
```

## Publishing (automated, public npm)

Pushing to `main` with a changed `version` auto-publishes to the **public npm registry** via
`.github/workflows/cd-api-client.yml`, using the repo's `NPM_TOKEN` secret. No tags to manage —
same "push auto-builds" standard as the backend image. If the version is unchanged the job builds
and skips publish, so the run stays green.

One-time setup: claim the `@ducktivity` org on npmjs.com, create an **automation** access token,
and add it as the `NPM_TOKEN` repository secret in the identity repo. `publishConfig.access` is
`public`, so the scoped package publishes publicly (scoped packages are private by default).

## Consuming it (from an app frontend)

The package is public, so there is **no `.npmrc` and no token** — install it like any other dep:

```bash
pnpm add @ducktivity/identity-client
```

```ts
import { createClient } from '@ducktivity/identity-client'

const identity = createClient(
  import.meta.env.VITE_IDENTITY_BASE_URL ?? 'http://localhost:8000',
)

const { data, error } = await identity.POST('/v1/auth/verify-code', {
  body: { email, code },
})
// data.token, data.user are fully typed from identity's Go DTOs.
```

## Scripts

| Script                    | What it does                                                                       |
| ------------------------- | ---------------------------------------------------------------------------------- |
| `pnpm run generate-types` | Regenerate `src/schema.ts`.                                                        |
| `pnpm run generate-types` | Regenerate `openapi.json` + `src/schema.ts` from `../shared-schemas/swagger.json`. |
| `pnpm run build`          | Compile `src/` to `dist/` (also runs on install via `prepare`).                    |
| `pnpm run check`          | Type-check without emitting.                                                       |
