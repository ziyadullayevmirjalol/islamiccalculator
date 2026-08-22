# Islamic Calculator — Backend

Go monolith serving a REST API of Shariah-compliant financial calculators
to the Flutter client. PostgreSQL holds reference data (fiqh parameters,
livestock zakat tiers, AAOIFI screener thresholds), market prices, and
user history. See [PLAN.md](../PLAN.md) for the full architecture and
build plan.

## Quick start

```bash
cp .env.example .env      # fill in POSTGRES_PASSWORD, DB_PASSWORD, JWT_SECRET
make docker-up            # start Postgres 16 (host port from POSTGRES_HOST_PORT)
make migrate-up           # apply migrations + seeds
make run                  # start the API on :8080
curl localhost:8080/readyz
```

Interactive API docs: **http://localhost:8080/api/v1/docs** (spec at
`/api/v1/docs/openapi.yaml`, source in `api/openapi.yaml`).

## Make targets

| Target | Does |
|---|---|
| `make run` / `make build` | run / build the server |
| `make test` | `go test ./... -race -cover` |
| `make lint` | golangci-lint (falls back to `go vet`) |
| `make docker-up` / `docker-down` | start/stop Postgres |
| `make migrate-up` / `migrate-down` | apply / roll back one migration |
| `make migrate-new name=x` | create a numbered migration pair |

## Layout

```
cmd/server        entrypoint; cmd/migrate: migration runner
internal/domain   pure calculators, one package per contract — all money is decimal
internal/service  workflows: validate → load reference data → calculate → save history
internal/handler  HTTP layer: DTOs (amounts as strings), validation, error envelope
internal/repository/postgres  pgx implementations
internal/provider/metals      live spot-price client (metals.dev)
migrations        SQL migrations + seeds (fiqh parameters are data, not code)
```

## Configuration

Everything comes from the environment (`.env` is auto-loaded locally,
gitignored). Key settings:

- `DB_*` / `POSTGRES_*` — database credentials (compose reads `POSTGRES_*`)
- `JWT_SECRET` — required; access tokens 15m, refresh 30d single-use
- `RATE_LIMIT_PER_MIN`, `HTTP_MAX_BODY_BYTES` — hardening knobs
- `METALS_API_KEY` — enables the 6h live gold/silver refresh; when empty,
  stored prices are served with `"stale": true` flags past 48h

## Deployment

`docker compose --profile full up -d --build` runs the whole stack: the
multi-stage Dockerfile produces a distroless static binary. For a managed
platform (Fly.io/Railway), deploy the Dockerfile, point `DB_*` at the
managed Postgres, run `go run ./cmd/migrate` as a release step, and set
the `.env` values as platform secrets. Backups: `pg_dump` on a schedule —
`app_settings`, rule tables, and `calculations` are the state that matters.

## Notes for reviewers (fiqh)

Defaults are Hanafi and live in the database: nisab 87.48 g gold /
612.36 g silver, zakat 2.5%, ushr 10%/5%, livestock tiers in
`livestock_zakat_rules`, AAOIFI thresholds in `screener_rules`, fidya in
`app_settings` (flagged `needs_review` until approved). Changing any of
them is a data update, not a deploy.
