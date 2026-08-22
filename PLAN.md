# Islamic Calculator — Backend Implementation Plan

**Stack:** Go monolith · REST API (`/api/v1`) · PostgreSQL 16 · Flutter client (separate track)
**Date:** 2026-08-21 · **Source:** `the idea.txt`

> **Status (2026-08-21): all phases 0–7 implemented.** All 16 calculators live
> (4 retail + 4 corporate + 6 religious + 2 investment), JWT auth + history,
> rate limiting + body limits, live metal-price refresher with staleness
> flags, docs served at `/api/v1/docs`, deployable Docker image verified.
> Remaining before launch: pick a metals API key, scholar review of seeded
> fiqh values (`needs_review` flags), and the Flutter client. See
> [backend/README.md](backend/README.md).

---

## 1. What the idea describes

The concept document (in Uzbek) specifies a mobile Islamic finance calculator organized into four audiences — 16 calculators total. Every feature is fundamentally a **deterministic calculation** (inputs in → schedule or verdict out), which shapes the whole architecture: the core is a library of pure functions, and the server around it is thin.

| Domain | Calculator | Essence |
|---|---|---|
| **Retail finance** (individuals) | Murabaha | Cost-plus installment sale (car, appliances, housing). Fixed sale price, equal installments, amortization schedule. |
| | Ijara Muntahia Bittamleek | Lease-to-own: rental schedule + ownership-transfer payment at term end. |
| | Qard al-Hasan | Benevolent loan: zero markup, fixed service fee only, repayment schedule. |
| | Mudaraba / Wakala deposit | Expected (non-guaranteed) profit on savings from pool rate × profit-sharing ratio. |
| **Corporate finance** (businesses) | Salam | Advance-payment purchase of future harvest: contracted price vs. expected delivery value. |
| | Istisna | Construction/manufacturing: milestone-based staged payment schedule. |
| | Musharaka | Partnership: profit split by agreed ratio, loss strictly by capital share. |
| | Business zakat | 2.5% on net working capital (cash + receivables + inventory − short-term debt). |
| | Late-payment charity | Penalty on overdue installments, routed 100% to charity — never booked as income. |
| **Religious obligations** | Gold / silver / cash zakat | Nisab from live metal spot price; 2.5% when holdings reach nisab. |
| | Ushr (harvest zakat) | 10% naturally watered, 5% irrigated crops. |
| | Livestock & silk-cocoon zakat | Rule-table lookup by head count (sheep, cattle, camels); cocoons as produce. |
| | Sadaqa / Fidya / Kaffarah | Missed fasts × fidya rate; kaffarah amounts. Rates are configurable reference data. |
| | Tazkiya (purification) | Identify accidental interest/doubtful income and compute the amount to donate. |
| **Investment** (investors) | Halal stock screener | AAOIFI compliance: business activity + financial ratio thresholds + purification ratio. |
| | Sukuk | Islamic bond portfolio: expected distribution schedule and yield. |

Two calculators need **external market data**: wealth zakat (gold/silver spot prices for nisab) and the stock screener (company financials). Everything else is self-contained math plus reference tables.

---

## 2. Architecture

A single deployable Go binary, layered strictly one direction. The domain layer holds all Shariah math as pure functions with no I/O — this is what makes the requested workflow unit tests trivial and fast.

```
Transport   internal/handler      chi router, JSON DTOs, validation, error envelope. No business logic.
    ↓
Service     internal/service      Workflows: validate → fetch reference data (rates, rules)
    ↓                             → call domain → persist history.
Domain      internal/domain       Pure calculators, one package per contract type.
    ↓                             Zero imports of DB or HTTP. All money is decimal.
Repository  internal/repository/  pgx implementations: rates, reference rules, history, users.
            postgres              (interfaces defined in service, implemented here)
```

### Decisions locked in

