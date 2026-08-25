# Flutter Development Plan — Task Breakdown

Written 2026-08-25 against the **current production backend** (27 API paths, 18 calculators, uz/ru/en dictionaries, percent-input convention). This is the build checklist; the wire-level details live in [FLUTTER_INTEGRATION.md](FLUTTER_INTEGRATION.md), the invariants in [PROJECT_CONTEXT.md](PROJECT_CONTEXT.md) (C1–C13), and the live contract at `https://islamiccalculator-api.vercel.app/api/v1/docs/openapi.json`.

**The web app (`web/`) is the reference implementation.** Every rule below is already working there — when in doubt, mirror it.

---

## Phase F0 — Project foundation

- [ ] `flutter create mobile` in the repo; min SDK: iOS 13 / Android 7 (or team choice, record in PROJECT_CONTEXT §5).
- [ ] Packages: `dio`, `decimal`, `flutter_secure_storage`, `freezed` + `json_serializable`, state mgmt of the team's choice.
- [ ] Config: `API_BASE_URL` via `--dart-define`; prod default `https://islamiccalculator-api.vercel.app`; dev `http://10.0.2.2:8080` (Android emu) / `http://localhost:8080` (iOS sim). No URL literals anywhere else.
- [ ] `ApiClient`: envelope unwrap (`data`/`error`), `ApiException{status, code, message, fields}` (C1).
- [ ] Auth interceptor: attach bearer when a session exists; on 401 (non-`/auth/*`) do a **single-flight** refresh, persist the NEW pair, retry once; failed refresh → clear session (C4). Tokens only in `flutter_secure_storage` (C13).
- [ ] `DecimalConverter` (`String ⇄ Decimal`) for freezed models — `double` banned for money project-wide, add a lint/danger rule (C2).
- [ ] **i18n: port the dictionaries** from [web/js/i18n.js](web/js/i18n.js) (≈340 uz + ≈340 ru entries, English fallback = key itself). Same key convention: English source string → translation. One-time script or manual port; keep keys byte-identical so future strings copy across.
- [ ] **Language setting**: in-app picker (O'zbekcha default / Русский / English), persisted (`shared_preferences` fine — not a secret), applied app-wide without restart if the state layer allows.

## Phase F1 — Core widget kit (everything else is assembled from these)

- [ ] `MoneyField` — text input, `inputmode` decimal, value stays a **String**; formatter shows space-grouped preview (`8 000 000`) but the model keeps the raw string.
- [ ] `PercentField` — **the unit-trap fix, non-negotiable**: user types human percent (`20` = 20%), a `%` suffix is rendered inside the field, and the value is converted to the API's fraction form with a **string decimal shift, never float division**. Port `pct()` from [web/js/calculators.js](web/js/calculators.js) verbatim logic: `"20"→"0.2"`, `"2.5"→"0.025"`, `"0.5"→"0.005"`, `"100"→"1"`. Unit-test all those cases.
- [ ] `IntField`, `DateField` (emits `YYYY-MM-DD`), `SelectField`, `CheckboxField`, `MultiCheckField`.
- [ ] `DynamicListEditor` — add/remove rows with typed columns (used by Istisna milestones, Musharaka partners, Sukuk positions).
- [ ] Result kit: `ResultRow` (label/value, emphasis + brass/negative tones), `ScheduleTable` (horizontally scrollable, `tabular-nums`), `VerdictBadge`, `NoticeCard` (info/warn/error).
- [ ] `fmtMoney(String)` — space grouping via string ops (port from web/js/ui.js), `fmtPercent(String)`.
- [ ] Error plumbing: `ApiException.fields` → inline errors under matching field names (match full key, then tail after `.`/`[n]`); unmatched codes → banner. Localize codes via the dictionary; unknown code → `code.replaceAll('_',' ')`.

## Phase F2 — Vertical slice (proves the whole stack)

- [ ] Murabaha screen end-to-end (spec below) + golden test: `cost 8000000, 20%` typed, term 12 → payload `{"markup":{"mode":"rate","value":"0.2"}}` → render `800 000.00`/month. This is the permanent percent-regression case.
- [ ] Wealth-zakat screen with the rates header (`GET /rates/metals`) and stale banner.
- [ ] Home: 4 groups × calculator cards (mirror web groups), rates widget, language picker in the app bar.

## Phases F3–F6 — All remaining calculators (same slice pattern)

Build order matches backend history: F3 retail rest, F4 zakat family, F5 corporate, F6 investment. The complete per-screen spec:

### Per-calculator specification (current API, exact)

Conventions for the table: **(P)** = `PercentField` (user types human %, send fraction) · **(M)** = money string · **(I)** = int · all POST under `/api/v1`. "Client validation" = mirror it in the form for UX; the backend remains the authority and every listed rule is enforced server-side with the field code shown.

| # | Screen / endpoint | Request fields & client validation | Response → render | Special UI obligations |
|---|---|---|---|---|
| 1 | **Murabaha** `finance/murabaha` | `cost`(M,>0) · `markup.mode` rate\|amount (separate fields per mode like web) · rate **(P)**, ≤500% (`out_of_range`) · amount (M) · `downPayment`(M,≥0,< sale) · `termMonths`(I,1–360) · `firstDueDate?` | `salePrice`·`markupTotal`·`financed`·`monthlyInstallment` + `schedule[n,dueDate?,amount,principal,markup,balance]` | Final installment may differ (residual) — render as-is, never recompute (C2) |
| 2 | **Ijara MB** `finance/ijara` | `mode` profit\|rent · profit rate **(P)** ≤500% / amount (M) · `monthlyRent`(M,>0 in rent mode) · `assetCost`(M,>0) · `transferPrice`(M,≥0) · `advancePayment`(M,≥0) · `termMonths`(1–360) · `firstDueDate?` | totals + `profitTotal` (**may be negative** — show as loss, red, with notice) + `profitRate` + rent `schedule` | Negative profit is information, not an error |
| 3 | **Qard al-Hasan** `finance/qard-hasan` | `principal`(M,>0) · `serviceFee`(M,≥0,< principal) · `termMonths`(1–360) · `firstDueDate?` | `totalRepayment`·`monthlyInstallment`·principal-only `schedule` | Never offer a % fee input — the concept is riba; fee is amount-only |
| 4 | **Mudaraba/Wakala** `finance/mudaraba` | `mode` · `amount`(M,>0) · `poolRateAnnual` **(P)** <100% · `shareRatio` **(P)** (0–100%] mudaraba · `wakalaFeeRate` **(P)** ≤ pool (`exceeds_pool_rate`) · `termMonths`(1–120) | `effectiveAnnualRate`·`expectedProfit`·`expectedMonthlyProfit`·`expectedTotal`·`guaranteed:false` | **Must show "expected, not guaranteed"** (C6) |
| 5 | **Diminishing Musharakah** `finance/diminishing-musharaka` | `propertyValue`(M,>0) · `downPayment`(M,≥0,< value: `must_be_less_than_property_value`) · `annualRentalRate` **(P)** <100% · `termMonths`(1–360) | `bankFinancing`·`initialOwnershipPercent`·`monthlyAcquisition`·`firstMonthPayment`·`totalRent`·`totalPaid` + `schedule[n,bankShareBefore,rent,acquisition,payment,ownershipPercent]` | Rent declines monthly — a small ownership-progress bar per row is a nice touch |
| 6 | **Salam** `finance/salam` | `quantity`(M,>0) · `unitPrice`(M,>0) · `expectedUnitPrice`(M,>0) · `deliveryDate?` | `advanceTotal`·`expectedMarketValue`·`expectedMargin`(**may be negative**)·`marginRate` | Notice: full price paid at contract; negative margin = buyer's risk |
| 7 | **Istisna** `finance/istisna` | `mode` milestones\|equal · `contractPrice`(M,>0) · milestones list (name?, `percent` human 0–100 — **NOT a PercentField-to-fraction: send as-is**, sum must be exactly 100: `percents_must_sum_to_100`) · `stages`(1–120 equal mode) | `schedule[n,name,percent,dueDate?,amount]` | Validate the 100-sum client-side before submit; final stage absorbs residual |
| 8 | **Musharaka** `finance/musharaka` | partners list 2–20 (name?, `capital`(M,>0), `profitSharePercent` human 0–100, sum = 100) · `resultType` profit\|loss · `amount`(M,>0) | `basis` (`agreed_ratio`/`capital_ratio`) + `shares[name,capital,capitalShare,profitSharePercent,appliedShare,amount]` | On loss, explain: agreed ratios don't apply — loss follows capital |
| 9 | **Late payment** `finance/late-payment` | `mode` rate\|flat · `overdueAmount`(M,>0) · `daysLate`(I,≥1) · `annualRate` **(P)** <100% / `flatFee`(M,≥0) | `charityDue`·`disposition:"charity"` | **Label as charity, never fee/income** (C7) |
| 10 | **Wealth zakat** `zakat/wealth` | `goldGrams`,`silverGrams`,`cash`,`otherAssets` (all M,≥0, empty=0) · `hawlComplete`(bool) | values·`totalWealth`·`nisab{goldValue,silverValue,applied,basis}`·`aboveNisab`·`zakatDue`·`prices{gold,silver:{pricePerGram,source,fetchedAt,stale}}` | Show nisab basis + price provenance; **stale:true → warning banner** (C8) |
| 11 | **Business zakat** `zakat/business` | `cash`,`receivables`,`inventory`,`shortTermLiabilities` (M,≥0) · `hawlComplete` | `zakatBase` (floored at 0)·nisab block·`zakatDue` | Same nisab/stale handling |
| 12 | **Ushr** `zakat/ushr` | `irrigationType` natural\|irrigated · `harvestValue`(M,>0) | `rate`·`ushrDue` | Show which rate applied (10%/5%) |
| 13 | **Livestock** `zakat/livestock` | `species` sheep_goats\|cattle\|camels\|silk_cocoons · `count`(I,≥1 animals) / `marketValue`(M,>0 cocoons) | `due[{animal,count}]`·`belowNisab`·`cashDue`/`rate` (cocoons)·`note` | Localize animal names; `belowNisab` = friendly zero-state; note `above_120_rules_vary_consult_scholar` → advise a scholar |
| 14 | **Fidya/Kaffarah** `zakat/fidya` | `kind` fidya\|kaffarah_fast\|kaffarah_oath · `count`(I,≥1) | `feedingsPerUnit`·`dailyRate`·`totalDue`·`currency`·`needsReview` | **needsReview:true → "pending scholar approval" notice** (C9) |
| 15 | **Zakat al-Fitr** `zakat/fitrah` | `people`(I,≥1) · `peoplePaidInFood`(I,0–people: `exceeds_people_covered`) · `pricePerKg`(M,>0) · `saKg?`(M,1–5) | `saKg`·`perPerson`·`totalDue`·`foodKg`·`cashDue` | Notice: no nisab; must reach recipients before Eid prayer |
| 16 | **Tazkiya** `zakat/tazkiya` | `mode` declared\|dividend · declared: `totalIncome`(M,>0),`impureAmount`(M,≥0,≤ total) · dividend: `dividendAmount`(M,>0),`impureRatio` **(P)** 0–100% | `purgeAmount`·`cleanAmount` | Label purge as charity; deep-link from screener's purification ratio |
| 17 | **Screener** `invest/screener` | `prohibitedActivities[]` (app-defined slugs) · `interestBearingDebt`,`interestBearingInvestments`,`impureIncome`(M,≥0, impure ≤ revenue) · `marketCap`,`totalRevenue`(M,>0) | `verdict`·`activityPassed`·`failedActivities`·`checks[{key,ratio,threshold,passed}]`·`purificationRatio` | Ratio rows with pass/fail; button "purify a dividend" → Tazkiya prefilled |
| 18 | **Sukuk** `invest/sukuk` | positions list 1–50: name?, `faceValue`,`purchasePrice`(M,>0), `distributionRateAnnual` **(P)** <100%, `frequency`∈{1,2,4,12}, `termMonths`(1–600, multiple of 12/freq: `must_align_with_frequency` — validate client-side) | per-position rows + portfolio totals + `guaranteed:false` | "Expected, not guaranteed" (C6) |

**Supporting screens:**

- [ ] **Rates** — `GET /rates/metals`: gold/silver cards, nisab + basis, stale badge.
- [ ] **Livestock reference** — `GET /reference/livestock-rules`: tier table grouped by species.
- [ ] **Auth** — register/login (email format, password 8–72; 409 = email taken; login errors always the neutral message), logout.
- [ ] **History** — `GET /history/?limit&calcType` list with filter; delete (404 = not yours); **"re-open"**: navigate to the calculator with the form prefilled from the entry's `inputs` and auto-recalculate (don't render stored `result` directly — its field names are internal, C12). Note: `inputs` store *fractions* for rates — convert back to human percent for the form (string shift ×100).

