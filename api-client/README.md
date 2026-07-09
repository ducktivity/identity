# @ducktivity/identity-client

Generated TypeScript client + types for the Ducktivity identity service (`id.ducktvt.com`) — the suite-wide sole issuer of login codes and session tokens.

## Installation

```bash
pnpm add @ducktivity/identity-client
```

## Usage

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

## Maintenance

#### Source of truth

```
backend/api/dto.go                 (Go DTOs — the SSOT)
  └─ swaggo (backend/scripts/export-swagger.sh) ─▶ shared-schemas/swagger.json  (OpenAPI 2.0)
       └─ swagger2openapi                        ─▶ shared-schemas/openapi.json (OpenAPI 3.0)
            └─ openapi-typescript                ─▶ src/schema.d.ts             (TS types, committed)
                 └─ tsc                          ─▶ dist/                       (published build)
```

To pick up an identity contract change:

```bash
# From api-client/. Regenerates types.
pnpm run generate-types

# Then bump the version and commit; the push to main auto-publishes (see below).
# edit package.json version -> e.g. 0.1.1
git commit -am "chore: bump version to 0.1.1"
```

#### Publishing (automated, public npm)

Pushing to `main` with a changed `version` auto-publishes to the **public npm registry** via `.github/workflows/cd-api-client.yml`, using the repo's `NPM_TOKEN` secret.

To publish manually:

```bash
# From /api-client dir

# Login to npm as Ducktivity's owner or maintainer
npm login
# Publish it as a public package
npm publish --access public
```

#### Scripts

| Script                    | What it does                                                                                           |
| ------------------------- | ------------------------------------------------------------------------------------------------------ |
| `pnpm run generate-types` | Regenerate `../shared-schemas/openapi.json` + `src/schema.d.ts` from `../shared-schemas/swagger.json`. |
| `pnpm run build`          | Compile `src/` to `dist/` (also runs on install via `prepare`).                                        |
| `pnpm run check`          | Type-check without emitting.                                                                           |