- **Monolith, modular by domain package** — `domain/murabaha`, `domain/zakat`, `domain/screener`… Clean seams if it ever needs splitting, no distributed complexity now.
- **Never `float64` for money.** `shopspring/decimal` in Go, `NUMERIC(24,4)` in Postgres, amounts as *strings* in JSON (`"12500000.50"`) so precision survives the Flutter boundary.
- **Calculators are anonymous-first.** Every calculation endpoint works without an account; auth (phase 6) only unlocks saved history.
- **Fiqh parameters are configuration, not constants.** Nisab grams, ushr percentages, fidya rate, screener thresholds live in DB reference tables (Hanafi defaults — the primary audience is Uzbekistan) so a mufti-approved update never needs a redeploy.
- **External data behind interfaces.** `MetalPriceProvider` and `StockFinancialsProvider` are interfaces; phase 1 ships a manual/DB-seeded implementation, a live API adapter comes later. Tests use fakes.

---

## 3. Tech stack

| Concern | Choice | Why |
|---|---|---|
| Language | Go 1.23+ | Single static binary, stdlib HTTP, first-class table-driven testing. |
| Router | `go-chi/chi/v5` | Idiomatic, stdlib-compatible, middleware ecosystem, no framework lock-in. |
| DB driver | `jackc/pgx/v5` | Native Postgres protocol, pool built in, first-class NUMERIC scanning. |
| Migrations | `golang-migrate` | Plain SQL up/down files, CLI + library, CI-friendly. |
| Decimal math | `shopspring/decimal` | Exact decimal arithmetic for all monetary and rate values. |
| Config | `ilyakaznacheev/cleanenv` | Env-var config with struct tags and defaults; 12-factor. |
| Logging | `log/slog` | Structured logging from stdlib; no dependency. |
| Testing | `stretchr/testify` | Assertions + suites on top of stdlib table tests. |
| Lint | `golangci-lint` | One binary, runs in CI. |
| Local env | Docker Compose | Postgres 16 + app, one command up. |
| API docs | `api/openapi.yaml` | Hand-maintained OpenAPI 3 spec — also the contract for the Flutter team. |

---

## 4. Repository layout

```
islamiccalculator/
├── backend/
│   ├── cmd/server/main.go            # wire config → db → services → router; graceful shutdown
│   ├── internal/
│   │   ├── config/config.go          # env config struct
│   │   ├── server/                   # router setup, middleware (logging, recover, CORS, request ID)
│   │   ├── handler/                  # one file per calculator group + DTOs + validation
│   │   ├── service/                  # workflows; defines repository/provider interfaces
│   │   ├── domain/
│   │   │   ├── money/                # Money type over decimal, rounding policy, currency
│   │   │   ├── murabaha/             # Calculate(Input) (Schedule, error) + tests
│   │   │   ├── ijara/
│   │   │   ├── qardhasan/
│   │   │   ├── mudaraba/
│   │   │   ├── salam/
│   │   │   ├── istisna/
│   │   │   ├── musharaka/
│   │   │   ├── zakat/                # wealth, business, ushr, livestock, fidya, tazkiya
│   │   │   ├── screener/             # AAOIFI rules engine
│   │   │   └── sukuk/
│   │   ├── repository/postgres/      # pgx implementations
│   │   └── pkg/apperr/               # typed app errors → HTTP codes
│   ├── migrations/                   # 0001_init.up.sql / .down.sql …
│   ├── api/openapi.yaml
│   ├── docker-compose.yml
│   ├── Dockerfile
│   ├── Makefile
│   ├── .golangci.yml
│   └── go.mod
└── mobile/                           # Flutter app (separate track)
```

---

## 5. REST API surface

**Conventions.** Base path `/api/v1`. Calculations are `POST` (rich input body), reference data is `GET`. Amounts are decimal strings with an ISO currency code (`"UZS"` default). Responses return both the result and an itemized breakdown so Flutter can render schedules without re-deriving math.

