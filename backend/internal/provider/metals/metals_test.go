package metals

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-key", r.URL.Query().Get("api_key"))
		assert.Equal(t, "UZS", r.URL.Query().Get("currency"))
		assert.Equal(t, "g", r.URL.Query().Get("unit"))
		w.Header().Set("Content-Type", "application/json")
		// Precision-hostile values on purpose: parsed as decimals, not floats.
		_, _ = w.Write([]byte(`{"status":"success","currency":"UZS","unit":"g",
			"metals":{"gold":1453210.55,"silver":17512.05}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", "UZS")
	prices, err := client.FetchLatest(context.Background())
	require.NoError(t, err)
	require.Len(t, prices, 2)

	gold := prices[0]
	assert.Equal(t, "gold", gold.Metal)
	assert.True(t, gold.PricePerGram.Equal(decimal.RequireFromString("1453210.55")),
		"exact decimal survives, got %s", gold.PricePerGram)
	assert.Equal(t, Source, gold.Source)
	assert.Equal(t, "UZS", gold.Currency)
}

func TestFetchLatest_Failures(t *testing.T) {
	t.Run("api error status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()
		_, err := NewClient(srv.URL, "k", "UZS").FetchLatest(context.Background())
		assert.Error(t, err)
	})

	t.Run("missing metal", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"status":"success","metals":{"gold":100}}`))
		}))
		defer srv.Close()
		_, err := NewClient(srv.URL, "k", "UZS").FetchLatest(context.Background())
		assert.ErrorContains(t, err, "silver")
	})

	t.Run("non-success status field", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"status":"error"}`))
		}))
		defer srv.Close()
		_, err := NewClient(srv.URL, "k", "UZS").FetchLatest(context.Background())
		assert.Error(t, err)
	})
}
