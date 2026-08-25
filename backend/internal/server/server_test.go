package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/diyorbek/islamiccalculator/internal/domain/screener"
	"github.com/diyorbek/islamiccalculator/internal/domain/zakat"
	"github.com/diyorbek/islamiccalculator/internal/handler"
	"github.com/diyorbek/islamiccalculator/internal/service"
)

// --- fakes: the repo layer is replaced, everything above it is real --------

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

type fakeSettings map[string]string

func (f fakeSettings) Value(_ context.Context, key string) (json.RawMessage, error) {
	v, ok := f[key]
	if !ok {
		return nil, service.ErrNotFound
	}
	return json.RawMessage(v), nil
}

type fakeMetals map[string]service.MetalPrice

func (f fakeMetals) Latest(_ context.Context, metal string) (service.MetalPrice, error) {
	p, ok := f[metal]
	if !ok {
		return service.MetalPrice{}, service.ErrNotFound
	}
	return p, nil
}

// fakeHistory implements both CalculationRepo (saves) and HistoryRepo
// (list/delete), like the real postgres.Calculations.
type fakeHistory struct {
	saved   []service.CalculationRecord
	entries []service.HistoryEntry
	owners  []string
}

func (f *fakeHistory) Save(_ context.Context, rec service.CalculationRecord) error {
	f.saved = append(f.saved, rec)
	inputs, _ := json.Marshal(rec.Inputs)
	result, _ := json.Marshal(rec.Result)
	owner := ""
	if rec.UserID != nil {
		owner = *rec.UserID
	}
	f.entries = append(f.entries, service.HistoryEntry{
		ID:        fmt.Sprintf("00000000-0000-0000-0000-%012d", len(f.entries)+1),
		CalcType:  rec.CalcType,
		Inputs:    inputs,
		Result:    result,
		CreatedAt: time.Unix(int64(len(f.entries)), 0),
	})
	f.owners = append(f.owners, owner)
	return nil
}

func (f *fakeHistory) ListByUser(_ context.Context, userID string, limit int, calcType string) ([]service.HistoryEntry, error) {
	var out []service.HistoryEntry
	for i := len(f.entries) - 1; i >= 0 && len(out) < limit; i-- {
		if f.owners[i] == userID && (calcType == "" || f.entries[i].CalcType == calcType) {
			out = append(out, f.entries[i])
		}
	}
	return out, nil
}

func (f *fakeHistory) DeleteByUser(_ context.Context, id, userID string) error {
	for i, e := range f.entries {
		if e.ID == id && f.owners[i] == userID {
			f.entries = append(f.entries[:i], f.entries[i+1:]...)
			f.owners = append(f.owners[:i], f.owners[i+1:]...)
			return nil
		}
	}
	return service.ErrNotFound
}

type fakeUsers struct {
	byEmail map[string]service.User
	next    int
}

func (f *fakeUsers) Create(_ context.Context, email, passwordHash string) (service.User, error) {
	if _, exists := f.byEmail[email]; exists {
		return service.User{}, service.ErrEmailTaken
	}
	f.next++
	u := service.User{ID: fmt.Sprintf("10000000-0000-0000-0000-%012d", f.next), Email: email, PasswordHash: passwordHash}
	f.byEmail[email] = u
	return u, nil
}

func (f *fakeUsers) ByEmail(_ context.Context, email string) (service.User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return service.User{}, service.ErrNotFound
	}
	return u, nil
}

type refreshRow struct {
	userID    string
	expiresAt time.Time
	revoked   bool
}

type fakeRefresh map[string]*refreshRow

func (f fakeRefresh) Store(_ context.Context, userID, tokenHash string, expiresAt time.Time) error {
	f[tokenHash] = &refreshRow{userID: userID, expiresAt: expiresAt}
	return nil
}

func (f fakeRefresh) Valid(_ context.Context, tokenHash string) (string, error) {
	row, ok := f[tokenHash]
	if !ok || row.revoked || time.Now().After(row.expiresAt) {
		return "", service.ErrNotFound
	}
	return row.userID, nil
}

func (f fakeRefresh) Revoke(_ context.Context, tokenHash string) error {
	if row, ok := f[tokenHash]; ok {
		row.revoked = true
	}
	return nil
}