| Endpoint | Purpose |
|---|---|
| `GET /healthz` · `/readyz` | Liveness / readiness (readiness pings Postgres). |
| `POST /api/v1/finance/murabaha` | Installment schedule from cost, markup, down payment, term. |
| `POST /api/v1/finance/ijara` | Rent schedule + transfer price; total cost of ownership. |
| `POST /api/v1/finance/qard-hasan` | Repayment schedule, principal + fixed service fee. |
| `POST /api/v1/finance/mudaraba` | Expected depositor profit from pool rate × sharing ratio (Wakala via fee mode). |
| `POST /api/v1/finance/salam` | Advance price vs. expected delivery value, buyer margin. |
| `POST /api/v1/finance/istisna` | Milestone payment schedule. |
| `POST /api/v1/finance/musharaka` | Profit/loss distribution across partners. |
| `POST /api/v1/finance/late-payment` | Accrued charity amount on overdue installments. |
| `POST /api/v1/zakat/wealth` | Gold/silver/cash zakat with live-nisab check. |
| `POST /api/v1/zakat/business` | 2.5% on net zakatable working capital. |
| `POST /api/v1/zakat/ushr` | Harvest zakat, 10% / 5% by irrigation type. |
| `POST /api/v1/zakat/livestock` | Due animals by species and head count (rule-table lookup). |
| `POST /api/v1/zakat/fidya` | Fidya / kaffarah amounts for missed obligations. |
| `POST /api/v1/zakat/tazkiya` | Purification amount from doubtful/interest income. |
| `POST /api/v1/invest/screener` | AAOIFI verdict per company + failed rules + purification ratio. |
| `POST /api/v1/invest/sukuk` | Distribution schedule and expected yield for a sukuk position/portfolio. |
| `GET /api/v1/rates/metals` | Current gold/silver price per gram + nisab values + fetched-at. |
| `GET /api/v1/reference/livestock-rules` | The seeded zakat rule table (for client display). |
| `POST /api/v1/auth/register` · `/login` · `/refresh` | Phase 6 — JWT auth. |
| `GET/POST/DELETE /api/v1/history` | Phase 6 — saved calculations for signed-in users. |

### Envelope

```jsonc
// success
{ "data": { "totalPrice": "156000000.00", "monthlyInstallment": "6500000.00",
            "schedule": [ { "n": 1, "due": "2026-10-01", "amount": "6500000.00",
                            "principal": "5416666.67", "markup": "1083333.33",
                            "balance": "149500000.00" } ] } }

// error — code is machine-readable, message is client-displayable
{ "error": { "code": "VALIDATION_FAILED", "message": "termMonths must be between 1 and 360",
             "fields": { "termMonths": "out_of_range" } } }
```

---

## 6. Database schema

The DB stores reference data, market data, and user history — never intermediate calculation state. Initial migration set:

| Table | Columns (essentials) | Role |
|---|---|---|
| `app_settings` | `key PK, value JSONB, updated_at` | Fiqh parameters: nisab grams (gold 87.48 / silver 612.36 — Hanafi defaults), ushr rates, zakat rate, fidya rate per day. |
| `metal_prices` | `id, metal, price_per_gram NUMERIC(24,4), currency, source, fetched_at` | Spot price cache; latest row per metal drives nisab. |
| `livestock_zakat_rules` | `id, species, min_count, max_count NULL, due_species, due_count, due_age_class, per_extra NULL` | Seeded full Hanafi tables for sheep/goats, cattle, camels. |
| `screener_rules` | `key PK, threshold NUMERIC(6,4), description` | AAOIFI thresholds: debt/mcap 0.30, interest-bearing securities/mcap 0.30, impure income 0.05. |
| `calculations` | `id UUID, user_id NULL FK, calc_type, inputs JSONB, result JSONB, created_at` | History. JSONB keeps schema stable as calculators evolve; `calc_type` indexed. |
| `users` | `id UUID, email UNIQUE, password_hash, created_at` | Phase 6. |
| `refresh_tokens` | `id, user_id FK, token_hash, expires_at` | Phase 6. |

---

## 7. Calculation specifications

Each domain package exposes `Calculate(Input) (Result, error)`. The Shariah-critical rules below are enforced as validation errors inside the domain, so an invalid contract can never produce a number:

