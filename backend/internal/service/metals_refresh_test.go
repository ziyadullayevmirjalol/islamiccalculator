package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeFetcher struct {
	prices []MetalPrice
	err    error
}

func (f fakeFetcher) FetchLatest(context.Context) ([]MetalPrice, error) {
	return f.prices, f.err
}

type fakeWriter struct {
	inserted []MetalPrice
	err      error
}

func (w *fakeWriter) Insert(_ context.Context, p MetalPrice) error {
	if w.err != nil {
		return w.err
	}
	w.inserted = append(w.inserted, p)
	return nil
}

func TestMetalRefresher_RefreshOnce(t *testing.T) {
	prices := []MetalPrice{
		{Metal: "gold", PricePerGram: d("1460000"), Currency: "UZS", Source: "metals.dev"},
		{Metal: "silver", PricePerGram: d("17600"), Currency: "UZS", Source: "metals.dev"},
	}

	t.Run("stores every fetched price", func(t *testing.T) {
		writer := &fakeWriter{}
		NewMetalRefresher(fakeFetcher{prices: prices}, writer, time.Hour).RefreshOnce(context.Background())
		assert.Len(t, writer.inserted, 2)
	})

	t.Run("fetch failure stores nothing and does not panic", func(t *testing.T) {
		writer := &fakeWriter{}
		NewMetalRefresher(fakeFetcher{err: errors.New("api down")}, writer, time.Hour).RefreshOnce(context.Background())
		assert.Empty(t, writer.inserted)
	})

	t.Run("write failure is non-fatal", func(t *testing.T) {
		writer := &fakeWriter{err: errors.New("db down")}
		NewMetalRefresher(fakeFetcher{prices: prices}, writer, time.Hour).RefreshOnce(context.Background())
		assert.Empty(t, writer.inserted)
	})
}

func TestMarkStale(t *testing.T) {
	fresh := MetalPrice{FetchedAt: time.Now()}
	markStale(&fresh, 48*time.Hour)
	assert.False(t, fresh.Stale)

	old := MetalPrice{FetchedAt: time.Now().Add(-49 * time.Hour)}
	markStale(&old, 48*time.Hour)
	assert.True(t, old.Stale)

	disabled := MetalPrice{FetchedAt: time.Unix(0, 0)}
	markStale(&disabled, 0)
	assert.False(t, disabled.Stale, "zero threshold disables the guard")
}