type fakeRules []zakat.LivestockRule

func (f fakeRules) BySpecies(_ context.Context, species string) ([]zakat.LivestockRule, error) {
	var out []zakat.LivestockRule
	for _, r := range f {
		if r.Species == species {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f fakeRules) All(_ context.Context) ([]zakat.LivestockRule, error) { return f, nil }

type fakeScreenerRules []service.ScreenerRule

func (f fakeScreenerRules) All(context.Context) ([]service.ScreenerRule, error) { return f, nil }

func intPtr(n int) *int { return &n }

func mustDecimal(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func newTestRouter(db fakePinger) (http.Handler, *fakeHistory) {
	settings := fakeSettings{
		"zakat.rate":               `{"rate": "0.025"}`,
		"zakat.nisab_gold_grams":   `{"grams": "87.48"}`,
		"zakat.nisab_silver_grams": `{"grams": "612.36"}`,
		"ushr.natural_rate":        `{"rate": "0.10"}`,
		"ushr.irrigated_rate":      `{"rate": "0.05"}`,
		"fidya.daily":              `{"amount": "15000", "currency": "UZS", "needs_review": true}`,
		"kaffarah.days":            `{"days": 60}`,
		"kaffarah.oath_feedings":   `{"count": 10}`,
		"fitrah.sa_kg":             `{"kg": "2.5", "needs_review": true}`,
	}
	metals := fakeMetals{
		"gold":   {Metal: "gold", PricePerGram: mustDecimal("1450000"), Currency: "UZS", Source: "seed", FetchedAt: time.Unix(0, 0)},
		"silver": {Metal: "silver", PricePerGram: mustDecimal("17500"), Currency: "UZS", Source: "seed", FetchedAt: time.Unix(0, 0)},
	}
	rules := fakeRules{
		{Species: zakat.SpeciesSheepGoats, MinCount: 40, MaxCount: intPtr(120), Due: []zakat.AnimalDue{{Animal: "sheep", Count: 1}}},
		{Species: zakat.SpeciesCamels, MinCount: 25, MaxCount: intPtr(35), Due: []zakat.AnimalDue{{Animal: "bint_makhad", Count: 1}}},
	}
	screenerRules := fakeScreenerRules{
		{Key: screener.CheckDebt, Threshold: mustDecimal("0.30")},
		{Key: screener.CheckInvest, Threshold: mustDecimal("0.30")},
		{Key: screener.CheckImpure, Threshold: mustDecimal("0.05")},
	}
	history := &fakeHistory{}
	authSvc := service.NewAuth(
		&fakeUsers{byEmail: map[string]service.User{}},
		fakeRefresh{},
		service.AuthConfig{Secret: "test-secret", AccessTTL: 15 * time.Minute, RefreshTTL: 24 * time.Hour},
	)

	return NewRouter(Handlers{
		Health:       handler.NewHealth(db),
		Finance:      handler.NewFinance(service.NewFinance(history)),
		Zakat:        handler.NewZakat(service.NewZakat(settings, metals, rules, history, 48*time.Hour)),
		Invest:       handler.NewInvest(service.NewInvest(screenerRules, history)),
		Rates:        handler.NewRates(service.NewRates(settings, metals, 48*time.Hour)),
		Reference:    handler.NewReference(service.NewReference(rules)),
		Auth:         handler.NewAuth(authSvc),
		History:      handler.NewHistory(service.NewHistory(history)),
		VerifyAccess: authSvc.VerifyAccess,
	}, Options{MaxBodyBytes: 4096, RateLimitPerMin: 0}), history
}

// doAuth is do() with a Bearer token attached.
func doAuth(t *testing.T, router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	r.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	return rec
}

func do(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	return rec
}

// --- health ----------------------------------------------------------------

func TestHealthEndpoints(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		db         fakePinger
		wantStatus int
		wantBody   string
	}{
		{"liveness is db-independent", "/healthz", fakePinger{err: errors.New("down")},
			http.StatusOK, `{"data":{"status":"ok"}}`},
		{"readiness ok when db pings", "/readyz", fakePinger{},
			http.StatusOK, `{"data":{"status":"ok"}}`},
		{"readiness degraded when db down", "/readyz", fakePinger{err: errors.New("down")},
			http.StatusServiceUnavailable, `{"data":{"status":"degraded","db":"unreachable"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, _ := newTestRouter(tc.db)
			rec := do(t, router, http.MethodGet, tc.path, "")
			assert.Equal(t, tc.wantStatus, rec.Code)
			assert.JSONEq(t, tc.wantBody, rec.Body.String())
		})
	}
}

// --- calculator endpoint workflows ----------------------------------------

func TestMurabahaEndpoint(t *testing.T) {
	t.Run("full workflow with schedule and history", func(t *testing.T) {
		router, history := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/finance/murabaha", `{
			"cost": "120000000",
			"markup": {"mode": "rate", "value": "0.10"},
			"termMonths": 12,
			"firstDueDate": "2026-10-01"
		}`)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var resp struct {
			Data struct {
				SalePrice          string `json:"salePrice"`
				MonthlyInstallment string `json:"monthlyInstallment"`
				Schedule           []struct {
					N       int    `json:"n"`
					DueDate string `json:"dueDate"`
					Amount  string `json:"amount"`
					Balance string `json:"balance"`
				} `json:"schedule"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, "132000000.00", resp.Data.SalePrice)
		assert.Equal(t, "11000000.00", resp.Data.MonthlyInstallment)
		require.Len(t, resp.Data.Schedule, 12)
		assert.Equal(t, "2026-10-01", resp.Data.Schedule[0].DueDate)
		assert.Equal(t, "0.00", resp.Data.Schedule[11].Balance)

		require.Len(t, history.saved, 1)
		assert.Equal(t, "finance.murabaha", history.saved[0].CalcType)
	})

	t.Run("field errors reported together", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/finance/murabaha", `{
			"cost": "abc",
			"markup": {"mode": "rate", "value": "0.10"},
			"termMonths": 12,
			"firstDueDate": "01.10.2026"
		}`)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), `"cost":"must_be_decimal_string"`)
		assert.Contains(t, rec.Body.String(), `"firstDueDate":"must_be_yyyy_mm_dd"`)
	})

	t.Run("domain validation surfaces as 400", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/finance/murabaha", `{
			"cost": "1000",
			"markup": {"mode": "rate", "value": "0.10"},
			"termMonths": 0
		}`)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "VALIDATION_FAILED")
		assert.Contains(t, rec.Body.String(), "termMonths")
	})
}

func TestIjaraEndpoint(t *testing.T) {
	router, history := newTestRouter(fakePinger{})
	rec := do(t, router, http.MethodPost, "/api/v1/finance/ijara", `{
		"mode": "profit",
		"assetCost": "100000000",
		"profit": {"mode": "rate", "value": "0.20"},
		"transferPrice": "10000000",
		"termMonths": 24
	}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp struct {
		Data struct {
			TotalRentals string `json:"totalRentals"`
			ProfitTotal  string `json:"profitTotal"`
			MonthlyRent  string `json:"monthlyRent"`
			Schedule     []struct {
				Amount string `json:"amount"`
			} `json:"schedule"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "110000000.00", resp.Data.TotalRentals)
	assert.Equal(t, "20000000.00", resp.Data.ProfitTotal)
	assert.Equal(t, "4583333.33", resp.Data.MonthlyRent)
	require.Len(t, resp.Data.Schedule, 24)
	assert.Equal(t, "4583333.41", resp.Data.Schedule[23].Amount)

	require.Len(t, history.saved, 1)
	assert.Equal(t, "finance.ijara", history.saved[0].CalcType)
}

func TestQardHasanEndpoint(t *testing.T) {
	router, _ := newTestRouter(fakePinger{})
	rec := do(t, router, http.MethodPost, "/api/v1/finance/qard-hasan", `{
		"principal": "10000000",
		"serviceFee": "100000",
		"termMonths": 10
	}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"totalRepayment":"10100000.00"`)
	assert.Contains(t, rec.Body.String(), `"monthlyInstallment":"1000000.00"`)
}

func TestMudarabaEndpoint(t *testing.T) {
	t.Run("mudaraba mode", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/finance/mudaraba", `{
			"mode": "mudaraba",
			"amount": "10000000",
			"poolRateAnnual": "0.18",
			"shareRatio": "0.60",
			"termMonths": 12
		}`)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"expectedProfit":"1080000.00"`)
		assert.Contains(t, rec.Body.String(), `"guaranteed":false`)
	})

	t.Run("wakala fee above pool rate rejected", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/finance/mudaraba", `{
			"mode": "wakala",
			"amount": "10000000",
			"poolRateAnnual": "0.10",
			"wakalaFeeRate": "0.12",
			"termMonths": 6
		}`)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), `"wakalaFeeRate":"exceeds_pool_rate"`)
	})
}

func TestZakatWealthEndpoint(t *testing.T) {
	router, history := newTestRouter(fakePinger{})
	rec := do(t, router, http.MethodPost, "/api/v1/zakat/wealth", `{
		"cash": "50000000",
		"hawlComplete": true
	}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp struct {
		Data struct {
			ZakatDue   string `json:"zakatDue"`
			AboveNisab bool   `json:"aboveNisab"`
			Nisab      struct {
				Applied string `json:"applied"`
				Basis   string `json:"basis"`
			} `json:"nisab"`
			Prices struct {
				Gold struct {
					Source string `json:"source"`
				} `json:"gold"`
			} `json:"prices"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "1250000.00", resp.Data.ZakatDue)
	assert.True(t, resp.Data.AboveNisab)
	assert.Equal(t, "10716300.00", resp.Data.Nisab.Applied)
	assert.Equal(t, "silver", resp.Data.Nisab.Basis)
	assert.Equal(t, "seed", resp.Data.Prices.Gold.Source)

	require.Len(t, history.saved, 1)
	assert.Equal(t, "zakat.wealth", history.saved[0].CalcType)
}

func TestCorporateEndpoints(t *testing.T) {
	t.Run("salam", func(t *testing.T) {
		router, history := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/finance/salam", `{
			"quantity": "100", "unitPrice": "2400000", "expectedUnitPrice": "3000000",
			"deliveryDate": "2027-09-01"
		}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"advanceTotal":"240000000.00"`)
		assert.Contains(t, rec.Body.String(), `"expectedMargin":"60000000.00"`)
		assert.Contains(t, rec.Body.String(), `"marginRate":"0.2500"`)
		assert.Equal(t, "finance.salam", history.saved[0].CalcType)
	})

	t.Run("istisna milestones", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/finance/istisna", `{
			"mode": "milestones",
			"contractPrice": "90000000",
			"milestones": [
				{"name": "foundation", "percent": "30", "dueDate": "2026-12-01"},
				{"name": "structure", "percent": "50"},
				{"name": "handover", "percent": "20"}
			]
		}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"amount":"27000000.00"`)
		assert.Contains(t, rec.Body.String(), `"amount":"45000000.00"`)
		assert.Contains(t, rec.Body.String(), `"name":"handover"`)
	})

	t.Run("istisna rejects percents not summing to 100", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/finance/istisna", `{
			"mode": "milestones",
			"contractPrice": "1000",
			"milestones": [{"percent": "60"}, {"percent": "30"}]
		}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "percents_must_sum_to_100")
	})

	t.Run("musharaka loss follows capital", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/finance/musharaka", `{
			"partners": [
				{"name": "A", "capital": "70000000", "profitSharePercent": "50"},
				{"name": "B", "capital": "30000000", "profitSharePercent": "50"}
			],
			"resultType": "loss",
			"amount": "10000000"
		}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"basis":"capital_ratio"`)
		assert.Contains(t, rec.Body.String(), `"amount":"7000000.00"`)
	})

	t.Run("musharaka rejects profit shares not summing to 100", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/finance/musharaka", `{
			"partners": [
				{"capital": "100", "profitSharePercent": "60"},
				{"capital": "100", "profitSharePercent": "30"}
			],
			"resultType": "profit",
			"amount": "10"
		}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "profit_shares_must_sum_to_100")
	})
}