- **Murabaha** — sale price `S = cost + markup` is fixed at contract; installment = `(S − down) / n`. No compounding, no recalculation on late payment (that's the late-payment-charity calculator's job).
- **Ijara MB** — rent covers cost recovery + profit over the term; transfer price is a separate promised sale. Output: rent schedule, transfer payment, total outlay vs. asset cost.
- **Qard al-Hasan** — service fee must be a *fixed amount*, never a percentage of principal (a percentage is riba by definition; the domain rejects it).
- **Mudaraba** — depositor profit = amount × pool rate × depositor ratio × term. Output labeled *expected*, never guaranteed. Wakala mode: pool rate − agency fee.
- **Musharaka** — profit by agreed ratio; **loss strictly proportional to capital** — the domain rejects any loss allocation that deviates from capital shares.
- **Salam** — full advance payment required at contract; margin = expected market value at delivery − advance paid.
- **Istisna** — milestone percentages must sum to 100%; schedule maps stages to amounts and dates.
- **Wealth zakat** — nisab = min(gold nisab value, silver nisab value) from latest spot; due = 2.5% of total zakatable wealth when ≥ nisab and hawl (lunar year) confirmed by the user.
- **Business zakat** — base = cash + receivables + inventory at market − short-term liabilities; same nisab gate, 2.5%.
- **Ushr** — 10% rain/natural water, 5% irrigated; applied to harvest value or quantity.
- **Livestock** — pure rule-table lookup; below minimum head count → zero due. Silk cocoons: treated as saleable produce, 2.5% of market value (configurable).
- **Fidya / Kaffarah** — days × configured daily rate; kaffarah = 60 × daily rate per broken oath/fast per the selected type.
- **Tazkiya** — purge = declared impure income, or for dividends: dividend × (company impure income ratio). Output: donate amount + clean remainder.
- **Screener** — activity blacklist first, then the three ratio rules from `screener_rules`; verdict + per-rule pass/fail + purification ratio for holders.
- **Sukuk** — expected coupon schedule from face × distribution rate / frequency; current yield = annual distribution / purchase price; portfolio = aggregation across positions.

> **Rounding policy** — one policy, in `domain/money`, used everywhere: round half-up to 2 decimals at *presentation boundaries* only; keep 4+ decimals internally; the final installment absorbs the residual so schedules always sum exactly to the total. This is the kind of invariant the workflow tests assert on every calculator.

---

## 8. Testing strategy

The requested workflow tests are table-driven unit tests at two levels, all runnable with `go test ./... -race` and no database:

1. **Domain tests** — golden-value tables per calculator: known inputs → exact expected schedules/verdicts, plus invariants (schedule sums to total; loss split matches capital; nisab boundary cases; livestock tier edges like 39 vs. 40 sheep).
2. **Service workflow tests** — full workflow with faked repositories/providers: request → validation → reference-data fetch → domain call → persisted history record → response DTO. One happy path + failure paths per calculator.
3. **Handler tests** — `net/http/httptest` against the real router: JSON in → status + envelope out, including validation errors.

```go
func TestMurabaha_Calculate(t *testing.T) {
    cases := []struct {
        name string
        in   murabaha.Input
        want murabaha.Result
        err  error
    }{
        {name: "12-month car, 10% markup, no down payment",
         in:   murabaha.Input{Cost: d("120000000"), MarkupRate: d("0.10"), TermMonths: 12},
         want: murabaha.Result{TotalPrice: d("132000000"), Installment: d("11000000")}},
        {name: "zero term rejected",
         in:   murabaha.Input{Cost: d("1000"), TermMonths: 0},
         err:  murabaha.ErrInvalidTerm},
        // … residual-rounding case, down-payment case, markup-as-amount case
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) { /* assert result + schedule-sum invariant */ })
    }
}
```

Optional phase-7 addition: repository integration tests via `testcontainers-go` against a throwaway Postgres. CI gate: lint + tests green, coverage reported.

---

## 9. Build phases

### Phase 0 — Foundation
- `go mod init`, directory skeleton from §4, `.golangci.yml`.
- Config loader (env), `slog` JSON logger, typed error package.
- chi server with middleware (request ID, logging, recover, CORS), graceful shutdown, `/healthz` + `/readyz`.
- Docker Compose (Postgres 16 + app), multi-stage Dockerfile, Makefile (`run · test · lint · migrate-up · migrate-down · docker-up`).
- Migration 0001: `app_settings`, `metal_prices`, `calculations` + seeds.
- GitHub Actions: lint + test on push.

