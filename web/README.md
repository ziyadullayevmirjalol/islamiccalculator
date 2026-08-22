# Islamic Calculator — Web UI

A dependency-free web client for the Go backend: Tailwind (CDN) + vanilla
ES modules, no build step. Covers all 16 calculators with forms and result
views, auth (register / login / single-use refresh rotation), calculation
history, live metal rates with staleness warnings, and the livestock
zakat reference table.

## Run

```bash
# 1. backend up (from backend/)
make docker-up && make run          # API on :8080

# 2. serve this folder (any static server works)
cd web
python3 -m http.server 3000
# open http://localhost:3000
```

Point at a different backend without touching code:

```js
localStorage.setItem('ic.apiBase', 'https://staging.example.com'); location.reload()
```

## Layout

```
index.html          shell: Tailwind CDN + brand theme (IBM Plex, emerald/brass)
js/config.js        API base URL
js/api.js           envelope unwrapping, ApiError, token storage,
                    single-flight refresh rotation (contract C1/C3/C4)
js/ui.js            DOM helpers + components: inputs, kv rows, tables,
                    badges, notices, money formatting (string-safe, C2)
js/calculators.js   declarative registry of all 16 calculators:
                    fields, payload mapping, result renderers
js/pages.js         form engine (conditional fields, dynamic row lists,
                    per-field server errors) + all pages
js/app.js           hash router + header/auth state
```

## Contract notes honored here

- Amounts stay **strings** end to end; `fmtMoney` groups thousands
  without ever parsing to a float; results are rendered exactly as the
  backend returns them (schedules are never recomputed).
- Calculators work signed-out; signing in makes every calculation land
  in History automatically.
- `guaranteed: false` → "expected, not guaranteed" notice (mudaraba,
  sukuk); late-payment amounts labeled charity; `stale: true` prices and
  `needsReview` fidya rates show warnings — per PROJECT_CONTEXT.md §3.

This UI doubles as a manual test bench for the API and a reference
implementation of the client contract for the Flutter team.

`e2e-test.html` is a browser smoke test (open it with the backend up):
anonymous calculation → register → authed calculation → history list →
delete → typed validation error, printing PASS/FAIL per step.