func TestZakatFamilyEndpoints(t *testing.T) {
	t.Run("business zakat", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/zakat/business", `{
			"cash": "30000000", "receivables": "20000000", "inventory": "50000000",
			"shortTermLiabilities": "40000000", "hawlComplete": true
		}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"zakatBase":"60000000.00"`)
		assert.Contains(t, rec.Body.String(), `"zakatDue":"1500000.00"`)
	})

	t.Run("ushr", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/zakat/ushr", `{
			"irrigationType": "natural", "harvestValue": "20000000"
		}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"ushrDue":"2000000.00"`)
		assert.Contains(t, rec.Body.String(), `"rate":"0.10"`)
	})

	t.Run("livestock herd", func(t *testing.T) {
		router, history := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/zakat/livestock", `{
			"species": "sheep_goats", "count": 50
		}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"due":[{"animal":"sheep","count":1}]`)
		require.Len(t, history.saved, 1)
		assert.Equal(t, "zakat.livestock", history.saved[0].CalcType)
	})

	t.Run("livestock below nisab", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/zakat/livestock", `{
			"species": "sheep_goats", "count": 39
		}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"belowNisab":true`)
	})

	t.Run("silk cocoons pay cash zakat", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/zakat/livestock", `{
			"species": "silk_cocoons", "marketValue": "8000000"
		}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"cashDue":"200000.00"`)
	})

	t.Run("fidya", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/zakat/fidya", `{
			"kind": "kaffarah_fast", "count": 1
		}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"totalDue":"900000.00"`)
		assert.Contains(t, rec.Body.String(), `"feedingsPerUnit":60`)
		assert.Contains(t, rec.Body.String(), `"needsReview":true`)
	})

	t.Run("tazkiya", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/zakat/tazkiya", `{
			"mode": "dividend", "dividendAmount": "2000000", "impureRatio": "0.03"
		}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"purgeAmount":"60000.00"`)
		assert.Contains(t, rec.Body.String(), `"disposition":"charity"`)
	})
}

func TestDiminishingMusharakaEndpoint(t *testing.T) {
	router, history := newTestRouter(fakePinger{})
	rec := do(t, router, http.MethodPost, "/api/v1/finance/diminishing-musharaka", `{
		"propertyValue": "300000", "downPayment": "60000",
		"annualRentalRate": "0.05", "termMonths": 240
	}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"monthlyAcquisition":"1000.00"`)
	assert.Contains(t, rec.Body.String(), `"firstMonthPayment":"2000.00"`)
	assert.Contains(t, rec.Body.String(), `"totalRent":"120500.00"`)
	assert.Contains(t, rec.Body.String(), `"totalPaid":"420500.00"`)
	require.NotEmpty(t, history.saved)
	assert.Equal(t, "finance.diminishing_musharaka", history.saved[0].CalcType)
}

func TestFitrahEndpoint(t *testing.T) {
	router, _ := newTestRouter(fakePinger{})
	rec := do(t, router, http.MethodPost, "/api/v1/zakat/fitrah", `{
		"people": 5, "peoplePaidInFood": 2, "pricePerKg": "12000"
	}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"perPerson":"30000.00"`)
	assert.Contains(t, rec.Body.String(), `"totalDue":"150000.00"`)
	assert.Contains(t, rec.Body.String(), `"foodKg":"5"`)
	assert.Contains(t, rec.Body.String(), `"cashDue":"90000.00"`)
}

func TestLatePaymentEndpoint(t *testing.T) {
	router, _ := newTestRouter(fakePinger{})
	rec := do(t, router, http.MethodPost, "/api/v1/finance/late-payment", `{
		"mode": "rate", "overdueAmount": "10000000", "daysLate": 73, "annualRate": "0.10"
	}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"charityDue":"200000.00"`)
	assert.Contains(t, rec.Body.String(), `"disposition":"charity"`)
}

func TestLivestockRulesReferenceEndpoint(t *testing.T) {
	router, _ := newTestRouter(fakePinger{})
	rec := do(t, router, http.MethodGet, "/api/v1/reference/livestock-rules", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"species":"sheep_goats"`)
	assert.Contains(t, rec.Body.String(), `"maxCount":120`)
	assert.Contains(t, rec.Body.String(), `"bint_makhad"`)
}