**Done when** — `make docker-up` then `curl :8080/readyz` returns 200 with DB ping; CI green on the empty test suite.

### Phase 1 — Money core + first vertical slice
- `domain/money`: Money type, rounding policy, schedule-residual helper — fully tested first, everything depends on it.
- End-to-end slice for **Murabaha** and **wealth zakat**: domain → service → handler → route, with all three test levels.
- `MetalPriceProvider` interface + DB-seeded implementation + `GET /rates/metals`.

**Done when** — Flutter can call two real endpoints; the slice is the copy-paste template for every remaining calculator.

### Phase 2 — Retail finance set
- Ijara MB, Qard al-Hasan, Mudaraba/Wakala — same slice pattern, golden-value tests each.

**Done when** — all four retail endpoints pass workflow tests; OpenAPI spec updated.

### Phase 3 — Zakat & religious set
- Business zakat, ushr, fidya/kaffarah, tazkiya, late-payment charity.
- Livestock: migration seeding full Hanafi rule tables + lookup domain + `GET /reference/livestock-rules`.

**Done when** — tier-boundary tests pass (39/40/121/201 sheep, cattle 29/30, camel 4/5…); rates provably come from `app_settings`, not constants.

### Phase 4 — Corporate finance set
- Salam, Istisna (milestones sum to 100% validation), Musharaka (loss-by-capital enforcement).

**Done when** — invalid Shariah structures are rejected with typed errors, covered by tests.

### Phase 5 — Investment set
- Screener rules engine reading thresholds from `screener_rules`; manual-financials input mode first, `StockFinancialsProvider` adapter later.
- Sukuk position + portfolio math.

**Done when** — screener returns verdict + failed rules + purification ratio for hand-entered financials.

### Phase 6 — Accounts & history
- JWT auth (bcrypt, access + refresh), `users`/`refresh_tokens` migrations, history endpoints, per-user rate limit.

**Done when** — anonymous calculators still work unchanged; signed-in history round-trips.

### Phase 7 — Hardening & launch
- Live metal-price adapter with cron refresh + stale-price guard.
- OpenAPI served at `/api/v1/docs`; global rate limiting; request size limits.
- Deploy: single VM or Fly.io/Railway, Postgres backups, structured logs shipped.
- Optional: testcontainers integration suite.

**Done when** — staging URL live, Flutter team building against the OpenAPI contract.

---

## 10. Bootstrap commands (Phase 0, step by step)

```bash
mkdir -p backend && cd backend
go mod init github.com/<you>/islamiccalculator

go get github.com/go-chi/chi/v5 \
       github.com/jackc/pgx/v5 \
       github.com/shopspring/decimal \
       github.com/ilyakaznacheev/cleanenv \
       github.com/stretchr/testify

brew install golang-migrate golangci-lint          # macOS dev machine
migrate create -ext sql -dir migrations -seq init  # 0001_init.up.sql / .down.sql

# docker-compose.yml: postgres:16-alpine with healthcheck + app service
# Makefile targets: run · test · lint · migrate-up · migrate-down · docker-up
make docker-up && make migrate-up
curl localhost:8080/readyz                          # → 200 {"status":"ok"}
```

---

## 11. Open questions & risks

- **Metal price source** — pick a provider (metals.dev, GoldAPI, or central-bank feed) by phase 7; until then admin-seeded prices with a visible `fetched_at` so stale data is never silent.
- **Stock financials** — no free reliable source for Uzbek/global fundamentals; manual-input mode is the launch feature, provider integration is post-launch.
- **Fiqh sign-off** — default parameters are Hanafi; every rate/threshold sits in reference tables so a scholar review changes data, not code. Get the seed values reviewed before launch.
- **Currency** — UZS-first with a currency field carried through; no FX conversion in v1.
- **i18n** — backend returns stable machine codes; Uzbek/Russian/English strings live in Flutter.
