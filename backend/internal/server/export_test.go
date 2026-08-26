package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// The export endpoint re-runs the real calculator in-process and turns
// its response into an .xlsx workbook — these tests exercise it exactly
// as a client would, through the full router (auth middleware, rate
// limiting, the works), not by calling the xlsx package directly.

func TestExportEndpoint_Murabaha(t *testing.T) {
	router, history := newTestRouter(fakePinger{})
	rec := do(t, router, http.MethodPost, "/api/v1/export/xlsx", `{
		"calcType": "finance.murabaha",
		"inputs": {"cost": "8000000", "markup": {"mode": "rate", "value": "0.2"}, "termMonths": 12},
		"lang": "en"
	}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "murabaha-")
	assert.Contains(t, rec.Header().Get("Content-Disposition"), ".xlsx")

	f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	require.NoError(t, err)
	defer f.Close()

	assert.Contains(t, f.GetSheetList(), "Result")
	assert.Contains(t, f.GetSheetList(), "Schedule")

	rows, err := f.GetRows("Result")
	require.NoError(t, err)
	var monthly string
	for _, r := range rows {
		if len(r) >= 2 && r[0] == "Monthly Installment" {
			monthly = r[1]
		}
	}
	assert.Equal(t, "800,000", monthly, "must match the real calculation, not a client-supplied number")

	// Every calculation is analytics-logged regardless of auth (see the
	// "anonymous calculation records no user" invariant elsewhere in this
	// package) — so a row IS expected here. What must hold is that it is
	// never attributed to a user, so it can never appear in anyone's
	// visible History tab as a surprise duplicate.
	require.Len(t, history.saved, 1)
	assert.Nil(t, history.saved[0].UserID)
}

func TestExportEndpoint_UnknownCalcType(t *testing.T) {
	router, _ := newTestRouter(fakePinger{})
	rec := do(t, router, http.MethodPost, "/api/v1/export/xlsx", `{
		"calcType": "not_a_real_calculator",
		"inputs": {}
	}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "unknown_calc_type")
}

func TestExportEndpoint_PropagatesCalculationValidationErrors(t *testing.T) {
	router, _ := newTestRouter(fakePinger{})
	rec := do(t, router, http.MethodPost, "/api/v1/export/xlsx", `{
		"calcType": "finance.murabaha",
		"inputs": {"cost": "1000", "markup": {"mode": "rate", "value": "0.1"}, "termMonths": 0}
	}`)

	// The underlying calculation is invalid (termMonths out of range) —
	// the export endpoint must surface that exact error, not a generic
	// "export failed".
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "VALIDATION_FAILED")
	assert.Contains(t, rec.Body.String(), "termMonths")
}

func TestExportEndpoint_DefaultsToUzbekAndDoesNotLeakAuthToTheSubRequest(t *testing.T) {
	router, history := newTestRouter(fakePinger{})

	// Register to get a real access token, then export WITH that token
	// attached — the recompute must still run anonymously (Authorization
	// stripped before the internal sub-request), so it is never
	// attributed to this user and can't surface as a phantom duplicate
	// in their History tab.
	reg := do(t, router, http.MethodPost, "/api/v1/auth/register",
		`{"email": "export-test@example.com", "password": "strong-password-1"}`)
	require.Equal(t, http.StatusCreated, reg.Code)
	var authBody struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(reg.Body.Bytes(), &authBody))

	rec := doAuth(t, router, http.MethodPost, "/api/v1/export/xlsx", `{
		"calcType": "zakat.wealth",
		"inputs": {"cash": "50000000", "hawlComplete": true}
	}`, authBody.Data.AccessToken)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	require.NoError(t, err)
	defer f.Close()
	assert.Contains(t, f.GetSheetList(), "Natija", "defaults to Uzbek when lang is omitted")

	require.Len(t, history.saved, 1)
	assert.Nil(t, history.saved[0].UserID,
		"the recompute must run anonymously even when the caller is authenticated")
}
