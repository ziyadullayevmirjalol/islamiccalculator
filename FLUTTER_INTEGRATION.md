# Flutter ↔ Backend Integration Guide

For the Flutter developer building the Islamic Calculator mobile app against the Go backend in [backend/](backend/).

**Contract source of truth:** `backend/api/openapi.yaml`, served live with Swagger UI at **`http://localhost:8080/api/v1/docs`** once the backend is running. When this document and the OpenAPI spec disagree, the spec wins.

---

## 1. Running the backend locally

```bash
cd backend
cp .env.example .env      # set POSTGRES_PASSWORD, DB_PASSWORD, JWT_SECRET
make docker-up            # Postgres 16
make migrate-up           # schema + seeded fiqh data
make run                  # API on :8080  (or: docker compose --profile full up -d)
```

Base URL per platform during development:

| Where the app runs | Base URL |
|---|---|
| iOS simulator / desktop / web | `http://localhost:8080` |
| Android emulator | `http://10.0.2.2:8080` |
| Physical device | `http://<your-mac-LAN-IP>:8080` |

Make the base URL a build-time config (`--dart-define=API_BASE_URL=...`), never a hardcoded string.

---

## 2. API conventions (read this before writing any model)

### 2.1 Envelope

Every response is wrapped:

```jsonc
// success — payload always under "data"
{ "data": { ... } }

// failure — always this shape
{ "error": {
    "code": "VALIDATION_FAILED",          // machine-readable, switch on this
    "message": "invalid murabaha request", // English, for logs — do NOT show raw to users
    "fields": { "termMonths": "out_of_range" }  // per-field machine codes, optional
} }
```

| HTTP | `error.code` | App behavior |
|---|---|---|
| 400 | `VALIDATION_FAILED` | Show per-field errors from `fields` (localized, §7) |
| 401 | `UNAUTHORIZED` | Trigger token refresh flow (§3.3); if refresh fails → logout |
| 404 | `NOT_FOUND` | "Not found" state |
| 409 | `CONFLICT` | e.g. email already registered |
| 429 | `RATE_LIMITED` | "Too many requests, try again shortly" + backoff |
| 500 | `INTERNAL` | Generic error + retry button; never show `message` verbatim |

### 2.2 Money is decimal strings — never `double`

All monetary amounts and rates travel as **strings**: `"12500000.50"`, `"0.025"`. The backend computes with exact decimals; parsing them into `double` in Dart would reintroduce the float errors the whole backend was built to avoid.

