# Islamic Calculation Formula Rules

Compiled 2026-08-25 from a deep analysis of all 18 Islamic calculator pages on
emicalculation.io (`/?category=Islamic`), cross-referenced against our own
backend. Purpose: a single formula reference for extending the product —
what the market-reference site computes, the exact math, the Shariah rules it
encodes, and what we should adopt.

Legend: ✅ = already implemented in our backend · 🔶 = partially implemented ·
🆕 = not in our backend (roadmap candidate)

---

## 0. Cross-cutting conventions observed on the site

1. **Flat-rate (simple) profit everywhere.** Every financing product computes
   profit as `Principal × Rate × Years` — never compounding, never reducing
   balance. This matches our backend's fixed-at-contract approach and is the
   defining contrast with conventional loans (which they model with standard
   amortization for comparison only).
2. **Fiqh parameters are editable defaults** (nisab grams, sa' weight, zakat
   rate) — the same "parameters are data, not code" principle our
   `app_settings` table implements.
3. **Every product page states its Shariah basis** (why it isn't riba) —
   mirrors our display rules C6–C9 in PROJECT_CONTEXT.md.

---

## 1. Financing contracts

### 1.1 Murabaha ✅ (ours: `POST /finance/murabaha`)

Site formulas:

```
Financed        = AssetCost − DownPayment
BankProfit      = Financed × AnnualProfitRate × TenureYears        (flat)
SellingPrice    = AssetCost + BankProfit
TotalToPay      = Financed + BankProfit
MonthlyPayment  = TotalToPay ÷ (TenureYears × 12)
```

Worked example (site): cost 200,000 · down 50,000 · 15 years → profit 75,000
(rate 3.33%/yr on financed 150,000), monthly 1,250, total 225,000.

Shariah rules stated: cost and margin disclosed upfront; no compounding; must
be a real tangible asset; bank bears ownership risk before sale.

**Delta vs ours:** we take markup as *total* rate-of-cost or absolute amount;
the site takes an **annual** rate × years (`markup = rate × years`). Adding an
`"annualRate"` markup mode (value × termYears) would match banking practice.
Also: their profit accrues on the *financed* amount, ours on full cost —
worth exposing as a mode (`profitBase: cost|financed`).

### 1.2 BBA — Bai Bithaman Ajil 🆕 (Malaysia-style deferred sale)

Mathematically identical to Murabaha flat rate:

```
SellingPrice   = Principal + (Principal × ProfitRate × Years)
MonthlyPayment = SellingPrice ÷ TotalMonths
TotalProfit    = Principal × ProfitRate × Years
```

Extra rules: asset must exist and be identifiable at sale; early settlement
grants **ibra' (rebate)** on unearned profit. → Implementable as a Murabaha
alias with an ibra' early-settlement sub-calculation:
`Ibra = TotalProfit × (RemainingMonths ÷ TotalMonths)` (standard practice).

### 1.3 Tawarruq (commodity monetization) 🆕

Cash financing via commodity trade. Same flat-rate core plus a fee:

```
SellingPrice      = FinancingAmount + FinancingAmount × Rate × Years
MonthlyInstallment= SellingPrice ÷ Months
ProcessingFee     = FinancingAmount × Fee%   (one-time, upfront)
TotalProfit       = SellingPrice − FinancingAmount
```

Rule: the bank buys a commodity, sells it to the client at markup on deferred
terms; client sells for cash. Tenures offered: 1/2/3/4/5/7/10 years.

### 1.4 Ijarah (operating lease) & Ijarah Muntahia Bittamleek ✅/🔶

Site formula (both types):

```
Financed      = AssetCost − DownPayment
TotalProfit   = Financed × AnnualRate × TermYears        (flat)
MonthlyRental = (Financed + TotalProfit) ÷ TermMonths
TotalCost     = DownPayment + ΣRentals (+ NominalSalePrice if Bay')
```

Site inputs we don't have: **asset type** (vehicle/property/equipment),
**residual value** (operating lease), **ownership transfer method** —
`hibah` (gift at end, price 0), `bay'` (sale at nominal price), `gradual`
(ownership % transfers monthly). Shariah rule: **two separate contracts**
(lease + transfer promise); lessor bears ownership risk.

**Delta vs ours:** our `/finance/ijara` already covers profit-mode/rent-mode
with a transfer price (= their `bay'`). To match fully: add
`transferMethod: hibah|bay|gradual` (hibah ⇒ transferPrice 0; gradual ⇒
ownership % column in the schedule = n/termMonths) and an "operating"
mode where the residual value stays with the lessor.

### 1.5 Musharakah (fixed partnership) ✅

Site rules — identical to ours:
- Profit split by **agreed ratio** (may differ from capital ratio).
- **"Losses must always be shared according to capital contribution ratio,
  not profit sharing ratio."** (Our API enforces this structurally.)
- All partners may participate in management; no riba.

### 1.6 Diminishing Musharakah / Musharakah Mutanaqisah 🆕 (high value)

The Islamic home-finance flagship. Formula structure:

```
BankFinancing      = PropertyValue − DownPayment
BankShare₀         = BankFinancing ÷ PropertyValue
MonthlyAcquisition = BankFinancing ÷ TermMonths                  (fixed mode)
MonthlyRent(m)     = BankShareValue(m) × AnnualRentalRate ÷ 12   (declines)
MonthlyPayment(m)  = MonthlyRent(m) + MonthlyAcquisition
BankShareValue(m+1)= BankShareValue(m) − MonthlyAcquisition
```

Ownership: client share grows each month until bank share = 0. Two payment
modes offered: *fixed total payment* (annuity-style, rent+acquisition rebalance)
or *fixed acquisition* (payment declines). Site example: property 300,000,
down 60,000 (20%), 5%/yr, 20 years → acquisition 1,000/mo; first-month
payment ≈ 1,833–2,000; total rental ≈ 120,000; total paid ≈ 420,000.
(Note: the site's own worked example is internally inconsistent on the
first-month rent — 833.33 vs the formula's 1,000; our implementation should
trust the formula, not their example.)

Shariah rules: genuine co-ownership; rent is for actual usage of the bank's
share, not interest on debt; no compounding.

**Roadmap: this is the most valuable missing calculator** — it's the standard
Islamic mortgage structure and pairs with our existing Musharaka domain.

### 1.7 Mudarabah ✅ (ours: `/finance/mudaraba`)

Site math matches ours:

```
TotalExpectedProfit = Capital × ExpectedAnnualRate × Months/12
InvestorShare       = TotalProfit × InvestorRatio      (Rab-ul-Maal)
EntrepreneurShare   = TotalProfit × (1 − InvestorRatio) (Mudarib)
TotalReturn         = Capital + InvestorShare
EffectiveRate       = InvestorShare ÷ Capital ÷ Years
```

Preset ratios offered: 50:50, 60:40, 70:30, 80:20.
Shariah rules to carry into our docs/UI: ratio must be a **percentage, never
a fixed amount**; **financial losses are borne by the investor alone** (the
mudarib loses effort), except in negligence/misconduct. Our calculator shows
the depositor side only — adding the **mudarib-share output** (both sides of
the split) is a trivial, worthwhile extension.

### 1.8 Istisna ✅/🔶 (ours: `/finance/istisna`)

Site model adds bank financing on top of our milestone schedule:

```
BankProfit        = ProjectCost × ProfitMargin%
TotalIstisnaPrice = ProjectCost + BankProfit
Payment split     = Advance% + Milestones% + Deferred%  (must total 100)
MonthlyInstallment= DeferredAmount ÷ DeferredMonths     (after completion)
```

Rules: price fixed at signing; specifications detailed upfront; flexible
payment structure allowed. **Delta vs ours:** we schedule the contract price
by milestones; the site adds (a) a financing margin on cost and (b) a
*deferred* tranche amortized monthly after delivery. Both fit our Input as
optional fields (`profitMargin`, `advancePercent`, `deferredPercent`,
`deferredMonths`).

### 1.9 Salam ✅/🔶 (ours: `/finance/salam`)

Site math matches ours, with additions:

```
TotalSalamPayment = SalamPrice × Quantity        (paid 100% upfront)
MarketValue       = CurrentMarketPrice × Quantity
ExpectedValue     = ExpectedFuturePrice × Quantity
PotentialProfit   = ExpectedValue − TotalSalamPayment
DiscountFromMarket= MarketValue − TotalSalamPayment
AnnualizedReturn  = (Profit ÷ Payment) × (12 ÷ DeliveryMonths)
```

New rules worth enforcing: **Salam cannot be used for gold, silver, or
currencies** (add a commodity-type validation); goods must be precisely
specified (type/quality/quantity/date/place). New concept: **Parallel Salam**
(an offsetting sale of the same commodity at a higher parallel price) —
candidate second mode. Our calculator already outputs margin; adding
`currentMarketPrice` (→ discount-from-market) and `deliveryMonths`
(→ annualized return) completes parity.

### 1.10 Comparison tools 🆕 (marketing/education)

*Murabaha vs Conventional* and *Islamic vs Bank Mortgage*:

```
Murabaha:      TotalProfit = P × r × y ;  Monthly = (P + Profit)/n     (flat)
Conventional:  i = APR/12 ;  Monthly M = P·i(1+i)ⁿ ÷ ((1+i)ⁿ − 1)      (amortized)
TotalInterest  = M×n − P ;  Savings = TotalInterest − TotalProfit
```

Site example: P=240,000, 5y → Murabaha total 360,000 vs conventional 412,663
(saving 52,663). Pure client-side feature — both formulas per this document;
no new backend needed beyond our murabaha endpoint.

---

## 2. Zakat family

### 2.1 Zakat al-Mal (comprehensive wealth) 🔶 (ours: `/zakat/wealth` + `/zakat/business`)

Site's canonical formula:

```
Net    = Σ zakatable assets − Σ deductible liabilities
Nisab  = NisabGrams × MetalPricePerGram      (silver standard default = lower)
Zakat  = Net × Rate   when Net ≥ Nisab and hawl complete, else 0
```

Parameters (all editable — identical philosophy to our `app_settings`):
- Gold nisab **87.48 g** (20 mithqal) · Silver nisab **612.36 g** (200 dirham)
  — *exactly our seeded values* ✅
- Rate **2.5% lunar** year (354 days) or **2.577% solar** year 🆕 — add
  `zakat.rate_solar = 0.025775` to settings and a `yearBasis: lunar|solar`
  input.

Asset categories (site) vs ours: cash ✅, bank ✅ (our `cash`), gold/silver ✅,
business stock ✅ (business calc), receivables ✅ (business calc),
**investments & shares** 🆕 — rule: long-term shares counted at ~25–30% of
market value (the zakatable-assets portion of the company).

**Liabilities on personal zakat 🆕:** the site deducts short-term debts and
bills due from *personal* wealth too (we only do this in business zakat).
Rules: credit cards/instalments due now fully deductible; mortgages only the
next 12 months' instalments.

Hawl rules to document in the app: starts when wealth first reaches nisab;
"mid-year dips are ignored by most scholars if above nisab at both ends."

### 2.2 Zakat on gold & silver with purity 🔶 (extends `/zakat/wealth`)

```
PureWeight = GrossWeight × PurityFactor
MetalValue = PureWeight × PricePerGram(pure)
ZakatDue   = TotalMetalValue × 2.5%   when ≥ nisab (combined value, one nisab)
```

**Purity factor tables (adopt as reference data):**

| Gold | factor | | Silver | factor |
|---|---|---|---|---|
| 24K | 1.0000 | | 999 fine | 0.9990 |
| 22K | 0.9167 | | 958 Britannia | 0.9580 |
| 21K | 0.8750 | | 925 sterling | 0.9250 |
| 18K | 0.7500 | | 800 | 0.8000 |
| 14K | 0.5833 | | | |

Madhhab rule (make it a toggle): **Hanafi — personal-use jewellery is fully
zakatable** (our default school ⇒ count it); majority (Shafi'i/Maliki/Hanbali)
— reasonable personal-use jewellery exempt. Valuation rules: use scrap/resale
value not retail; deduct stones/clasps weight; value everything on hawl
completion day; gemstones not zakatable.

**Ours today takes pure grams directly** — adding `karat`/`fineness` inputs
with these factors is a small, high-value upgrade.

### 2.3 Zakat al-Fitr (fitrah) 🆕

```
ValuePerPerson = SaKg × StaplePricePerKg
TotalDue       = PeopleCovered × ValuePerPerson
FoodKg         = PeoplePaidInFood × SaKg
CashDue        = (People − PeoplePaidInFood) × ValuePerPerson
```

Parameters: **1 sa' = 2.0–3.0 kg, default 2.5 kg** (volume measure — four
mudd — so weight varies by staple: rice/wheat/barley/dates/raisins/maize).
Rules: **no nisab** — due from every Muslim with surplus food on Eid,
including infants/dependants; must reach recipients **before Eid prayer**
(payable from start of Ramadan). Madhhab: Hanafi permits cash value; the
other three require the staple itself. → New settings keys:
`fitrah.sa_kg = 2.5` (+ per-staple table), fits our zakat service pattern.

### 2.4 Hajj & Umrah savings 🆕

Standard future-value/annuity math:

```
FV  = TodayCost × (1 + inflation)^years                 (cost at departure)
i   = annualReturn ÷ 12
PMT = (FV − Existing×(1+i)ⁿ) × i ÷ ((1+i)ⁿ − 1)         (required monthly)
```

Year-by-year table: opening balance, deposits, profit, closing. Shariah note:
returns model **wadiah/mudarabah accounts or sukuk funds**, not interest
deposits — output labeled "expected profit," never guaranteed (our C6 rule
applies). Deposit timing toggle (start/end of month) shifts PMT by (1+i).

### 2.5 Faraid — Islamic inheritance 🆕 (large, separate module)

Strict order of claims:

```
Distributable = Gross − Funeral − Debts − min(Bequest, ⅓ × (Gross − Funeral − Debts))
```

Fixed shares (ashab al-furud) for the modeled heirs:

| Heir | With children | Without children |
|---|---|---|
| Husband | 1/4 | 1/2 |
| Wife/wives (share equally) | 1/8 | 1/4 |
| Father | 1/6 (+ residue if only daughters) | residue |
| Mother | 1/6 | 1/3 (1/6 if 2+ siblings) |
| One daughter (no son) | 1/2 | — |
| Two+ daughters (no son) | 2/3 shared | — |
| Sons (+daughters) | residue, male share = 2 × female | — |

Blocking (hajb): son ⇒ husband 1/2→1/4, wife 1/4→1/8, father capped at 1/6,
siblings excluded; 2+ siblings ⇒ mother 1/3→1/6.
**Awl**: if fixed fractions sum > 1, scale all down proportionally (e.g.
27/24 → denominator 27). **Radd**: surplus (no residuary) returned to fixed
sharers except spouses, proportionally. **Umariyyatan**: spouse+mother+father,
no children ⇒ mother gets 1/3 *of the remainder* after the spouse.
Site's own scope disclaimer (adopt verbatim): only spouse/sons/daughters/
parents modeled; grandparents, grandchildren, siblings-as-heirs, half-blood,
madhhab differences excluded; **a qualified scholar/Shariah court must confirm
any real distribution.**

---

## 3. Gap analysis — recommended roadmap

| Priority | Feature | Effort | Basis |
|---|---|---|---|
| 1 | **Diminishing Musharakah** (Islamic mortgage) | new domain pkg, formulas §1.6 | flagship product everywhere |
| 2 | **Zakat al-Fitr** | small domain + 2 settings | seasonal, high usage |
| 3 | **Gold purity factors** on wealth zakat | input + factor table §2.2 | correctness for real users |
| 4 | **Personal-zakat liabilities** + solar rate + shares-at-30% | field additions §2.1 | fiqh completeness |
| 5 | Tawarruq + BBA (+ ibra') | thin variants of murabaha math | bank-product parity |
| 6 | Salam upgrades (market price, annualized return, no-gold/silver rule) | field additions | quick wins |
| 7 | Istisna financing margin + deferred tranche | field additions | parity |
| 8 | Hajj savings planner | client-side or small endpoint §2.4 | engagement |
| 9 | Mudaraba: show mudarib side + loss-bearing note | output addition | education |
| 10 | Comparison views (flat vs amortized) | web/Flutter only | marketing |
| 11 | **Faraid inheritance** | large separate module §2.5 | needs scholar sign-off |

All flat-rate formulas here are decimal-exact and slot directly into our
`internal/domain` + `money.Split` residual pattern. Every fiqh parameter
above (sa' kg, purity factors, solar rate, share-of-shares %) belongs in
`app_settings`/reference tables per our standing rule: **fiqh values are
data, not code, and need scholar review before launch.**

*Source: emicalculation.io Islamic category, 18 pages, fetched 2026-08-25.
Their pages are educational tools; where their worked examples contradict
their own formulas (noted in §1.6), the formula governs.*
