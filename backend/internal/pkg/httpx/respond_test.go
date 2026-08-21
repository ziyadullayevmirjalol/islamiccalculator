package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

func TestData(t *testing.T) {
	rec := httptest.NewRecorder()
	Data(rec, http.StatusOK, map[string]string{"total": "132000000.00"})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"data":{"total":"132000000.00"}}`, rec.Body.String())
}

func TestErr(t *testing.T) {
	t.Run("typed validation error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		Err(rec, apperr.Validation("termMonths must be positive", map[string]string{"termMonths": "out_of_range"}))

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.JSONEq(t, `{"error":{"code":"VALIDATION_FAILED","message":"termMonths must be positive","fields":{"termMonths":"out_of_range"}}}`, rec.Body.String())
	})

	t.Run("unknown error hides cause", func(t *testing.T) {
		rec := httptest.NewRecorder()
		Err(rec, errors.New("secret db detail"))

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.NotContains(t, rec.Body.String(), "secret")
	})
}

func TestDecode(t *testing.T) {
	type in struct {
		Cost string `json:"cost"`
	}

	t.Run("valid body", func(t *testing.T) {
		var v in
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"cost":"100"}`))
		require.NoError(t, Decode(r, &v))
		assert.Equal(t, "100", v.Cost)
	})

	t.Run("unknown field rejected", func(t *testing.T) {
		var v in
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"cost":"100","bogus":1}`))
		err := Decode(r, &v)
		require.Error(t, err)
		assert.Equal(t, apperr.CodeValidation, apperr.From(err).Code)
	})

	t.Run("malformed json rejected", func(t *testing.T) {
		var v in
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{`))
		assert.Error(t, Decode(r, &v))
	})
}