func TestInvestEndpoints(t *testing.T) {
	t.Run("screener compliant company", func(t *testing.T) {
		router, history := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/invest/screener", `{
			"interestBearingDebt": "200000000",
			"interestBearingInvestments": "100000000",
			"marketCap": "1000000000",
			"impureIncome": "4000000",
			"totalRevenue": "200000000"
		}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"verdict":"compliant"`)
		assert.Contains(t, rec.Body.String(), `"purificationRatio":"0.02"`)
		assert.Equal(t, "invest.screener", history.saved[0].CalcType)
	})

	t.Run("screener fails on prohibited activity", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/invest/screener", `{
			"prohibitedActivities": ["gambling"],
			"marketCap": "1000000000",
			"totalRevenue": "200000000"
		}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"verdict":"non_compliant"`)
		assert.Contains(t, rec.Body.String(), `"activityPassed":false`)
	})

	t.Run("sukuk portfolio", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/invest/sukuk", `{
			"positions": [
				{"name": "A", "faceValue": "100000000", "purchasePrice": "95000000",
				 "distributionRateAnnual": "0.09", "frequency": 2, "termMonths": 60},
				{"name": "B", "faceValue": "50000000", "purchasePrice": "50000000",
				 "distributionRateAnnual": "0.12", "frequency": 4, "termMonths": 36}
			]
		}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"portfolioCurrentYield":"0.1034"`)
		assert.Contains(t, rec.Body.String(), `"guaranteed":false`)
	})

	t.Run("sukuk term misaligned with frequency rejected", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodPost, "/api/v1/invest/sukuk", `{
			"positions": [{"faceValue": "100", "purchasePrice": "100",
				"distributionRateAnnual": "0.1", "frequency": 2, "termMonths": 13}]
		}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "must_align_with_frequency")
	})
}

