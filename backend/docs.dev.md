# Backend Development Documentation

### Start up local dev (live reload)

```bash
go tool air
```

- Rebuilds and restarts on save (config in [.air.toml](.air.toml), build output in `tmp/`).
- Reads `backend/.env` for local config. At minimum set `DATABASE_URL`; leave
  `AUTH_SIGNING_KEY` and `RESEND_API_KEY` empty in dev (an ephemeral signing key is
  generated and login codes are logged instead of emailed).

### Run all backend checks

```bash
./scripts/check.sh
```

- Runs `go fmt`, `go build`, `go vet`, `staticcheck`, and `gosec` — the same gates CI enforces.

### Create a new DB schema migration

```bash
./scripts/db-new-migration.sh add_some_column
```

- Use-case: When you need a new DB schema change file.
- Usage: Replace `add_some_column` with your migration name.
- Output: [sql/schema/](sql/schema/)

Migrations are **expand-only** — never write a destructive `down`; roll back by
reverting the image, not the schema.

### Run DB migration

```bash
./scripts/db-migrate.sh up
```

- Use-case: When you need to apply, rollback, or inspect migrations.
- Usage: Replace `up` with `down`, `status`, or another Goose command.

Make sure `DATABASE_URL` is in `backend/.env` (identity's tables live in the
`identity` schema — the connection string carries `options=-c search_path=identity`).

### Generate type-safe SQL queries and models

```bash
./scripts/db-codegen.sh
```

or

```bash
go tool sqlc generate
```

- Use-case: After a migration or writing anything new into [sql/](sql/).
- Output: [database/dbgen/](database/dbgen/) — do not edit by hand.