## Phase F7 — Hardening & release

- [ ] Anonymous-first everywhere; History is the only auth-gated tab (C3). Expired-session UX: silent refresh, else a gentle re-login prompt.
- [ ] Network states: offline / 429 (`RATE_LIMITED` — friendly retry) / 500 (generic, never raw `message`) / Vercel cold-start latency (show skeletons, generous timeouts ~35 s).
- [ ] Localization QA pass over all three languages on every screen; unknown-code fallback verified.
- [ ] Icons, splash, store metadata (uz-first).

## Test checklist (mirrors the backend's own goldens)

- [ ] `pct()` port: the 9 conversion cases, plus round-trip ×100 for history re-open.
- [ ] Payload golden tests (unit, no network): murabaha `8000000/20%/12 → value "0.2"`; mudaraba `18% pool, 60% share → "0.18"/"0.6"`.
- [ ] Integration against local backend (`make docker-up && make run`): murabaha → `800000.00`; wealth zakat 50M hawl → `1250000.00`, basis silver; livestock cattle 150 → 1 tabi' + 3 musinna; sukuk 100M/95M/9%/2/60 → yield `0.0947`; dim-musharaka 300k/60k/5%/240 → first payment `2000.00`, total `420500.00`; fitrah 5/2/12000 → `150000.00`.
- [ ] Auth integration: register → authed calc → history shows it → delete → refresh rotation (reuse = 401 = logout).
- [ ] Widget tests: percent suffix renders; field errors appear under the right inputs; stale/needsReview/guaranteed notices show when flags set.

## What should be ADDED (not ported — new work)

1. **History re-open prefill** — exists nowhere yet (web only shows raw inputs); Flutter should do it properly, including fraction→percent back-conversion.
2. **Screener → Tazkiya deep link** with the purification ratio carried over.
3. **Language onboarding** — first-launch picker (web just defaults to uz).
4. **Input persistence per calculator** (last-used values, local only) — cheap UX win.
5. Optional, backend-ready but unscheduled: share/PDF of a result, hijri date display next to Gregorian pickers, push reminder for zakat anniversary (needs backend work — contract-first per PROJECT_CONTEXT).

**Definition of done for v1:** PROJECT_CONTEXT §6 — all C1–C13 hold, byte-identical values vs. direct API calls, uz complete, the four Shariah notices reviewed by the scholar who signs off the seeded values.