- Use the [`decimal`](https://pub.dev/packages/decimal) package: `Decimal.parse(json['zakatDue'])`.
- Send user input as strings too: `{"cash": "50000000"}`.
- Format for display with your own formatter (thousands separators, `so'm`), from `Decimal` — not from `double`.
- **Never recompute backend math client-side.** Schedules are engineered so installments sum *exactly* to the total (the final installment absorbs the rounding residual — it may differ from the others by a few tiyin; render it as-is).

### 2.3 Dates

- Request dates: `YYYY-MM-DD` (e.g. `firstDueDate`, `deliveryDate`) — all optional.
- Response timestamps: RFC3339 UTC (`2026-08-21T12:09:50Z`).

### 2.4 Anonymous-first

Every calculator works **without any auth**. Send `Authorization: Bearer <accessToken>` only when the user is signed in — then the calculation is auto-saved to their history. Important: a *presented but invalid/expired* token gets a hard 401 even on calculators (so the app notices dead sessions); simply omit the header for guests.

---

## 3. Auth integration

### 3.1 Endpoints

```
POST /api/v1/auth/register   {"email": "...", "password": "..."}   → 201
POST /api/v1/auth/login      {"email": "...", "password": "..."}   → 200
POST /api/v1/auth/refresh    {"refreshToken": "..."}               → 200
```

All three return the same payload:

```json
{ "data": {
    "user": { "id": "dd64b262-…", "email": "user@example.com" },
    "accessToken": "eyJhbGciOi…",
    "refreshToken": "9f3ab4…",
    "accessExpiresInSeconds": 900
} }
```

Rules the backend enforces (mirror them in form validation for better UX):
- email must be a valid address; password 8–72 chars
- register on an existing email → 409
- login failure is always the same neutral message (no "email exists" leak)

### 3.2 Token storage

Store both tokens in **`flutter_secure_storage`** (Keychain/Keystore). Never in SharedPreferences.

### 3.3 Refresh flow — the refresh token is SINGLE-USE

Access tokens live 15 minutes; refresh tokens 30 days but **die on first use** (rotation). Each successful `/auth/refresh` returns a *new* pair — you must persist the new `refreshToken` immediately. Reusing an old one → 401 → the session is over, log the user out.

Dio interceptor logic:

```
on 401 (and request was not /auth/*):
  1. acquire a single-flight lock (only ONE refresh in flight —
     concurrent 401s must await the same future, or the second
     refresh call will burn the new token and log the user out)
  2. POST /auth/refresh with stored refreshToken
  3. success → persist BOTH new tokens, retry original request once
  4. failure → clear tokens, emit logged-out state
```

### 3.4 History (auth required)

```
GET    /api/v1/history/?limit=20&calcType=finance.murabaha   → {"data":{"entries":[…]}}
DELETE /api/v1/history/{id}                                  → 200 / 404
```

Entry shape: `{id, calcType, inputs, result, createdAt}` — `inputs`/`result` are the exact JSON the calculation used, so a history row can re-open a pre-filled calculator screen and re-render its result **without a network call**. Note: `result` field names inside history are the backend's internal (PascalCase) forms, not the DTO camelCase — treat history `result` as display-opaque or re-run the calculation from `inputs` for a fully interactive re-open.

---

## 4. Endpoint catalog

All calculators are `POST` with a JSON body; reference data is `GET`. Full request/response schemas are in Swagger — this table maps app features to endpoints and flags the response fields with UI obligations.

| App module (from the idea) | Endpoint | UI notes |
|---|---|---|
| Murabaha (nasiya savdo) | `POST /api/v1/finance/murabaha` | markup: `{mode: "rate"\|"amount", value}`; render `schedule[]` with principal/markup split + running balance |
| Ijara Muntahia Bittamleek | `POST /api/v1/finance/ijara` | two modes: `profit` (derive rent) / `rent` (derive profit); `profitTotal` may be **negative** in rent mode — show as loss, it's not an error |
| Qard al-Hasan | `POST /api/v1/finance/qard-hasan` | fee is a fixed amount only — don't build a % input, the API rejects the concept |
| Mudaraba / Wakala deposit | `POST /api/v1/finance/mudaraba` | **`guaranteed: false` must be shown** as "expected, not guaranteed" (kutilayotgan, kafolatlanmagan) |
| Salam | `POST /api/v1/finance/salam` | advance is always the full price; `expectedMargin` can be negative |
| Istisna | `POST /api/v1/finance/istisna` | milestone percents must sum to exactly 100 — validate in the form before submit |
| Musharaka | `POST /api/v1/finance/musharaka` | profit shares must sum to 100; on loss the response `basis: "capital_ratio"` — explain that agreed ratios don't apply to losses |
| Late payment (xayriya jarimasi) | `POST /api/v1/finance/late-payment` | `disposition: "charity"` — label the amount as charity, never as a fee/income |
| Gold/silver/cash zakat | `POST /api/v1/zakat/wealth` | show `nisab.basis` (which metal set the threshold) and `prices.*` provenance |
| Business zakat | `POST /api/v1/zakat/business` | itemized inputs; base floors at zero |
| Ushr | `POST /api/v1/zakat/ushr` | `irrigationType: "natural"` (10%) / `"irrigated"` (5%) |
| Livestock & pilla zakat | `POST /api/v1/zakat/livestock` | species `sheep_goats\|cattle\|camels` + `count`, or `silk_cocoons` + `marketValue`; render `due[]` animal names (tabi, musinna, bint_makhad…) via localized labels; handle `belowNisab: true` and `note` codes |
| Fidya / Kaffarah | `POST /api/v1/zakat/fidya` | kind `fidya\|kaffarah_fast\|kaffarah_oath`; **`needsReview: true` → show "rate pending scholar approval" notice** |
| Tazkiya | `POST /api/v1/zakat/tazkiya` | modes `declared` / `dividend`; purge amount goes to charity |
| Halal stock screener | `POST /api/v1/invest/screener` | render each `checks[]` row (ratio vs threshold, pass/fail) + `purificationRatio` linking to Tazkiya |
| Sukuk portfolio | `POST /api/v1/invest/sukuk` | 1–50 positions; `termMonths` must be a multiple of `12/frequency` — validate in form; show `guaranteed: false` |
| Metal rates widget | `GET /api/v1/rates/metals` | **`stale: true` → warn "price data outdated"**; show `fetchedAt` |
| Livestock rules info screen | `GET /api/v1/reference/livestock-rules` | static tier table for display |

### Worked example — Murabaha

Request:
```json
POST /api/v1/finance/murabaha
{ "cost": "120000000",
  "markup": { "mode": "rate", "value": "0.10" },
  "downPayment": "0",
  "termMonths": 12,
  "firstDueDate": "2026-10-01" }
```

Response:
```json
{ "data": {
    "cost": "120000000.00", "markupTotal": "12000000.00",
    "salePrice": "132000000.00", "downPayment": "0.00",
    "financed": "132000000.00", "monthlyInstallment": "11000000.00",
    "termMonths": 12,
    "schedule": [
      { "n": 1, "dueDate": "2026-10-01", "amount": "11000000.00",
        "principal": "10000000.00", "markup": "1000000.00",
        "balance": "121000000.00" },
      …
    ] } }
```

### Worked example — Wealth zakat

```json
POST /api/v1/zakat/wealth
{ "goldGrams": "100", "cash": "500000", "hawlComplete": true }
```
```json
{ "data": {
    "goldValue": "145000000.00", "silverValue": "0.00",
    "totalWealth": "145500000.00",
    "nisab": { "goldValue": "126846000.00", "silverValue": "10716300.00",
               "applied": "10716300.00", "basis": "silver" },
    "aboveNisab": true, "hawlComplete": true,
    "zakatDue": "3637500.00", "currency": "UZS",
    "prices": {
      "gold":   { "pricePerGram": "1450000.00", "currency": "UZS",
                  "source": "seed", "fetchedAt": "2026-08-21T12:09:50Z",
                  "stale": false },
      "silver": { … } } } }
```

Empty optional amount fields may simply be omitted (they default to zero server-side).

---

## 5. Suggested Flutter architecture

Recommended packages (adapt to your team's standard state management):

| Concern | Package |
|---|---|
| HTTP + interceptors | `dio` |
| Exact decimals | `decimal` |
| Token storage | `flutter_secure_storage` |
| Models | `freezed` + `json_serializable` (hand-written; the API surface is small and stable) |
| State | your choice — `riverpod` or `bloc` both fit |

Layering that mirrors the backend:

```
lib/
├── core/
│   ├── api/            # dio client, auth interceptor (§3.3), envelope unwrapping,
│   │                   # ApiException{code, message, fields}
│   └── money/          # Decimal parsing + UZS formatting, single place
├── features/
│   ├── auth/           # repository + state + screens
│   ├── finance/        # murabaha, ijara, qard, mudaraba, salam, istisna,
│   │                   # musharaka, late-payment  (one folder each)
│   ├── zakat/          # wealth, business, ushr, livestock, fidya, tazkiya
│   ├── invest/         # screener, sukuk
│   ├── rates/          # metals widget
│   └── history/
└── l10n/               # uz (default), ru, en
```

One generic pattern covers every calculator:

```dart
// core/api/envelope.dart
T unwrap<T>(Response r, T Function(Map<String, dynamic>) fromJson) {
  final body = r.data as Map<String, dynamic>;
  if (body.containsKey('error')) throw ApiException.fromJson(body['error']);
  return fromJson(body['data']);
}

// features/finance/murabaha/repository.dart
Future<MurabahaResult> calculate(MurabahaInput input) async =>
    unwrap(await dio.post('/api/v1/finance/murabaha', data: input.toJson()),
           MurabahaResult.fromJson);
```

Optionally generate models from `backend/api/openapi.yaml` with `openapi-generator` — but hand-written freezed models are fine at this size and give better control over `Decimal` converters:

```dart
class DecimalConverter implements JsonConverter<Decimal, String> {
  const DecimalConverter();
  @override Decimal fromJson(String json) => Decimal.parse(json);
  @override String toJson(Decimal d) => d.toString();
}
```

---

## 6. App structure ↔ the four idea domains

Suggested navigation: 4 tabs matching `the idea.txt` audiences, plus history:

1. **Aholi** (retail) — Murabaha, Ijara, Qard al-Hasan, Mudaraba/Wakala
2. **Biznes** (corporate) — Salam, Istisna, Musharaka, Business zakat, Late payment
3. **Zakot** (religious) — Wealth (with live rates header), Ushr, Livestock/Pilla, Fidya/Kaffarah, Tazkiya
4. **Invest** — Screener, Sukuk
5. **Profil** — login/register, history (filterable by `calcType`), settings

Guests see everything; only the history screen prompts to sign in.

---

## 7. Localization

**The backend returns machine codes, never display text.** All uz/ru/en strings live in the Flutter app. You must localize:

- `error.fields` values — the complete set currently in use:
  `required`, `must_be_positive`, `must_not_be_negative`, `must_be_decimal_string`,
  `out_of_range`, `too_large`, `must_be_yyyy_mm_dd`, `must_be_rate_or_amount`,
  `must_be_profit_or_rent`, `must_be_mudaraba_or_wakala`, `exceeds_pool_rate`,
  `must_be_less_than_principal`, `must_be_natural_or_irrigated`, `unknown_species`,
  `must_be_fidya_or_kaffarah`, `must_be_declared_or_dividend`, `exceeds_total_income`,
  `must_be_rate_or_flat`, `percents_must_sum_to_100`, `percent_must_be_positive`,
  `profit_shares_must_sum_to_100`, `need_2_to_20_partners`, `must_be_profit_or_loss`,
  `exceeds_total_revenue`, `must_be_1_2_4_or_12`, `must_align_with_frequency`,
  `need_1_to_50_positions`, `must_be_valid_email`, `must_be_at_least_8_chars`,
  `must_be_less_than_sale_price` (treat unknown codes with a generic "invalid value")
- animal names: `sheep`, `tabi`, `musinna`, `bint_makhad`, `bint_labun`, `hiqqa`, `jadhaa`
- note codes: `computed_by_combination_rule`, `above_120_rules_vary_consult_scholar`,
  `one_more_sheep_per_100_above_400`, `cocoons_zakat_as_saleable_produce`
- screener check keys: `debt_to_market_cap`, `interest_investments_to_market_cap`,
  `impure_income_to_revenue`, plus prohibited-activity slugs the app itself defines
  (e.g. `gambling`, `alcohol`, `conventional_banking` — the backend echoes whatever
  slugs you send)
- calcType labels for history: `finance.murabaha`, `finance.ijara`, `finance.qard_hasan`,
  `finance.mudaraba`, `finance.salam`, `finance.istisna`, `finance.musharaka`,
  `finance.late_payment`, `zakat.wealth`, `zakat.business`, `zakat.ushr`,
  `zakat.livestock`, `zakat.fidya`, `zakat.tazkiya`, `invest.screener`, `invest.sukuk`

---

## 8. Non-negotiable display rules (Shariah-driven)

These come from the backend's design and must survive into the UI:

1. **Never present expected profit as guaranteed.** Wherever `guaranteed: false` appears (mudaraba, sukuk), the screen must say "expected" — this is a religious-correctness requirement, not a legal footnote.
2. **Late-payment amounts are charity**, not fees. Label accordingly (`disposition: "charity"`).
3. **Stale prices must be visible.** `stale: true` on any metal price → banner on nisab/zakat results: "Narxlar eskirgan bo'lishi mumkin".
4. **`needsReview: true`** on fidya → "rate pending scholar approval" notice.
5. **Don't recompute or "fix" schedules client-side** — the residual on the final installment is intentional and exact.
6. Livestock `note: above_120_rules_vary_consult_scholar` → advise consulting a scholar rather than presenting the number as final.

---

## 9. Testing checklist for the client

- Repository tests with mocked dio: envelope unwrapping, `ApiException` mapping, per-field error extraction.
- Auth interceptor test: concurrent 401s produce exactly **one** refresh call; rotation persists the new pair; failed refresh clears the session.
- Golden values to verify formatting end-to-end against a running backend (these match the backend's own test suite):
  - murabaha `cost=120000000, rate=0.10, term=12` → installment `11 000 000.00`, final balance `0.00`
  - zakat wealth `cash=50000000, hawl=true` (seed prices) → due `1 250 000.00`, basis `silver`
  - livestock `cattle, count=150` → `1 tabi + 3 musinna`
  - sukuk `face=100M, price=95M, rate=0.09, freq=2, term=60` → yield `0.0947`
- Decimal round-trip: `"333.68"` parses and re-serializes unchanged (no `333.67999…`).

## 10. First-milestone definition of done

1. dio client + envelope handling + auth interceptor with rotation
2. Murabaha screen end-to-end (form → schedule table) against local backend
3. Wealth zakat screen with rates header + stale banner
4. Login/register + history list wired
5. uz localization for the codes in §7 (ru/en can lag)

After that, the remaining calculators are copies of the same pattern — exactly how the backend itself was built.
