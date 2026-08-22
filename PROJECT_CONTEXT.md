# Project Context — Islamic Calculator

Orientation document for everyone building the mobile app. It explains what this product is, what **must** exist on both sides for the system to be valid, and what the Flutter team is free to decide on its own.

Companion documents:
- [PLAN.md](PLAN.md) — backend architecture and how it was built (phases 0–7, all done)
- [FLUTTER_INTEGRATION.md](FLUTTER_INTEGRATION.md) — the technical how-to: endpoints, examples, code patterns
- `backend/api/openapi.yaml` — the API contract, served live at `http://localhost:8080/api/v1/docs`

---

## 1. What this product is

A mobile Islamic finance calculator for the Uzbek market (source concept: `the idea.txt`, in Uzbek). It serves four audiences with sixteen calculators:

| Audience | Calculators |
|---|---|
| **Individuals** (chakana moliya) | Murabaha installments, Ijara lease-to-own, Qard al-Hasan, Mudaraba/Wakala deposits |
| **Businesses** (korporativ) | Salam advance purchase, Istisna staged payments, Musharaka profit/loss split, business zakat, late-payment charity |
| **Believers** (diniy majburiyatlar) | Gold/silver/cash zakat with live nisab, ushr, livestock & silk-cocoon zakat, fidya/kaffarah, tazkiya (income purification) |
| **Investors** | AAOIFI halal stock screener, sukuk portfolio |

The product's core promise is **Shariah correctness**: every number is computed with exact decimal math, every fiqh parameter (nisab grams, rates, livestock tiers, AAOIFI thresholds) is data reviewed by scholars — not code — and the app never misrepresents an expectation as a guarantee. Defaults follow the **Hanafi** school (the local norm); the seeded fidya rate is flagged `needs_review` until a mufti confirms it.

## 2. System state today

- **Backend: complete and running.** Go monolith, REST under `/api/v1`, PostgreSQL. 22 endpoints (16 calculators + auth + history + rates + reference + docs). Full test suite, Docker image verified, CI green. See PLAN.md status block.
- **Flutter app: not started.** This is the remaining build. The API contract is stable and browsable in Swagger.
- Division of responsibility, by design:
  - **Backend owns all calculation logic and all fiqh data.** The app never computes zakat, schedules, or verdicts itself.
  - **App owns all presentation, all languages, and all UX.** The backend returns machine codes only — no display text.

---

## 3. The shared contract — MUST exist on both sides

These are the invariants the system's validity depends on. The backend half already exists; the app half is a hard requirement, not a preference. If any row is broken on either side, the product is wrong even if it "works".

| # | Invariant | Backend already does | The app MUST |
|---|---|---|---|
| C1 | **One envelope** | Every response is `{"data": …}` or `{"error": {code, message, fields}}` | Unwrap centrally; branch on `error.code`, never on raw strings or bare HTTP status alone |
| C2 | **Exact money** | All amounts/rates are decimal **strings**; schedules sum exactly (final installment absorbs residual) | Parse with a Decimal type — `double` is forbidden for money; send inputs as strings; display schedules as received, never recompute or "fix" them |
| C3 | **Anonymous-first** | Every calculator works with no auth; a *presented* invalid token = hard 401 | Full calculator functionality for guests; attach `Authorization` only when signed in; treat 401 as "session ended", not "feature broken" |
| C4 | **Single-use refresh tokens** | `/auth/refresh` rotates: old token dies on first use | Persist the *new* pair on every refresh; single-flight the refresh call (concurrent 401s → one refresh); failed refresh = clean logout |
| C5 | **Machine codes, localized client-side** | Field errors, animal names, note codes, calcTypes are stable slugs (full list in FLUTTER_INTEGRATION.md §7) | Ship uz (minimum) translations for every code; render unknown codes with a generic fallback, never crash or show the raw slug |
| C6 | **Expectation ≠ guarantee** | Mudaraba & sukuk responses carry hard-wired `guaranteed: false` | Display "expected / kutilayotgan, kafolatlanmagan" wherever that flag appears — this is a religious-correctness requirement |
| C7 | **Late fees are charity** | `disposition: "charity"` on late-payment and tazkiya outputs | Label these amounts as charity, never as fee, penalty income, or discount |
| C8 | **Staleness is visible** | Metal prices carry `source`, `fetchedAt`, `stale: true` past 48h | Show a warning on nisab/zakat screens when `stale` is true; show price provenance |
| C9 | **Pending scholar review is visible** | Fidya responses carry `needsReview: true` until values are approved | Show a "rate pending approval" notice when set |
| C10 | **Backend is the validation authority** | Rejects invalid Shariah structures with typed field errors (percents ≠ 100, loss ≠ capital split, % fee on qard, term/frequency misalignment…) | Mirror the *known* rules in form validation for good UX, but always handle a 400 gracefully — the backend's answer wins, and new rules may appear |
| C11 | **Versioned base path** | Everything lives under `/api/v1`; breaking changes would come as `/api/v2` | Hardcode `/api/v1` in one place only; base host from build config (`--dart-define`), never inline |
| C12 | **History round-trip** | Saved entries return the original `inputs` verbatim | "Re-open" a history entry by pre-filling the calculator form from `inputs` and re-running — don't try to render the stored `result` as if it were a DTO (its field names are internal) |
| C13 | **Secrets stay put** | JWT secret, DB credentials in server env only | Tokens in secure storage (Keychain/Keystore) only; no secrets, keys, or tokens in code, logs, or analytics |