func TestRatesMetalsEndpoint(t *testing.T) {
	router, _ := newTestRouter(fakePinger{})
	rec := do(t, router, http.MethodGet, "/api/v1/rates/metals", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"pricePerGram":"1450000.00"`)
	assert.Contains(t, rec.Body.String(), `"basis":"silver"`)
}

// The phase-6 definition of done: anonymous calculators unchanged (the
// tests above run tokenless), and signed-in history round-trips.
func TestAuthAndHistoryRoundTrip(t *testing.T) {
	router, _ := newTestRouter(fakePinger{})

	// Register.
	rec := do(t, router, http.MethodPost, "/api/v1/auth/register",
		`{"email": "diyorbek@example.com", "password": "strong-password-1"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var auth struct {
		Data struct {
			User         struct{ ID string }
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &auth))
	token := auth.Data.AccessToken

	// An authenticated calculation is recorded under the user.
	rec = doAuth(t, router, http.MethodPost, "/api/v1/finance/qard-hasan",
		`{"principal": "10000000", "termMonths": 10}`, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// History lists it.
	rec = doAuth(t, router, http.MethodGet, "/api/v1/history/", "", token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var history struct {
		Data struct {
			Entries []struct {
				ID       string `json:"id"`
				CalcType string `json:"calcType"`
			} `json:"entries"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &history))
	require.Len(t, history.Data.Entries, 1)
	assert.Equal(t, "finance.qard_hasan", history.Data.Entries[0].CalcType)

	// Delete it; the list is empty afterwards.
	rec = doAuth(t, router, http.MethodDelete, "/api/v1/history/"+history.Data.Entries[0].ID, "", token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = doAuth(t, router, http.MethodGet, "/api/v1/history/", "", token)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &history))
	assert.Empty(t, history.Data.Entries)

	// Refresh rotation works over HTTP too.
	rec = do(t, router, http.MethodPost, "/api/v1/auth/refresh",
		`{"refreshToken": "`+auth.Data.RefreshToken+`"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = do(t, router, http.MethodPost, "/api/v1/auth/refresh",
		`{"refreshToken": "`+auth.Data.RefreshToken+`"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "used refresh token must be dead")
}

func TestAuthBoundaries(t *testing.T) {
	router, history := newTestRouter(fakePinger{})

	t.Run("history requires auth", func(t *testing.T) {
		rec := do(t, router, http.MethodGet, "/api/v1/history/", "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("garbage bearer token is a hard 401 even on calculators", func(t *testing.T) {
		rec := doAuth(t, router, http.MethodPost, "/api/v1/finance/qard-hasan",
			`{"principal": "1000", "termMonths": 2}`, "garbage")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("anonymous calculation records no user", func(t *testing.T) {
		rec := do(t, router, http.MethodPost, "/api/v1/finance/qard-hasan",
			`{"principal": "1000", "termMonths": 2}`)
		require.Equal(t, http.StatusOK, rec.Code)
		require.NotEmpty(t, history.saved)
		assert.Nil(t, history.saved[len(history.saved)-1].UserID)
	})

	t.Run("wrong login is rejected", func(t *testing.T) {
		do(t, router, http.MethodPost, "/api/v1/auth/register",
			`{"email": "x@y.co", "password": "strong-password-1"}`)
		rec := do(t, router, http.MethodPost, "/api/v1/auth/login",
			`{"email": "x@y.co", "password": "wrong-password-1"}`)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("deleting someone else's entry is 404", func(t *testing.T) {
		rec := do(t, router, http.MethodPost, "/api/v1/auth/register",
			`{"email": "z@y.co", "password": "strong-password-1"}`)
		var auth struct {
			Data struct {
				AccessToken string `json:"accessToken"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &auth))
		// The anonymous entry saved above has id ...0001 but no owner.
		rec = doAuth(t, router, http.MethodDelete,
			"/api/v1/history/00000000-0000-0000-0000-000000000001", "", auth.Data.AccessToken)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestHardening(t *testing.T) {
	t.Run("oversized body rejected", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{}) // 4KB cap in tests
		huge := `{"cost": "` + strings.Repeat("9", 5000) + `"`
		rec := do(t, router, http.MethodPost, "/api/v1/finance/murabaha", huge)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "request body too large")
	})

	t.Run("rate limit returns 429 envelope", func(t *testing.T) {
		router, _ := newTestRouterWithOptions(Options{RateLimitPerMin: 2})
		do(t, router, http.MethodGet, "/healthz", "")
		do(t, router, http.MethodGet, "/healthz", "")
		rec := do(t, router, http.MethodGet, "/healthz", "")
		assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		assert.Contains(t, rec.Body.String(), "RATE_LIMITED")
	})

	t.Run("stale seed prices are flagged", func(t *testing.T) {
		// Test metals carry FetchedAt = epoch, far past the 48h threshold.
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodGet, "/api/v1/rates/metals", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"stale":true`)
	})

	t.Run("docs are served", func(t *testing.T) {
		router, _ := newTestRouter(fakePinger{})
		rec := do(t, router, http.MethodGet, "/api/v1/docs/openapi.yaml", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "Islamic Calculator API")

		rec = do(t, router, http.MethodGet, "/api/v1/docs", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "swagger-ui")
	})
}

// newTestRouterWithOptions builds the standard test router with custom
// hardening options.
func newTestRouterWithOptions(opts Options) (http.Handler, *fakeHistory) {
	history := &fakeHistory{}
	authSvc := service.NewAuth(
		&fakeUsers{byEmail: map[string]service.User{}},
		fakeRefresh{},
		service.AuthConfig{Secret: "test-secret", AccessTTL: 15 * time.Minute, RefreshTTL: 24 * time.Hour},
	)
	settings := fakeSettings{}
	metals := fakeMetals{}
	rules := fakeRules{}
	return NewRouter(Handlers{
		Health:       handler.NewHealth(fakePinger{}),
		Finance:      handler.NewFinance(service.NewFinance(history)),
		Zakat:        handler.NewZakat(service.NewZakat(settings, metals, rules, history, 0)),
		Invest:       handler.NewInvest(service.NewInvest(fakeScreenerRules{}, history)),
		Rates:        handler.NewRates(service.NewRates(settings, metals, 0)),
		Reference:    handler.NewReference(service.NewReference(rules)),
		Auth:         handler.NewAuth(authSvc),
		History:      handler.NewHistory(service.NewHistory(history)),
		VerifyAccess: authSvc.VerifyAccess,
	}, opts), history
}

func TestUnknownRouteIs404(t *testing.T) {
	router, _ := newTestRouter(fakePinger{})
	rec := do(t, router, http.MethodGet, "/api/v1/nope", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