**Contract-change rule:** if the app needs something the API doesn't provide, the change is made *contract-first* — edit `backend/api/openapi.yaml` together, then implement backend and app against it. Neither side improvises fields the other doesn't know about.

---

## 4. What the backend guarantees to the app team

So you can build without defensive paranoia:

- Endpoint paths, request/response field names, and error codes under `/api/v1` are **stable**; additive changes only (new optional fields, new endpoints).
- Fiqh values can change **without notice or redeploy** (they're DB rows) — never cache a nisab value or fidya rate beyond a session; amounts on screen should come from a fresh calculation.
- Rate limit is 120 requests/min per IP, body limit 64KB — no realistic app flow hits either; a 429 still deserves a graceful message.
- The staging/production base URL will be provided at deploy time; everything else is identical to local.

## 5. What the Flutter team decides freely

The backend imposes **nothing** here. Decide, record the decision in this file, and build:

| Area | Your call (examples) |
|---|---|
| State management | riverpod / bloc / whatever the team is fluent in |
| Navigation & structure | the 4-tab layout suggested in FLUTTER_INTEGRATION.md §6 is a suggestion, not a contract |
| Design system | colors, typography, components, dark mode; no brand exists yet — creating it is part of this work |
| Language strategy | uz-first is required; whether ru/en ship at v1 is yours |
| Offline behavior | pure-online is acceptable for v1; any offline caching must respect C2 (no recomputation) and the fiqh-freshness note in §4 |
| Model generation | hand-written freezed models vs. openapi-generator |
| Extra UX features | hijri date display, zakat reminders, result sharing/PDF, onboarding — all welcome; anything needing new API data goes through the contract-change rule |
| Min OS versions, device support, tablets | yours |
| App analytics/crash reporting | yours — but see C13: no tokens or personal financial inputs in analytics events |

**Open decisions to record here once made:**

- [ ] State management: ______
- [ ] Design direction / design system: ______
- [ ] v1 languages: uz + ______
- [ ] Offline strategy: ______
- [ ] Model approach (freezed by hand / generated): ______
- [ ] Analytics/crash tool: ______

---

## 6. Definition of a valid v1 (both sides together)

The system is releasable when:

1. Every calculator screen produces results **byte-identical in value** to a direct API call (spot-check against the golden values in FLUTTER_INTEGRATION.md §9).
2. All contract invariants C1–C13 demonstrably hold (each is testable; C4 and C2 deserve dedicated tests).
3. A guest can use all 16 calculators; a signed-in user's calculations appear in history and survive re-open.
4. Uzbek localization covers every machine code the backend can emit.
5. The four Shariah display rules (C6–C9) are visible in the running app and have been reviewed by the same scholar who signs off the seeded fiqh values.
6. Backend deployed with a real `METALS_API_KEY` (live nisab) and scholar-approved settings (no `needs_review` flags left).

Items 5–6 are the only ones that need people outside the dev team — line them up early.
